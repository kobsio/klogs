// The fluent-bit-clickhouse Fluent Bit plugin can be used to write the logs collected by Fluent Bit to ClickHouse. It
// is heavily inspired by https://github.com/devcui/clickhouse-fluent-bit with some adjustments to the configuration and
// the table structure in ClickHouse, so that it is also possible to search through als fields of the logs. The way we
// are saving all the json fields of a log line is taken from the following gist:
// https://gist.github.com/alexey-milovidov/d6ffc9e0bc0bc72dd7bca90e76e3b83b.
package main

import (
	"C"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/kobsio/klogs/pkg/clickhouse"
	flatten "github.com/kobsio/klogs/pkg/flatten/interface"
	"github.com/kobsio/klogs/pkg/log"
	"github.com/kobsio/klogs/pkg/version"

	"github.com/fluent/fluent-bit-go/output"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)
import "github.com/kobsio/klogs/pkg/metrics"

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
)

func getTimestamp(ts interface{}) time.Time {
	switch t := ts.(type) {
	case output.FLBTime:
		return ts.(output.FLBTime).Time
	case uint64:
		return time.Unix(int64(t), 0)
	// Since Fluent Bit v2.1.0, Event format is represented as 2-element array with a nested array as the first element
	// The timestamp field is allocated in the first position of that nested array: [[TIMESTAMP, METADATA], MESSAGE]
	// https://docs.fluentbit.io/manual/concepts/key-concepts#event-format
	case []interface{}:
		return getTimestamp(ts.([]interface{})[0])
	default:
		log.Warn(nil, "The provided time is invalid, defaulting to now")
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

	// Configure our logging library. The logs can be written in console format (the console format is compatible with
	// logfmt) or in json format. The default is console, because it is better to read during development. In a
	// production environment you should consider to use json, so that the logs can be parsed by a logging system like
	// Elasticsearch.
	// Next to the log format it is also possible to configure the log leven. The accepted values are "debug", "info",
	// "warn", "error", "fatal" and "panic". The default log level is "info".
	logFormat := output.FLBPluginConfigKey(plugin, "log_format")
	if logFormat != "json" {
		logFormat = "console"
	}

	logLevel := output.FLBPluginConfigKey(plugin, "log_level")

	zapEncoderCfg := zap.NewProductionEncoderConfig()
	zapEncoderCfg.TimeKey = "timestamp"
	zapEncoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	zapConfig := zap.Config{
		Level:            log.ParseLevel(logLevel),
		Development:      false,
		Encoding:         logFormat,
		EncoderConfig:    zapEncoderCfg,
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
	}

	logger, err := zapConfig.Build(zap.AddCaller(), zap.AddCallerSkip(1))
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	zap.ReplaceGlobals(logger)
	log.Info(nil, "Version information", version.Info()...)
	log.Info(nil, "Build context", version.BuildContext()...)

	// Read the configuration for the address where the metrics server should listen on. Then create a new metrics
	// server and start the server in a new Go routine via the `Start` method.
	//
	// When the plugin exits the metrics server should be stopped via the `Stop` method.
	metricsServerAddress := output.FLBPluginConfigKey(plugin, "metrics_server_address")
	if metricsServerAddress == "" {
		metricsServerAddress = defaultMetricsServerAddress
	}

	metricsServer = metrics.New(metricsServerAddress)
	go metricsServer.Start()

	// Read all configuration values required for the ClickHouse client. Once we have all configuration values we
	// create a new ClickHouse client, which can then be used to write the logs from Fluent Bit into ClickHouse when the
	// FLBPluginFlushCtx function is called.
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
		log.Warn(nil, "Could not parse maxIdleConns setting, use default setting", zap.Error(err), zap.Int("default", defaultMaxIdleConns))
		maxIdleConns = defaultMaxIdleConns
	}

	maxOpenConnsStr := output.FLBPluginConfigKey(plugin, "max_open_conns")
	maxOpenConns, err := strconv.Atoi(maxOpenConnsStr)
	if err != nil || maxOpenConns < 0 {
		log.Warn(nil, "Could not parse maxOpenConns setting, use default setting", zap.Error(err), zap.Int("default", defaultMaxOpenConns))
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
		log.Warn(nil, "Could not parse batchSize setting, use default setting", zap.Error(err), zap.Int64("default", defaultBatchSize))
		batchSize = defaultBatchSize
	}

	flushIntervalStr := output.FLBPluginConfigKey(plugin, "flush_interval")
	flushInterval, err = time.ParseDuration(flushIntervalStr)
	if err != nil || flushInterval < 1*time.Second {
		log.Warn(nil, "Could not parse flushInterval setting, use default setting", zap.Error(err), zap.Duration("default", defaultFlushInterval))
		flushInterval = defaultFlushInterval
	}

	forceNumberFieldsStr := output.FLBPluginConfigKey(plugin, "force_number_fields")
	forceNumberFields = strings.Split(forceNumberFieldsStr, ",")

	forceUnderscoresStr := output.FLBPluginConfigKey(plugin, "force_underscores")
	forceUnderscores, err = strconv.ParseBool(forceUnderscoresStr)
	if err != nil {
		log.Warn(nil, "Could not parse forceUnderscores setting, use default setting", zap.Error(err), zap.Bool("default", defaultForceUnderscores))
		forceUnderscores = defaultForceUnderscores
	}

	log.Info(nil, "Clickhouse configuration", zap.String("address", address), zap.String("username", username), zap.String("password", "*****"), zap.String("database", database), zap.String("dialTimeout", dialTimeout), zap.String("connMaxLifetime", connMaxLifetime), zap.Int("maxIdleConns", maxIdleConns), zap.Int("maxOpenConns", maxOpenConns), zap.Int64("batchSize", batchSize), zap.Duration("flushInterval", flushInterval))

	clickhouseClient, err := clickhouse.NewClient(address, username, password, database, dialTimeout, connMaxLifetime, maxIdleConns, maxOpenConns, asyncInsert, waitForAsyncInsert)
	if err != nil {
		log.Fatal(nil, "Could not create ClickHouse client", zap.Error(err))
		return output.FLB_ERROR
	}

	client = clickhouseClient

	return output.FLB_OK
}

