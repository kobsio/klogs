# ClickHouse Ingester

The ClickHouse ingester can be used to write logs from Kafka into ClickHouse. To write the logs from Fluent Bit into Kafka the official [Kafka output plugin](https://docs.fluentbit.io/manual/pipeline/outputs/kafka) can be used.

![Ingester](../../assets/ingester.png)

## Configuration

An example Deployment for the ClickHouse ingester can be found in the [ingester.yaml](../../cluster/fluent-bit/ingester/ingester.yaml) file. The following command-line flags and environment variables can be used to configure the ingester:

| Command-Line Flag | Environment Variable | Description | Default |
| ----------------- | -------------------- | ----------- | ------- |
| `--metrics-server.address` | `METRICS_SERVER_ADDRESS` | The address, where the metrics server should listen on. | `:2021` |
| `--clickhouse.address` | `CLICKHOUSE_ADDRESS` | ClickHouse address to connect to. | |
| `--clickhouse.database` | `CLICKHOUSE_DATABASE` | ClickHouse database name. | `logs` |
| `--clickhouse.username` | `CLICKHOUSE_USERNAME` | ClickHouse username for the connection. | |
| `--clickhouse.password` | `CLICKHOUSE_PASSWORD` | ClickHouse password for the connection. | |
| `--clickhouse.dial-timeout` | `CLICKHOUSE_DIAL_TIMEOUT` | ClickHouse dial timeout. | `10s` |
| `--clickhouse.conn-max-lifetime` | `CLICKHOUSE_CONN_MAX_LIFETIME` | ClickHouse maximum connection lifetime. | `1h` |
| `--clickhouse.max-idle-conns` | `CLICKHOUSE_MAX_IDLE_CONNS` | ClickHouse maximum number of idle connections. | `1` |
| `--clickhouse.max-open-conns` | `CLICKHOUSE_MAX_OPEN_CONNS` | ClickHouse maximum number of open connections. | `1` |
| `--clickhouse.async-insert` | `CLICKHOUSE_ASYNC_INSERT` | Enable async inserts. | `false` |
| `--clickhouse.wait-for-async-insert` | `CLICKHOUSE_WAIT_FOR_ASYNC_INSERT` | Wait for async inserts. | `false` |
| `--clickhouse.batch-size` | `CLICKHOUSE_BATCH_SIZE` | The size for how many log lines should be buffered, before they are written to ClickHouse. | `100000` |
| `--clickhouse.flush-interval` | `CLICKHOUSE_FLUSH_INTERVAL` | The maximum amount of time to wait, before logs are written to ClickHouse. | `60s` |
| `--clickhouse.force-number-fields` | `CLICKHOUSE_FORCE_NUMBER_FIELDS` | A list of fields which should be parsed as number. | `[]` |
| `--clickhouse.force-underscores` | `CLICKHOUSE_FORCE_UNDERSCORES` | Replace all `.` with `_` in keys. | `false` |
| `--kafka.brokers` | `KAFKA_BROKERS` | Kafka bootstrap brokers to connect to, as a comma separated list | |
| `--kafka.group` | `KAFKA_GROUP` | Kafka consumer group definition | `kafka-clickhouse` |
| `--kafka.version` | `KAFKA_VERSION` | Kafka cluster version | `2.1.1` |
| `--kafka.topics` | `KAFKA_TOPICS` | Kafka topics to be consumed, as a comma separated list | `fluent-bit` |
| `--log.format` | `LOG_FORMAT` | The log format. Must be `console` or `json`. | `console` |
| `--log.level` | `LOG_LEVEL` | The log level. Must be `debug`, `info`, `warn`, `error`, `fatal` or `panic`. | `info` |

## Development

We are using [kind](https://kind.sigs.k8s.io/docs/user/quick-start/) for local development. To create a new Kubernetes cluster using kind you can run the `cluster/cluster.sh` script, which will create such a cluster with a Docker registry:

```sh
./cluster/cluster.sh
```

Once the cluster is running we can build and push the Docker image for Fluent Bit:

```sh
docker build -f cmd/ingester/Dockerfile -t localhost:5000/klogs:latest-ingester .
docker push localhost:5000/klogs:latest-ingester

# To run the Docker image locally, the following command can be used:
docker run -it --rm localhost:5000/klogs:latest-ingester
```

In the next step we have to create our ClickHouse cluster via the [ClickHouse Operator](https://github.com/Altinity/clickhouse-operator). To do that we can deploy all the files from the `cluster/clickhouse-operator` and `cluster/clickhouse` folder:

```sh
k apply -f cluster/clickhouse-operator
k apply -f cluster/clickhouse
```

Once ClickHouse is running we have to connect to one ClickHouse instance to create our SQL schema. The schema can be found in the [`schema.sql`](../../schema.sql) file, just execute each SQL command one by one on the ClickHouse instance:

```sh
k exec -n clickhouse -it chi-clickhouse-sharded-0-0-0 -c clickhouse -- clickhouse-client
```

Before we can deploy Fluent Bit and the ingester, we have to deploy Kafka using the following command:

```sh
k apply -f cluster/kafka
```

Now we can deploy Fluent Bit to ingest all logs into Kafka and the ingester to write the logs from Kafka into ClickHouse:

```sh
k apply -f cluster/fluent-bit/ingester
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
