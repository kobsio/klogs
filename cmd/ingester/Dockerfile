FROM golang:1.20.6-alpine3.18 as build
RUN apk update && apk add git make
WORKDIR /ingester
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN make build-ingester

FROM alpine:3.17.4
RUN apk update && apk add --no-cache ca-certificates
RUN mkdir /ingester
COPY --from=build /ingester/ingester /ingester
WORKDIR /ingester
USER nobody
ENTRYPOINT  [ "/ingester/ingester" ]
