FROM golang:1.16.5-alpine3.14 as build
ARG TARGETPLATFORM
ARG BUILDPLATFORM
RUN echo "Building on $BUILDPLATFORM, for $TARGETPLATFORM" > /log
RUN apk update && apk add git make
WORKDIR /kafka-clickhouse
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN make build-kafka-clickhouse

FROM alpine:3.14
RUN apk update && apk add --no-cache ca-certificates
RUN mkdir /kafka-clickhouse
COPY --from=build /kafka-clickhouse/kafka-clickhouse /kafka-clickhouse
WORKDIR /kafka-clickhouse
USER nobody
ENTRYPOINT  [ "/kafka-clickhouse/kafka-clickhouse" ]
