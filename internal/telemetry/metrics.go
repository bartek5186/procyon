package telemetry

import (
	"database/sql"
	"fmt"
	"io"
	"math"
	"net/http"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bartek5186/procyon/internal"
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

type httpMetricKey struct {
	Method     string
	Route      string
	StatusCode string
}

type httpMetricValue struct {
	Requests             uint64
	DurationSecondsSum   float64
	DurationBucketCounts []uint64
	DurationExemplars    []histogramExemplar
}

type histogramExemplar struct {
	TraceID string
	Value   float64
}

type metricsStore struct {
	config          internal.ObservabilityConfig
	sqlDB           *sql.DB
	startedAt       time.Time
	inFlight        atomic.Int64
	durationBuckets []float64
	mu              sync.RWMutex
	httpMetrics     map[httpMetricKey]*httpMetricValue
}

func newMetricsStore(cfg internal.ObservabilityConfig, sqlDB *sql.DB) *metricsStore {
	return &metricsStore{
		config:          cfg,
		sqlDB:           sqlDB,
		startedAt:       time.Now().UTC(),
		durationBuckets: append([]float64(nil), defaultHTTPRequestDurationBuckets...),
		httpMetrics:     make(map[httpMetricKey]*httpMetricValue),
	}
}

func (m *metricsStore) IncInFlight() {
	m.inFlight.Add(1)
}

func (m *metricsStore) DecInFlight() {
	m.inFlight.Add(-1)
}

func (m *metricsStore) ObserveRequest(method, route string, statusCode int, duration time.Duration, traceID string) {
	key := httpMetricKey{
		Method:     method,
		Route:      route,
		StatusCode: strconv.Itoa(statusCode),
	}

	seconds := duration.Seconds()
	bucketIndex := m.durationBucketIndex(seconds)

	m.mu.Lock()
	defer m.mu.Unlock()

	value := m.httpMetrics[key]
	if value == nil {
		value = &httpMetricValue{
			DurationBucketCounts: make([]uint64, len(m.durationBuckets)),
			DurationExemplars:    make([]histogramExemplar, len(m.durationBuckets)+1),
		}
		m.httpMetrics[key] = value
	}

	value.Requests++
	value.DurationSecondsSum += seconds
	if bucketIndex < len(m.durationBuckets) {
		value.DurationBucketCounts[bucketIndex]++
	}
	if traceID != "" {
		value.DurationExemplars[bucketIndex] = histogramExemplar{
			TraceID: traceID,
			Value:   seconds,
		}
	}
}

func (m *metricsStore) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/openmetrics-text; version=0.0.1; charset=utf-8")
		_, _ = io.WriteString(w, m.Render())
	})
}

