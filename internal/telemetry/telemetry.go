package telemetry

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bartek5186/procyon/internal"
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.31.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const httpInstrumentationName = "github.com/bartek5186/procyon/internal/telemetry/http"
const otelErrorLogInterval = 30 * time.Second

type Manager struct {
	config         internal.ObservabilityConfig
	logger         *zap.Logger
	tracer         oteltrace.Tracer
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	sqlDB          *sql.DB
	metrics        *metricsStore
}

func New(ctx context.Context, cfg internal.ObservabilityConfig, logger *zap.Logger, db *gorm.DB) (*Manager, error) {
	otel.SetErrorHandler(newRateLimitedOTelErrorHandler(logger.Named("otel"), otelErrorLogInterval))

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	resourceAttributes := []attribute.KeyValue{
		semconv.ServiceName(cfg.ServiceName),
		semconv.ServiceVersion(cfg.ServiceVersion),
		semconv.DeploymentEnvironmentName(cfg.Environment),
	}

	res, err := resource.New(ctx, resource.WithAttributes(resourceAttributes...))
	if err != nil {
		return nil, err
	}

	options := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.TraceSampleRatio))),
	}

	traceExporter, err := newTraceExporter(ctx, cfg, logger.Named("otel"))
	if err != nil {
		return nil, err
	}
	if traceExporter != nil {
		options = append(options, sdktrace.WithBatcher(traceExporter))
	}

	tracerProvider := sdktrace.NewTracerProvider(options...)
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	manualMetricReader := sdkmetric.NewManualReader()
	metricReaders := []sdkmetric.Reader{manualMetricReader}
	metricExporter, err := newMetricExporter(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if metricExporter != nil {
		metricReaders = append(metricReaders, sdkmetric.NewPeriodicReader(metricExporter))
	}

	meterOptions := []sdkmetric.Option{
		sdkmetric.WithResource(res),
		sdkmetric.WithView(sdkmetric.NewView(
			sdkmetric.Instrument{Name: "http_request_duration_seconds"},
			sdkmetric.Stream{Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
				Boundaries: defaultHTTPRequestDurationBuckets,
				NoMinMax:   true,
			}},
		)),
	}
	for _, reader := range metricReaders {
		meterOptions = append(meterOptions, sdkmetric.WithReader(reader))
	}
	meterProvider := sdkmetric.NewMeterProvider(meterOptions...)
	otel.SetMeterProvider(meterProvider)

	metrics, err := newMetricsStore(cfg, manualMetricReader, meterProvider.Meter(httpInstrumentationName), sqlDB)
	if err != nil {
		return nil, err
	}

	return &Manager{
		config:         cfg,
		logger:         logger,
		tracer:         tracerProvider.Tracer(httpInstrumentationName),
		tracerProvider: tracerProvider,
		meterProvider:  meterProvider,
		sqlDB:          sqlDB,
		metrics:        metrics,
	}, nil
}

func newTraceExporter(ctx context.Context, cfg internal.ObservabilityConfig, logger *zap.Logger) (sdktrace.SpanExporter, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.TraceExporter)) {
	case "", "log":
		return newLogSpanExporter(logger), nil
	case "none":
		return nil, nil
	case "otlp_grpc":
		return newOTLPGRPCTraceExporter(ctx, cfg)
	default:
		return nil, fmt.Errorf("unsupported trace exporter %q", cfg.TraceExporter)
	}
}

func newOTLPGRPCTraceExporter(ctx context.Context, cfg internal.ObservabilityConfig) (sdktrace.SpanExporter, error) {
	endpoint := strings.TrimSpace(cfg.TraceOTLPEndpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("trace_otlp_endpoint is required when trace_exporter=otlp_grpc")
	}

	options := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithTimeout(10 * time.Second),
	}
	if cfg.TraceOTLPInsecure {
		options = append(options, otlptracegrpc.WithInsecure())
	}

	return otlptracegrpc.New(ctx, options...)
}

