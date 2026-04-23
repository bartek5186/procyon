package telemetry

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bartek5186/procyon/internal"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestNewTraceExporterRejectsMissingOTLPEndpoint(t *testing.T) {
	_, err := newTraceExporter(context.Background(), internal.ObservabilityConfig{
		TraceExporter: "otlp_grpc",
	}, zap.NewNop())
	if err == nil {
		t.Fatal("expected error for missing OTLP endpoint")
	}
	if !strings.Contains(err.Error(), "trace_otlp_endpoint") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewTraceExporterSupportsNone(t *testing.T) {
	exporter, err := newTraceExporter(context.Background(), internal.ObservabilityConfig{
		TraceExporter: "none",
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exporter != nil {
		t.Fatalf("expected nil exporter, got %#v", exporter)
	}
}

func TestRateLimitedOTelErrorHandlerSuppressesRepeatedErrors(t *testing.T) {
	core, observed := observer.New(zapcore.WarnLevel)
	logger := zap.New(core)
	now := time.Unix(100, 0)
	handler := newRateLimitedOTelErrorHandler(logger, time.Minute)
	handler.now = func() time.Time {
		return now
	}

	handler.Handle(errors.New("first"))
	handler.Handle(errors.New("second"))

	if got := observed.Len(); got != 1 {
		t.Fatalf("expected one log entry before interval passes, got %d", got)
	}

	now = now.Add(time.Minute)
	handler.Handle(errors.New("third"))

	if got := observed.Len(); got != 2 {
		t.Fatalf("expected second log entry after interval passes, got %d", got)
	}

	fields := observed.All()[1].ContextMap()
	if got := fields["suppressed_errors"]; got != int64(1) {
		t.Fatalf("expected one suppressed error, got %#v", got)
	}
}
