BRANCH    ?= $(shell git rev-parse --abbrev-ref HEAD)
BUILDTIME ?= $(shell date '+%Y-%m-%d@%H:%M:%S')
BUILDUSER ?= $(shell id -un)
REPO      ?= github.com/kobsio/klogs
REVISION  ?= $(shell git rev-parse HEAD)
VERSION   ?= $(shell git describe --tags)

.PHONY: build-plugin
build:
	go build -ldflags "-X ${REPO}/pkg/version.Version=${VERSION} \
		-X ${REPO}/pkg/version.Revision=${REVISION} \
		-X ${REPO}/pkg/version.Branch=${BRANCH} \
		-X ${REPO}/pkg/version.BuildUser=${BUILDUSER} \
		-X ${REPO}/pkg/version.BuildDate=${BUILDTIME}" \
		-buildmode=c-shared -o out_clickhouse.so .;

.PHONY: test
test:
	# Run tests and generate coverage report. To view the coverage report in a
	# browser run "go tool cover -html=coverage.out".
	go test -covermode=atomic -coverpkg=./... -coverprofile=coverage.out -v ./...
