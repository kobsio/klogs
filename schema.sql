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