func newMetricExporter(ctx context.Context, cfg internal.ObservabilityConfig) (sdkmetric.Exporter, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.MetricsExporter)) {
	case "", "none":
		return nil, nil
	case "otlp_grpc":
		endpoint := strings.TrimSpace(cfg.TraceOTLPEndpoint)
		if endpoint == "" {
			return nil, fmt.Errorf("trace_otlp_endpoint is required when metrics_exporter=otlp_grpc")
		}

		options := []otlpmetricgrpc.Option{
			otlpmetricgrpc.WithEndpoint(endpoint),
			otlpmetricgrpc.WithTimeout(10 * time.Second),
		}
		if cfg.TraceOTLPInsecure {
			options = append(options, otlpmetricgrpc.WithInsecure())
		}

		return otlpmetricgrpc.New(ctx, options...)
	default:
		return nil, fmt.Errorf("unsupported metrics exporter %q", cfg.MetricsExporter)
	}
}

type rateLimitedOTelErrorHandler struct {
	logger     *zap.Logger
	interval   time.Duration
	now        func() time.Time
	mu         sync.Mutex
	lastLog    time.Time
	suppressed int
}

func newRateLimitedOTelErrorHandler(logger *zap.Logger, interval time.Duration) *rateLimitedOTelErrorHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	if interval <= 0 {
		interval = otelErrorLogInterval
	}
	return &rateLimitedOTelErrorHandler{
		logger:   logger,
		interval: interval,
		now:      time.Now,
	}
}

func (h *rateLimitedOTelErrorHandler) Handle(err error) {
	if err == nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	now := h.now()
	if h.lastLog.IsZero() || now.Sub(h.lastLog) >= h.interval {
		fields := []zap.Field{zap.Error(err)}
		if h.suppressed > 0 {
			fields = append(fields, zap.Int("suppressed_errors", h.suppressed))
			h.suppressed = 0
		}
		h.lastLog = now
		h.logger.Warn("otel export error", fields...)
		return
	}

	h.suppressed++
}

func (m *Manager) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			route := normalizedRoute(c)
			spanName := requestSpanName(req.Method, route)
			ctx := otel.GetTextMapPropagator().Extract(req.Context(), propagation.HeaderCarrier(req.Header))
			ctx, span := m.tracer.Start(
				ctx,
				spanName,
				oteltrace.WithSpanKind(oteltrace.SpanKindServer),
				oteltrace.WithAttributes(
					attribute.String("http.request.method", req.Method),
					attribute.String("http.route", route),
					attribute.String("url.path", req.URL.Path),
					attribute.String("url.scheme", c.Scheme()),
					attribute.String("server.address", req.Host),
					attribute.String("user_agent.original", req.UserAgent()),
				),
			)
			defer span.End()

			c.SetRequest(req.WithContext(ctx))

			traceID := ""
			spanID := ""
			if sc := span.SpanContext(); sc.IsValid() {
				traceID = sc.TraceID().String()
				spanID = sc.SpanID().String()
				c.Response().Header().Set("X-Trace-ID", traceID)
			}

			startedAt := time.Now()
			m.metrics.IncInFlight(ctx)
			defer m.metrics.DecInFlight(ctx)

			err := next(c)
			if err != nil {
				c.Error(err)
			}

			statusCode := c.Response().Status
			if statusCode == 0 {
				statusCode = http.StatusOK
			}

			statusLabel := http.StatusText(statusCode)
			if statusLabel == "" {
				statusLabel = "unknown"
			}

			finalRoute := normalizedRoute(c)
			duration := time.Since(startedAt)

			span.SetName(requestSpanName(req.Method, finalRoute))
			span.SetAttributes(attribute.Int("http.response.status_code", statusCode))

			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			} else if statusCode >= http.StatusInternalServerError {
				span.SetStatus(codes.Error, statusLabel)
			}

			m.metrics.ObserveRequest(ctx, req.Method, finalRoute, statusCode, duration)
			m.logHTTPRequest(c, finalRoute, statusCode, duration, traceID, spanID, err)

			return nil
		}
	}
}

