package main

import (
	"C"
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"time"
	"unsafe"

	"github.com/kobsio/fluent-bit-clickhouse/pkg/flatten"
	"github.com/kobsio/fluent-bit-clickhouse/pkg/version"

	"github.com/ClickHouse/clickhouse-go"
	"github.com/fluent/fluent-bit-go/output"
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
	defaultWriteTimeout string = "20"
	defaultReadTimeout  string = "10"
	defaultBatchSize    int64  = 10000
)

var (
	database  string
	batchSize int64
	buffer    = make([]Row, 0)
	client    *sql.DB
)

//export FLBPluginRegister
func FLBPluginRegister(def unsafe.Pointer) int {
	return output.FLBPluginRegister(def, "clickhouse", "ClickHouse Output Plugin for Fluent Bit")
}

//export FLBPluginInit
func FLBPluginInit(plugin unsafe.Pointer) int {
	var err error

	log.Printf("[clickhouse] %s", version.Info())
	log.Printf("[clickhouse] %s", version.BuildContext())

	address := output.FLBPluginConfigKey(plugin, "address")
	log.Printf("[clickhouse] address = %q", address)

	database = output.FLBPluginConfigKey(plugin, "database")
	log.Printf("[clickhouse] database = %q", database)

	username := output.FLBPluginConfigKey(plugin, "username")
	log.Printf("[clickhouse] username = %q", username)

	password := output.FLBPluginConfigKey(plugin, "password")
	log.Printf("[clickhouse] password = *****")

	writeTimeout := output.FLBPluginConfigKey(plugin, "write_timeout")
	if writeTimeout == "" {
		writeTimeout = defaultReadTimeout
	}
	log.Printf("[clickhouse] writeTimeout = %s", writeTimeout)

	readTimeout := output.FLBPluginConfigKey(plugin, "read_timeout")
	if readTimeout == "" {
		readTimeout = defaultReadTimeout
	}
	log.Printf("[clickhouse] readTimeout = %s", readTimeout)

	batchSizeStr := output.FLBPluginConfigKey(plugin, "batch_size")
	batchSize, err = strconv.ParseInt(batchSizeStr, 10, 64)
	if err != nil || batchSize < 0 {
		batchSize = defaultBatchSize
	}
	log.Printf("[clickhouse] batchSize = %d", batchSize)

	dns := "tcp://" + address + "?username=" + username + "&password=" + password + "&database=" + database + "&write_timeout=" + writeTimeout + "&read_timeout=" + readTimeout

	connect, err := sql.Open("clickhouse", dns)
	if err != nil {
		log.Printf("[clickhouse] could not initialize database connection: %#v", err)
		return output.FLB_ERROR
	}

	if err := connect.Ping(); err != nil {
		if exception, ok := err.(*clickhouse.Exception); ok {
			log.Printf("[clickhouse] [%d] %s \n%s\n", exception.Code, exception.Message, exception.StackTrace)
		} else {
			log.Printf("[clickhouse] could not ping database: %#v", err)
		}

		return output.FLB_ERROR
	}

	client = connect

	return output.FLB_OK
}

//export FLBPluginFlush
func FLBPluginFlush(data unsafe.Pointer, length C.int, tag *C.char) int {
	log.Printf("[clickhouse] flush called for unknown instance")
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
			log.Printf("[clickhouse] time provided invalid, defaulting to now.")
			timestamp = time.Now()
		}

		data, err := flatten.Flatten(record)
		if err != nil {
			log.Printf("[clickhouse] could not flat data: %#v", err)
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

	if len(buffer) < int(batchSize) {
		return output.FLB_OK
	}

	sql := fmt.Sprintf("INSERT INTO %s.logs(timestamp, cluster, namespace, app, pod_name, container_name, host, fields_string.key, fields_string.value, fields_number.key, fields_number.value, log) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)", database)

	tx, err := client.Begin()
	if err != nil {
		log.Printf("[clickhouse] begin transaction failure: %#v", err)
		return output.FLB_ERROR
	}

	smt, err := tx.Prepare(sql)
	if err != nil {
		log.Printf("[clickhouse] prepare statement failure: %#v", err)
		return output.FLB_ERROR
	}

	for _, l := range buffer {
		_, err = smt.Exec(l.Timestamp, l.Cluster, l.Namespace, l.App, l.Pod, l.Container, l.Host, l.FieldsString.Key, l.FieldsString.Value, l.FieldsNumber.Key, l.FieldsNumber.Value, l.Log)

		if err != nil {
			log.Printf("[clickhouse] statement exec failure: %#v", err)
			return output.FLB_ERROR
		}
	}

	if err = tx.Commit(); err != nil {
		log.Printf("[clickhouse] commit failed failure: %#v", err)
		return output.FLB_ERROR
	}

	buffer = make([]Row, 0)

	return output.FLB_OK
}

//export FLBPluginExit
func FLBPluginExit() int {
	log.Printf("[clickhouse] exit called for unknown instance")
	return output.FLB_OK
}

//export FLBPluginExitCtx
func FLBPluginExitCtx(ctx unsafe.Pointer) int {
	return output.FLB_OK
}

func main() {}
