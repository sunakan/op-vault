//go:build darwin

// Package tracing initializes the global OpenTelemetry TracerProvider.
package tracing

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

// Init sets up the global TracerProvider and returns a shutdown function.
// OP_KEYCHAIN_TRACES_EXPORTER selects the exporter: none (default) | stdout | otlp.
// For otlp, OP_KEYCHAIN_OTLP_ENDPOINT sets the collector endpoint (required).
func Init(ctx context.Context, serviceName, version string) (func(context.Context) error, error) {
	exporterName := os.Getenv("OP_KEYCHAIN_TRACES_EXPORTER")
	if exporterName == "" || exporterName == "none" {
		return func(context.Context) error { return nil }, nil
	}

	exp, err := newExporter(ctx, exporterName)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		// Not async: the default async batcher drops spans when the process exits before flushing.
		sdktrace.WithBatcher(exp, sdktrace.WithBlocking()),
		// Not ParentBased: if TRACEPARENT propagation is added, a parent's no-sample decision would suppress spans even when the user opted in.
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(newResource(serviceName, version)),
	)
	otel.SetTracerProvider(tp)
	// Without this, OTel internal errors are silently discarded.
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		slog.Error("OpenTelemetry error", "err", err)
	}))
	return tp.Shutdown, nil
}

func newResource(serviceName, version string) *resource.Resource {
	// Not resource.New: it merges with resource.Default() which adds host.name and sdk attributes we don't need.
	return resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(serviceName),
		semconv.ServiceVersion(version),
	)
}

// stdout writes to stderr to keep span JSON out of the command's stdout.
func newExporter(ctx context.Context, name string) (sdktrace.SpanExporter, error) {
	switch name {
	case "stdout":
		return stdouttrace.New(stdouttrace.WithWriter(os.Stderr))
	case "otlp":
		endpoint := os.Getenv("OP_KEYCHAIN_OTLP_ENDPOINT")
		if endpoint == "" {
			return nil, errors.New("OP_KEYCHAIN_OTLP_ENDPOINT is required when OP_KEYCHAIN_TRACES_EXPORTER=otlp")
		}
		return otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(endpoint))
	default:
		return nil, fmt.Errorf("unknown OP_KEYCHAIN_TRACES_EXPORTER: %q (none|stdout|otlp)", name)
	}
}
