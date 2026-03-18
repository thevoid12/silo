.PHONY: build clean test desktop desktop-test desktop-dev test-playwright test-playwright-file

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

# desktop: build Go binary then package the Electron app for all targets
desktop: build
	cp $(BINARY_NAME) desktop/resources/sidecar/$(BINARY_NAME)
	cd desktop && bun run package

# desktop-test: build unpacked app for the current arch only (faster, used by test-playwright)
desktop-test: build
	cp $(BINARY_NAME) desktop/resources/sidecar/$(BINARY_NAME)
	rm -rf desktop/dist
	cd desktop && bun run package-test

# desktop-dev: run Electron in dev mode against an already-running silo server
# Usage: SILO_DEV_PORT=5110 SILO_DEV_TOKEN=<token> make desktop-dev
desktop-dev:
	cd desktop && bun run dev

# test-playwright: build unpacked app for current arch, start silo server, run tests, stop server
test-playwright: desktop-test
	bash scripts/test-e2e.sh

# test-playwright-file: build then run a single playwright test file
# Usage: make test-playwright-file FILE=playwright_tests/vault.spec.ts
test-playwright-file: desktop-test
	bash scripts/test-e2e.sh $(FILE)
