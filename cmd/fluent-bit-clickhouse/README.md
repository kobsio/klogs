# Fluent Bit -> ClickHouse

The Fluent Bit output plugin for ClickHouse can be used to write the collected logs from Fluent Bit into ClickHouse.

![Fluent Bit -> ClickHouse](../../assets/fluent-bit-clickhouse.png)

## Configuration

An example configuration file can be found in the [fluent-bit-cm.yaml](../../cluster/fluent-bit/clickhouse/fluent-bit-cm.yaml) ConfigMap. The following options are available:

| Option | Description | Default |
| ------ | ----------- | ------- |
| `Address` | The address, where ClickHouse is listining on, e.g. `clickhouse-clickhouse.clickhouse.svc.cluster.local:9000`. | |
| `Database` | The name of the database for the logs. | `logs` |
| `Username` | The username, to authenticate to ClickHouse. | |
| `Password` | The password, to authenticate to ClickHouse. | |
| `Write_Timeout` | The write timeout for ClickHouse. | `10` |
| `Read_Timeout` | The read timeout for ClickHouse. | `10` |
| `Batch_Size` | The size for how many log lines should be buffered, before they are written to ClickHouse. | `10000` |
| `Flush_Interval` | The maximum amount of time to wait, before logs are written to ClickHouse. | `60s` |
| `Log_Format` | The log format for the Fluent Bit ClickHouse plugin. Must be `plain` or `json`. | `plain` |

## Development

We are using [kind](https://kind.sigs.k8s.io/docs/user/quick-start/) for local development. To create a new Kubernetes cluster using kind you can run the `cluster/cluster.sh` script, which will create such a cluster with a Docker registry:

```sh
./cluster/cluster.sh
```

Once the cluster is running we can build and push the Docker image for Fluent Bit:

```sh
docker build -f cmd/fluent-bit-clickhouse/Dockerfile -t localhost:5000/fluent-bit-clickhouse:latest .
docker push localhost:5000/fluent-bit-clickhouse:latest

# To run the Docker image locally, the following command can be used:
docker run -it --rm localhost:5000/fluent-bit-clickhouse:latest
```

In the next step we have to create our ClickHouse cluster via the [ClickHouse Operator](https://github.com/Altinity/clickhouse-operator). To do that we can deploy all the files from the `cluster/clickhouse-operator` and `cluster/clickhouse` folder:

```sh
k apply -f cluster/clickhouse-operator
k apply -f cluster/clickhouse
```

Once ClickHouse is running we have to connect to the two ClickHouse nodes to create our SQL schema. The schema can be found in the  [`schema.sql`](../../schema.sql) file, just execute each SQL command one by one on both ClickHouse nodes:

```sh
k exec -n clickhouse -it chi-clickhouse-sharded-0-0-0 -c clickhouse -- clickhouse-client
k exec -n clickhouse -it chi-clickhouse-sharded-1-0-0 -c clickhouse -- clickhouse-client
```

Now we can deploy Fluent Bit to ingest all logs into ClickHouse:

```sh
k apply -f cluster/fluent-bit/clickhouse
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