//export FLBPluginFlush
func FLBPluginFlush(data unsafe.Pointer, length C.int, tag *C.char) int {
	log.Error(nil, "Flush called for unknown instance")
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

		metrics.InputRecordsTotalMetric.Inc()

		timestamp := getTimestamp(ts)

		data, err := flatten.Flatten(record)
		if err != nil {
			log.Error(nil, "Could not flatten data", zap.Error(err))
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

	log.Info(nil, "Start flushing", zap.Int("batchSize", currentBatchSize), zap.Duration("flushInterval", startFlushTime.Sub(lastFlush)))
	err := client.BufferWrite()
	if err != nil {
		metrics.ErrorsTotalMetric.Inc()
		log.Error(nil, "Error while writing buffer", zap.Error(err))
		return output.FLB_ERROR
	}

	lastFlush = time.Now()
	metrics.BatchSizeMetric.Observe(float64(currentBatchSize))
	metrics.FlushTimeSecondsMetric.Observe(lastFlush.Sub(startFlushTime).Seconds())
	log.Info(nil, "End flushing", zap.Duration("flushTime", lastFlush.Sub(startFlushTime)))

	return output.FLB_OK
}

//export FLBPluginExit
func FLBPluginExit() int {
	log.Error(nil, "Exit called for unknown instance")
	return output.FLB_OK
}

//export FLBPluginExitCtx
func FLBPluginExitCtx(ctx unsafe.Pointer) int {
	metricsServer.Stop()
	return output.FLB_OK
}

func main() {}
