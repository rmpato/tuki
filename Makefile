BINARY := tuki
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/rmpato/tuki/internal/cli.Version=$(VERSION)

.PHONY: build install test fmt vet check clean

build:
	go build -ldflags '$(LDFLAGS)' -o $(BINARY) .

install:
	go install -ldflags '$(LDFLAGS)' .

test:
	go test ./...

fmt:
	gofmt -l -w .

vet:
	go vet ./...

check: fmt vet test

clean:
	rm -f $(BINARY)
