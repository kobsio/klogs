FROM golang:1.17.3 as build
WORKDIR /root
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN make build-plugin

FROM fluent/fluent-bit:1.9.1
COPY --from=build /root/out_clickhouse.so /fluent-bit/bin/
EXPOSE 2020
CMD ["/fluent-bit/bin/fluent-bit", "--plugin", "/fluent-bit/bin/out_clickhouse.so", "--config", "/fluent-bit/etc/fluent-bit.conf"]
