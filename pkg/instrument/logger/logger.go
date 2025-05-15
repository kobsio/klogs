package logger

import (
	"context"
	"log/slog"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type ctxSlogFieldsKey int

const slogFields ctxSlogFieldsKey = 0

var (
	logCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "klogs",
		Name:      "logs_total",
		Help:      "Number of logs, partitioned by log level.",
	}, []string{"level"})
)

func parseLevel(s string) slog.Level {
	var level slog.Level
	if err := level.UnmarshalText([]byte(s)); err != nil {
		return slog.LevelInfo
	}
	return level
}

func New(format, level string) *slog.Logger {
	var handler slog.Handler

	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: true,
			Level:     parseLevel(level),
		})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: true,
			Level:     parseLevel(level),
		})
	}

	handler = &CustomHandler{handler}
	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger
}

// CustomHandler is a custom handler for our logger, which adds the request Id
// and trace Id to the log record, if they exists in the provided context.
type CustomHandler struct {
	slog.Handler
}

func (h *CustomHandler) Handle(ctx context.Context, r slog.Record) error {
	logCount.WithLabelValues(r.Level.String()).Inc()

	if attrs, ok := ctx.Value(slogFields).([]slog.Attr); ok {
		for _, v := range attrs {
			r.AddAttrs(v)
		}
	}

	return h.Handler.Handle(ctx, r)
}

func (c *CustomHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return c.clone()
}

func (c *CustomHandler) clone() *CustomHandler {
	clone := *c
	return &clone
}

func AppendCtx(parent context.Context, attrs ...slog.Attr) context.Context {
	if parent == nil {
		parent = context.Background()
	}

	if v, ok := parent.Value(slogFields).([]slog.Attr); ok {
		v = append(v, attrs...)
		return context.WithValue(parent, slogFields, v)
	}

	v := []slog.Attr{}
	v = append(v, attrs...)
	return context.WithValue(parent, slogFields, v)
}
