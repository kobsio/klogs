package clickhouse

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/kobsio/klogs/pkg/log"

	"github.com/ClickHouse/clickhouse-go"
	"go.uber.org/zap"
)

// FieldString is the structure of the nested field for string values.
type FieldString struct {
	Key   []string
	Value []string
}

// FieldNumber is the structure of the nested field for number values.
type FieldNumber struct {
	Key   []string
	Value []float64
}

// Row is the structure of a single row in ClickHouse.
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

// Client can be used to write data to a ClickHouse instance. The client can be created via the NewClient function.
type Client struct {
	client             *sql.DB
	database           string
	asyncInsert        bool
	waitForAsyncInsert bool
}

// Write writes a list of rows to the configured ClickHouse instance.
func (c *Client) Write(buffer []Row) error {
	var settings string

	if c.asyncInsert {
		if c.waitForAsyncInsert {
			settings = "SETTINGS async_insert = 1, wait_for_async_insert = 1"
		} else {
			settings = "SETTINGS async_insert = 1, wait_for_async_insert = 0"
		}
	}

	sql := fmt.Sprintf("INSERT INTO %s.logs(timestamp, cluster, namespace, app, pod_name, container_name, host, fields_string.key, fields_string.value, fields_number.key, fields_number.value, log) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) %s", c.database, settings)

	tx, err := c.client.Begin()
	if err != nil {
		log.Error(nil, "Begin transaction failure", zap.Error(err))
		return err
	}

	smt, err := tx.Prepare(sql)
	if err != nil {
		log.Error(nil, "Prepare statement failure", zap.Error(err))
		return err
	}

	for _, l := range buffer {
		_, err = smt.Exec(l.Timestamp, l.Cluster, l.Namespace, l.App, l.Pod, l.Container, l.Host, l.FieldsString.Key, l.FieldsString.Value, l.FieldsNumber.Key, l.FieldsNumber.Value, l.Log)

		if err != nil {
			log.Error(nil, "Statement exec failure", zap.Error(err))
			return err
		}
	}

	if err = tx.Commit(); err != nil {
		log.Error(nil, "Commit failed failure", zap.Error(err))
		return err
	}

	return nil
}

// NewClient returns a new client for ClickHouse. The client can then be used to write data to ClickHouse via the
// "Write" method.
func NewClient(address, username, password, database, writeTimeout, readTimeout string, asyncInsert, waitForAsyncInsert bool) (*Client, error) {
	dns := "tcp://" + address + "?username=" + username + "&password=" + password + "&database=" + database + "&write_timeout=" + writeTimeout + "&read_timeout=" + readTimeout

	connect, err := sql.Open("clickhouse", dns)
	if err != nil {
		log.Error(nil, "could not initialize database connection", zap.Error(err))
		return nil, err
	}

	if err := connect.Ping(); err != nil {
		if exception, ok := err.(*clickhouse.Exception); ok {
			log.Error(nil, fmt.Sprintf("[%d] %s \n%s\n", exception.Code, exception.Message, exception.StackTrace))
		} else {
			log.Error(nil, "could not ping database", zap.Error(err))
		}

		return nil, err
	}

	return &Client{
		client:             connect,
		database:           database,
		asyncInsert:        asyncInsert,
		waitForAsyncInsert: waitForAsyncInsert,
	}, nil
}
