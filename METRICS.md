# Metrics

This project uses OpenTelemetry for application metrics and keeps a Prometheus-compatible `/metrics` endpoint for scraping.

## Runtime Metrics

The telemetry manager records standard runtime and HTTP metrics automatically:

- HTTP request count by method, route, and status code
- HTTP request duration histogram
- in-flight HTTP requests
- DB connection pool stats
- Go runtime stats
- process uptime
- build/service info

The metrics endpoint is configured by:

```json
{
  "observability": {
    "metrics_path": "/metrics"
  }
}
```

Prometheus can scrape:

```text
GET /metrics
```

## OTLP Export

Prometheus scraping is always available through the metrics endpoint. OTLP metrics export is optional.

Default local config:

```json
{
  "observability": {
    "metrics_exporter": "none"
  }
}
```

To push metrics through OTLP/gRPC:

```json
{
  "observability": {
    "metrics_exporter": "otlp_grpc",
    "trace_otlp_endpoint": "tempo:4317",
    "trace_otlp_insecure": true,
    "trace_otlp_timeout_seconds": 10
  }
}
```

The metrics exporter currently reuses the OTLP endpoint, headers, insecure flag, and timeout fields used by tracing.

## Business Metrics

Application services receive `telemetry.BusinessMetrics` through `services.AppService`.

Use `Event` for countable domain events:

```go
s.metrics.Event(ctx, "payment_completed",
    attribute.String("provider", "stripe"),
    attribute.String("status", "success"),
)
```

This records:

```text
business_events_total{event="payment_completed",provider="stripe",status="success"}
```

Use `Value` for measured business values:

```go
s.metrics.Value(ctx, "payment_amount", 49.99,
    attribute.String("currency", "PLN"),
    attribute.String("provider", "stripe"),
)
```

This records values in a histogram-like instrument:

```text
business_value{name="payment_amount",currency="PLN",provider="stripe"}
```

## Label Rules

Keep metric labels low-cardinality.

Good labels:

- `provider`
- `status`
- `plan`
- `currency`
- `country`
- `source`

Avoid labels like:

- `user_id`
- `email`
- `session_id`
- `order_id`
- raw URLs
- free-form text

Put high-cardinality details in logs or traces, not metrics.

## Adding Domain Metrics

For a new service:

1. Add `metrics *telemetry.BusinessMetrics` to the service struct.
2. Pass it from `services.NewAppService`.
3. Record events or values at the point where the business outcome is known.

Example:

```go
type PaymentService struct {
    Store   store.Datastore
    metrics *telemetry.BusinessMetrics
}

func (s *PaymentService) Complete(ctx context.Context, payment Payment) error {
    // business logic...

    s.metrics.Event(ctx, "payment_completed",
        attribute.String("provider", payment.Provider),
        attribute.String("currency", payment.Currency),
    )
    s.metrics.Value(ctx, "payment_amount", payment.Amount,
        attribute.String("currency", payment.Currency),
    )

    return nil
}
```

Record metrics after the state change succeeds, not before, unless the metric intentionally tracks attempts.
