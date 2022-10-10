FROM golang:1.19.2-alpine3.16 as build
RUN apk update && apk add git make
WORKDIR /ingester
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN make build-ingester

FROM alpine:3.14.2
RUN apk update && apk add --no-cache ca-certificates
RUN mkdir /ingester
COPY --from=build /ingester/ingester /ingester
WORKDIR /ingester
USER nobody
ENTRYPOINT  [ "/ingester/ingester" ]
