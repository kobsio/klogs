# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

NOTE: As semantic versioning states all 0.y.z releases can contain breaking changes in API (flags, grpc API, any backward compatibility). We use :warning: *Breaking change:* :warning: to mark changes that are not backward compatible (relates only to v0.y.z releases).

## Unreleased

- [#15](https://github.com/kobsio/klogs/pull/15): Update the used SQL schema to use `ReplicatedMergeTree` instead of `MergeTree`.

## [v0.6.0](https://github.com/kobsio/klogs/releases/tag/v0.6.0) (2021-11-19)

- [#11](https://github.com/kobsio/klogs/pull/11): Update Fluent Bit to version 1.8.9.
- [#12](https://github.com/kobsio/klogs/pull/12): Add support for [async inserts](https://clickhouse.com/blog/en/2021/clickhouse-v21.11-released/#async-inserts).
- [#13](https://github.com/kobsio/klogs/pull/13): Add GitHub Action to run tests and build on every push.
- [#14](https://github.com/kobsio/klogs/pull/14): Rename repository to `klogs`. The Docker images were also renamed and are available at [kobsio/klogs](https://hub.docker.com/r/kobsio/klogs).

## [v0.5.2](https://github.com/kobsio/klogs/releases/tag/v0.5.2) (2021-10-21)

- [#10](https://github.com/kobsio/klogs/pull/10): Update Fluent Bit to version 1.8.8.

## [v0.5.1](https://github.com/kobsio/klogs/releases/tag/v0.5.1) (2021-10-14)

- [#9](https://github.com/kobsio/klogs/pull/9): Improve parsing of number fields.

## [v0.5.0](https://github.com/kobsio/klogs/releases/tag/v0.5.0) (2021-10-03)

- [#4](https://github.com/kobsio/klogs/pull/4): Use logrus for logging and remove conf directory.
- [#5](https://github.com/kobsio/klogs/pull/5): Add support for Kafka, so that Fluent Bit writes all logs to Kafka and we then write the logs from Kafka to ClickHouse.
- [#6](https://github.com/kobsio/klogs/pull/6): Use consistent naming.
- [#7](https://github.com/kobsio/klogs/pull/7): Adjust documentation.
- [#8](https://github.com/kobsio/klogs/pull/8): Make timestamp key configurable.

## [v0.4.0](https://github.com/kobsio/klogs/releases/tag/v0.4.0) (2021-09-08)

- [#2](https://github.com/kobsio/klogs/pull/2): Add changelog.
- [#3](https://github.com/kobsio/klogs/pull/3): Add flush interval setting, so that the buffer is flushed after the defined interval or when the buffer size is reached.

## [v0.3.0](https://github.com/kobsio/klogs/releases/tag/v0.3.0) (2021-09-03)

- [#1](https://github.com/kobsio/klogs/pull/1): Add option to get user information from a request.

## [v0.2.0](https://github.com/kobsio/klogs/releases/tag/v0.2.0) (2021-08-28)

- [082ae83](https://github.com/kobsio/klogs/commit/082ae831865160a0c2884aea900384c6535cbcea): Update schema for ClickHouse.

## [v0.1.0](https://github.com/kobsio/klogs/releases/tag/v0.1.0) (2021-08-27)

- [29b67eb](https://github.com/kobsio/klogs/commit/29b67eb4f3088387d8fb52798e36cc8686a7da36): Initial version of the Fluent Bit ClickHouse plugin.
