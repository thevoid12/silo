.PHONY: build clean test desktop desktop-dev

BINARY_NAME=silo
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE=$(shell date -u +%Y-%m-%d)
LDFLAGS=-ldflags "-s -w -X silo/pkg/cli.commit=$(COMMIT) -X silo/pkg/cli.buildDate=$(BUILD_DATE)"

build:
	go build $(LDFLAGS) -o $(BINARY_NAME) .

clean:
	rm -f $(BINARY_NAME)
	rm -rf desktop/out desktop/dist

test:
	go test ./...

# desktop: build Go binary then package the Electron app
desktop: build
	cp $(BINARY_NAME) desktop/resources/sidecar/$(BINARY_NAME)
	cd desktop && npm run package

# desktop-dev: run Electron in dev mode against an already-running silo server
# Usage: SILO_DEV_PORT=5110 SILO_DEV_TOKEN=<token> make desktop-dev
desktop-dev:
	cd desktop && npm run dev
