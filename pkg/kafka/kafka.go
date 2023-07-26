package kafka

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/kobsio/klogs/pkg/clickhouse"
	"github.com/kobsio/klogs/pkg/log"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

// Run creates a new client for the given Kafka configuration and listens for incomming messages. These messages are
// then written to ClickHouse when the batch size or flush interval is over.
func Run(kafkaBrokers, kafkaGroup, kafkaVersion, kafkaTopics, kafkaTimestampKey string, clickhouseBatchSize int64, clickhouseFlushInterval time.Duration, clickhouseForceNumberFields []string, clickhouseForceUnderscores bool, clickhouseClient *clickhouse.Client) {
	version, err := sarama.ParseKafkaVersion(kafkaVersion)
	if err != nil {
		log.Fatal(nil, "Error parsing Kafka version", zap.Error(err))
	}

	config := sarama.NewConfig()
	config.Version = version
	config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRange
	config.Consumer.Offsets.Initial = sarama.OffsetOldest

	// Create a new consumer, which handles all incomming messages from Kafka and writes the messages to ClickHouse.
	consumer := Consumer{
		ready:                       make(chan bool),
		timestampKey:                kafkaTimestampKey,
		lastFlush:                   time.Now(),
		clickhouseBatchSize:         clickhouseBatchSize,
		clickhouseFlushInterval:     clickhouseFlushInterval,
		clickhouseForceNumberFields: clickhouseForceNumberFields,
		clickhouseForceUnderscores:  clickhouseForceUnderscores,
		clickhouseClient:            clickhouseClient,
	}

	ctx, cancel := context.WithCancel(context.Background())
	client, err := sarama.NewConsumerGroup(strings.Split(kafkaBrokers, ","), kafkaGroup, config)
	if err != nil {
		log.Fatal(nil, "Error creating consumer group client", zap.Error(err))
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			// `Consume` should be called inside an infinite loop, when a server-side rebalance happens, the consumer
			// session will need to be recreated to get the new claims.
			if err := client.Consume(ctx, strings.Split(kafkaTopics, ","), &consumer); err != nil {
				log.Fatal(nil, "Error from consumer", zap.Error(err))
			}
			// Check if context was cancelled, signaling that the consumer should stop.
			if ctx.Err() != nil {
				return
			}
			consumer.ready = make(chan bool)
		}
	}()

	<-consumer.ready
	log.Info(nil, "Sarama consumer up and running")

	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-ctx.Done():
		log.Info(nil, "Terminating: context cancelled")
	case <-sigterm:
		log.Info(nil, "Terminating: via signal")
	}
	cancel()
	wg.Wait()
	if err = client.Close(); err != nil {
		log.Fatal(nil, "Error closing client", zap.Error(err))
	}
}
