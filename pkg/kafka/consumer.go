package kafka

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/kobsio/klogs/pkg/clickhouse"
	flatten "github.com/kobsio/klogs/pkg/flatten/string"
	"github.com/kobsio/klogs/pkg/log"
	"github.com/kobsio/klogs/pkg/metrics"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

// Consumer represents a Sarama consumer group consumer.
type Consumer struct {
	ready                       chan bool
	timestampKey                string
	lastFlush                   time.Time
	clickhouseBatchSize         int64
	clickhouseFlushInterval     time.Duration
	clickhouseForceNumberFields []string
	clickhouseForceUnderscores  bool
	clickhouseClient            *clickhouse.Client
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
	// https://github.com/IBM/sarama/blob/main/consumer_group.go#L27-L29
	for message := range claim.Messages() {
		metrics.InputRecordsTotalMetric.Inc()

		var record map[string]interface{}
		record = make(map[string]interface{})

		err := json.Unmarshal(message.Value, &record)
		if err != nil {
			metrics.ErrorsTotalMetric.Inc()
			log.Error(nil, "Could not unmarshal log line", zap.Error(err))
		} else {
			data, err := flatten.Flatten(record)
			if err != nil {
				metrics.ErrorsTotalMetric.Inc()
				log.Error(nil, "Could not flat data", zap.Error(err))
			} else {
				row := clickhouse.Row{
					FieldsString: make(map[string]string),
					FieldsNumber: make(map[string]float64),
				}

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
						case "kubernetes_namespace_name":
							row.Namespace = stringValue
						case "kubernetes_labels_k8s-app":
							row.App = stringValue
						case "kubernetes_labels_app":
							row.App = stringValue
						case "kubernetes_pod_name":
							row.Pod = stringValue
						case "kubernetes_container_name":
							row.Container = stringValue
						case "kubernetes_host":
							row.Host = stringValue
						case "log":
							row.Log = stringValue
						default:
							formattedKey := k
							if consumer.clickhouseForceUnderscores {
								formattedKey = strings.ReplaceAll(k, ".", "_")
							}

							if isNumber {
								row.FieldsNumber[formattedKey] = numberValue
							} else {
								if contains(k, consumer.clickhouseForceNumberFields) {
									parsedNumber, err := strconv.ParseFloat(stringValue, 64)
									if err == nil {
										row.FieldsNumber[formattedKey] = parsedNumber
									} else {
										row.FieldsString[formattedKey] = stringValue
									}
								} else {
									row.FieldsString[formattedKey] = stringValue
								}
							}
						}
					}
				}

				consumer.clickhouseClient.BufferAdd(row)
				session.MarkMessage(message, "")
				startFlushTime := time.Now()
				currentBatchSize := consumer.clickhouseClient.BufferLen()

				if currentBatchSize >= int(consumer.clickhouseBatchSize) || consumer.lastFlush.Add(consumer.clickhouseFlushInterval).Before(time.Now()) {
					err := consumer.clickhouseClient.BufferWrite()
					if err != nil {
						metrics.ErrorsTotalMetric.Inc()
						log.Error(nil, "Could nor write buffer", zap.Error(err))
					} else {
						consumer.lastFlush = time.Now()
						metrics.BatchSizeMetric.Observe(float64(currentBatchSize))
						metrics.FlushTimeSecondsMetric.Observe(consumer.lastFlush.Sub(startFlushTime).Seconds())
					}
				}
			}
		}
	}

	return nil
}