func (m *Manager) logHTTPRequest(c echo.Context, route string, statusCode int, duration time.Duration, traceID, spanID string, err error) {
	if m == nil || m.logger == nil {
		return
	}

	req := c.Request()
	fields := []zap.Field{
		zap.String("component", "http"),
		zap.String("method", req.Method),
		zap.String("route", route),
		zap.String("path", req.URL.Path),
		zap.Int("status_code", statusCode),
		zap.Float64("duration_ms", float64(duration.Microseconds())/1000),
	}

	requestID := strings.TrimSpace(req.Header.Get(echo.HeaderXRequestID))
	if requestID == "" {
		requestID = strings.TrimSpace(c.Response().Header().Get(echo.HeaderXRequestID))
	}
	if requestID != "" {
		fields = append(fields, zap.String("request_id", requestID))
	}
	if traceID != "" {
		fields = append(fields, zap.String("trace_id", traceID))
	}
	if spanID != "" {
		fields = append(fields, zap.String("span_id", spanID))
	}
	if err != nil {
		fields = append(fields, zap.Error(err))
	}

	switch {
	case err != nil || statusCode >= http.StatusInternalServerError:
		m.logger.Error("http request", fields...)
	case statusCode >= http.StatusBadRequest:
		m.logger.Warn("http request", fields...)
	default:
		m.logger.Info("http request", fields...)
	}
}

func (m *Manager) MetricsHandler() http.Handler {
	return m.metrics.Handler()
}

func (m *Manager) BusinessMetrics() *BusinessMetrics {
	if m == nil || m.metrics == nil {
		return nil
	}
	return m.metrics.business
}

func (m *Manager) HealthHandler(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{
		"status":      "ok",
		"service":     m.config.ServiceName,
		"version":     m.config.ServiceVersion,
		"environment": m.config.Environment,
		"time":        time.Now().UTC().Format(time.RFC3339),
	})
}

func (m *Manager) ReadyHandler(c echo.Context) error {
	ready := true
	databaseStatus := "ok"

	ctx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
	defer cancel()

	if err := m.sqlDB.PingContext(ctx); err != nil {
		ready = false
		databaseStatus = err.Error()
	}

	status := "ready"
	code := http.StatusOK
	if !ready {
		status = "not_ready"
		code = http.StatusServiceUnavailable
	}

	return c.JSON(code, map[string]any{
		"status":      status,
		"service":     m.config.ServiceName,
		"version":     m.config.ServiceVersion,
		"environment": m.config.Environment,
		"checks": map[string]any{
			"database": databaseStatus,
		},
		"time": time.Now().UTC().Format(time.RFC3339),
	})
}

func (m *Manager) InfoHandler(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{
		"service":     m.config.ServiceName,
		"version":     m.config.ServiceVersion,
		"environment": m.config.Environment,
		"observability": map[string]any{
			"trace_exporter":      m.config.TraceExporter,
			"metrics_exporter":    m.config.MetricsExporter,
			"trace_sample_ratio":  m.config.TraceSampleRatio,
			"trace_otlp_endpoint": m.config.TraceOTLPEndpoint,
			"trace_otlp_insecure": m.config.TraceOTLPInsecure,
		},
		"time": time.Now().UTC().Format(time.RFC3339),
	})
}

func (m *Manager) Shutdown(ctx context.Context) error {
	var err error
	if m.tracerProvider != nil {
		err = m.tracerProvider.Shutdown(ctx)
	}
	if m.meterProvider != nil {
		if meterErr := m.meterProvider.Shutdown(ctx); meterErr != nil && err == nil {
			err = meterErr
		}
	}
	return err
}

func attributesToMap(attrs []attribute.KeyValue) map[string]any {
	out := make(map[string]any, len(attrs))
	for _, attr := range attrs {
		out[string(attr.Key)] = attr.Value.AsInterface()
	}
	return out
}

func normalizedRoute(c echo.Context) string {
	path := strings.TrimSpace(c.Path())
	if path != "" {
		return path
	}
	return "/unmatched"
}

func requestSpanName(method, route string) string {
	method = strings.TrimSpace(method)
	if method == "" {
		method = http.MethodGet
	}
	return method + " " + route
}
