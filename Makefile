GO := GO111MODULE=on go
GOPATH ?= $(shell $(GO) env GOPATH)
GOBIN ?= $(GOPATH)/bin

default: build fmt lint test

fmt:
	@echo "FORMATTING"
	find . -type f -name '*.go' -not -path './vendor/*' | xargs gofumpt -s -l -w | tee /dev/stderr

lint: fmt
	@echo "LINTING"
	GOBIN=$(GOBIN) ./lint.sh

tidy:
	@echo "TIDYING"
	$(GO) mod tidy -v

check-tidy: tidy
	git diff --quiet

test:
	@echo "TESTING"
	$(GO) test -race -v ./...

test-coverage:
	$(GO) test -race -coverprofile=coverage.txt -covermode=atomic

build: build.sh
	@echo "BUILDING"
	./build.sh

clean:
	rm -rf build dist
	rm -f $(BIN)

.PHONY: test fmt lint tidy clean build
