package logging

import (
	"log/slog"
	"os"
	"strings"
)

func Init() {
	level := slog.LevelWarn
	v := os.Getenv("OP_KEYCHAIN_DEBUG")
	if v == "1" || strings.ToLower(v) == "true" {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})))
}
