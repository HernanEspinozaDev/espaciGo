#!/usr/bin/env bash
set -euo pipefail
ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
export GO_TEST_RUN='TestDurableFakePaymentInboxDeduplicatesTimeoutAndRecoversAfterRestart'
exec bash "$ROOT_DIR/scripts/test-m04-attributes-postgres.sh" ./internal/adapters/postgres/booking
