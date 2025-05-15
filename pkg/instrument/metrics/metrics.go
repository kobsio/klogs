package metrics

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server is the interface of a metrics service, which provides the options to
// start and stop the underlying http server.
type Server interface {
	Start()
	Stop()
}

// server implements the Server interface.
type server struct {
	*http.Server
}

// Start starts serving the metrics server.
func (s *server) Start() {
	slog.Info("Metrics server started")

	if err := s.ListenAndServe(); err != nil {
		if err != http.ErrServerClosed {
			slog.Error("Metrics server died unexpected", slog.Any("error", err))
		}
	}
}

// Stop terminates the metrics server gracefully.
func (s *server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	slog.Debug("Start shutdown of the metrics server")

	err := s.Shutdown(ctx)
	if err != nil {
		slog.Error("Graceful shutdown of the metrics server failed", slog.Any("error", err))
	}
}

// New return a new metrics server, which is used to serve Prometheus metrics on
// the specified address under the /metrics path.
func New(address string) Server {
	router := http.DefaultServeMux
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "OK")
	})
	router.Handle("/metrics", promhttp.Handler())

	return &server{
		&http.Server{
			Addr:              address,
			Handler:           router,
			ReadHeaderTimeout: 5 * time.Second,
		},
	}
}
