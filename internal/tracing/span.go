//go:build darwin

package tracing

import (
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// SetSpanError sets the span status to Error and records the error.
// Not just RecordError: RecordError alone does not mark the span as failed in Jaeger and similar backends.
func SetSpanError(span trace.Span, err error) {
	if err == nil {
		return
	}
	span.SetStatus(codes.Error, err.Error())
	span.RecordError(err)
}
