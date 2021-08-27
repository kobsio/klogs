CREATE DATABASE IF NOT EXISTS logs ENGINE=Atomic;

CREATE TABLE IF NOT EXISTS logs.logs_local(
  timestamp DateTime64(3),
  cluster String,
  namespace String,
  app String,
  pod_name String,
  container_name String,
  host String,
  fields_string Nested(key String, value String),
  fields_number Nested(key String, value Float64),
  log String
) ENGINE = MergeTree() PARTITION BY toDate(timestamp) ORDER BY (cluster, -toUnixTimestamp(timestamp), namespace, app, pod_name, container_name, host);

CREATE TABLE IF NOT EXISTS logs.logs(
  timestamp DateTime64(3),
  cluster String,
  namespace String,
  app String,
  pod_name String,
  container_name String,
  host String,
  fields_string Nested(key String, value String),
  fields_number Nested(key String, value Float64),
  log String
) ENGINE = Distributed('{cluster}', logs, logs_local, cityHash64(pod_name));
