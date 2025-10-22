package logx

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"
)

var (
	defaultOnce   sync.Once
	defaultLogger *slog.Logger
)

// New returns a JSON slog.Logger configured with the requested level.
// The logger writes to stdout to simplify integration with container runtimes.
func New(level string) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parseLevel(level)})
	return slog.New(handler)
}

// Default returns a process-wide shared logger initialised lazily at info level.
func Default() *slog.Logger {
	defaultOnce.Do(func() {
		defaultLogger = New("info")
	})
	return defaultLogger
}

// WithContext attaches the provided logger to the context for downstream retrieval.
func WithContext(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, logger)
}

// FromContext returns the logger from the context if present, otherwise Default().
func FromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return Default()
	}
	if logger, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok && logger != nil {
		return logger
	}
	return Default()
}

type ctxKey struct{}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
