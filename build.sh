#!/usr/bin/env bash

set -Eeuo pipefail

build_date="$(date '+%Y%m%d-%H:%M:%S')"
git_version="$(git describe --tags --match "v*" --dirty --abbrev=4 || true)"
if [ -z "$git_version" ]; then
    git_version="v0.0.0-$(git describe --always --dirty --abbrev=4)"
fi
git_sha=${1:-"$(git rev-parse HEAD)"}
git_ref=${2:-"$(git rev-parse --abbrev-ref HEAD)"}
username=${3:-"$USER@${HOST:-unknown}"}
ldflags=(
    "-X github.com/knl/pulley/internal/version.Version=$git_version"
    "-X github.com/knl/pulley/internal/version.Revision=$git_sha"
    "-X github.com/knl/pulley/internal/version.Branch=$git_ref"
    "-X github.com/knl/pulley/internal/version.BuildUser=$username"
    "-X github.com/knl/pulley/internal/version.BuildDate=$build_date"
)

go build -o dist/pulley -ldflags="${ldflags[*]}" .
