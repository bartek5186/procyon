package telemetry

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bartek5186/procyon/internal"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

func TestMetricsHandlerRendersOpenMetricsFromOTelReader(t *testing.T) {
	namespace := "procyon"
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
		sdkmetric.WithView(sdkmetric.NewView(
			sdkmetric.Instrument{Name: "http_request_duration_seconds"},
			sdkmetric.Stream{Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
				Boundaries: defaultHTTPRequestDurationBuckets,
				NoMinMax:   true,
			}},
		)),
	)

	store, err := newMetricsStore(internal.ObservabilityConfig{
		ServiceName:    "procyon",
		ServiceVersion: "test",
		Environment:    "test",
	}, reader, provider.Meter(httpInstrumentationName), new(sql.DB))
	if err != nil {
		t.Fatalf("new metrics store: %v", err)
	}

	ctx := t.Context()
	store.ObserveRequest(ctx, http.MethodGet, "/hello", http.StatusOK, 250*time.Millisecond)
	store.ObserveRequest(ctx, http.MethodGet, "/hello", http.StatusOK, 12*time.Second)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	store.Handler().ServeHTTP(recorder, request)

	if got, want := recorder.Header().Get("Content-Type"), "application/openmetrics-text; version=0.0.1; charset=utf-8"; got != want {
		t.Fatalf("unexpected content type: got %q want %q", got, want)
	}

	body := recorder.Body.String()
	metricName := sanitizeMetricName(namespace) + "_http_request_duration_seconds_bucket"
	if !strings.Contains(body, metricName+`{method="GET",route="/hello",status_code="200",le="0.25"} 1`) {
		t.Fatalf("expected finite bucket in body:\n%s", body)
	}
	if !strings.Contains(body, metricName+`{method="GET",route="/hello",status_code="200",le="+Inf"} 2`) {
		t.Fatalf("expected +Inf bucket in body:\n%s", body)
	}
	if !strings.Contains(body, sanitizeMetricName(namespace)+`_http_requests_total{method="GET",route="/hello",status_code="200"} 2`) {
		t.Fatalf("expected request counter in body:\n%s", body)
	}
	if !strings.HasSuffix(body, "# EOF\n") {
		t.Fatalf("expected OpenMetrics EOF terminator, body:\n%s", body)
	}
}
