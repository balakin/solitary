BINARY := solitary
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/dm-balakin/solitary/internal/cli.version=$(VERSION)

.PHONY: build
build:
	go build -ldflags '$(LDFLAGS)' -o $(BINARY) .

.PHONY: test
test:
	go test ./...

.PHONY: fmt
fmt:
	golangci-lint fmt ./...

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: clean
clean:
	rm -f $(BINARY)
	rm -rf dist/