func (m *metricsStore) Render() string {
	var out strings.Builder
	namespace := sanitizeMetricName(m.config.Namespace)

	writeHelpAndType(&out, metricName(namespace, "build_info"), "Static build and runtime metadata.", "gauge")
	writeMetricSample(&out, metricName(namespace, "build_info"), []metricLabel{
		{Key: "service", Value: m.config.ServiceName},
		{Key: "version", Value: m.config.ServiceVersion},
		{Key: "environment", Value: m.config.Environment},
		{Key: "go_version", Value: runtime.Version()},
	}, "1")

	writeHelpAndType(&out, metricName(namespace, "http_requests_in_flight"), "Current number of in-flight HTTP requests.", "gauge")
	writeMetricSample(&out, metricName(namespace, "http_requests_in_flight"), nil, strconv.FormatInt(m.inFlight.Load(), 10))

	writeHelpAndType(&out, metricName(namespace, "http_requests_total"), "Total number of HTTP requests served.", "counter")
	writeHelpAndType(&out, metricName(namespace, "http_request_duration_seconds"), "HTTP request latency histogram in seconds.", "histogram")
	for _, key := range m.sortedHTTPMetricKeys() {
		value := m.httpMetricValue(key)
		labels := []metricLabel{
			{Key: "method", Value: key.Method},
			{Key: "route", Value: key.Route},
			{Key: "status_code", Value: key.StatusCode},
		}

		writeMetricSample(&out, metricName(namespace, "http_requests_total"), labels, strconv.FormatUint(value.Requests, 10))

		var cumulative uint64
		for idx, upperBound := range m.durationBuckets {
			cumulative += value.DurationBucketCounts[idx]
			writeMetricSampleWithExemplar(
				&out,
				metricName(namespace, "http_request_duration_seconds_bucket"),
				appendMetricLabel(labels, "le", formatOpenMetricsFloat(upperBound)),
				strconv.FormatUint(cumulative, 10),
				value.histogramBucketExemplar(idx),
			)
		}

		writeMetricSampleWithExemplar(
			&out,
			metricName(namespace, "http_request_duration_seconds_bucket"),
			appendMetricLabel(labels, "le", "+Inf"),
			strconv.FormatUint(value.Requests, 10),
			value.histogramBucketExemplar(len(m.durationBuckets)),
		)
		writeMetricSample(&out, metricName(namespace, "http_request_duration_seconds_sum"), labels, formatOpenMetricsFloat(value.DurationSecondsSum))
		writeMetricSample(&out, metricName(namespace, "http_request_duration_seconds_count"), labels, strconv.FormatUint(value.Requests, 10))
	}

	stats := m.sqlDB.Stats()
	writeHelpAndType(&out, metricName(namespace, "db_connections_open"), "Open database connections.", "gauge")
	writeMetricSample(&out, metricName(namespace, "db_connections_open"), nil, strconv.Itoa(stats.OpenConnections))
	writeHelpAndType(&out, metricName(namespace, "db_connections_in_use"), "Database connections currently in use.", "gauge")
	writeMetricSample(&out, metricName(namespace, "db_connections_in_use"), nil, strconv.Itoa(stats.InUse))
	writeHelpAndType(&out, metricName(namespace, "db_connections_idle"), "Idle database connections.", "gauge")
	writeMetricSample(&out, metricName(namespace, "db_connections_idle"), nil, strconv.Itoa(stats.Idle))
	writeHelpAndType(&out, metricName(namespace, "db_wait_count_total"), "Total waits for a database connection.", "counter")
	writeMetricSample(&out, metricName(namespace, "db_wait_count_total"), nil, strconv.FormatInt(stats.WaitCount, 10))
	writeHelpAndType(&out, metricName(namespace, "db_wait_duration_seconds_total"), "Total time blocked waiting for a database connection.", "counter")
	writeMetricSample(&out, metricName(namespace, "db_wait_duration_seconds_total"), nil, formatOpenMetricsFloat(stats.WaitDuration.Seconds()))
	writeHelpAndType(&out, metricName(namespace, "db_max_idle_closed_total"), "Connections closed due to idle limit.", "counter")
	writeMetricSample(&out, metricName(namespace, "db_max_idle_closed_total"), nil, strconv.FormatInt(stats.MaxIdleClosed, 10))
	writeHelpAndType(&out, metricName(namespace, "db_max_idle_time_closed_total"), "Connections closed due to max idle time.", "counter")
	writeMetricSample(&out, metricName(namespace, "db_max_idle_time_closed_total"), nil, strconv.FormatInt(stats.MaxIdleTimeClosed, 10))
	writeHelpAndType(&out, metricName(namespace, "db_max_lifetime_closed_total"), "Connections closed due to max lifetime.", "counter")
	writeMetricSample(&out, metricName(namespace, "db_max_lifetime_closed_total"), nil, strconv.FormatInt(stats.MaxLifetimeClosed, 10))

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	writeHelpAndType(&out, metricName(namespace, "runtime_goroutines"), "Current number of goroutines.", "gauge")
	writeMetricSample(&out, metricName(namespace, "runtime_goroutines"), nil, strconv.Itoa(runtime.NumGoroutine()))
	writeHelpAndType(&out, metricName(namespace, "runtime_gomaxprocs"), "Configured GOMAXPROCS value.", "gauge")
	writeMetricSample(&out, metricName(namespace, "runtime_gomaxprocs"), nil, strconv.Itoa(runtime.GOMAXPROCS(0)))
	writeHelpAndType(&out, metricName(namespace, "runtime_memory_alloc_bytes"), "Bytes of allocated heap objects.", "gauge")
	writeMetricSample(&out, metricName(namespace, "runtime_memory_alloc_bytes"), nil, strconv.FormatUint(mem.Alloc, 10))
	writeHelpAndType(&out, metricName(namespace, "runtime_memory_heap_alloc_bytes"), "Bytes of allocated heap memory.", "gauge")
	writeMetricSample(&out, metricName(namespace, "runtime_memory_heap_alloc_bytes"), nil, strconv.FormatUint(mem.HeapAlloc, 10))
	writeHelpAndType(&out, metricName(namespace, "process_uptime_seconds"), "Process uptime in seconds.", "gauge")
	writeMetricSample(&out, metricName(namespace, "process_uptime_seconds"), nil, formatOpenMetricsFloat(time.Since(m.startedAt).Seconds()))

	out.WriteString("# EOF\n")

	return out.String()
}

