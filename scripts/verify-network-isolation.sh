#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
export LOCAL_UID="$(id -u)"
export LOCAL_GID="$(id -g)"
IMAGE="espacigo-local-netprobe:dev"

compose() {
  docker compose --project-directory "$ROOT_DIR" -f "$ROOT_DIR/compose.yaml" "$@"
}

compose build netprobe
mock_id="$(compose ps -q mock-frontend)"
database_id="$(compose ps -q database)"
if [[ -z "$mock_id" || -z "$database_id" ]]; then
  printf '%s\n' 'FAIL: database and mock must be running before network verification' >&2
  exit 1
fi

for container_id in "$mock_id" "$database_id"; do
  health="$(docker inspect --format '{{.State.Health.Status}}' "$container_id")"
  if [[ "$health" != "healthy" ]]; then
    printf '%s\n' 'FAIL: database and mock must be healthy before network verification' >&2
    exit 1
  fi
done

mock_networks="$(docker inspect --format '{{json .NetworkSettings.Networks}}' "$mock_id")"
api_network="$(printf '%s' "$mock_networks" | python3 -c 'import json,sys; nets=json.load(sys.stdin); assert len(nets)==1; print(next(iter(nets)))')"
database_networks="$(docker inspect --format '{{json .NetworkSettings.Networks}}' "$database_id")"
read -r data_network database_ip < <(printf '%s' "$database_networks" | python3 -c 'import ipaddress,json,sys; nets=json.load(sys.stdin); assert len(nets)==1; name,info=next(iter(nets.items())); ip=info.get("IPAddress",""); ipaddress.ip_address(ip); print(name,ip)')

if [[ "$api_network" == "$data_network" ]]; then
  printf '%s\n' 'FAIL: mock and database unexpectedly share a network' >&2
  exit 1
fi

dns_metadata="$(docker run --rm --network "$api_network" --entrypoint /netprobe "$IMAGE" diagnose)"
printf 'mock-network DNS diagnostic: %s\n' "$dns_metadata"

positive="$(docker run --rm --network "$data_network" --entrypoint /netprobe "$IMAGE" connect "$database_ip")"
if [[ "$positive" != 'connect=success' ]]; then
  printf '%s\n' 'FAIL: positive control could not connect from the database network' >&2
  exit 1
fi
printf 'PASS: authorized database-network positive control: %s\n' "$positive"

set +e
negative="$(docker run --rm --network "$api_network" --entrypoint /netprobe "$IMAGE" connect "$database_ip" 2>&1)"
negative_status=$?
set -e
if [[ "$negative_status" -eq 0 ]]; then
  printf '%s\n' 'FAIL: a probe on the mock network connected to PostgreSQL' >&2
  exit 1
fi
if [[ ! "$negative" =~ ^connect=failed\ timeout=(true|false)$ ]]; then
  printf '%s\n' 'FAIL: mock-network TCP probe did not return a classified connection failure' >&2
  exit 1
fi
printf 'PASS: mock-network probe to the dynamic PostgreSQL IP:5432 failed with a 2s timeout (timeout=%s)\n' "${BASH_REMATCH[1]}"
