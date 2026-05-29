#!/usr/bin/env bash
set -euo pipefail

export LOG_LEVEL="${LOG_LEVEL:-debug}"
export PORT="${PORT:-8080}"

go run ./cmd/switchdeck/ --port "${PORT}" --log-level "${LOG_LEVEL}"
