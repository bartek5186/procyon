package telemetry

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bartek5186/procyon/internal"
)

func TestMetricsHandlerRendersOpenMetricsHistogramExemplars(t *testing.T) {
	store := newMetricsStore(internal.ObservabilityConfig{
		ServiceName:    "procyon",
		ServiceVersion: "test",
		Environment:    "test",
		Namespace:      "procyon",
	}, new(sql.DB))

	store.ObserveRequest(http.MethodGet, "/hello", http.StatusOK, 250*time.Millisecond, "trace-finite")
	store.ObserveRequest(http.MethodGet, "/hello", http.StatusOK, 12*time.Second, "trace-slow")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	store.Handler().ServeHTTP(recorder, request)

	if got, want := recorder.Header().Get("Content-Type"), "application/openmetrics-text; version=0.0.1; charset=utf-8"; got != want {
		t.Fatalf("unexpected content type: got %q want %q", got, want)
	}

	body := recorder.Body.String()
	if !strings.Contains(body, `procyon_http_request_duration_seconds_bucket{method="GET",route="/hello",status_code="200",le="0.25"} 1 # {trace_id="trace-finite"} 0.25`) {
		t.Fatalf("expected finite bucket exemplar in body:\n%s", body)
	}
	if !strings.Contains(body, `procyon_http_request_duration_seconds_bucket{method="GET",route="/hello",status_code="200",le="+Inf"} 2 # {trace_id="trace-slow"} 12.0`) {
		t.Fatalf("expected +Inf bucket exemplar in body:\n%s", body)
	}
	if !strings.HasSuffix(body, "# EOF\n") {
		t.Fatalf("expected OpenMetrics EOF terminator, body:\n%s", body)
	}
}
