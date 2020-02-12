#!/usr/bin/env bash

set -Eeuo pipefail

build_date="$(date '+%Y%m%d-%H:%M:%S')"
git_version="$(git describe --tags --match "v*" --dirty --abbrev=4 2>/dev/null || true)"
if [ -z "$git_version" ]; then
    git_version="v0.0.0-$(git describe --always --dirty --abbrev=4)"
fi
pulley_git_sha=${1:-${pulley_git_sha:-"$(git rev-parse HEAD)"}}
pulley_git_ref=${2:-${pulley_git_ref:-"$(git rev-parse --abbrev-ref HEAD)"}}
if command -v hostname >&/dev/null; then
    fallback_hostname="$(hostname -f 2>/dev/null || true)"
fi
pulley_build_username=${3:-${pulley_build_username:-"$USER@${HOST:-${HOSTNAME:-${fallback_hostname:-unknown}}}"}}
ldflags=(
    "-X github.com/knl/pulley/internal/version.Version=$git_version"
    "-X github.com/knl/pulley/internal/version.Revision=$pulley_git_sha"
    "-X github.com/knl/pulley/internal/version.Branch=$pulley_git_ref"
    "-X github.com/knl/pulley/internal/version.BuildUser=$pulley_build_username"
    "-X github.com/knl/pulley/internal/version.BuildDate=$build_date"
)

# Use CGO_ENABLED=0 as we don't call any C builds, and will be doing cross compiling
GO111MODULE=on CGO_ENABLED=0 go build -ldflags="${ldflags[*]}"
