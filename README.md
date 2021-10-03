# fluent-bit-clickhouse

The **fluent-bit-clickhouse** plugin can be used to write the logs collected by [Fluent Bit](https://fluentbit.io) to [ClickHouse](https://clickhouse.tech). You can use the plugin with or without Kafka:

- **[fluent-bit-clickhouse](cmd/fluent-bit-clickhouse):** Use the Fluent Bit ClickHouse output plugin to directly write the collected logs from Fluent Bit to ClickHouse.
- **[fluent-bit-kafka-clickhouse](cmd/fluent-bit-kafka-clickhouse):** Use the Fluent Bit Kafka ClickHouse connector to write the logs from Fluent Bit into Kafka and then from Kafka to ClickHouse. This mode is recommended for larger clusters, to improve the write performance for ClickHouse.

You can use [kobs](https://kobs.io) as interface to get the logs from ClickHouse. More information regarding the ClickHouse plugin can be found in the [plugin](https://kobs.io/plugins/clickhouse/) and [configuration](https://kobs.io/configuration/plugins/#clickhouse) documentation of kobs.

![kobs](assets/kobs.png)

## Configuration

The configuration for the **fluent-bit-clickhouse** and **fluent-bit-kafka-clickhouse** can be found in the corresponding directories in the `cmd` folder.

The SQL schema for ClickHouse must be created on each ClickHouse node and looks as follows:

```sql
CREATE DATABASE IF NOT EXISTS logs ENGINE=Atomic;

CREATE TABLE IF NOT EXISTS logs.logs_local(
  timestamp DateTime64(3) CODEC(Delta, ZSTD(1)),
  cluster LowCardinality(String) CODEC(ZSTD(1)),
  namespace LowCardinality(String) CODEC(ZSTD(1)),
  app String CODEC(ZSTD(1)),
  pod_name String CODEC(ZSTD(1)),
  container_name String CODEC(ZSTD(1)),
  host String CODEC(ZSTD(1)),
  fields_string Nested(key String, value String) CODEC(ZSTD(1)),
  fields_number Nested(key String, value Float64) CODEC(ZSTD(1)),
  log String CODEC(ZSTD(1))
) ENGINE = MergeTree()
  TTL toDateTime(timestamp) + INTERVAL 30 DAY DELETE
  PARTITION BY toDate(timestamp)
  ORDER BY (cluster, namespace, app, pod_name, container_name, host, -toUnixTimestamp(timestamp));

CREATE TABLE IF NOT EXISTS logs.logs AS logs.logs_local ENGINE = Distributed('{cluster}', logs, logs_local, cityHash64(cluster, namespace, app, pod_name, container_name, host));
```
