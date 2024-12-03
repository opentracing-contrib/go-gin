package ginhttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/mocktracer"
)

func TestOperationNameOption(t *testing.T) {
	t.Parallel()
	fn := func(r *http.Request) string {
		return "HTTP " + r.Method + ": /root"
	}

	tests := []struct {
		opName  string
		options []MWOption
	}{
		{"HTTP GET", nil},
		{"HTTP GET: /root", []MWOption{OperationNameFunc(fn)}},
	}

	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.opName, func(t *testing.T) {
			t.Parallel()
			tr := &mocktracer.MockTracer{}
			mw := Middleware(tr, testCase.options...)
			r := gin.New()
			r.Use(mw)
			srv := httptest.NewServer(r)
			defer srv.Close()

			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			resp, err := srv.Client().Do(req)
			if err != nil {
				t.Fatalf("server returned error: %v", err)
			}
			defer resp.Body.Close()

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
	t.Parallel()
	opNamefn := func(r *http.Request) string {
		return "HTTP " + r.Method + ": /root"
	}
	spanObserverfn := func(sp opentracing.Span, r *http.Request) {
		sp.SetTag("http.uri", r.URL.EscapedPath())
	}
	wantTags := map[string]interface{}{"http.uri": "/"}

	tests := []struct {
		Tags    map[string]interface{}
		opName  string
		options []MWOption
	}{
		{nil, "HTTP GET", nil},
		{nil, "HTTP GET: /root", []MWOption{OperationNameFunc(opNamefn)}},
		{wantTags, "HTTP GET", []MWOption{MWSpanObserver(spanObserverfn)}},
		{wantTags, "HTTP GET: /root", []MWOption{OperationNameFunc(opNamefn), MWSpanObserver(spanObserverfn)}},
	}

	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.opName, func(t *testing.T) {
			t.Parallel()
			tr := &mocktracer.MockTracer{}
			mw := Middleware(tr, testCase.options...)
			r := gin.New()
			r.Use(mw)
			srv := httptest.NewServer(r)
			defer srv.Close()

			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			resp, err := srv.Client().Do(req)
			if err != nil {
				t.Fatalf("server returned error: %v", err)
			}
			defer resp.Body.Close()

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
				if tag, ok := spans[0].Tag(k).(string); !ok || v != tag {
					t.Fatalf("got %v tag, expected %v", tag, v)
				}
			}
		})
	}
}

func TestURLTagOption(t *testing.T) {
	t.Parallel()
	fn := func(u *url.URL) string {
		// Log path only (no query parameters etc)
		return u.Path
	}

	tests := []struct {
		url     string
		tag     string
		options []MWOption
	}{
		{"/root?token=123", "/root?token=123", []MWOption{}},
		{"/root?token=123", "/root", []MWOption{MWURLTagFunc(fn)}},
	}

	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.tag, func(t *testing.T) {
			t.Parallel()
			tr := &mocktracer.MockTracer{}
			mw := Middleware(tr, testCase.options...)
			r := gin.New()
			r.Use(mw)
			srv := httptest.NewServer(r)
			defer srv.Close()

			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+testCase.url, nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			resp, err := srv.Client().Do(req)
			if err != nil {
				t.Fatalf("server returned error: %v", err)
			}
			defer resp.Body.Close()

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
