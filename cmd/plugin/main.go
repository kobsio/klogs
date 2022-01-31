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
import "strings"

const (
	defaultDatabase      string        = "logs"
	defaultWriteTimeout  string        = "10"
	defaultReadTimeout   string        = "10"
	defaultBatchSize     int64         = 10000
	defaultFlushInterval time.Duration = 60 * time.Second
)

var (
	database          string
	batchSize         int64
	flushInterval     time.Duration
	forceNumberFields []string
	lastFlush         = time.Now()
	buffer            = make([]clickhouse.Row, 0)
	client            *clickhouse.Client
)

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

	logger, err := zapConfig.Build()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	zap.ReplaceGlobals(logger)
	log.Info(nil, "Version information", version.Info()...)
	log.Info(nil, "Build context", version.BuildContext()...)

	address := output.FLBPluginConfigKey(plugin, "address")

	database = output.FLBPluginConfigKey(plugin, "database")
	if database == "" {
		database = defaultDatabase
	}

	username := output.FLBPluginConfigKey(plugin, "username")

	password := output.FLBPluginConfigKey(plugin, "password")

	writeTimeout := output.FLBPluginConfigKey(plugin, "write_timeout")
	if writeTimeout == "" {
		writeTimeout = defaultReadTimeout
	}

	readTimeout := output.FLBPluginConfigKey(plugin, "read_timeout")
	if readTimeout == "" {
		readTimeout = defaultReadTimeout
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

	log.Info(nil, "Clickhouse configuration", zap.String("clickhouseAddress", address), zap.String("clickhouseUsername", username), zap.String("clickhousePassword", "*****"), zap.String("clickhouseDatabase", database), zap.String("clickhouseWriteTimeout", writeTimeout), zap.String("clickhouseReadTimeout", readTimeout), zap.Int64("clickhouseBatchSize", batchSize), zap.Duration("clickhouseFlushInterval", flushInterval), zap.Strings("forceNumberFields", forceNumberFields))

	clickhouseClient, err := clickhouse.NewClient(address, username, password, database, writeTimeout, readTimeout, asyncInsert, waitForAsyncInsert)
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

		var timestamp time.Time
		switch t := ts.(type) {
		case output.FLBTime:
			timestamp = ts.(output.FLBTime).Time
		case uint64:
			timestamp = time.Unix(int64(t), 0)
		default:
			log.Warn(nil, "The provided time is invalid, defaulting to now")
			timestamp = time.Now()
		}

		data, err := flatten.Flatten(record)
		if err != nil {
			log.Error(nil, "Could not flatten data", zap.Error(err))
			break
		}

		row := clickhouse.Row{
			Timestamp: timestamp,
		}

		for k, v := range data {
			var stringValue string
			var numberValue float64
			var isNumber bool

			switch t := v.(type) {
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

			switch k {
			case "cluster":
				row.Cluster = stringValue
			case "kubernetes.namespace_name":
				row.Namespace = stringValue
			case "kubernetes.labels.k8s-app":
				row.App = stringValue
			case "kubernetes.labels.app":
				row.App = stringValue
			case "kubernetes.pod_name":
				row.Pod = stringValue
			case "kubernetes.container_name":
				row.Container = stringValue
			case "kubernetes.host":
				row.Host = stringValue
			case "log":
				row.Log = stringValue
			default:
				if isNumber {
					row.FieldsNumber.Key = append(row.FieldsNumber.Key, k)
					row.FieldsNumber.Value = append(row.FieldsNumber.Value, numberValue)
				} else {
					if contains(k, forceNumberFields) {
						parsedNumber, err := strconv.ParseFloat(stringValue, 64)
						if err == nil {
							row.FieldsNumber.Key = append(row.FieldsNumber.Key, k)
							row.FieldsNumber.Value = append(row.FieldsNumber.Value, parsedNumber)
						} else {
							row.FieldsString.Key = append(row.FieldsString.Key, k)
							row.FieldsString.Value = append(row.FieldsString.Value, stringValue)
						}
					} else {
						row.FieldsString.Key = append(row.FieldsString.Key, k)
						row.FieldsString.Value = append(row.FieldsString.Value, stringValue)
					}
				}
			}
		}

		buffer = append(buffer, row)
	}

	startFlushTime := time.Now()
	if len(buffer) < int(batchSize) && lastFlush.Add(flushInterval).After(startFlushTime) {
		return output.FLB_OK
	}

	log.Info(nil, "Start flushing", zap.Int("batchSize", len(buffer)), zap.Duration("flushInterval", startFlushTime.Sub(lastFlush)))
	client.Write(buffer)

	lastFlush = time.Now()
	buffer = make([]clickhouse.Row, 0)
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
	return output.FLB_OK
}

func main() {}
