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
CREATE DATABASE IF NOT EXISTS logs ON CLUSTER `{cluster}` ENGINE=Atomic;

CREATE TABLE IF NOT EXISTS logs.logs_local ON CLUSTER `{cluster}`
(
    `timestamp` DateTime64(3) CODEC(Delta, LZ4),
    `cluster` LowCardinality(String),
    `namespace` LowCardinality(String),
    `app` LowCardinality(String),
    `pod_name` LowCardinality(String),
    `container_name` LowCardinality(String),
    `host` LowCardinality(String),
    `fields_string` Map(LowCardinality(String), String),
    `fields_number` Map(LowCardinality(String), Float64),
    `log` String CODEC(ZSTD(1))
)
ENGINE = ReplicatedMergeTree
PARTITION BY toDate(timestamp)
ORDER BY (cluster, namespace, app, pod_name, container_name, host, timestamp)
TTL toDateTime(timestamp) + INTERVAL 30 DAY;

CREATE TABLE IF NOT EXISTS logs.logs ON CLUSTER '{cluster}' AS logs.logs_local ENGINE = Distributed('{cluster}', logs, logs_local, rand());
```

To speedup queries for the most frequently queried fields we can create dedicated columns for specific fiels:

```sql
ALTER TABLE logs.logs_local ON CLUSTER '{cluster}' ADD COLUMN content_level String DEFAULT fields_string['content.level']
ALTER TABLE logs.logs ON CLUSTER '{cluster}' ADD COLUMN content_level String DEFAULT fields_string['content.level']

ALTER TABLE logs.logs_local ON CLUSTER '{cluster}' ADD COLUMN content_response_code Float64 DEFAULT fields_number['content.response_code']
ALTER TABLE logs.logs ON CLUSTER '{cluster}' ADD COLUMN content_response_code Float64 DEFAULT fields_number['content.response_code']
```

But those columns will be materialized only for new data and after merges. In order to materialize those columns for old data:

- You can use `ALTER TABLE MATERIALIZE COLUMN` for ClickHouse version > 21.10.

```sql
ALTER TABLE logs.logs_local ON CLUSTER '{cluster}' MATERIALIZE COLUMN content_level;
ALTER TABLE logs.logs_local ON CLUSTER '{cluster}' MATERIALIZE COLUMN content_response_code;
```

- Or for older ClickHouse versions, `ALTER TABLE UPDATE`.

```sql
ALTER TABLE logs.logs_local ON CLUSTER '{cluster}' UPDATE content_level = content_level WHERE 1;
ALTER TABLE logs.logs_local ON CLUSTER '{cluster}' UPDATE content_response_code = content_response_code WHERE 1;
```
