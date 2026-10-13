#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
TEMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/espacigo-local-qa-backend.XXXXXX")"
cleanup() {
  local status=$?
  trap - EXIT INT TERM
  python3 -c 'import shutil,sys; shutil.rmtree(sys.argv[1],ignore_errors=True)' "$TEMP_DIR" || status=1
  if [[ -e "$TEMP_DIR" ]]; then
    printf 'LOCAL-QA backend temporary report directory survived cleanup: %s\n' "$TEMP_DIR" >&2
    status=1
  fi
  exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

for command in docker go python3 pg_dump; do
  command -v "$command" >/dev/null 2>&1 || { printf 'required backend QA tool not found: %s\n' "$command" >&2; exit 2; }
done
docker info >/dev/null

printf '%s\n' '== Runner gate self-tests: missing, skipped, and successful mandatory integrations =='
python3 scripts/test_local_qa_results.py

printf '%s\n' '== Go: complete unit suite without access to persistent development services =='
env -u TEST_DATABASE_URL -u LOCAL_SMTP_ADDR -u LOCAL_MAILPIT_API go test -p 1 ./...
printf '%s\n' '== Go vet =='
go vet ./...

printf '%s\n' '== Go: mandatory integrations, isolated PostgreSQL/PostGIS + Mailpit per package =='
GO_TEST_JSON="$TEMP_DIR/go-test.jsonl"
GO_TEST_ERRORS="$TEMP_DIR/go-test-stderr.log"
set -o pipefail
bash scripts/run-local-qa-integrations.sh scripts/local-required-integrations.txt "$GO_TEST_JSON" "$GO_TEST_ERRORS"

CHECKIN_REPEAT="${LOCAL_QA_CHECKIN_REPEAT:-5}"
if ! [[ "$CHECKIN_REPEAT" =~ ^([2-9]|[1-9][0-9]+)$ ]]; then
  printf 'LOCAL_QA_CHECKIN_REPEAT must be an integer >= 2\n' >&2
  exit 2
fi
printf '== Check-in stability follow-up: repeat the reported integration test %s times ==\n' "$CHECKIN_REPEAT"
env -u LOCAL_QA_GO_JSON GO_TEST_COUNT="$CHECKIN_REPEAT" GO_TEST_RUN='^TestLocalBookingTrialPostgresLifecycleAndConcurrentRetry$' \
  bash scripts/test-local-comm-mailpit.sh ./internal/adapters/postgres/booking

printf '%s\n' '== Repository hygiene =='
git diff --check
printf '%s\n' 'PASS: Go/backend local QA completed without Node/npm; disposable integration resources were verified and removed.'
