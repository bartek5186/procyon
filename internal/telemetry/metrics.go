package telemetry

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"math"
	"net/http"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bartek5186/procyon/internal"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

var defaultHTTPRequestDurationBuckets = []float64{
	0.005,
	0.01,
	0.025,
	0.05,
	0.1,
	0.25,
	0.5,
	1,
	2.5,
	5,
	10,
}

type metricsStore struct {
	config internal.ObservabilityConfig
	reader metricReader
	sqlDB  *sql.DB
	start  time.Time

	httpRequests metric.Int64Counter
	httpDuration metric.Float64Histogram
	inFlight     metric.Int64UpDownCounter
	business     *BusinessMetrics
}

type metricReader interface {
	Collect(context.Context, *metricdata.ResourceMetrics) error
}

type BusinessMetrics struct {
	events metric.Int64Counter
	values metric.Float64Histogram
}

func newMetricsStore(cfg internal.ObservabilityConfig, reader metricReader, meter metric.Meter, sqlDB *sql.DB) (*metricsStore, error) {
	httpRequests, err := meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total number of HTTP requests served."),
	)
	if err != nil {
		return nil, err
	}

	httpDuration, err := meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("HTTP request latency histogram in seconds."),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	inFlight, err := meter.Int64UpDownCounter(
		"http_requests_in_flight",
		metric.WithDescription("Current number of in-flight HTTP requests."),
	)
	if err != nil {
		return nil, err
	}

	business, err := newBusinessMetrics(meter)
	if err != nil {
		return nil, err
	}

	store := &metricsStore{
		config:       cfg,
		reader:       reader,
		sqlDB:        sqlDB,
		start:        time.Now().UTC(),
		httpRequests: httpRequests,
		httpDuration: httpDuration,
		inFlight:     inFlight,
		business:     business,
	}

	if err := store.registerRuntimeCallbacks(meter); err != nil {
		return nil, err
	}

	return store, nil
}

func newBusinessMetrics(meter metric.Meter) (*BusinessMetrics, error) {
	events, err := meter.Int64Counter(
		"business_events_total",
		metric.WithDescription("Business domain events recorded by application code."),
	)
	if err != nil {
		return nil, err
	}

	values, err := meter.Float64Histogram(
		"business_value",
		metric.WithDescription("Business domain values recorded by application code."),
	)
	if err != nil {
		return nil, err
	}

	return &BusinessMetrics{events: events, values: values}, nil
}

func (m *BusinessMetrics) Event(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	if m == nil || m.events == nil {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "unknown"
	}
	attrs = append([]attribute.KeyValue{attribute.String("event", name)}, attrs...)
	m.events.Add(ctx, 1, metric.WithAttributes(attrs...))
}

func (m *BusinessMetrics) Value(ctx context.Context, name string, value float64, attrs ...attribute.KeyValue) {
	if m == nil || m.values == nil {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "unknown"
	}
	attrs = append([]attribute.KeyValue{attribute.String("name", name)}, attrs...)
	m.values.Record(ctx, value, metric.WithAttributes(attrs...))
}

func (m *metricsStore) IncInFlight(ctx context.Context) {
	m.inFlight.Add(ctx, 1)
}

func (m *metricsStore) DecInFlight(ctx context.Context) {
	m.inFlight.Add(ctx, -1)
}

func (m *metricsStore) ObserveRequest(ctx context.Context, method, route string, statusCode int, duration time.Duration) {
	attrs := []attribute.KeyValue{
		attribute.String("method", method),
		attribute.String("route", route),
		attribute.String("status_code", strconv.Itoa(statusCode)),
	}

	m.httpRequests.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.httpDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

func (m *metricsStore) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/openmetrics-text; version=0.0.1; charset=utf-8")
		if err := m.Render(r.Context(), w); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
}

func (m *metricsStore) Render(ctx context.Context, out io.Writer) error {
	var rm metricdata.ResourceMetrics
	if err := m.reader.Collect(ctx, &rm); err != nil {
		return err
	}

	renderer := openMetricsRenderer{namespace: sanitizeMetricName(m.config.ServiceName)}
	renderer.Render(out, rm)
	return nil
}

