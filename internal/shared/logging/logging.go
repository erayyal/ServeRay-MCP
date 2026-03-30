package logging

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/erayyal/serveray-mcp/internal/shared/redaction"
)

type Logger struct {
	component string
	inner     *slog.Logger
	redactor  *redaction.Redactor
}

func New(component, level string, redactor *redaction.Redactor) *Logger {
	if redactor == nil {
		redactor = redaction.New()
	}

	handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: parseLevel(level),
	})

	return &Logger{
		component: component,
		inner:     slog.New(handler).With(slog.String("component", component)),
		redactor:  redactor,
	}
}

func (l *Logger) Debug(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.LogAttrs(ctx, slog.LevelDebug, msg, attrs...)
}

func (l *Logger) Info(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.LogAttrs(ctx, slog.LevelInfo, msg, attrs...)
}

func (l *Logger) Warn(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.LogAttrs(ctx, slog.LevelWarn, msg, attrs...)
}

func (l *Logger) Error(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.LogAttrs(ctx, slog.LevelError, msg, attrs...)
}

func (l *Logger) LogAttrs(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
	safeAttrs := make([]slog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		safeAttrs = append(safeAttrs, l.sanitizeAttr(attr))
	}
	l.inner.LogAttrs(ctx, level, l.redactor.RedactText(msg), safeAttrs...)
}

func (l *Logger) sanitizeAttr(attr slog.Attr) slog.Attr {
	attr.Value = attr.Value.Resolve()
	if l.redactor.IsSensitiveKey(attr.Key) {
		return slog.String(attr.Key, redaction.Placeholder)
	}

	switch attr.Value.Kind() {
	case slog.KindString:
		return slog.String(attr.Key, l.redactor.RedactText(attr.Value.String()))
	case slog.KindAny:
		return slog.Any(attr.Key, l.redactor.RedactValue(attr.Key, attr.Value.Any()))
	case slog.KindGroup:
		group := attr.Value.Group()
		sanitized := make([]slog.Attr, 0, len(group))
		for _, inner := range group {
			sanitized = append(sanitized, l.sanitizeAttr(inner))
		}
		return slog.Attr{Key: attr.Key, Value: slog.GroupValue(sanitized...)}
	default:
		return attr
	}
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
