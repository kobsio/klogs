package metrics

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/kobsio/klogs/pkg/log"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

var (
	InputRecordsTotalMetric = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "klogs",
		Name:      "input_records_total",
		Help:      "Number of received records.",
	})
	ErrorsTotalMetric = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "klogs",
		Name:      "errors_total",
		Help:      "Number of errors when writing records to ClickHouse",
	})
	BatchSizeMetric = promauto.NewSummary(prometheus.SummaryOpts{
		Namespace:  "klogs",
		Name:       "batch_size",
		Help:       "The number of records which are written to ClickHouse.",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.95: 0.005, 0.99: 0.001},
	})
	FlushTimeSecondsMetric = promauto.NewSummary(prometheus.SummaryOpts{
		Namespace:  "klogs",
		Name:       "flush_time_seconds",
		Help:       "The time needed to write the records in seconds.",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.95: 0.005, 0.99: 0.001},
	})
)

// Server is the interface of a metrics service, which provides the options to start and stop the underlying http
// server.
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
	log.Info(nil, "Metrics server started", zap.String("address", s.Addr))

	if err := s.ListenAndServe(); err != nil {
		if err != http.ErrServerClosed {
			log.Error(nil, "Metrics server died unexpected", zap.Error(err))
		}
	}
}

// Stop terminates the metrics server gracefully.
func (s *server) Stop() {
	log.Debug(nil, "Start shutdown of the metrics server")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := s.Shutdown(ctx)
	if err != nil {
		log.Error(nil, "Graceful shutdown of the metrics server failed", zap.Error(err))
	}
}

// New return a new metrics server, which is used to serve Prometheus metrics on the specified address under the
// /metrics path.
func New(address string) Server {
	router := http.DefaultServeMux
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "OK")
	})
	router.Handle("/metrics", promhttp.Handler())

	return &server{
		&http.Server{
			Addr:    address,
			Handler: router,
		},
	}
}