func (m *metricsStore) registerRuntimeCallbacks(meter metric.Meter) error {
	buildInfo, err := meter.Int64ObservableGauge(
		"build_info",
		metric.WithDescription("Static build and runtime metadata."),
	)
	if err != nil {
		return err
	}
	dbOpen, err := meter.Int64ObservableGauge("db_connections_open", metric.WithDescription("Open database connections."))
	if err != nil {
		return err
	}
	dbInUse, err := meter.Int64ObservableGauge("db_connections_in_use", metric.WithDescription("Database connections currently in use."))
	if err != nil {
		return err
	}
	dbIdle, err := meter.Int64ObservableGauge("db_connections_idle", metric.WithDescription("Idle database connections."))
	if err != nil {
		return err
	}
	dbWaitCount, err := meter.Int64ObservableCounter("db_wait_count_total", metric.WithDescription("Total waits for a database connection."))
	if err != nil {
		return err
	}
	dbWaitDuration, err := meter.Float64ObservableCounter("db_wait_duration_seconds_total", metric.WithDescription("Total time blocked waiting for a database connection."), metric.WithUnit("s"))
	if err != nil {
		return err
	}
	dbMaxIdleClosed, err := meter.Int64ObservableCounter("db_max_idle_closed_total", metric.WithDescription("Connections closed due to idle limit."))
	if err != nil {
		return err
	}
	dbMaxIdleTimeClosed, err := meter.Int64ObservableCounter("db_max_idle_time_closed_total", metric.WithDescription("Connections closed due to max idle time."))
	if err != nil {
		return err
	}
	dbMaxLifetimeClosed, err := meter.Int64ObservableCounter("db_max_lifetime_closed_total", metric.WithDescription("Connections closed due to max lifetime."))
	if err != nil {
		return err
	}
	goroutines, err := meter.Int64ObservableGauge("runtime_goroutines", metric.WithDescription("Current number of goroutines."))
	if err != nil {
		return err
	}
	gomaxprocs, err := meter.Int64ObservableGauge("runtime_gomaxprocs", metric.WithDescription("Configured GOMAXPROCS value."))
	if err != nil {
		return err
	}
	memAlloc, err := meter.Int64ObservableGauge("runtime_memory_alloc_bytes", metric.WithDescription("Bytes of allocated heap objects."), metric.WithUnit("By"))
	if err != nil {
		return err
	}
	heapAlloc, err := meter.Int64ObservableGauge("runtime_memory_heap_alloc_bytes", metric.WithDescription("Bytes of allocated heap memory."), metric.WithUnit("By"))
	if err != nil {
		return err
	}
	uptime, err := meter.Float64ObservableGauge("process_uptime_seconds", metric.WithDescription("Process uptime in seconds."), metric.WithUnit("s"))
	if err != nil {
		return err
	}

	_, err = meter.RegisterCallback(func(_ context.Context, observer metric.Observer) error {
		observer.ObserveInt64(buildInfo, 1, metric.WithAttributes(
			attribute.String("service", m.config.ServiceName),
			attribute.String("version", m.config.ServiceVersion),
			attribute.String("environment", m.config.Environment),
			attribute.String("go_version", runtime.Version()),
		))

		stats := m.sqlDB.Stats()
		observer.ObserveInt64(dbOpen, int64(stats.OpenConnections))
		observer.ObserveInt64(dbInUse, int64(stats.InUse))
		observer.ObserveInt64(dbIdle, int64(stats.Idle))
		observer.ObserveInt64(dbWaitCount, stats.WaitCount)
		observer.ObserveFloat64(dbWaitDuration, stats.WaitDuration.Seconds())
		observer.ObserveInt64(dbMaxIdleClosed, stats.MaxIdleClosed)
		observer.ObserveInt64(dbMaxIdleTimeClosed, stats.MaxIdleTimeClosed)
		observer.ObserveInt64(dbMaxLifetimeClosed, stats.MaxLifetimeClosed)

		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)
		observer.ObserveInt64(goroutines, int64(runtime.NumGoroutine()))
		observer.ObserveInt64(gomaxprocs, int64(runtime.GOMAXPROCS(0)))
		observer.ObserveInt64(memAlloc, int64(mem.Alloc))
		observer.ObserveInt64(heapAlloc, int64(mem.HeapAlloc))
		observer.ObserveFloat64(uptime, time.Since(m.start).Seconds())

		return nil
	},
		buildInfo,
		dbOpen,
		dbInUse,
		dbIdle,
		dbWaitCount,
		dbWaitDuration,
		dbMaxIdleClosed,
		dbMaxIdleTimeClosed,
		dbMaxLifetimeClosed,
		goroutines,
		gomaxprocs,
		memAlloc,
		heapAlloc,
		uptime,
	)
	return err
}

type openMetricsRenderer struct {
	namespace string
}

func (r openMetricsRenderer) Render(out io.Writer, rm metricdata.ResourceMetrics) {
	for _, scope := range rm.ScopeMetrics {
		for _, item := range scope.Metrics {
			r.renderMetric(out, item)
		}
	}
	_, _ = io.WriteString(out, "# EOF\n")
}

