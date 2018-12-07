// +build go1.7

// This is the middleware from github.com/opentracing-contrib/go-stdlib
// tweaked slightly to work as a native gin middleware.
//
// It removes the need for the additional complexity of using a middleware
// adapter.

package ginhttp

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
)

const defaultComponentName = "net/http"

type mwOptions struct {
	opNameFunc    func(r *http.Request) string
	spanObserver  func(span opentracing.Span, r *http.Request)
	errorFunc     func(ctx *gin.Context) bool
	componentName string
}

// MWOption controls the behavior of the Middleware.
type MWOption func(*mwOptions)

// OperationNameFunc returns a MWOption that uses given function f
// to generate operation name for each server-side span.
func OperationNameFunc(f func(r *http.Request) string) MWOption {
	return func(options *mwOptions) {
		options.opNameFunc = f
	}
}

// MWComponentName returns a MWOption that sets the component name
// for the server-side span.
func MWComponentName(componentName string) MWOption {
	return func(options *mwOptions) {
		options.componentName = componentName
	}
}

// MWSpanObserver returns a MWOption that observe the span
// for the server-side span.
func MWSpanObserver(f func(span opentracing.Span, r *http.Request)) MWOption {
	return func(options *mwOptions) {
		options.spanObserver = f
	}
}

// MWErrorFunc returns a MWOption that sets the span error tag
func MWErrorFunc(f func(ctx *gin.Context) bool) MWOption {
	return func(options *mwOptions) {
		options.errorFunc = f
	}
}

// Middleware is a gin native version of the equivalent middleware in:
//   https://github.com/opentracing-contrib/go-stdlib/
func Middleware(tr opentracing.Tracer, options ...MWOption) gin.HandlerFunc {
	opts := mwOptions{
		opNameFunc: func(r *http.Request) string {
			return "HTTP " + r.Method
		},
		spanObserver: func(span opentracing.Span, r *http.Request) {},
	}
	for _, opt := range options {
		opt(&opts)
	}

	return func(c *gin.Context) {
		carrier := opentracing.HTTPHeadersCarrier(c.Request.Header)
		ctx, _ := tr.Extract(opentracing.HTTPHeaders, carrier)
		op := opts.opNameFunc(c.Request)
		sp := tr.StartSpan(op, ext.RPCServerOption(ctx))
		ext.HTTPMethod.Set(sp, c.Request.Method)
		ext.HTTPUrl.Set(sp, c.Request.URL.String())
		opts.spanObserver(sp, c.Request)

		// set component name, use "net/http" if caller does not specify
		componentName := opts.componentName
		if componentName == "" {
			componentName = defaultComponentName
		}
		ext.Component.Set(sp, componentName)
		c.Request = c.Request.WithContext(
			opentracing.ContextWithSpan(c.Request.Context(), sp))

		c.Next()

		if opts.errorFunc != nil {
			ext.Error.Set(sp, opts.errorFunc(c))
		}
		ext.HTTPStatusCode.Set(sp, uint16(c.Writer.Status()))
		sp.Finish()
	}
}
