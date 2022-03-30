FROM golang:1.17.3-alpine3.14 as build
RUN apk update && apk add git make
WORKDIR /filebeat
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN export CGO_ENABLED=0 && make build-filebeat

FROM docker.elastic.co/beats/filebeat:7.17.1
COPY --from=build /filebeat/filebeat /usr/share/filebeat
