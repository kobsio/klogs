FROM golang:1.25.3 AS build
WORKDIR /root
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN make build

FROM fluent/fluent-bit:4.1.1
COPY --from=build /root/out_clickhouse.so /fluent-bit/bin/
EXPOSE 2020
CMD ["/fluent-bit/bin/fluent-bit", "--plugin", "/fluent-bit/bin/out_clickhouse.so", "--config", "/fluent-bit/etc/fluent-bit.conf"]
