package ginhttp

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/mocktracer"
)

func TestOperationNameOption(t *testing.T) {

	fn := func(r *http.Request) string {
		return "HTTP " + r.Method + ": /root"
	}

	tests := []struct {
		options []MWOption
		opName  string
	}{
		{nil, "HTTP GET"},
		{[]MWOption{OperationNameFunc(fn)}, "HTTP GET: /root"},
	}

	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.opName, func(t *testing.T) {
			tr := &mocktracer.MockTracer{}
			mw := Middleware(tr, testCase.options...)
			r := gin.New()
			r.Use(mw)
			srv := httptest.NewServer(r)
			defer srv.Close()

			_, err := http.Get(srv.URL)
			if err != nil {
				t.Fatalf("server returned error: %v", err)
			}

			spans := tr.FinishedSpans()
			if got, want := len(spans), 1; got != want {
				t.Fatalf("got %d spans, expected %d", got, want)
			}

			if got, want := spans[0].OperationName, testCase.opName; got != want {
				t.Fatalf("got %s operation name, expected %s", got, want)
			}
		})
	}
}

func TestSpanObserverOption(t *testing.T) {

	opNamefn := func(r *http.Request) string {
		return "HTTP " + r.Method + ": /root"
	}
	spanObserverfn := func(sp opentracing.Span, r *http.Request) {
		sp.SetTag("http.uri", r.URL.EscapedPath())
	}
	wantTags := map[string]interface{}{"http.uri": "/"}

	tests := []struct {
		options []MWOption
		opName  string
		Tags    map[string]interface{}
	}{
		{nil, "HTTP GET", nil},
		{[]MWOption{OperationNameFunc(opNamefn)}, "HTTP GET: /root", nil},
		{[]MWOption{MWSpanObserver(spanObserverfn)}, "HTTP GET", wantTags},
		{[]MWOption{OperationNameFunc(opNamefn), MWSpanObserver(spanObserverfn)}, "HTTP GET: /root", wantTags},
	}

	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.opName, func(t *testing.T) {
			tr := &mocktracer.MockTracer{}
			mw := Middleware(tr, testCase.options...)
			r := gin.New()
			r.Use(mw)
			srv := httptest.NewServer(r)
			defer srv.Close()

			_, err := http.Get(srv.URL)
			if err != nil {
				t.Fatalf("server returned error: %v", err)
			}

			spans := tr.FinishedSpans()
			if got, want := len(spans), 1; got != want {
				t.Fatalf("got %d spans, expected %d", got, want)
			}

			if got, want := spans[0].OperationName, testCase.opName; got != want {
				t.Fatalf("got %s operation name, expected %s", got, want)
			}

			defaultLength := 5
			if len(spans[0].Tags()) != len(testCase.Tags)+defaultLength {
				t.Fatalf("got tag length %d, expected %d", len(spans[0].Tags()), len(testCase.Tags))
			}
			for k, v := range testCase.Tags {
				if tag := spans[0].Tag(k); v != tag.(string) {
					t.Fatalf("got %v tag, expected %v", tag, v)
				}
			}
		})
	}
}

func TestURLTagOption(t *testing.T) {

	fn := func(u *url.URL) string {
		// Log path only (no query parameters etc)
		return u.Path
	}

	tests := []struct {
		options []MWOption
		url     string
		tag     string
	}{
		{[]MWOption{}, "/root?token=123", "/root?token=123"},
		{[]MWOption{MWURLTagFunc(fn)}, "/root?token=123", "/root"},
	}

	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.tag, func(t *testing.T) {
			tr := &mocktracer.MockTracer{}
			mw := Middleware(tr, testCase.options...)
			r := gin.New()
			r.Use(mw)
			srv := httptest.NewServer(r)
			defer srv.Close()

			_, err := http.Get(srv.URL + testCase.url)
			if err != nil {
				t.Fatalf("server returned error: %v", err)
			}

			spans := tr.FinishedSpans()
			if got, want := len(spans), 1; got != want {
				t.Fatalf("got %d spans, expected %d", got, want)
			}

			tag := spans[0].Tags()["http.url"]
			if got, want := tag, testCase.tag; got != want {
				t.Fatalf("got %s tag name, expected %s", got, want)
			}
		})
	}
}

func TestPanic(t *testing.T) {

	tests := []struct {
		handler         gin.HandlerFunc
		spanFinishCount int
		tag             string
	}{
		{
			func(c *gin.Context) {
				c.String(http.StatusOK, "OK")
			},
			1,
			"OK",
		},
		{
			func(c *gin.Context) {
				panic("panic test")
			},
			1,
			"Panic",
		},
	}

	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.tag, func(t *testing.T) {
			tr := &mocktracer.MockTracer{}
			mw := Middleware(tr)
			r := gin.New()
			r.Use(gin.Recovery(), mw)
			r.GET("/hello", testCase.handler)
			srv := httptest.NewServer(r)
			defer srv.Close()

			_, err := http.Get(srv.URL + "/hello")
			if err != nil {
				t.Fatalf("server returned error: %v", err)
			}

			spans := tr.FinishedSpans()
			if got, want := len(spans), testCase.spanFinishCount; got != want {
				t.Fatalf("got %d spans, expected %d", got, want)
			}
		})
	}
}
