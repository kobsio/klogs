// The klogs Fluent Bit plugin can be used to write the logs collected by Fluent
// Bit to ClickHouse. It is heavily inspired by
// https://github.com/devcui/clickhouse-fluent-bit with some adjustments to the
// configuration and the table structure in ClickHouse, so that it is also
// possible to search through als fields of the logs. The way we are saving all
// the json fields of a log line is taken from the following gist:
// https://gist.github.com/alexey-milovidov/d6ffc9e0bc0bc72dd7bca90e76e3b83b.
package main

import (
	"C"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/kobsio/klogs/pkg/clickhouse"
	"github.com/kobsio/klogs/pkg/flatten"
	"github.com/kobsio/klogs/pkg/instrument/logger"
	"github.com/kobsio/klogs/pkg/instrument/metrics"
	"github.com/kobsio/klogs/pkg/version"

	"github.com/fluent/fluent-bit-go/output"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	defaultMetricsServerAddress string        = ":2021"
	defaultDatabase             string        = "logs"
	defaultDialTimeout          string        = "10s"
	defaultConnMaxLifetime      string        = "1h"
	defaultMaxIdleConns         int           = 1
	defaultMaxOpenConns         int           = 1
	defaultBatchSize            int64         = 10000
	defaultFlushInterval        time.Duration = 60 * time.Second
	defaultForceUnderscores     bool          = false
)

var (
	database          string
	batchSize         int64
	flushInterval     time.Duration
	forceNumberFields []string
	forceUnderscores  bool
	lastFlush         = time.Now()
	client            *clickhouse.Client
	metricsServer     metrics.Server

	inputRecordsTotalMetric = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "klogs",
		Name:      "input_records_total",
		Help:      "Number of received records.",
	})
	errorsTotalMetric = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "klogs",
		Name:      "errors_total",
		Help:      "Number of errors when writing records to ClickHouse",
	})
	batchSizeMetric = promauto.NewSummary(prometheus.SummaryOpts{
		Namespace:  "klogs",
		Name:       "batch_size",
		Help:       "The number of records which are written to ClickHouse.",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.95: 0.005, 0.99: 0.001},
	})
	flushTimeSecondsMetric = promauto.NewSummary(prometheus.SummaryOpts{
		Namespace:  "klogs",
		Name:       "flush_time_seconds",
		Help:       "The time needed to write the records in seconds.",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.95: 0.005, 0.99: 0.001},
	})
)

func contains(field string, fields []string) bool {
	for _, f := range fields {
		if f == field {
			return true
		}
	}
	return false
}

func getTimestamp(ts interface{}) time.Time {
	switch t := ts.(type) {
	case output.FLBTime:
		return ts.(output.FLBTime).Time
	case uint64:
		return time.Unix(int64(t), 0)
	// Since Fluent Bit v2.1.0, Event format is represented as 2-element array
	// with a nested array as the first element. The timestamp field is
	// allocated in the first position of that nested array:
	// [[TIMESTAMP, METADATA], MESSAGE]
	//
	// See: https://docs.fluentbit.io/manual/concepts/key-concepts#event-format
	case []interface{}:
		return getTimestamp(ts.([]interface{})[0])
	default:
		slog.Warn("Failed to parse time, defaulting to now")
		return time.Now()
	}
}

//export FLBPluginRegister
func FLBPluginRegister(def unsafe.Pointer) int {
	return output.FLBPluginRegister(def, "clickhouse", "ClickHouse Output Plugin for Fluent Bit")
}

