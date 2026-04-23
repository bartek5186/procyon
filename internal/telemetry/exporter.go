package telemetry

import (
	"context"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"
)

type logSpanExporter struct {
	logger *zap.Logger
}

func newLogSpanExporter(logger *zap.Logger) sdktrace.SpanExporter {
	return &logSpanExporter{logger: logger}
}

func (e *logSpanExporter) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) error {
	for _, span := range spans {
		fields := []zap.Field{
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.String("span_id", span.SpanContext().SpanID().String()),
			zap.String("parent_span_id", span.Parent().SpanID().String()),
			zap.String("name", span.Name()),
			zap.String("kind", span.SpanKind().String()),
			zap.Time("start_time", span.StartTime()),
			zap.Time("end_time", span.EndTime()),
			zap.Duration("duration", span.EndTime().Sub(span.StartTime())),
			zap.String("status_code", span.Status().Code.String()),
			zap.String("status_message", span.Status().Description),
			zap.String("instrumentation_scope", span.InstrumentationScope().Name),
			zap.Any("attributes", attributesToMap(span.Attributes())),
			zap.Int("events_count", len(span.Events())),
			zap.Int("links_count", len(span.Links())),
		}

		e.logger.Info("otel span", fields...)
	}

	return nil
}

func (e *logSpanExporter) Shutdown(context.Context) error {
	return nil
}
