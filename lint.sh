#!/usr/bin/env bash

set -Eeuo pipefail

if [[ ! -x "$GOBIN/golangci-lint" ]]; then
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "$GOBIN" v1.23.3
fi

"$GOBIN/golangci-lint" run
