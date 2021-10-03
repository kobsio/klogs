package kafka

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/kobsio/fluent-bit-clickhouse/pkg/clickhouse"
	flatten "github.com/kobsio/fluent-bit-clickhouse/pkg/flatten/string"

	"github.com/Shopify/sarama"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	flushIntervalMetric = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "kafka_clickhouse",
		Name:      "flush_interval_seconds",
		Help:      "The flush interval describes after which time the messages from Kafka are written to ClickHouse",
	})
	batchSizeMetric = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "kafka_clickhouse",
		Name:      "batch_size_count",
		Help:      "The batch size describes how many messages from Kafka are written to ClickHouse",
	})
)

// Consumer represents a Sarama consumer group consumer.
type Consumer struct {
	ready                   chan bool
	timestampKey            string
	lastFlush               time.Time
	clickhouseBatchSize     int64
	clickhouseFlushInterval time.Duration
	clickhouseClient        *clickhouse.Client
	buffer                  []clickhouse.Row
}

// Setup is run at the beginning of a new session, before ConsumeClaim.
func (consumer *Consumer) Setup(sarama.ConsumerGroupSession) error {
	// Mark the consumer as ready
	close(consumer.ready)
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited.
func (consumer *Consumer) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages().
func (consumer *Consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	// NOTE: Do not move the code below to a goroutine. The `ConsumeClaim` itself is called within a goroutine, see:
	// https://github.com/Shopify/sarama/blob/main/consumer_group.go#L27-L29
	for message := range claim.Messages() {
		var record map[string]interface{}
		record = make(map[string]interface{})

		err := json.Unmarshal(message.Value, &record)
		if err != nil {
			log.WithError(err).Errorf("could not unmarshal log line")
		}

		data, err := flatten.Flatten(record)
		if err != nil {
			log.WithError(err).Errorf("could not flat data")
			break
		}

		row := clickhouse.Row{}

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
			case consumer.timestampKey:
				parsedTime, err := strconv.ParseFloat(value, 64)
				if err != nil {
					log.WithError(err).Warnf("could not parse timestamp")
					row.Timestamp = time.Now()
				} else {
					sec, dec := math.Modf(parsedTime)
					row.Timestamp = time.Unix(int64(sec), int64(dec*(1e9)))
				}
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

		consumer.buffer = append(consumer.buffer, row)
		session.MarkMessage(message, "")

		if len(consumer.buffer) >= int(consumer.clickhouseBatchSize) || consumer.lastFlush.Add(consumer.clickhouseFlushInterval).Before(time.Now()) {
			flushIntervalMetric.Set(time.Now().Sub(consumer.lastFlush).Seconds())
			batchSizeMetric.Set(float64(len(consumer.buffer)))

			err := consumer.clickhouseClient.Write(consumer.buffer)
			if err != nil {
				log.WithError(err).Errorf("could not write buffer")
			} else {
				consumer.buffer = make([]clickhouse.Row, 0)
				consumer.lastFlush = time.Now()
			}
		}
	}

	return nil
}