func (r openMetricsRenderer) renderMetric(out io.Writer, item metricdata.Metrics) {
	name := metricName(r.namespace, item.Name)
	help := strings.TrimSpace(item.Description)
	if help == "" {
		help = item.Name
	}

	switch data := item.Data.(type) {
	case metricdata.Sum[int64]:
		writeHelpAndType(out, name, help, sumMetricType(data.IsMonotonic))
		for _, point := range data.DataPoints {
			writeMetricSample(out, name, labelsFromAttributes(point.Attributes), strconv.FormatInt(point.Value, 10))
		}
	case metricdata.Sum[float64]:
		writeHelpAndType(out, name, help, sumMetricType(data.IsMonotonic))
		for _, point := range data.DataPoints {
			writeMetricSample(out, name, labelsFromAttributes(point.Attributes), formatOpenMetricsFloat(point.Value))
		}
	case metricdata.Gauge[int64]:
		writeHelpAndType(out, name, help, "gauge")
		for _, point := range data.DataPoints {
			writeMetricSample(out, name, labelsFromAttributes(point.Attributes), strconv.FormatInt(point.Value, 10))
		}
	case metricdata.Gauge[float64]:
		writeHelpAndType(out, name, help, "gauge")
		for _, point := range data.DataPoints {
			writeMetricSample(out, name, labelsFromAttributes(point.Attributes), formatOpenMetricsFloat(point.Value))
		}
	case metricdata.Histogram[int64]:
		writeHelpAndType(out, name, help, "histogram")
		for _, point := range data.DataPoints {
			r.renderHistogramDataPoint(out, name, labelsFromAttributes(point.Attributes), point.Bounds, point.BucketCounts, float64(point.Sum), point.Count)
		}
	case metricdata.Histogram[float64]:
		writeHelpAndType(out, name, help, "histogram")
		for _, point := range data.DataPoints {
			r.renderHistogramDataPoint(out, name, labelsFromAttributes(point.Attributes), point.Bounds, point.BucketCounts, point.Sum, point.Count)
		}
	}
}

func sumMetricType(monotonic bool) string {
	if monotonic {
		return "counter"
	}
	return "gauge"
}

func (r openMetricsRenderer) renderHistogramDataPoint(out io.Writer, name string, labels []metricLabel, bounds []float64, bucketCounts []uint64, sum float64, count uint64) {
	var cumulative uint64
	for idx, upperBound := range bounds {
		if idx < len(bucketCounts) {
			cumulative += bucketCounts[idx]
		}
		writeMetricSample(out, name+"_bucket", appendMetricLabel(labels, "le", formatOpenMetricsFloat(upperBound)), strconv.FormatUint(cumulative, 10))
	}
	if len(bucketCounts) > len(bounds) {
		cumulative += bucketCounts[len(bounds)]
	}
	writeMetricSample(out, name+"_bucket", appendMetricLabel(labels, "le", "+Inf"), strconv.FormatUint(cumulative, 10))
	writeMetricSample(out, name+"_sum", labels, formatOpenMetricsFloat(sum))
	writeMetricSample(out, name+"_count", labels, strconv.FormatUint(count, 10))
}

type metricLabel struct {
	Key   string
	Value string
}

func labelsFromAttributes(attrs attribute.Set) []metricLabel {
	items := attrs.ToSlice()
	out := make([]metricLabel, 0, len(items))
	for _, item := range items {
		out = append(out, metricLabel{
			Key:   string(item.Key),
			Value: item.Value.Emit(),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Key < out[j].Key
	})
	return out
}

func writeHelpAndType(out io.Writer, name, help, metricType string) {
	_, _ = fmt.Fprintf(out, "# HELP %s %s\n", name, help)
	_, _ = fmt.Fprintf(out, "# TYPE %s %s\n", name, metricType)
}

func writeMetricSample(out io.Writer, name string, labels []metricLabel, value string) {
	_, _ = io.WriteString(out, name)
	if len(labels) > 0 {
		writeMetricLabels(out, labels)
	}
	_, _ = fmt.Fprintf(out, " %s\n", value)
}

func writeMetricLabels(out io.Writer, labels []metricLabel) {
	_, _ = io.WriteString(out, "{")
	for i, label := range labels {
		if i > 0 {
			_, _ = io.WriteString(out, ",")
		}
		_, _ = fmt.Fprintf(out, `%s="%s"`, sanitizeMetricName(label.Key), escapeLabelValue(label.Value))
	}
	_, _ = io.WriteString(out, "}")
}

func metricName(namespace, suffix string) string {
	suffix = sanitizeMetricName(suffix)
	if namespace == "" {
		return suffix
	}
	return sanitizeMetricName(namespace) + "_" + suffix
}

func sanitizeMetricName(value string) string {
	if value == "" {
		return "metric"
	}

	var out strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			out.WriteRune(r)
			continue
		}
		out.WriteByte('_')
	}

	return out.String()
}

func escapeLabelValue(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, "\n", `\n`, `"`, `\"`)
	return replacer.Replace(value)
}

func appendMetricLabel(labels []metricLabel, key, value string) []metricLabel {
	next := make([]metricLabel, 0, len(labels)+1)
	next = append(next, labels...)
	next = append(next, metricLabel{Key: key, Value: value})
	return next
}

func formatOpenMetricsFloat(value float64) string {
	switch {
	case value == 1:
		return "1.0"
	case value == 0:
		return "0.0"
	case value == -1:
		return "-1.0"
	case math.IsNaN(value):
		return "NaN"
	case math.IsInf(value, 1):
		return "+Inf"
	case math.IsInf(value, -1):
		return "-Inf"
	default:
		formatted := strconv.FormatFloat(value, 'g', -1, 64)
		if !strings.ContainsAny(formatted, "e.") {
			return formatted + ".0"
		}
		return formatted
	}
}
