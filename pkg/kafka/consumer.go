package kafka

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/kobsio/klogs/pkg/clickhouse"
	flatten "github.com/kobsio/klogs/pkg/flatten/string"
	"github.com/kobsio/klogs/pkg/log"

	"github.com/Shopify/sarama"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
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
	ready                       chan bool
	timestampKey                string
	lastFlush                   time.Time
	clickhouseBatchSize         int64
	clickhouseFlushInterval     time.Duration
	clickhouseForceNumberFields []string
	clickhouseClient            *clickhouse.Client
	buffer                      []clickhouse.Row
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
			log.Error(nil, "Could not unmarshal log line", zap.Error(err))
		}

		data, err := flatten.Flatten(record)
		if err != nil {
			log.Error(nil, "Could not flat data", zap.Error(err))
			break
		}

		row := clickhouse.Row{}

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
				case consumer.timestampKey:
					parsedTime, err := strconv.ParseFloat(stringValue, 64)
					if err != nil {
						log.Warn(nil, "Could not parse timestamp, defaulting to now", zap.Error(err))
						row.Timestamp = time.Now()
					} else {
						sec, dec := math.Modf(parsedTime)
						row.Timestamp = time.Unix(int64(sec), int64(dec*(1e9)))
					}
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
						if contains(k, consumer.clickhouseForceNumberFields) {
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
		}

		consumer.buffer = append(consumer.buffer, row)
		session.MarkMessage(message, "")

		if len(consumer.buffer) >= int(consumer.clickhouseBatchSize) || consumer.lastFlush.Add(consumer.clickhouseFlushInterval).Before(time.Now()) {
			flushIntervalMetric.Set(time.Now().Sub(consumer.lastFlush).Seconds())
			batchSizeMetric.Set(float64(len(consumer.buffer)))

			err := consumer.clickhouseClient.Write(consumer.buffer)
			if err != nil {
				log.Error(nil, "Could nor write buffer", zap.Error(err))
			} else {
				consumer.buffer = make([]clickhouse.Row, 0)
				consumer.lastFlush = time.Now()
			}
		}
	}

	return nil
}
