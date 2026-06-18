#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

export GOMOCK_PERF="${GOMOCK_PERF:-1}"
export GOMOCK_PERF_MAPPINGS="${GOMOCK_PERF_MAPPINGS:-100}"
export GOMOCK_PERF_CONCURRENCY="${GOMOCK_PERF_CONCURRENCY:-16}"
export GOMOCK_PERF_DURATION="${GOMOCK_PERF_DURATION:-3s}"
export GOMOCK_PERF_REQUEST_TIMEOUT="${GOMOCK_PERF_REQUEST_TIMEOUT:-2s}"

cd "$ROOT_DIR"
go test ./test/perf -run TestPerformanceSmoke -count=1 -v "$@"
