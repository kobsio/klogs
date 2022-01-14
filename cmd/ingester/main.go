package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kobsio/fluent-bit-clickhouse/pkg/clickhouse"
	"github.com/kobsio/fluent-bit-clickhouse/pkg/kafka"
	"github.com/kobsio/fluent-bit-clickhouse/pkg/log"
	"github.com/kobsio/fluent-bit-clickhouse/pkg/version"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	flag "github.com/spf13/pflag"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	clickhouseAddress            string
	clickhouseUsername           string
	clickhousePassword           string
	clickhouseDatabase           string
	clickhouseWriteTimeout       string
	clickhouseReadTimeout        string
	clickhouseAsyncInsert        bool
	clickhouseWaitForAsyncInsert bool
	clickhouseBatchSize          int64
	clickhouseFlushInterval      time.Duration
	clickhouseForceNumberFields  []string
	kafkaBrokers                 string
	kafkaGroup                   string
	kafkaVersion                 string
	kafkaTopics                  string
	kafkaTimestampKey            string
	logFormat                    string
	logLevel                     string
	showVersion                  bool
)

// init is used to set the defaults for all configuration parameters and to set all flags and environment variables, for
// the ClickHouse, Kafka and logging configuration.
func init() {
	defaultCickhouseAddress := ""
	if os.Getenv("CLICKHOUSE_ADDRESS") != "" {
		defaultCickhouseAddress = os.Getenv("CLICKHOUSE_ADDRESS")
	}

	defaultClickHouseUsername := ""
	if os.Getenv("CLICKHOUSE_USERNAME") != "" {
		defaultClickHouseUsername = os.Getenv("CLICKHOUSE_USERNAME")
	}

	defaultClickHousePassword := ""
	if os.Getenv("CLICKHOUSE_PASSWORD") != "" {
		defaultClickHousePassword = os.Getenv("CLICKHOUSE_PASSWORD")
	}

	defaultClickHouseDatabase := "logs"
	if os.Getenv("CLICKHOUSE_DATABASE") != "" {
		defaultClickHouseDatabase = os.Getenv("CLICKHOUSE_DATABASE")
	}

	defaultClickHouseWriteTimeout := "10"
	if os.Getenv("CLICKHOUSE_WRITE_TIMEOUT") != "" {
		defaultClickHouseWriteTimeout = os.Getenv("CLICKHOUSE_WRITE_TIMEOUT")
	}

	defaultClickHouseReadTimeout := "10"
	if os.Getenv("CLICKHOUSE_READ_TIMEOUT") != "" {
		defaultClickHouseReadTimeout = os.Getenv("CLICKHOUSE_READ_TIMEOUT")
	}

	defaultClickHouseAsyncInsert := false
	if os.Getenv("CLICKHOUSE_ASYNC_INSERT") != "" {
		defaultClickHouseAsyncInsertString := os.Getenv("CLICKHOUSE_ASYNC_INSERT")
		defaultClickHouseAsyncInsertParsed, err := strconv.ParseBool(defaultClickHouseAsyncInsertString)
		if err != nil {
			defaultClickHouseAsyncInsert = defaultClickHouseAsyncInsertParsed
		}
	}

	defaultClickHouseWaitForAsyncInsert := false
	if os.Getenv("CLICKHOUSE_WAIT_FOR_ASYNC_INSERT") != "" {
		defaultClickHouseWaitForAsyncInsertString := os.Getenv("CLICKHOUSE_WAIT_FOR_ASYNC_INSERT")
		defaultClickHouseWaitForAsyncInsertParsed, err := strconv.ParseBool(defaultClickHouseWaitForAsyncInsertString)
		if err != nil {
			defaultClickHouseWaitForAsyncInsert = defaultClickHouseWaitForAsyncInsertParsed
		}
	}

	defaultClickHouseBatchSize := int64(100000)
	if os.Getenv("CLICKHOUSE_BATCH_SIZE") != "" {
		defaultClickHouseBatchSizeString := os.Getenv("CLICKHOUSE_BATCH_SIZE")
		defaultClickHouseBatchSizeParsed, err := strconv.ParseInt(defaultClickHouseBatchSizeString, 10, 64)
		if err == nil && defaultClickHouseBatchSizeParsed > 0 {
			defaultClickHouseBatchSize = defaultClickHouseBatchSizeParsed
		}
	}

	defaultClickHouseFlushInterval := 60 * time.Second
	if os.Getenv("CLICKHOUSE_FLUSH_INTERVAL") != "" {
		defaultClickHouseFlushIntervalString := os.Getenv("CLICKHOUSE_FLUSH_INTERVAL")
		defaultClickHouseFlushIntervalParsed, err := time.ParseDuration(defaultClickHouseFlushIntervalString)
		if err == nil {
			defaultClickHouseFlushInterval = defaultClickHouseFlushIntervalParsed
		}
	}

	var defaultClickHouseForceNumberFields []string
	if os.Getenv("CLICKHOUSE_FORCE_NUMBER_FIELDS") != "" {
		defaultClickHouseForceNumberFieldsString := os.Getenv("CLICKHOUSE_FORCE_NUMBER_FIELDS")
		defaultClickHouseForceNumberFields = strings.Split(defaultClickHouseForceNumberFieldsString, ",")
	}

	defaultKafkaBrokers := ""
	if os.Getenv("KAFKA_BROKERS") != "" {
		defaultKafkaBrokers = os.Getenv("KAFKA_BROKERS")
	}

	defaultKafkaGroup := "kafka-clickhouse"
	if os.Getenv("KAFKA_GROUP") != "" {
		defaultKafkaGroup = os.Getenv("KAFKA_GROUP")
	}

	defaultKafkaVersion := "2.1.1"
	if os.Getenv("KAFKA_VERSION") != "" {
		defaultKafkaVersion = os.Getenv("KAFKA_VERSION")
	}

	defaultKafkaTopics := "fluent-bit"
	if os.Getenv("KAFKA_TOPICS") != "" {
		defaultKafkaTopics = os.Getenv("KAFKA_TOPICS")
	}

	defaultKafkaTimestampKey := "@timestamp"
	if os.Getenv("KAFKA_TIMESTAMP_KEY") != "" {
		defaultKafkaTimestampKey = os.Getenv("KAFKA_TIMESTAMP_KEY")
	}

	defaultLogFormat := "console"
	if os.Getenv("LOG_FORMAT") != "" {
		defaultLogFormat = os.Getenv("LOG_FORMAT")
	}

	defaultLogLevel := "info"
	if os.Getenv("LOG_LEVEL") != "" {
		defaultLogLevel = os.Getenv("LOG_LEVEL")
	}

	flag.StringVar(&clickhouseAddress, "clickhouse.address", defaultCickhouseAddress, "ClickHouse address to connect to.")
	flag.StringVar(&clickhouseUsername, "clickhouse.username", defaultClickHouseUsername, "ClickHouse username for the connection.")
	flag.StringVar(&clickhousePassword, "clickhouse.password", defaultClickHousePassword, "ClickHouse password for the connection.")
	flag.StringVar(&clickhouseDatabase, "clickhouse.database", defaultClickHouseDatabase, "ClickHouse database name.")
	flag.StringVar(&clickhouseWriteTimeout, "clickhouse.write-timeout", defaultClickHouseWriteTimeout, "ClickHouse write timeout for the connection.")
	flag.StringVar(&clickhouseReadTimeout, "clickhouse.read-timeout", defaultClickHouseReadTimeout, "ClickHouse read timeout for the connection.")
	flag.BoolVar(&clickhouseAsyncInsert, "clickhouse.async-insert", defaultClickHouseAsyncInsert, "Enable async inserts.")
	flag.BoolVar(&clickhouseWaitForAsyncInsert, "clickhouse.wait-for-async-insert", defaultClickHouseWaitForAsyncInsert, "Wait for async inserts.")
	flag.Int64Var(&clickhouseBatchSize, "clickhouse.batch-size", defaultClickHouseBatchSize, "The size for how many log lines should be buffered, before they are written to ClickHouse.")
	flag.DurationVar(&clickhouseFlushInterval, "clickhouse.flush-interval", defaultClickHouseFlushInterval, "The maximum amount of time to wait, before logs are written to ClickHouse.")
	flag.StringArrayVar(&clickhouseForceNumberFields, "clickhouse.force-number-fields", defaultClickHouseForceNumberFields, "A list of fields which should be parsed as number.")

	flag.StringVar(&kafkaBrokers, "kafka.brokers", defaultKafkaBrokers, "Kafka bootstrap brokers to connect to, as a comma separated list.")
	flag.StringVar(&kafkaGroup, "kafka.group", defaultKafkaGroup, "Kafka consumer group definition.")
	flag.StringVar(&kafkaVersion, "kafka.version", defaultKafkaVersion, "Kafka cluster version.")
	flag.StringVar(&kafkaTopics, "kafka.topics", defaultKafkaTopics, "Kafka topics to be consumed, as a comma separated list.")
	flag.StringVar(&kafkaTimestampKey, "kafka.timestamp-key", defaultKafkaTimestampKey, "JSON key where the record timestamp is stored.")

	flag.StringVar(&logFormat, "log.format", defaultLogFormat, "Set the output format of the logs. Must be \"console\" or \"json\".")
	flag.StringVar(&logLevel, "log.level", defaultLogLevel, "Set the log level. Must be \"debug\", \"info\", \"warn\", \"error\", \"fatal\" or \"panic\".")
	flag.BoolVar(&showVersion, "version", false, "Print version information.")
}