//export FLBPluginInit
func FLBPluginInit(plugin unsafe.Pointer) int {
	var err error

	// Configure our logging library. The logs can be written in "console"
	// format or in "json" format. The default is "console", because it is
	// better to read during development. In a production environment you should
	// consider to use json, so that the logs can be parsed by Fluent Bit.
	//
	// Next to the log format it is also possible to configure the log level.
	// The accepted values are "DEBUG", "INFO", "WARN" and "ERROR".
	logFormat := output.FLBPluginConfigKey(plugin, "log_format")
	logLevel := output.FLBPluginConfigKey(plugin, "log_level")

	logger := logger.New(logFormat, logLevel)
	logger.Info("Version information.", "version", slog.GroupValue(version.Info()...))
	logger.Info("Build information.", "build", slog.GroupValue(version.BuildContext()...))

	// Read the configuration for the address where the metrics server should
	// listen on. Then create a new metrics server and start the server in a new
	// Go routine via the `Start` method.
	//
	// When the plugin exits the metrics server should be stopped via the `Stop`
	// method.
	metricsServerAddress := output.FLBPluginConfigKey(plugin, "metrics_server_address")
	if metricsServerAddress == "" {
		metricsServerAddress = defaultMetricsServerAddress
	}

	metricsServer = metrics.New(metricsServerAddress)
	go metricsServer.Start()

	// Read all configuration values required for the ClickHouse client. Once we
	// have all configuration values we create a new ClickHouse client, which
	// can then be used to write the logs from Fluent Bit into ClickHouse when
	// the FLBPluginFlushCtx function is called.
	address := output.FLBPluginConfigKey(plugin, "address")

	database = output.FLBPluginConfigKey(plugin, "database")
	if database == "" {
		database = defaultDatabase
	}

	username := output.FLBPluginConfigKey(plugin, "username")

	password := output.FLBPluginConfigKey(plugin, "password")

	dialTimeout := output.FLBPluginConfigKey(plugin, "dial_timeout")
	if dialTimeout == "" {
		dialTimeout = defaultDialTimeout
	}

	connMaxLifetime := output.FLBPluginConfigKey(plugin, "conn_max_lifetime")
	if connMaxLifetime == "" {
		connMaxLifetime = defaultConnMaxLifetime
	}

	maxIdleConnsStr := output.FLBPluginConfigKey(plugin, "max_idle_conns")
	maxIdleConns, err := strconv.Atoi(maxIdleConnsStr)
	if err != nil || maxIdleConns < 0 {
		slog.Warn("Failed to parse maxIdleConns setting, use default setting", slog.Any("error", err), slog.String("provided", maxIdleConnsStr), slog.Int("default", defaultMaxIdleConns))
		maxIdleConns = defaultMaxIdleConns
	}

	maxOpenConnsStr := output.FLBPluginConfigKey(plugin, "max_open_conns")
	maxOpenConns, err := strconv.Atoi(maxOpenConnsStr)
	if err != nil || maxOpenConns < 0 {
		slog.Warn("Failed to parse maxOpenConns setting, use default setting", slog.Any("error", err), slog.String("provided", maxOpenConnsStr), slog.Int("default", defaultMaxOpenConns))
		maxOpenConns = defaultMaxOpenConns
	}

	asyncInsertStr := output.FLBPluginConfigKey(plugin, "async_insert")
	var asyncInsert bool
	if asyncInsertStr == "true" {
		asyncInsert = true
	}

	waitForAsyncInsertStr := output.FLBPluginConfigKey(plugin, "wait_for_async_insert")
	var waitForAsyncInsert bool
	if waitForAsyncInsertStr == "true" {
		waitForAsyncInsert = true
	}

	batchSizeStr := output.FLBPluginConfigKey(plugin, "batch_size")
	batchSize, err = strconv.ParseInt(batchSizeStr, 10, 64)
	if err != nil || batchSize < 0 {
		slog.Warn("Failed to parse batchSize setting, use default setting", slog.Any("error", err), slog.String("provided", batchSizeStr), slog.Int64("default", defaultBatchSize))
		batchSize = defaultBatchSize
	}

	flushIntervalStr := output.FLBPluginConfigKey(plugin, "flush_interval")
	flushInterval, err = time.ParseDuration(flushIntervalStr)
	if err != nil || flushInterval < 1*time.Second {
		slog.Warn("Failed to parse flushInterval setting, use default setting", slog.Any("error", err), slog.String("provided", flushIntervalStr), slog.Duration("default", defaultFlushInterval))
		flushInterval = defaultFlushInterval
	}

	forceNumberFieldsStr := output.FLBPluginConfigKey(plugin, "force_number_fields")
	forceNumberFields = strings.Split(forceNumberFieldsStr, ",")

	forceUnderscoresStr := output.FLBPluginConfigKey(plugin, "force_underscores")
	forceUnderscores, err = strconv.ParseBool(forceUnderscoresStr)
	if err != nil {
		slog.Warn("Failed to parse forceUnderscores setting, use default setting", slog.Any("error", err), slog.String("provided", forceUnderscoresStr), slog.Bool("default", defaultForceUnderscores))
		forceUnderscores = defaultForceUnderscores
	}

	slog.Info("Clickhouse configuration", slog.String("address", address), slog.String("username", username), slog.String("password", "*****"), slog.String("database", database), slog.String("dialTimeout", dialTimeout), slog.String("connMaxLifetime", connMaxLifetime), slog.Int("maxIdleConns", maxIdleConns), slog.Int("maxOpenConns", maxOpenConns), slog.Int64("batchSize", batchSize), slog.Duration("flushInterval", flushInterval))

	clickhouseClient, err := clickhouse.NewClient(address, username, password, database, dialTimeout, connMaxLifetime, maxIdleConns, maxOpenConns, asyncInsert, waitForAsyncInsert)
	if err != nil {
		slog.Error("Failed to create ClickHouse client", slog.Any("error", err))
		return output.FLB_ERROR
	}

	client = clickhouseClient

	return output.FLB_OK
}

