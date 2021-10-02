# fluent-bit-clickhouse

The **fluent-bit-clickhouse** plugin can be used to write the logs collected by [Fluent Bit](https://fluentbit.io) to [ClickHouse](https://clickhouse.tech).

You can use [kobs](https://kobs.io) as interface to get the logs from ClickHouse. More information regarding the ClickHouse plugin can be found in the [plugin](https://kobs.io/plugins/clickhouse/) and [configuration](https://kobs.io/configuration/plugins/#clickhouse) documentation of kobs.

![kobs](https://kobs.io/plugins/assets/clickhouse-logs.png)

## Configuration

In the following you found an example configuration for Fluent Bit:

```
[SERVICE]
    Flush        5
    Daemon       Off
    Log_Level    ${LOG_LEVEL}
    HTTP_Server  On
    HTTP_Listen  0.0.0.0
    HTTP_Port    2020
    Parsers_File parsers.conf
    Parsers_File parsers_custom.conf

[INPUT]
    Name              tail
    Path              /var/log/containers/*.log
    Parser            docker
    Tag               kube.*
    Refresh_Interval  5
    Mem_Buf_Limit     20MB
    Skip_Long_Lines   On
    DB                /tail-db/tail-containers-state.db
    DB.Sync           Normal

[FILTER]
    Name                kubernetes
    Match               kube.*
    Kube_Tag_Prefix     kube.var.log.containers.
    Kube_URL            https://kubernetes.default.svc:443
    Kube_CA_File        /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
    Kube_Token_File     /var/run/secrets/kubernetes.io/serviceaccount/token
    Merge_Log           On
    Merge_Log_Key       content
    K8S-Logging.Parser  On
    K8S-Logging.Exclude On

[FILTER]
    Name  modify
    Match *
    Add   cluster fluent-bit-clickhouse

[OUTPUT]
    Name           clickhouse
    Match          *
    Address        clickhouse-clickhouse.clickhouse.svc.cluster.local:9000
    Database       logs
    Username       admin
    Password       admin
    Write_Timeout  20
    Read_Timeout   10
    Batch_Size     10000
    Flush_Interval 1m
```

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
