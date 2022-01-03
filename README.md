# klogs

**klogs** can be used to write the logs collected by [Fluent Bit](https://fluentbit.io) to [ClickHouse](https://clickhouse.tech). You can use klogs with or without Kafka:

- **[Fluent Bit Plugin](cmd/plugin):** The klogs Fluent Bit plugin allows you to directly write the collected logs from Fluent Bit into ClickHouse.
- **[ClickHouse Ingester](cmd/ingester):** The klogs ClickHouse ingester allows you to write your logs from Fluent Bit into Kafka, so that the ingester can write them from Kafka into ClickHouse.

You can use [kobs](https://kobs.io) as interface to get the logs from ClickHouse. More information regarding the klogs plugin for kobs can be found in the [klogs](https://kobs.io/plugins/klogs/) documentation of kobs.

![kobs](assets/kobs.png)

## Configuration

The configuration for the **Fluent Bit Plugin** and **ClickHouse Ingester** can be found in the corresponding directories in the `cmd` folder.

The SQL schema for ClickHouse must be created on each ClickHouse node and looks as follows:

```sql
CREATE DATABASE IF NOT EXISTS logs ENGINE=Atomic;

CREATE TABLE IF NOT EXISTS logs.logs_local ON CLUSTER '{cluster}' (
  timestamp DateTime64(3) CODEC (Delta, ZSTD(1)),
  cluster LowCardinality(String) CODEC (ZSTD(1)),
  namespace LowCardinality(String) CODEC (ZSTD(1)),
  app String CODEC (ZSTD(1)),
  pod_name String CODEC (ZSTD(1)),
  container_name String CODEC (ZSTD(1)),
  host String CODEC (ZSTD(1)),
  fields_string Nested(key String, value String) CODEC (ZSTD(1)),
  fields_number Nested(key String, value Float64) CODEC (ZSTD(1)),
  log String CODEC (ZSTD(1))
) ENGINE = ReplicatedMergeTree()
  TTL toDateTime(timestamp) + INTERVAL 30 DAY DELETE
  PARTITION BY toDate(timestamp)
  ORDER BY (cluster, namespace, app, pod_name, container_name, host, -toUnixTimestamp(timestamp));

CREATE TABLE IF NOT EXISTS logs.logs ON CLUSTER '{cluster}' AS logs.logs_local ENGINE = Distributed('{cluster}', logs, logs_local, rand());
```

To speedup queries for the most frequently queried fields we can materializing them to dedicated columns:

```sql
ALTER TABLE logs.logs_local ON CLUSTER '{cluster}' ADD COLUMN content.level String DEFAULT fields_string.value[indexOf(fields_string.key, 'content.level')]
ALTER TABLE logs.logs ON CLUSTER '{cluster}' ADD COLUMN content.level String DEFAULT fields_string.value[indexOf(fields_string.key, 'content.level')]

ALTER TABLE logs.logs_local ON CLUSTER '{cluster}' ADD COLUMN content.response_code Float64 DEFAULT fields_number.value[indexOf(fields_number.key, 'content.response_code')]
ALTER TABLE logs.logs ON CLUSTER '{cluster}' ADD COLUMN content.response_code Float64 DEFAULT fields_number.value[indexOf(fields_number.key, 'content.response_code')]
```
