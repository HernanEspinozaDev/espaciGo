#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
scratch="$(mktemp -d "${TMPDIR:-/tmp}/espacigo-docker03-preflight.XXXXXX")"
cleanup() {
  python3 - "$scratch" <<'PY'
import pathlib, shutil, sys
shutil.rmtree(pathlib.Path(sys.argv[1]), ignore_errors=True)
PY
}
trap cleanup EXIT

mkdir -p "$scratch/bin"
cat > "$scratch/bin/docker" <<'SH'
#!/usr/bin/env bash
: > "$DOCKER03_FAKE_DOCKER_CALLED"
exit 99
SH
chmod 0700 "$scratch/bin/docker"

case_number=0
expect_rejected_before_side_effects() {
  local source_volume="$1" restore_volume="$2" test_volume="$3"
  case_number=$((case_number + 1))
  local state="$scratch/state-$case_number" output="$scratch/output-$case_number"
  set +e
  env \
    PATH="$scratch/bin:$PATH" \
    DOCKER03_FAKE_DOCKER_CALLED="$scratch/docker-called" \
    LOCAL_DOCKER03_PROJECT_NAME="espacigo_docker03_preflight_$case_number" \
    LOCAL_DOCKER03_STATE_DIR="$state" \
    LOCAL_DOCKER03_PGDATA_VOLUME="$source_volume" \
    LOCAL_DOCKER03_RESTORE_VOLUME="$restore_volume" \
    LOCAL_DOCKER03_TEST_PGDATA_VOLUME="$test_volume" \
    LOCAL_DOCKER03_API_PORT=19180 LOCAL_DOCKER03_MOCK_PORT=19181 \
    LOCAL_DOCKER03_MAILPIT_PORT=19125 LOCAL_DOCKER03_DATABASE_PORT=19132 \
    LOCAL_DOCKER03_RESTORE_API_PORT=19182 LOCAL_DOCKER03_TLS_PROBE_PORT=19183 \
    "$ROOT_DIR/scripts/local-docker03.sh" config > "$output" 2>&1
  local result=$?
  set -e
  [[ "$result" -ne 0 ]] || { cat "$output" >&2; echo 'FAIL: coincident volume names were accepted' >&2; exit 1; }
  grep -q 'volume names must be distinct' "$output" || { cat "$output" >&2; echo 'FAIL: rejected for an unexpected reason' >&2; exit 1; }
  [[ ! -e "$state" ]] || { echo "FAIL: rejected configuration created state at $state" >&2; exit 1; }
  [[ ! -e "$scratch/docker-called" ]] || { echo 'FAIL: rejected configuration invoked Docker' >&2; exit 1; }
}

expect_rejected_before_side_effects \
  espacigo_docker03_collision_a espacigo_docker03_collision_a espacigo_docker03_collision_c
expect_rejected_before_side_effects \
  espacigo_docker03_collision_a espacigo_docker03_collision_b espacigo_docker03_collision_a
expect_rejected_before_side_effects \
  espacigo_docker03_collision_a espacigo_docker03_collision_c espacigo_docker03_collision_c

echo 'PASS: every source/restore/test volume collision is rejected before state files are created or Docker is invoked.'
