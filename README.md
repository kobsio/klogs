# klogs

**klogs** can be used to write the logs collected by
[Fluent Bit](https://fluentbit.io) to [ClickHouse](https://clickhouse.com).

You can use [kobs](https://kobs.io) as interface to get the logs from
ClickHouse. More information regarding the klogs plugin for kobs can be found in
the [klogs](https://kobs.io/plugins/klogs/) documentation of kobs.

![kobs](./assets/kobs.png)

## Configuration

An example configuration file can be found in the
[fluent-bit.yaml](./cluster/fluent-bit.yaml) file. The following options are
available:

| Option                   | Description                                                                                                     | Default   |
| ------------------------ | --------------------------------------------------------------------------------------------------------------- | --------- |
| `Metrics_Server_Address` | The address, where the metrics server should listen on.                                                         | `:2021`   |
| `Address`                | The address, where ClickHouse is listining on, e.g. `clickhouse-clickhouse.kube-system.svc.cluster.local:9000`. |           |
| `Database`               | The name of the database for the logs.                                                                          | `logs`    |
| `Username`               | The username, to authenticate to ClickHouse.                                                                    |           |
| `Password`               | The password, to authenticate to ClickHouse.                                                                    |           |
| `Dial_Timeout`           | ClickHouse dial timeout.                                                                                        | `10s`     |
| `Conn_Max_Lifetime`      | ClickHouse maximum connection lifetime.                                                                         | `1h`      |
| `Max_Idle_Conns`         | ClickHouse maximum number of idle connections.                                                                  | `1`       |
| `Max_Open_Conns`         | ClickHouse maximum number of open connections.                                                                  | `1`       |
| `Async_Insert`           | Use async inserts to write logs into ClickHouse.                                                                | `false`   |
| `Wait_For_Async_Insert`  | Wait for the async insert operation.                                                                            | `false`   |
| `Batch_Size`             | The size for how many log lines should be buffered, before they are written to ClickHouse.                      | `10000`   |
| `Flush_Interval`         | The maximum amount of time to wait, before logs are written to ClickHouse.                                      | `60s`     |
| `Force_Number_Fields`    | A list of fields which should be parsed as number.                                                              | `60s`     |
| `Force_Underscores`      | Replace all `.` with `_` in keys.                                                                               | `false`   |
| `Log_Format`             | The log format for the Fluent Bit ClickHouse plugin. Must be `console` or `json`.                               | `console` |
| `Log_Level`              | The log level for the Fluent Bit ClickHouse plugin. Must be `DEBUG`, `INFO`, `WARN` or `ERROR`.                 | `INFO`    |

The SQL schema for ClickHouse must be created on each ClickHouse node and looks
as follows:

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

To speedup queries for the most frequently queried fields we can create
dedicated columns for specific fiels:

```sql
ALTER TABLE logs.logs_local ON CLUSTER '{cluster}' ADD COLUMN content_level String DEFAULT fields_string['content.level']
ALTER TABLE logs.logs ON CLUSTER '{cluster}' ADD COLUMN content_level String DEFAULT fields_string['content.level']

ALTER TABLE logs.logs_local ON CLUSTER '{cluster}' ADD COLUMN content_response_code Float64 DEFAULT fields_number['content.response_code']
ALTER TABLE logs.logs ON CLUSTER '{cluster}' ADD COLUMN content_response_code Float64 DEFAULT fields_number['content.response_code']
```

But those columns will be materialized only for new data and after merges. In
order to materialize those columns for old data:

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

## Development

We are using [kind](https://kind.sigs.k8s.io/docs/user/quick-start/) for local
development. To create a new Kubernetes cluster using kind you can run the
`cluster/cluster.sh` script, which will create such a cluster with a Docker
registry:

```sh
./cluster/cluster.sh
```

Once the cluster is running we can build and push the Docker image for Fluent
Bit:

```sh
docker build -f Dockerfile -t localhost:5001/klogs:latest .
docker push localhost:5001/klogs:latest
```

In the next step we have to create our ClickHouse cluster via the
[ClickHouse Operator](https://github.com/Altinity/clickhouse-operator):

```sh
kubectl apply -f https://raw.githubusercontent.com/Altinity/clickhouse-operator/refs/heads/master/deploy/operator/clickhouse-operator-install-bundle.yaml
kubectl apply -f ./cluster/clickhouse.yaml
```

Once ClickHouse is running we can to connect to the instance to check if the
database schema was created:

```sh
kubectl exec -n kube-system -it chi-clickhouse-example-0-0-0 -c clickhouse -- clickhouse-client -h 127.0.0.1
```

```sql
SHOW DATABASES;
USE logs;
SHOW TABLES;
DESCRIBE logs_local;
DESCRIBE logs;
```

Now we can deploy Fluent Bit to ingest all logs into ClickHouse:

```sh
kubectl apply -f ./cluster/fluent-bit.yaml
kubectl logs -n kube-system -l app=fluent-bit -f
```

To check if the logs are arriving in ClickHouse you can use the following SQL
commands:

```sql
SELECT count(*) FROM logs.logs;
SELECT * FROM logs.logs LIMIT 10;

SELECT count(*) FROM logs.logs_local;
SELECT * FROM logs.logs_local LIMIT 10;
```

To clean up all the created resources run the following commands:

```sh
kind delete cluster
docker stop kind-registry
docker rm kind-registry
```
