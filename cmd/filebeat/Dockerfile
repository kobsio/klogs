FROM golang:1.17.3-alpine3.14 as build
RUN apk update && apk add git make
WORKDIR /filebeat
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN export CGO_ENABLED=0 && make build-filebeat

FROM alpine:3.14.2
RUN apk update && apk add --no-cache ca-certificates
RUN mkdir /filebeat
COPY --from=build /filebeat/filebeat /filebeat
WORKDIR /filebeat
USER nobody
ENTRYPOINT  [ "/filebeat/filebeat" ]
