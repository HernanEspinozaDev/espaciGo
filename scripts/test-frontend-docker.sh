#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
STAMP="$(date -u +%Y%m%d%H%M%S)_$$"
PROJECT="espacigo_frontend_test_${STAMP}"
CLEANUP_TARGETS=()

cleanup() {
  set +e
  for target in "${CLEANUP_TARGETS[@]}"; do
    docker rm -f "$target" >/dev/null 2>&1 || true
  done
  if [[ -n "${PROJECT:-}" ]]; then
    LOCAL_DOCKER_PROJECT_NAME="$PROJECT" \
    LOCAL_IMAGE_PREFIX="$PROJECT" \
    LOCAL_DOCKER_STATE_DIR="/tmp/${PROJECT}_state" \
    LOCAL_SECRETS_DIR="/tmp/${PROJECT}_secrets" \
    LOCAL_M03_EVIDENCE_DIR="/tmp/${PROJECT}_evidence" \
    LOCAL_PRIVACY_REPLAY_DIR="/tmp/${PROJECT}_privacy" \
    LOCAL_DATABASE_TLS_DIR="/tmp/${PROJECT}_tls" \
    bash "$ROOT_DIR/scripts/local-docker-stack.sh" down >/dev/null 2>&1 || true
    rm -rf "/tmp/${PROJECT}_"* 2>/dev/null || true
  fi
}
trap cleanup EXIT

main() {
  export LOCAL_DOCKER_PROJECT_NAME="$PROJECT"
  export LOCAL_IMAGE_PREFIX="$PROJECT"
  export LOCAL_DOCKER_STATE_DIR="/tmp/${PROJECT}_state"
  export LOCAL_SECRETS_DIR="/tmp/${PROJECT}_secrets"
  export LOCAL_M03_EVIDENCE_DIR="/tmp/${PROJECT}_evidence"
  export LOCAL_PRIVACY_REPLAY_DIR="/tmp/${PROJECT}_privacy"
  export LOCAL_DATABASE_TLS_DIR="/tmp/${PROJECT}_tls"
  export LOCAL_UID="$(id -u)"
  export LOCAL_GID="$(id -g)"
  export LOCAL_API_PORT="$(( 20000 + RANDOM % 10000 ))"
  export LOCAL_DATABASE_PORT="$(( 30000 + RANDOM % 10000 ))"
  export LOCAL_MAIL_PORT="$(( 40000 + RANDOM % 10000 ))"
  export LOCAL_ANGULAR_PORT="$(( 50000 + RANDOM % 10000 ))"
  export LOCAL_NEXT_PORT="$(( 51000 + RANDOM % 10000 ))"

  mkdir -p \
    "$LOCAL_DOCKER_STATE_DIR" \
    "$LOCAL_SECRETS_DIR" \
    "$LOCAL_M03_EVIDENCE_DIR" \
    "$LOCAL_PRIVACY_REPLAY_DIR" \
    "$LOCAL_DATABASE_TLS_DIR"

  bash "$ROOT_DIR/scripts/local-docker-stack.sh" config >/dev/null

  printf 'Building Next.js frontend image...\n'
  bash "$ROOT_DIR/scripts/local-docker-stack.sh" build-next

  printf 'Verifying standalone frontend image execution...\n'
  IMAGE_NAME="${PROJECT}-next:dev"
  CONTAINER_NAME="${PROJECT}_runner"
  CLEANUP_TARGETS+=("$CONTAINER_NAME")

  docker run -d \
    --name "$CONTAINER_NAME" \
    --network none \
    -e PORT=4000 \
    "$IMAGE_NAME"

  sleep 3

  RUNNING="$(docker inspect --format '{{.State.Running}}' "$CONTAINER_NAME" 2>/dev/null || true)"
  if [[ "$RUNNING" != "true" ]]; then
    printf 'Frontend container exited unexpectedly:\n' >&2
    docker logs "$CONTAINER_NAME" >&2 || true
    exit 1
  fi

  USER_ID="$(docker exec "$CONTAINER_NAME" id -u)"
  if [[ "$USER_ID" == "0" ]]; then
    printf 'Frontend container is running as root (UID 0)\n' >&2
    exit 1
  fi

  printf 'Frontend runs safely as non-root UID %s\n' "$USER_ID"

  # Verify no database env or credentials
  ENV_DUMP="$(docker exec "$CONTAINER_NAME" env)"
  if echo "$ENV_DUMP" | grep -iqE '(DATABASE_PASSWORD|POSTGRES_PASSWORD|DB_PASS|SECRET_KEY)'; then
    printf 'Frontend container has sensitive credentials in environment!\n' >&2
    exit 1
  fi

  # Verify health endpoint responds
  docker exec "$CONTAINER_NAME" wget -q -O /dev/null http://127.0.0.1:4000/api/health 2>/dev/null || \
  docker exec "$CONTAINER_NAME" wget -q -O /dev/null http://127.0.0.1:4000/healthz 2>/dev/null || \
  docker exec "$CONTAINER_NAME" node -e "fetch('http://127.0.0.1:4000/api/health').then(r => process.exit(r.ok ? 0 : 1))"

  printf 'Frontend healthcheck endpoint responded successfully\n'

  docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true

  printf 'PASS: Frontend Docker integration verified successfully\n'
}

main "$@"
