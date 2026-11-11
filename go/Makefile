GO ?= go
BIN := bin/glh

.PHONY: all build test vet fmt check clean parity-session

all: check build

build:
	$(GO) build -o $(BIN) ./cmd/glh

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -w .

check:
	gofmt -l . | tee /dev/stderr | test -z "$$(cat)"
	$(GO) vet ./...
	$(GO) test ./...

parity-session:
	./scripts/session-parity.sh

clean:
	rm -rf bin
