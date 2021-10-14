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

	"github.com/kobsio/fluent-bit-clickhouse/pkg/clickhouse"
	flatten "github.com/kobsio/fluent-bit-clickhouse/pkg/flatten/interface"
	"github.com/kobsio/fluent-bit-clickhouse/pkg/version"

	"github.com/fluent/fluent-bit-go/output"
	"github.com/sirupsen/logrus"
)

const (
	defaultDatabase      string        = "logs"
	defaultWriteTimeout  string        = "10"
	defaultReadTimeout   string        = "10"
	defaultBatchSize     int64         = 10000
	defaultFlushInterval time.Duration = 60 * time.Second
)

var (
	log = logrus.WithFields(logrus.Fields{"package": "clickhouse"})

	database      string
	batchSize     int64
	flushInterval time.Duration
	lastFlush     = time.Now()
	buffer        = make([]clickhouse.Row, 0)
	client        *clickhouse.Client
)

//export FLBPluginRegister
func FLBPluginRegister(def unsafe.Pointer) int {
	return output.FLBPluginRegister(def, "clickhouse", "ClickHouse Output Plugin for Fluent Bit")
}

//export FLBPluginInit
func FLBPluginInit(plugin unsafe.Pointer) int {
	var err error

	logFormat := output.FLBPluginConfigKey(plugin, "log_format")

	if logFormat == "json" {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logrus.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	}

	log.WithFields(version.Info()).Infof("Version information")
	log.WithFields(version.BuildContext()).Infof("Build context")

	address := output.FLBPluginConfigKey(plugin, "address")
	log.WithFields(logrus.Fields{"address": address}).Infof("set address")

	database = output.FLBPluginConfigKey(plugin, "database")
	if database == "" {
		database = defaultDatabase
	}
	log.WithFields(logrus.Fields{"database": database}).Infof("set database")

	username := output.FLBPluginConfigKey(plugin, "username")
	log.WithFields(logrus.Fields{"username": username}).Infof("set username")

	password := output.FLBPluginConfigKey(plugin, "password")
	log.WithFields(logrus.Fields{"password": "*****"}).Infof("set password")

	writeTimeout := output.FLBPluginConfigKey(plugin, "write_timeout")
	if writeTimeout == "" {
		writeTimeout = defaultReadTimeout
	}
	log.WithFields(logrus.Fields{"writeTimeout": writeTimeout}).Infof("set writeTimeout")

	readTimeout := output.FLBPluginConfigKey(plugin, "read_timeout")
	if readTimeout == "" {
		readTimeout = defaultReadTimeout
	}
	log.WithFields(logrus.Fields{"readTimeout": readTimeout}).Infof("set readTimeout")

	batchSizeStr := output.FLBPluginConfigKey(plugin, "batch_size")
	batchSize, err = strconv.ParseInt(batchSizeStr, 10, 64)
	if err != nil || batchSize < 0 {
		log.WithError(err).Errorf("could not parse batchSize setting, use default setting %d", defaultBatchSize)
		batchSize = defaultBatchSize
	}
	log.WithFields(logrus.Fields{"batchSize": batchSize}).Infof("set batchSize")

	flushIntervalStr := output.FLBPluginConfigKey(plugin, "flush_interval")
	flushInterval, err = time.ParseDuration(flushIntervalStr)
	if err != nil || flushInterval < 1*time.Second {
		log.WithError(err).Errorf("could not parse flushInterval setting, use default setting %s", defaultFlushInterval)
		flushInterval = defaultFlushInterval
	}
	log.WithFields(logrus.Fields{"flushInterval": flushInterval}).Infof("set flushInterval")

	clickhouseClient, err := clickhouse.NewClient(address, username, password, database, writeTimeout, readTimeout)
	if err != nil {
		log.WithError(err).Errorf("could not create ClickHouse client")
		return output.FLB_ERROR
	}

	client = clickhouseClient

	return output.FLB_OK
}

//export FLBPluginFlush
func FLBPluginFlush(data unsafe.Pointer, length C.int, tag *C.char) int {
	log.Errorf("flush called for unknown instance")
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
			log.Warnf("time provided invalid, defaulting to now.")
			timestamp = time.Now()
		}

		data, err := flatten.Flatten(record)
		if err != nil {
			log.WithError(err).Errorf("could not flat data")
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
					row.FieldsString.Key = append(row.FieldsString.Key, k)
					row.FieldsString.Value = append(row.FieldsString.Value, stringValue)
				}
			}
		}

		buffer = append(buffer, row)
	}

	startFlushTime := time.Now()
	if len(buffer) < int(batchSize) && lastFlush.Add(flushInterval).After(startFlushTime) {
		return output.FLB_OK
	}

	log.WithFields(logrus.Fields{"batchSize": len(buffer), "flushInterval": startFlushTime.Sub(lastFlush)}).Infof("start flushing")
	client.Write(buffer)

	lastFlush = time.Now()
	buffer = make([]clickhouse.Row, 0)
	log.WithFields(logrus.Fields{"flushTime": lastFlush.Sub(startFlushTime)}).Infof("end flushing")

	return output.FLB_OK
}

//export FLBPluginExit
func FLBPluginExit() int {
	log.Errorf("exit called for unknown instance")
	return output.FLB_OK
}

//export FLBPluginExitCtx
func FLBPluginExitCtx(ctx unsafe.Pointer) int {
	return output.FLB_OK
}

func main() {}
