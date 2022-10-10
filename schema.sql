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
