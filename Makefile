all: format lint

.PHONY: smoketest
smoketest:
	go run . --version
	go test ./...
	golangci-lint run

build: build.sh
	./build.sh

.PHONY: format
format:
	find . -name \*.go | xargs gofumpt -s -w

.PHONY: lint
lint:
	golangci-lint run

.PHONY: test
test:
	go test -race ./...
