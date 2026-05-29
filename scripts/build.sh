#!/usr/bin/env bash
set -euo pipefail

VERSION="${VERSION:-dev}"
LDFLAGS="-s -w -X main.version=${VERSION}"
OUT="dist"

mkdir -p "${OUT}"

targets=(
  "linux/amd64"
  "linux/arm64"
  "linux/arm/v7"
  "darwin/amd64"
  "darwin/arm64"
  "windows/amd64"
)

for target in "${targets[@]}"; do
  IFS='/' read -r GOOS GOARCH GOARM <<< "${target}/"
  GOARM="${GOARM:-}"
  NAME="switchdeck-${GOOS}-${GOARCH}${GOARM:+-armv${GOARM}}"
  [[ "${GOOS}" == "windows" ]] && NAME="${NAME}.exe"
  echo "Building ${NAME}..."
  CGO_ENABLED=0 GOOS="${GOOS}" GOARCH="${GOARCH}" GOARM="${GOARM}" \
    go build -trimpath -ldflags "${LDFLAGS}" \
    -o "${OUT}/${NAME}" ./cmd/switchdeck/
done

echo "Done. Artifacts in ${OUT}/"
