FROM golang:1.16.5-alpine3.14 as build
ARG TARGETPLATFORM
ARG BUILDPLATFORM
RUN echo "Building on $BUILDPLATFORM, for $TARGETPLATFORM" > /log
RUN apk update && apk add git make
WORKDIR /fluent-bit-kafka-clickhouse
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN make build-fluent-bit-kafka-clickhouse

FROM alpine:3.14
RUN apk update && apk add --no-cache ca-certificates
RUN mkdir /fluent-bit-kafka-clickhouse
COPY --from=build /fluent-bit-kafka-clickhouse/fluent-bit-kafka-clickhouse /fluent-bit-kafka-clickhouse
WORKDIR /fluent-bit-kafka-clickhouse
USER nobody
ENTRYPOINT  [ "/fluent-bit-kafka-clickhouse/fluent-bit-kafka-clickhouse" ]
