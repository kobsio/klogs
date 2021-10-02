// The fluent-bit-clickhouse Fluent Bit plugin can be used to write the logs collected by Fluent Bit to ClickHouse. It
// is heavily inspired by https://github.com/devcui/clickhouse-fluent-bit with some adjustments to the configuration and
// the table structure in ClickHouse, so that it is also possible to search through als fields of the logs. The way we
// are saving all the json fields of a log line is taken from the following gist:
// https://gist.github.com/alexey-milovidov/d6ffc9e0bc0bc72dd7bca90e76e3b83b.
package main

import (
	"C"
	"database/sql"
	"fmt"
	"strconv"
	"time"
	"unsafe"

	"github.com/kobsio/fluent-bit-clickhouse/pkg/flatten"
	"github.com/kobsio/fluent-bit-clickhouse/pkg/version"

	"github.com/ClickHouse/clickhouse-go"
	"github.com/fluent/fluent-bit-go/output"
	"github.com/sirupsen/logrus"
)

type FieldString struct {
	Key   []string
	Value []string
}

type FieldNumber struct {
	Key   []string
	Value []float64
}

type Row struct {
	Timestamp    time.Time
	Cluster      string
	Namespace    string
	App          string
	Pod          string
	Container    string
	Host         string
	FieldsString FieldString
	FieldsNumber FieldNumber
	Log          string
}

const (
	defaultWriteTimeout  string        = "20"
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
	buffer        = make([]Row, 0)
	client        *sql.DB
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

	dns := "tcp://" + address + "?username=" + username + "&password=" + password + "&database=" + database + "&write_timeout=" + writeTimeout + "&read_timeout=" + readTimeout

	connect, err := sql.Open("clickhouse", dns)
	if err != nil {
		log.WithError(err).Errorf("could not initialize database connection")
		return output.FLB_ERROR
	}

	if err := connect.Ping(); err != nil {
		if exception, ok := err.(*clickhouse.Exception); ok {
			log.Errorf("[%d] %s \n%s\n", exception.Code, exception.Message, exception.StackTrace)
		} else {
			log.WithError(err).Errorf("could not ping database")
		}

		return output.FLB_ERROR
	}

	client = connect

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

		row := Row{
			Timestamp: timestamp,
		}

		for k, v := range data {
			value := ""

			switch t := v.(type) {
			case string:
				value = t
			case []byte:
				value = string(t)
			default:
				value = fmt.Sprintf("%v", v)
			}

			switch k {
			case "cluster":
				row.Cluster = value
			case "kubernetes.namespace_name":
				row.Namespace = value
			case "kubernetes.labels.k8s-app":
				row.App = value
			case "kubernetes.labels.app":
				row.App = value
			case "kubernetes.pod_name":
				row.Pod = value
			case "kubernetes.container_name":
				row.Container = value
			case "kubernetes.host":
				row.Host = value
			case "log":
				row.Log = value
			default:
				parsedValue, err := strconv.ParseFloat(value, 64)
				if err != nil {
					row.FieldsString.Key = append(row.FieldsString.Key, k)
					row.FieldsString.Value = append(row.FieldsString.Value, value)
				} else {
					row.FieldsNumber.Key = append(row.FieldsNumber.Key, k)
					row.FieldsNumber.Value = append(row.FieldsNumber.Value, parsedValue)
				}
			}
		}

		buffer = append(buffer, row)
	}

	now := time.Now()
	startFlushTime := now

	if len(buffer) < int(batchSize) && lastFlush.Add(flushInterval).After(now) {
		return output.FLB_OK
	}

	log.WithFields(logrus.Fields{"batchSize": len(buffer), "flushInterval": now.Sub(lastFlush)}).Infof("start flushing")

	sql := fmt.Sprintf("INSERT INTO %s.logs(timestamp, cluster, namespace, app, pod_name, container_name, host, fields_string.key, fields_string.value, fields_number.key, fields_number.value, log) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)", database)

	tx, err := client.Begin()
	if err != nil {
		log.WithError(err).Errorf("begin transaction failure")
		return output.FLB_ERROR
	}

	smt, err := tx.Prepare(sql)
	if err != nil {
		log.WithError(err).Errorf("prepare statement failure")
		return output.FLB_ERROR
	}

	for _, l := range buffer {
		_, err = smt.Exec(l.Timestamp, l.Cluster, l.Namespace, l.App, l.Pod, l.Container, l.Host, l.FieldsString.Key, l.FieldsString.Value, l.FieldsNumber.Key, l.FieldsNumber.Value, l.Log)

		if err != nil {
			log.WithError(err).Errorf("statement exec failure")
			return output.FLB_ERROR
		}
	}

	if err = tx.Commit(); err != nil {
		log.WithError(err).Errorf("commit failed failure")
		return output.FLB_ERROR
	}

	now = time.Now()
	lastFlush = now
	buffer = make([]Row, 0)

	log.WithFields(logrus.Fields{"flushTime": now.Sub(startFlushTime)}).Infof("end flushing")

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