func (m *metricsStore) sortedHTTPMetricKeys() []httpMetricKey {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]httpMetricKey, 0, len(m.httpMetrics))
	for key := range m.httpMetrics {
		keys = append(keys, key)
	}

	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Route != keys[j].Route {
			return keys[i].Route < keys[j].Route
		}
		if keys[i].Method != keys[j].Method {
			return keys[i].Method < keys[j].Method
		}
		return keys[i].StatusCode < keys[j].StatusCode
	})

	return keys
}

func (m *metricsStore) httpMetricValue(key httpMetricKey) httpMetricValue {
	m.mu.RLock()
	defer m.mu.RUnlock()

	value := m.httpMetrics[key]
	if value == nil {
		return httpMetricValue{}
	}

	return httpMetricValue{
		Requests:             value.Requests,
		DurationSecondsSum:   value.DurationSecondsSum,
		DurationBucketCounts: append([]uint64(nil), value.DurationBucketCounts...),
		DurationExemplars:    append([]histogramExemplar(nil), value.DurationExemplars...),
	}
}

func (m *metricsStore) durationBucketIndex(seconds float64) int {
	for idx, upperBound := range m.durationBuckets {
		if seconds <= upperBound {
			return idx
		}
	}
	return len(m.durationBuckets)
}

type metricLabel struct {
	Key   string
	Value string
}

type metricExemplar struct {
	Labels []metricLabel
	Value  string
}

func writeHelpAndType(out *strings.Builder, name, help, metricType string) {
	fmt.Fprintf(out, "# HELP %s %s\n", name, help)
	fmt.Fprintf(out, "# TYPE %s %s\n", name, metricType)
}

func writeMetricSample(out *strings.Builder, name string, labels []metricLabel, value string) {
	writeMetricSampleWithExemplar(out, name, labels, value, nil)
}

func writeMetricSampleWithExemplar(out *strings.Builder, name string, labels []metricLabel, value string, exemplar *metricExemplar) {
	out.WriteString(name)
	if len(labels) > 0 {
		out.WriteByte('{')
		for i, label := range labels {
			if i > 0 {
				out.WriteByte(',')
			}
			fmt.Fprintf(out, `%s="%s"`, sanitizeMetricName(label.Key), escapeLabelValue(label.Value))
		}
		out.WriteByte('}')
	}
	out.WriteByte(' ')
	out.WriteString(value)
	if exemplar != nil && len(exemplar.Labels) > 0 && exemplar.Value != "" {
		out.WriteString(" # ")
		writeMetricLabels(out, exemplar.Labels)
		out.WriteByte(' ')
		out.WriteString(exemplar.Value)
	}
	out.WriteByte('\n')
}

func writeMetricLabels(out *strings.Builder, labels []metricLabel) {
	out.WriteByte('{')
	for i, label := range labels {
		if i > 0 {
			out.WriteByte(',')
		}
		fmt.Fprintf(out, `%s="%s"`, sanitizeMetricName(label.Key), escapeLabelValue(label.Value))
	}
	out.WriteByte('}')
}

func metricName(namespace, suffix string) string {
	return sanitizeMetricName(namespace) + "_" + sanitizeMetricName(suffix)
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

func (v httpMetricValue) histogramBucketExemplar(idx int) *metricExemplar {
	if idx < 0 || idx >= len(v.DurationExemplars) {
		return nil
	}

	exemplar := v.DurationExemplars[idx]
	if exemplar.TraceID == "" {
		return nil
	}

	return &metricExemplar{
		Labels: []metricLabel{{Key: "trace_id", Value: exemplar.TraceID}},
		Value:  formatOpenMetricsFloat(exemplar.Value),
	}
}
