.PHONY: build clean

BINARY_NAME=silo
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE=$(shell date -u +%Y-%m-%d)
LDFLAGS=-ldflags "-s -w -X silo/pkg/cli.commit=$(COMMIT) -X silo/pkg/cli.buildDate=$(BUILD_DATE)"

build:
	go build $(LDFLAGS) -o $(BINARY_NAME) .

clean:
	rm -f $(BINARY_NAME)
