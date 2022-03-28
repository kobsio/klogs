package output

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/kobsio/klogs/pkg/clickhouse"

	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/outputs"
	"github.com/elastic/beats/v7/libbeat/publisher"
	"go.uber.org/zap"
)

type Client struct {
	log                         *logp.Logger
	observer                    outputs.Observer
	cluster                     string
	clickhouseAddress           string
	clickhouseForceNumberFields []string
	clickhouseClient            *clickhouse.Client
}

func (c *Client) Close() error {
	return c.clickhouseClient.Close()
}

func (c *Client) Publish(ctx context.Context, batch publisher.Batch) error {
	st := c.observer
	events := batch.Events()
	st.NewBatch(len(events))

	if len(events) == 0 {
		batch.ACK()
		return nil
	}

	c.log.Infow("Publish events", zap.Int("events", len(events)))

	startTime := time.Now()
	var buffer []clickhouse.Row

	for _, event := range events {
		row := clickhouse.Row{
			Timestamp: event.Content.Timestamp,
			Cluster:   c.cluster,
		}

		fields := event.Content.Fields.Flatten()

		for k, v := range fields {
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
			case "kubernetes.namespace":
				row.Namespace = stringValue
			case "kubernetes.labels.k8s-app":
				row.App = stringValue
			case "kubernetes.labels.app":
				row.App = stringValue
			case "kubernetes.pod.name":
				row.Pod = stringValue
			case "kubernetes.container.name":
				row.Container = stringValue
			case "kubernetes.node.hostname":
				row.Host = stringValue
			case "message":
				row.Log = stringValue
			default:
				if isNumber {
					row.FieldsNumber.Key = append(row.FieldsNumber.Key, k)
					row.FieldsNumber.Value = append(row.FieldsNumber.Value, numberValue)
				} else {
					if contains(k, c.clickhouseForceNumberFields) {
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

		buffer = append(buffer, row)
	}

	err := c.clickhouseClient.Write(buffer)
	if err != nil {
		c.log.Errorw("Could not write events to ClickHouse", zap.Error(err))
		batch.RetryEvents(events)
		st.Failed(len(events))
		return nil
	}

	c.log.Infow("Events were written to ClickHouse", zap.Int("events", len(events)), zap.Int("buffer", len(buffer)), zap.Float64("writeTimeSeconds", time.Now().Sub(startTime).Seconds()))
	batch.ACK()
	st.Acked(len(events))
	return nil
}

func (c *Client) String() string {
	return "clickhouse(" + c.clickhouseAddress + ")"
}

func NewClient(log *logp.Logger, observer outputs.Observer, cluster string, clickhouseAddress string, clickhouseForceNumberFields []string, clickhouseClient *clickhouse.Client) *Client {
	return &Client{
		log:                         log,
		observer:                    observer,
		cluster:                     cluster,
		clickhouseAddress:           clickhouseAddress,
		clickhouseForceNumberFields: clickhouseForceNumberFields,
		clickhouseClient:            clickhouseClient,
	}
}
