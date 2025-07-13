package clickhouse

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// Row is the structure of a single row in ClickHouse.
type Row struct {
	Timestamp    time.Time
	Cluster      string
	Namespace    string
	App          string
	Pod          string
	Container    string
	Host         string
	FieldsString map[string]string
	FieldsNumber map[string]float64
	Log          string
}

// Client can be used to write data to a ClickHouse instance. The client can be
// created via the NewClient function.
type Client struct {
	client             *sql.DB
	database           string
	asyncInsert        bool
	waitForAsyncInsert bool
	bufferMutex        *sync.RWMutex
	buffer             []Row
}

// BufferAdd adds a new row to the Clickhouse buffer. This doesn't write the
// added row. To write the rows in the buffer the `write` method must be called.
func (c *Client) BufferAdd(row Row) {
	c.bufferMutex.Lock()
	defer c.bufferMutex.Unlock()

	c.buffer = append(c.buffer, row)
}

// BufferLen returns the number of items in the buffer.
func (c *Client) BufferLen() int {
	c.bufferMutex.Lock()
	defer c.bufferMutex.Unlock()

	return len(c.buffer)
}

// BufferWrite writes a list of rows from the buffer to the configured
// ClickHouse instance.
func (c *Client) BufferWrite() error {
	ctx := context.Background()

	c.bufferMutex.Lock()
	defer c.bufferMutex.Unlock()

	var settings string

	if c.asyncInsert {
		if c.waitForAsyncInsert {
			settings = "SETTINGS async_insert = 1, wait_for_async_insert = 1"
		} else {
			settings = "SETTINGS async_insert = 1, wait_for_async_insert = 0"
		}
	}

	// #nosec G201
	sql := fmt.Sprintf("INSERT INTO %s.logs (timestamp, cluster, namespace, app, pod_name, container_name, host, fields_string, fields_number, log) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) %s", c.database, settings)

	tx, err := c.client.BeginTx(ctx, nil)
	if err != nil {
		slog.Error("Begin transaction failure", slog.Any("error", err))
		return err
	}

	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, sql)
	if err != nil {
		slog.Error("Prepare statement failure", slog.Any("error", err))
		return err
	}

	for _, l := range c.buffer {
		_, err = stmt.ExecContext(ctx, l.Timestamp, l.Cluster, l.Namespace, l.App, l.Pod, l.Container, l.Host, l.FieldsString, l.FieldsNumber, l.Log)

		if err != nil {
			slog.Error("Statement exec failure", slog.Any("error", err))
			return err
		}
	}

	if err = tx.Commit(); err != nil {
		slog.Error("Commit failure", slog.Any("error", err))
		return err
	}

	c.buffer = make([]Row, 0)
	return nil
}

// Close can be used to close the underlying sql client for ClickHouse.
func (c *Client) Close() error {
	return c.client.Close()
}

// NewClient returns a new client for ClickHouse. The client can then be used to
// write data to ClickHouse via the "Write" method.
func NewClient(address, username, password, database, dialTimeout, connMaxLifetime string, maxIdleConns, maxOpenConns int, asyncInsert, waitForAsyncInsert bool) (*Client, error) {
	parsedDialTimeout, err := time.ParseDuration(dialTimeout)
	if err != nil {
		return nil, err
	}

	parsedConnMaxLifetime, err := time.ParseDuration(connMaxLifetime)
	if err != nil {
		return nil, err
	}

	conn := clickhouse.OpenDB(&clickhouse.Options{
		Addr: strings.Split(address, ","),
		Auth: clickhouse.Auth{
			Database: database,
			Username: username,
			Password: password,
		},
		DialTimeout: parsedDialTimeout,
	})
	conn.SetMaxIdleConns(maxIdleConns)
	conn.SetMaxOpenConns(maxOpenConns)
	conn.SetConnMaxLifetime(parsedConnMaxLifetime)

	if err := conn.PingContext(context.Background()); err != nil {
		if exception, ok := err.(*clickhouse.Exception); ok {
			slog.Error(fmt.Sprintf("[%d] %s \n%s\n", exception.Code, exception.Message, exception.StackTrace))
		} else {
			slog.Error("Failed to ping database", slog.Any("error", err))
		}

		return nil, err
	}

	return &Client{
		client:             conn,
		database:           database,
		asyncInsert:        asyncInsert,
		waitForAsyncInsert: waitForAsyncInsert,
		bufferMutex:        &sync.RWMutex{},
		buffer:             make([]Row, 0),
	}, nil
}
