package logger

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("should succeed with valid config", func(t *testing.T) {
		logger := New("json", "DEBUG")
		require.NotNil(t, logger)
	})

	t.Run("should succeed with invalid config", func(t *testing.T) {
		logger := New("console", "INFO")
		require.NotNil(t, logger)
	})
}

func TestHandle(t *testing.T) {
	logger := New("json", "DEBUG")

	// nolint:staticcheck
	ctx := AppendCtx(nil, slog.String("key1", "value1"))
	ctx = AppendCtx(ctx, slog.String("key2", "value2"))

	require.NotPanics(t, func() {
		logger.DebugContext(ctx, "test")
	})
}
