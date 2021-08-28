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
    Name          clickhouse
    Match         *
    Address       clickhouse-clickhouse.clickhouse.svc.cluster.local:9000
    Database      logs
    Username      admin
    Password      admin
    Write_Timeout 20
    Read_Timeout  10
    Batch_Size    10000
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

## Development

We are using [kind](https://kind.sigs.k8s.io/docs/user/quick-start/) for local development. To create a new Kubernetes cluster using kind you can run the `cluster/cluster.sh` script, which will create such a cluster with a Docker registry:

```sh
./cluster/cluster.sh
```

Once the cluster is running we can build and push the Docker image for Fluent Bit:

```sh
docker build -f cmd/out_clickhouse/Dockerfile -t localhost:5000/fluent-bit-clickhouse:latest .
docker push localhost:5000/fluent-bit-clickhouse:latest

# To run the Docker image locally, the following command can be used:
docker run -it --rm localhost:5000/fluent-bit-clickhouse:latest
```

In the next step we have to create our ClickHouse cluster via the [ClickHouse Operator](https://github.com/Altinity/clickhouse-operator). To do that we can deploy all the files from the `cluster/clickhouse-operator` and `cluster/clickhouse` folder:

```sh
k apply -f cluster/clickhouse-operator
k apply -f cluster/clickhouse
```

Once ClickHouse is running we have to connect to the two ClickHouse nodes to create our SQL schema. The schema can be found in the `schema.sql` file, just execute each SQL command one by one on both ClickHouse nodes:

```sh
k exec -n clickhouse -it chi-clickhouse-sharded-0-0-0 -c clickhouse -- clickhouse-client
k exec -n clickhouse -it chi-clickhouse-sharded-1-0-0 -c clickhouse -- clickhouse-client
```

Now we can deploy Fluent Bit to ingest all logs into ClickHouse:

```sh
k apply -f cluster/fluent-bit
k logs -n fluent-bit -l app=fluent-bit -f
```

To check if the logs are arriving in ClickHouse you can use the following SQL commands:

```sql
SELECT count(*) FROM logs.logs;
SELECT * FROM logs.logs LIMIT 10;

SELECT count(*) FROM logs.logs_local;
SELECT * FROM logs.logs_local LIMIT 10;
```

To clean up all the created resources run the following commands:

```sh
kind delete cluster --name fluent-bit-clickhouse
docker stop kind-registry
docker rm kind-registry
```
