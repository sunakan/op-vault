//go:build darwin

// Package tracing initializes the global OpenTelemetry TracerProvider.
package tracing

import (
	"runtime"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// Tracer returns a tracer named after the calling package's import path.
func Tracer() trace.Tracer {
	pc, _, _, ok := runtime.Caller(1)
	if !ok {
		return otel.Tracer("")
	}
	return otel.Tracer(extractImportPath(runtime.FuncForPC(pc).Name()))
}

// extractImportPath strips the function name from a fully-qualified Go symbol.
// e.g. "github.com/sunakan/op-vault/internal/cli.(*VersionCmd).Run" → "github.com/sunakan/op-vault/internal/cli"
func extractImportPath(funcName string) string {
	lastSlash := strings.LastIndex(funcName, "/")
	suffix := funcName
	if lastSlash >= 0 {
		suffix = funcName[lastSlash+1:]
	}
	dot := strings.Index(suffix, ".")
	if dot < 0 {
		return funcName
	}
	if lastSlash < 0 {
		return funcName[:dot]
	}
	return funcName[:lastSlash+1+dot]
}
