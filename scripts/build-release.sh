#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
OUTPUT_DIR=${OUTPUT_DIR:-"${ROOT_DIR}/dist"}
VERSION=${VERSION:-dev}
COMMIT=${COMMIT:-$(git -C "${ROOT_DIR}" rev-parse --short HEAD 2>/dev/null || printf unknown)}
PLATFORMS=${PLATFORMS:-"linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64"}

mkdir -p "${OUTPUT_DIR}"
checksum_files=()

for platform in ${PLATFORMS}; do
  os=${platform%/*}
  arch=${platform#*/}
  binary="gomock"
  if [[ "${os}" == "windows" ]]; then
    binary="gomock.exe"
  fi

  target_dir="${OUTPUT_DIR}/gomock-${VERSION}-${os}-${arch}"
  rm -rf "${target_dir}"
  mkdir -p "${target_dir}"
  CGO_ENABLED=0 GOOS="${os}" GOARCH="${arch}" go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o "${target_dir}/${binary}" "${ROOT_DIR}/cmd/gomock"
  if [[ "${os}" == "windows" ]]; then
    archive="${target_dir}.zip"
    rm -f "${archive}"
    (cd "${OUTPUT_DIR}" && zip -qr "$(basename "${archive}")" "$(basename "${target_dir}")")
  else
    archive="${target_dir}.tar.gz"
    rm -f "${archive}"
    tar -C "${OUTPUT_DIR}" -czf "${archive}" "$(basename "${target_dir}")"
  fi
  checksum_files+=("$(basename "${archive}")")
done

(cd "${OUTPUT_DIR}" && shasum -a 256 "${checksum_files[@]}" > SHA256SUMS.txt)