func main() {
	flag.Parse()

	// Configure our logging library. The logs can be written in console format (the console format is compatible with
	// logfmt) or in json format. The default is console, because it is better to read during development. In a
	// production environment you should consider to use json, so that the logs can be parsed by a logging system like
	// Elasticsearch.
	// Next to the log format it is also possible to configure the log leven. The accepted values are "debug", "info",
	// "warn", "error", "fatal" and "panic". The default log level is "info".
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

	// When the version value is set to "true" (--version) we will print the version information for kobs. After we
	// printed the version information the application is stopped.
	// The short form of the version information is also printed in two lines, when the version option is set to
	// "false".
	if showVersion {
		v, err := version.Print("kobs")
		if err != nil {
			log.Fatal(nil, "Failed to print version information", zap.Error(err))
		}

		fmt.Fprintln(os.Stdout, v)
		return
	}

	log.Info(nil, "Version information", version.Info()...)
	log.Info(nil, "Build context", version.BuildContext()...)
	log.Info(nil, "Clickhouse configuration", zap.String("clickhouseAddress", clickhouseAddress), zap.String("clickhouseUsername", clickhouseUsername), zap.String("clickhousePassword", "*****"), zap.String("clickhouseDatabase", clickhouseDatabase), zap.String("clickhouseWriteTimeout", clickhouseWriteTimeout), zap.String("clickhouseReadTimeout", clickhouseReadTimeout), zap.Int64("clickhouseBatchSize", clickhouseBatchSize), zap.Duration("clickhouseFlushInterval", clickhouseFlushInterval))
	log.Info(nil, "Kafka configuration", zap.String("kafkaBrokers", kafkaBrokers), zap.String("kafkaGroup", kafkaGroup), zap.String("kafkaVersion", kafkaVersion), zap.String("kafkaTopics", kafkaTopics))

	// Create a http server, which can be used for the liveness and readiness probe in Kubernetes. The server also
	// serves our Prometheus metrics.
	router := http.NewServeMux()
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "OK")
	})
	router.Handle("/metrics", promhttp.Handler())

	server := http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	go server.ListenAndServe()

	// Create a new client for the configured ClickHouse instance. Then pass the ClickHouse client to the Run function
	// of the Kafka package, which listens for message in the configured Kafka instance. These messages are then written
	// to ClickHouse via the created ClickHouse client.
	client, err := clickhouse.NewClient(clickhouseAddress, clickhouseUsername, clickhousePassword, clickhouseDatabase, clickhouseWriteTimeout, clickhouseReadTimeout, clickhouseAsyncInsert, clickhouseWaitForAsyncInsert)
	if err != nil {
		log.Fatal(nil, "Could not create ClickHouse client", zap.Error(err))
	}

	kafka.Run(kafkaBrokers, kafkaGroup, kafkaVersion, kafkaTopics, kafkaTimestampKey, clickhouseBatchSize, clickhouseFlushInterval, clickhouseForceNumberFields, client)
	server.Shutdown(context.Background())
}