//export FLBPluginFlush
func FLBPluginFlush(data unsafe.Pointer, length C.int, tag *C.char) int {
	slog.Error("Flush called for unknown instance")
	return output.FLB_OK
}

//export FLBPluginFlushCtx
func FLBPluginFlushCtx(ctx, data unsafe.Pointer, length C.int, tag *C.char) int {
	dec := output.NewDecoder(data, int(length))

	for {
		ret, ts, record := output.GetRecord(dec)
		if ret != 0 {
			break
		}

		inputRecordsTotalMetric.Inc()

		timestamp := getTimestamp(ts)

		data, err := flatten.Flatten(record)
		if err != nil {
			slog.Error("Failed to flatten data", slog.Any("error", err))
			break
		}

		row := clickhouse.Row{
			Timestamp:    timestamp,
			FieldsString: make(map[string]string),
			FieldsNumber: make(map[string]float64),
		}

		for k, v := range data {
			var stringValue string
			var numberValue float64
			var isNumber bool
			var isNil bool

			switch t := v.(type) {
			case nil:
				isNil = true
			case string:
				stringValue = t
			case []byte:
				stringValue = string(t)
			case int:
				isNumber = true
				numberValue = float64(t)
			case int8:
				isNumber = true
				numberValue = float64(t)
			case int16:
				isNumber = true
				numberValue = float64(t)
			case int32:
				isNumber = true
				numberValue = float64(t)
			case int64:
				isNumber = true
				numberValue = float64(t)
			case float32:
				isNumber = true
				numberValue = float64(t)
			case float64:
				isNumber = true
				numberValue = float64(t)
			case uint8:
				isNumber = true
				numberValue = float64(t)
			case uint16:
				isNumber = true
				numberValue = float64(t)
			case uint32:
				isNumber = true
				numberValue = float64(t)
			case uint64:
				isNumber = true
				numberValue = float64(t)
			default:
				stringValue = fmt.Sprintf("%v", v)
			}

			if !isNil {
				switch k {
				case "cluster":
					row.Cluster = stringValue
				case "kubernetes_namespace_name":
					row.Namespace = stringValue
				case "kubernetes_labels_k8s-app":
					row.App = stringValue
				case "kubernetes_labels_app":
					row.App = stringValue
				case "kubernetes_pod_name":
					row.Pod = stringValue
				case "kubernetes_container_name":
					row.Container = stringValue
				case "kubernetes_host":
					row.Host = stringValue
				case "log":
					row.Log = stringValue
				default:
					formattedKey := k
					if forceUnderscores {
						formattedKey = strings.ReplaceAll(k, ".", "_")
					}

					if isNumber {
						row.FieldsNumber[formattedKey] = numberValue
					} else {
						if contains(k, forceNumberFields) {
							parsedNumber, err := strconv.ParseFloat(stringValue, 64)
							if err == nil {
								row.FieldsNumber[formattedKey] = parsedNumber
							} else {
								row.FieldsString[formattedKey] = stringValue
							}
						} else {
							row.FieldsString[formattedKey] = stringValue
						}
					}
				}
			}
		}

		client.BufferAdd(row)
	}

	startFlushTime := time.Now()
	currentBatchSize := client.BufferLen()
	if currentBatchSize < int(batchSize) && lastFlush.Add(flushInterval).After(startFlushTime) {
		return output.FLB_OK
	}

	slog.Info("Start flushing", slog.Int("batchSize", currentBatchSize), slog.Duration("flushInterval", startFlushTime.Sub(lastFlush)))
	err := client.BufferWrite()
	if err != nil {
		errorsTotalMetric.Inc()
		slog.Error("Error while writing buffer", slog.Any("error", err))
		return output.FLB_ERROR
	}

	lastFlush = time.Now()
	batchSizeMetric.Observe(float64(currentBatchSize))
	flushTimeSecondsMetric.Observe(lastFlush.Sub(startFlushTime).Seconds())
	slog.Info("End flushing", slog.Duration("flushTime", lastFlush.Sub(startFlushTime)))

	return output.FLB_OK
}

//export FLBPluginExit
func FLBPluginExit() int {
	slog.Error("Exit called for unknown instance")
	return output.FLB_OK
}

//export FLBPluginExitCtx
func FLBPluginExitCtx(ctx unsafe.Pointer) int {
	slog.Info("Shutdown Fluent Bit plugin")
	defer metricsServer.Stop()

	err := client.BufferWrite()
	if err != nil {
		slog.Error("Error while writing buffer", slog.Any("error", err))
		return output.FLB_ERROR
	}
	return output.FLB_OK
}

func main() {}
