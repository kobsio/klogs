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
  
CREATE TABLE IF NOT EXISTS logs.logs_local ON CLUSTER `{cluster}`
(
    `timestamp` DateTime64(3) CODEC(Delta, LZ4),
    `cluster` LowCardinality(String),
    `namespace` LowCardinality(String),
    `app` LowCardinality(String),
    `pod_name` LowCardinality(String),
    `container_name` LowCardinality(String),
    `host` LowCardinality(String),
    `fields_string.key` Array(LowCardinality(String)),
    `fields_string.value` Array(String) CODEC(ZSTD(1)),
    `fields_number.key` Array(LowCardinality(String)),
    `fields_number.value` Array(Float64),
    `log` String CODEC(ZSTD(1))
)
ENGINE = ReplicatedMergeTree
PARTITION BY toDate(timestamp)
ORDER BY (cluster, namespace, app, pod_name, container_name, host, timestamp)
TTL toDateTime(timestamp) + INTERVAL 30 DAY

CREATE TABLE IF NOT EXISTS logs.logs ON CLUSTER '{cluster}' AS logs.logs_local ENGINE = Distributed('{cluster}', logs, logs_local, rand());
