FROM golang:1.16.5 as build
WORKDIR /root
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN make build

FROM fluent/fluent-bit:1.7.9
COPY --from=build /root/out_clickhouse.so /fluent-bit/bin/
COPY --from=build /root/conf/fluent-bit.conf /fluent-bit/etc/
COPY --from=build /root/conf/plugins.conf /fluent-bit/etc/
EXPOSE 2020
CMD ["/fluent-bit/bin/fluent-bit", "--plugin", "/fluent-bit/bin/out_clickhouse.so", "--config", "/fluent-bit/etc/fluent-bit.conf"]
