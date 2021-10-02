# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

NOTE: As semantic versioning states all 0.y.z releases can contain breaking changes in API (flags, grpc API, any backward compatibility). We use :warning: *Breaking change:* :warning: to mark changes that are not backward compatible (relates only to v0.y.z releases).

## Unreleased

- [#4](https://github.com/kobsio/fluent-bit-clickhouse/pull/4): Use logrus for logging and remove conf directory.

## [v0.4.0](https://github.com/kobsio/fluent-bit-clickhouse/releases/tag/v0.4.0) (2021-09-08)

- [#2](https://github.com/kobsio/fluent-bit-clickhouse/pull/2): Add changelog.
- [#3](https://github.com/kobsio/fluent-bit-clickhouse/pull/3): Add flush interval setting, so that the buffer is flushed after the defined interval or when the buffer size is reached.

## [v0.3.0](https://github.com/kobsio/fluent-bit-clickhouse/releases/tag/v0.3.0) (2021-09-03)

- [#1](https://github.com/kobsio/fluent-bit-clickhouse/pull/1): Add option to get user information from a request.

## [v0.2.0](https://github.com/kobsio/fluent-bit-clickhouse/releases/tag/v0.2.0) (2021-08-28)

- [082ae83](https://github.com/kobsio/fluent-bit-clickhouse/commit/082ae831865160a0c2884aea900384c6535cbcea): Update schema for ClickHouse.

## [v0.1.0](https://github.com/kobsio/fluent-bit-clickhouse/releases/tag/v0.1.0) (2021-08-27)

- [29b67eb](https://github.com/kobsio/fluent-bit-clickhouse/commit/29b67eb4f3088387d8fb52798e36cc8686a7da36): Initial version of the Fluent Bit ClickHouse plugin.
