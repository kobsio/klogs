package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/kobsio/fluent-bit-clickhouse/pkg/clickhouse"
	"github.com/kobsio/fluent-bit-clickhouse/pkg/kafka"
	"github.com/kobsio/fluent-bit-clickhouse/pkg/version"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
)

var (
	log                     = logrus.WithFields(logrus.Fields{"package": "main"})
	clickhouseAddress       string
	clickhouseUsername      string
	clickhousePassword      string
	clickhouseDatabase      string
	clickhouseWriteTimeout  string
	clickhouseReadTimeout   string
	clickhouseBatchSize     int64
	clickhouseFlushInterval time.Duration
	kafkaBrokers            string
	kafkaGroup              string
	kafkaVersion            string
	kafkaTopics             string
	kafkaTimestampKey       string
	logFormat               string
	logLevel                string
	showVersion             bool
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

	defaultLogFormat := "plain"
	if os.Getenv("LOG_FORMAT") != "" {
		defaultLogFormat = os.Getenv("LOG_FORMAT")
	}

	defaultLogLevel := "info"
	if os.Getenv("LOG_LEVEL") != "" {
		defaultLogLevel = os.Getenv("LOG_LEVEL")
	}

	flag.StringVar(&clickhouseAddress, "clickhouse.address", defaultCickhouseAddress, "ClickHouse address to connect to")
	flag.StringVar(&clickhouseUsername, "clickhouse.username", defaultClickHouseUsername, "ClickHouse username for the connection")
	flag.StringVar(&clickhousePassword, "clickhouse.password", defaultClickHousePassword, "ClickHouse password for the connection")
	flag.StringVar(&clickhouseDatabase, "clickhouse.database", defaultClickHouseDatabase, "ClickHouse database name")
	flag.StringVar(&clickhouseWriteTimeout, "clickhouse.write-timeout", defaultClickHouseWriteTimeout, "ClickHouse write timeout for the connection")
	flag.StringVar(&clickhouseReadTimeout, "clickhouse.read-timeout", defaultClickHouseReadTimeout, "ClickHouse read timeout for the connection")
	flag.Int64Var(&clickhouseBatchSize, "clickhouse.batch-size", defaultClickHouseBatchSize, "The size for how many log lines should be buffered, before they are written to ClickHouse")
	flag.DurationVar(&clickhouseFlushInterval, "clickhouse.flush-interval", defaultClickHouseFlushInterval, "The maximum amount of time to wait, before logs are written to ClickHouse")

	flag.StringVar(&kafkaBrokers, "kafka.brokers", defaultKafkaBrokers, "Kafka bootstrap brokers to connect to, as a comma separated list")
	flag.StringVar(&kafkaGroup, "kafka.group", defaultKafkaGroup, "Kafka consumer group definition")
	flag.StringVar(&kafkaVersion, "kafka.version", defaultKafkaVersion, "Kafka cluster version")
	flag.StringVar(&kafkaTopics, "kafka.topics", defaultKafkaTopics, "Kafka topics to be consumed, as a comma separated list")
	flag.StringVar(&kafkaTimestampKey, "kafka.timestamp-key", defaultKafkaTimestampKey, "JSON key where the record timestamp is stored")

	flag.StringVar(&logFormat, "log.format", defaultLogFormat, "Set the output format of the logs. Must be \"plain\" or \"json\".")
	flag.StringVar(&logLevel, "log.level", defaultLogLevel, "Set the log level. Must be \"trace\", \"debug\", \"info\", \"warn\", \"error\", \"fatal\" or \"panic\".")
	flag.BoolVar(&showVersion, "version", false, "Print version information.")
}

func main() {
	flag.Parse()

	// Configure our logging library. The logs can be written in plain format (the plain format is compatible with
	// logfmt) or in json format. The default is plain, because it is better to read during development. In a production
	// environment you should consider to use json, so that the logs can be parsed by a logging system like
	// Elasticsearch.
	// Next to the log format it is also possible to configure the log leven. The accepted values are "trace", "debug",
	// "info", "warn", "error", "fatal" and "panic". The default log level is "info". When the log level is set to
	// "trace" or "debug" we will also print the caller in the logs.
	if logFormat == "json" {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logrus.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	}

	lvl, err := logrus.ParseLevel(logLevel)
	if err != nil {
		log.WithError(err).WithFields(logrus.Fields{"log.level": logLevel}).Fatal("Could not set log level")
	}
	logrus.SetLevel(lvl)

	if lvl == logrus.TraceLevel || lvl == logrus.DebugLevel {
		logrus.SetReportCaller(true)
	}

	// When the version value is set to "true" (--version) we will print the version information for kobs. After we
	// printed the version information the application is stopped.
	// The short form of the version information is also printed in two lines, when the version option is set to
	// "false".
	if showVersion {
		v, err := version.Print("kobs")
		if err != nil {
			log.WithError(err).Fatalf("Failed to print version information")
		}

		fmt.Fprintln(os.Stdout, v)
		return
	}

	log.WithFields(version.Info()).Infof("Version information")
	log.WithFields(version.BuildContext()).Infof("Build context")
	log.WithFields(logrus.Fields{"clickhouseAddress": clickhouseAddress, "clickhouseUsername": clickhouseUsername, "clickhousePassword": "*****", "clickhouseDatabase": clickhouseDatabase, "clickhouseWriteTimeout": clickhouseWriteTimeout, "clickhouseReadTimeout": clickhouseReadTimeout, "clickhouseBatchSize": clickhouseBatchSize, "clickhouseFlushInterval": clickhouseFlushInterval}).Infof("ClickHouse configuration")
	log.WithFields(logrus.Fields{"kafkaBrokers": kafkaBrokers, "kafkaGroup": kafkaGroup, "kafkaVersion": kafkaVersion, "kafkaTopics": kafkaTopics}).Infof("Kafka configuration")

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
	client, err := clickhouse.NewClient(clickhouseAddress, clickhouseUsername, clickhousePassword, clickhouseDatabase, clickhouseWriteTimeout, clickhouseReadTimeout)
	if err != nil {
		log.WithError(err).Fatalf("could not create ClickHouse client")
	}

	kafka.Run(kafkaBrokers, kafkaGroup, kafkaVersion, kafkaTopics, kafkaTimestampKey, clickhouseBatchSize, clickhouseFlushInterval, client)
	server.Shutdown(context.Background())
}
