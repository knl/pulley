GO := GO111MODULE=on go
GOPATH ?= $(shell $(GO) env GOPATH)
GOBIN ?= $(GOPATH)/bin

default: test

test: build fmt lint
	go test -race -v ./...

fmt:
	@echo "FORMATTING"
	find . -type f -name '*.go' -not -path './vendor/*' | xargs gofumpt -s -l -w | tee /dev/stderr

lint:
	@echo "LINTING"
	GOBIN=$(GOBIN) ./lint.sh

test-coverage:
	go test -race -coverprofile=coverage.txt -covermode=atomic

clean:
	rm -rf build dist
	rm -f $(BIN)

build: build.sh
	./build.sh

.PHONY: test fmt lint clean build
