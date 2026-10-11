#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
STATE="${LOCAL_DOCKER03_STATE_DIR:-${XDG_STATE_HOME:-$HOME/.local/state}/espacigo/docker03-20261011}"
API_PORT="${LOCAL_DOCKER03_API_PORT:-18280}"
DATABASE_PORT="${LOCAL_DOCKER03_DATABASE_PORT:-18132}"
PROJECT="${LOCAL_DOCKER03_PROJECT_NAME:-espacigo_docker03_20261011}"
export LOCAL_DOCKER03_STATE_DIR="$STATE" LOCAL_DOCKER03_PROJECT_NAME="$PROJECT"
export LOCAL_DOCKER03_API_PORT="$API_PORT" LOCAL_DOCKER03_DATABASE_PORT="$DATABASE_PORT"
export LOCAL_DOCKER03_RESTORE_API_PORT="${LOCAL_DOCKER03_RESTORE_API_PORT:-18282}"
export LOCAL_DOCKER03_TLS_PROBE_PORT="${LOCAL_DOCKER03_TLS_PROBE_PORT:-18283}"
export LOCAL_IMAGE_PREFIX="$PROJECT" LOCAL_DOCKER_PROJECT_NAME="$PROJECT"
export LOCAL_API_PORT="$API_PORT" LOCAL_MOCK_PORT="${LOCAL_DOCKER03_MOCK_PORT:-18281}"
export LOCAL_MAILPIT_PORT="${LOCAL_DOCKER03_MAILPIT_PORT:-18225}"
export LOCAL_PGDATA_VOLUME="${LOCAL_DOCKER03_PGDATA_VOLUME:-espacigo_docker03_pgdata_20261011}"
export LOCAL_DOCKER03_RESTORE_VOLUME="${LOCAL_DOCKER03_RESTORE_VOLUME:-espacigo_docker03_restore_pgdata_20261011}"
export LOCAL_DOCKER03_TEST_PGDATA_VOLUME="${LOCAL_DOCKER03_TEST_PGDATA_VOLUME:-espacigo_docker03_test_pgdata_20261011}"
export LOCAL_DOCKER03_RESTORE_DATABASE_NAME="${LOCAL_DOCKER03_RESTORE_DATABASE_NAME:-espacigo_local_restored}"
export LOCAL_SECRETS_DIR="$STATE/secrets" LOCAL_M03_EVIDENCE_DIR="$STATE/private-evidence"
export LOCAL_PRIVACY_REPLAY_DIR="$STATE/privacy-replay" LOCAL_DATABASE_TLS_DIR="$STATE/database-ca"
export LOCAL_DOCKER03_TEST_NO_RUNTIME_INIT_FILE="$STATE/test-no-runtime-init.sh"
export LOCAL_DOCKER03_POSTGRES_TLS_DIR="$STATE/database-tls"
export LOCAL_UID="$(id -u)" LOCAL_GID="$(id -g)"
export LOCAL_DATABASE_NAME="${LOCAL_DATABASE_NAME:-espacigo_local}" LOCAL_DATABASE_HOST=database
export LOCAL_DATABASE_PORT="${LOCAL_DATABASE_PORT:-5432}" LOCAL_DATABASE_ADMIN_USER="${LOCAL_DATABASE_ADMIN_USER:-espacigo_admin}"
export DATABASE_SSLMODE=verify-full DATABASE_SSLROOTCERT=/run/database-tls/root.pem
COMPOSE=(docker compose --project-directory "$ROOT_DIR" -f "$ROOT_DIR/compose.yaml" -f "$ROOT_DIR/compose.docker03.yaml" -p "$PROJECT")

"$ROOT_DIR/scripts/test-local-docker03-preflight.sh"
command -v docker >/dev/null
command -v openssl >/dev/null
command -v psql >/dev/null
command -v go >/dev/null
command -v curl >/dev/null
command -v jq >/dev/null
"$ROOT_DIR/scripts/local-docker03.sh" up

# Make one small synthetic private file so the dump/restore step exercises the
# file tree as well as the database. It remains under the DOCKER-03 state dir.
python3 - "$STATE/private-evidence/docker03-private-restore-fixture.png" <<'PY'
import pathlib, struct, sys, zlib
def chunk(kind,data):
    return struct.pack('>I',len(data))+kind+data+struct.pack('>I',zlib.crc32(kind+data)&0xffffffff)
png=b'\x89PNG\r\n\x1a\n'+chunk(b'IHDR',struct.pack('>IIBBBBB',1,1,8,2,0,0,0))+chunk(b'IDAT',zlib.compress(b'\x00\x22\x88\xcc'))+chunk(b'IEND',b'')
path=pathlib.Path(sys.argv[1]); path.write_bytes(png); path.chmod(0o600)
PY
private_file_hash="$(sha256sum "$STATE/private-evidence/docker03-private-restore-fixture.png" | awk '{print $1}')"

# Trusted TLS must reach the runtime role and keep it below the migrator.
tls_result="$("${COMPOSE[@]}" exec -T database sh -ec '
  export PGPASSWORD="$(cat /run/secrets/runtime_password)"
  PGSSLMODE=verify-full PGSSLROOTCERT=/run/postgres-tls/root.pem psql -X -h localhost -U espacigo_runtime -d "$POSTGRES_DB" -Atqc "SELECT (SELECT ssl FROM pg_stat_ssl WHERE pid=pg_backend_pid())::text || chr(124) || has_table_privilege(current_user,\$\$public.usuario\$\$,\$\$SELECT\$\$)::text || chr(124) || has_table_privilege(current_user,\$\$public.schema_migrations\$\$,\$\$INSERT\$\$)::text || chr(124) || has_schema_privilege(current_user,\$\$public\$\$,\$\$CREATE\$\$)::text"
')"
[[ "$tls_result" == "true|true|false|false" ]] || { echo "TLS/runtime privilege check failed: $tls_result" >&2; exit 1; }
echo 'PASS: alternate PostGIS accepts verify-full TLS; runtime can read application data but cannot migrate/create objects.'

# The same hostname/certificate with a separately generated CA must fail closed.
if "${COMPOSE[@]}" exec -T database sh -ec '
  export PGPASSWORD="$(cat /run/secrets/runtime_password)"
  PGSSLMODE=verify-full PGSSLROOTCERT=/run/postgres-tls/untrusted-root.pem psql -X -h localhost -U espacigo_runtime -d "$POSTGRES_DB" -Atqc "SELECT 1"
' >/dev/null 2>"$STATE/untrusted-tls.log"; then
  echo 'FAIL: PostgreSQL accepted the untrusted CA' >&2
  exit 1
fi
grep -Eqi 'certificate verify failed|self-signed certificate|unable to get local issuer' "$STATE/untrusted-tls.log" || { echo 'FAIL: TLS rejection did not identify certificate validation' >&2; exit 1; }
rm -f -- "$STATE/untrusted-tls.log"
echo 'PASS: PostgreSQL refuses a certificate signed by an untrusted local CA.'
untrusted_probe_id=''
cleanup_untrusted_probe() {
  if [[ -n "$untrusted_probe_id" ]]; then docker stop "$untrusted_probe_id" >/dev/null 2>&1 || true; fi
}
trap cleanup_untrusted_probe EXIT
untrusted_probe_id="$(LOCAL_API_PORT="$LOCAL_DOCKER03_TLS_PROBE_PORT" "${COMPOSE[@]}" run -d --rm --service-ports --no-deps -e DATABASE_SSLROOTCERT=/run/database-tls/untrusted-root.pem backend)"
untrusted_http=''
for _ in $(seq 1 20); do
  untrusted_http="$(curl --silent --output /dev/null --max-time 2 --write-out '%{http_code}' "http://127.0.0.1:$LOCAL_DOCKER03_TLS_PROBE_PORT/health/ready" || true)"
  [[ "$untrusted_http" == 503 ]] && break
  sleep 1
done
[[ "$untrusted_http" == 503 ]] || { echo "FAIL: Backend readiness with an untrusted CA returned HTTP $untrusted_http, expected 503" >&2; exit 1; }
docker stop "$untrusted_probe_id" >/dev/null
untrusted_probe_id=''
trap - EXIT
echo 'PASS: Backend readiness rejects the same PostgreSQL instance when configured with an untrusted CA.'

API_ORIGIN="http://127.0.0.1:$API_PORT" MOCK_ORIGIN="http://127.0.0.1:${LOCAL_DOCKER03_MOCK_PORT:-18281}" python3 "$ROOT_DIR/scripts/verify-http.py"

# Make the recovery test deterministic: stop the API/workers first, then seed an
# already-expired durable lease. The row cannot be claimed until this Backend is
# explicitly started again.
"${COMPOSE[@]}" stop backend >/dev/null
running_backend="$("${COMPOSE[@]}" ps --status running --services | awk '$0 == "backend" { print; found=1 } END { if (!found) print "stopped" }')"
[[ "$running_backend" == stopped ]] || { echo 'FAIL: Backend must be stopped before seeding the recovery lease' >&2; exit 1; }

# Seed only a synthetic recipient and a durable expired lease. Starting the
# stopped container must let its worker reclaim and finish this row.
seed_file="$STATE/restart-fixture.tsv"
"${COMPOSE[@]}" exec -T database psql -X -qAt -v ON_ERROR_STOP=1 --username "${LOCAL_DATABASE_ADMIN_USER:-espacigo_admin}" --dbname "${LOCAL_DATABASE_NAME:-espacigo_local}" <<'SQL' > "$seed_file"
SELECT gen_random_uuid() AS account_id \gset
INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado)
VALUES(:'account_id','docker03-worker@example.invalid','docker03-worker@example.invalid','synthetic-worker-only','activo');
INSERT INTO public.rol_usuario(usuario_id,rol) VALUES(:'account_id','administrador');
SELECT gen_random_uuid() AS notice_id \gset
INSERT INTO public.aviso_local(id,tipo_evento,agregado_tipo,agregado_id,destinatario_id,clave_deduplicacion,estado,ciclo,intentos_total,intentos_ciclo,creada_en,proximo_intento_en,lease_hasta)
VALUES(:'notice_id','checkin_registrado','reserva',gen_random_uuid(),:'account_id','docker03-expired-lease-'||:'notice_id','procesando',1,1,1,clock_timestamp()-interval '2 minutes',clock_timestamp()-interval '2 minutes',clock_timestamp()-interval '1 minute');
INSERT INTO public.aviso_local_ciclo(aviso_id,ciclo,estado,iniciada_en,intentos,codigo_resultado)
VALUES(:'notice_id',1,'pendiente',clock_timestamp()-interval '2 minutes',1,NULL);
\echo :account_id
\echo :notice_id
SQL
chmod 0600 "$seed_file"
mapfile -t fixture < "$seed_file"
[[ ${#fixture[@]} -eq 2 ]] || { echo 'failed to persist restart fixture identifiers' >&2; exit 1; }
account_id="${fixture[0]}"; notice_id="${fixture[1]}"
"${COMPOSE[@]}" start backend >/dev/null
deadline=$((SECONDS + 45))
notice_state=''
while (( SECONDS < deadline )); do
  notice_state="$("${COMPOSE[@]}" exec -T database psql -X -qAt --username "${LOCAL_DATABASE_ADMIN_USER:-espacigo_admin}" --dbname "${LOCAL_DATABASE_NAME:-espacigo_local}" -c "SELECT estado||'|'||intentos_total FROM public.aviso_local WHERE id='$notice_id'")"
  [[ "$notice_state" == entregada\|2 ]] && break
  sleep 1
done
[[ "$notice_state" == entregada\|2 ]] || { echo "FAIL: restarted worker did not reclaim the persisted expired lease (state=$notice_state)" >&2; exit 1; }
message_count="$(curl --fail --silent "http://127.0.0.1:${LOCAL_DOCKER03_MAILPIT_PORT:-18225}/api/v1/messages" | jq --arg to docker03-worker@example.invalid '[.messages[] | select(.To[]?.Address == $to)] | length')"
[[ "$message_count" == 1 ]] || { echo "FAIL: lease recovery delivered $message_count messages, expected exactly one in Mailpit" >&2; exit 1; }
echo 'PASS: with Backend stopped, an expired durable lease remained unclaimed; after Backend start it was reclaimed and delivered once to Mailpit.'

# Database/volume and private-file restore are isolated. A snapshot gets a
# separate target DB and separate file tree. The restored API is gated until
# the sidecar replay command confirms the external suppression registry.
"$ROOT_DIR/scripts/local-docker03.sh" dump
"$ROOT_DIR/scripts/local-docker03.sh" restore-db
admin_id="$account_id"
restore_id="$(python3 -c 'import uuid; print(uuid.uuid4())')"
if curl --fail --silent "http://127.0.0.1:${LOCAL_DOCKER03_RESTORE_API_PORT:-18282}/health/ready" >/dev/null; then
  echo 'FAIL: restored API was exposed before the external registry replay' >&2
  exit 1
fi
if "$ROOT_DIR/scripts/local-docker03.sh" start-restored-backend >"$STATE/replay-gate.log" 2>&1; then
  echo 'FAIL: restore helper allowed API startup without a replay marker' >&2
  exit 1
fi
grep -q 'restored registry replay is not confirmed' "$STATE/replay-gate.log" || { echo 'FAIL: restore helper rejected startup for an unexpected reason' >&2; exit 1; }
python3 - "$STATE/replay-gate.log" <<'PY'
import pathlib,sys
pathlib.Path(sys.argv[1]).unlink(missing_ok=True)
PY
echo 'PASS: restore helper blocks API startup until the external registry replay marker is present.'
RESTORE_ID="$restore_id" ADMIN_ACCOUNT_ID="$admin_id" "$ROOT_DIR/scripts/local-docker03.sh" replay
"$ROOT_DIR/scripts/local-docker03.sh" start-restored-backend
restored_health="http://127.0.0.1:${LOCAL_DOCKER03_RESTORE_API_PORT:-18282}/health/ready"
for _ in $(seq 1 30); do
  if curl --fail --silent "$restored_health" >/dev/null; then break; fi
  sleep 1
done
curl --fail --silent "$restored_health" >/dev/null
# Validate the restored API's CORS preflight against the actual published mock
# origin, not the development/default mock port.
API_ORIGIN="http://127.0.0.1:${LOCAL_DOCKER03_RESTORE_API_PORT:-18282}" MOCK_ORIGIN="http://127.0.0.1:${LOCAL_DOCKER03_MOCK_PORT:-18281}" python3 "$ROOT_DIR/scripts/verify-http.py"
cmp <(find "$STATE/private-evidence" -type f -printf '%P %s\n' | sort) <(find "$STATE/restored-private-evidence" -type f -printf '%P %s\n' | sort)
[[ "$(sha256sum "$STATE/restored-private-evidence/docker03-private-restore-fixture.png" | awk '{print $1}')" == "$private_file_hash" ]]
[[ -s "$STATE/backups/source.dump" && "$(stat -c %a "$STATE/backups/source.dump")" == 600 ]]
for secret_name in db_admin_password runtime_password local_payment_webhook_secret local_contract_encryption_key; do
  [[ -s "$STATE/secrets/$secret_name" && "$(stat -c %a "$STATE/secrets/$secret_name")" == 600 ]] || { echo "FAIL: isolated restore secret missing or too permissive: $secret_name" >&2; exit 1; }
done
restored_counts="$("${COMPOSE[@]}" exec -T restore-database psql -X -qAt --username "${LOCAL_DATABASE_ADMIN_USER:-espacigo_admin}" --dbname "$LOCAL_DOCKER03_RESTORE_DATABASE_NAME" -c "SELECT (SELECT count(*) FROM public.usuario)::text || chr(124) || (SELECT count(*) FROM public.aviso_local WHERE id='$notice_id')::text")"
[[ "$restored_counts" == "1|1" ]] || { echo "FAIL: restored relations differ from the source snapshot: $restored_counts" >&2; exit 1; }
echo 'PASS: restored PostgreSQL and private files are separate; replay marker was required before the restored API started.'

# These tests create and drop espacigo_runtime, so give them their own cluster
# rather than perturbing either long-lived source/restore runtime account. They
# can be skipped when validating only the restored CORS/recovery wiring.
if [[ "${LOCAL_DOCKER03_SKIP_INTEGRATION_TESTS:-0}" == 1 ]]; then
  echo 'SKIP: optional payment/privacy PostgreSQL integration tests (focused Docker wiring run).'
  exit 0
fi
"${COMPOSE[@]}" up -d --wait test-database >/dev/null
test_database_ip="$(docker inspect --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$PROJECT-test-database-1")"
test_admin_password="$(cat "$STATE/secrets/db_admin_password")"
test_ca_path="$(python3 -c 'import pathlib,sys; sys.stdout.write(str(pathlib.Path(sys.argv[1]).resolve()))' "$STATE/database-ca/root.pem")"
export TEST_DATABASE_URL="postgres://$(printf '%s' "${LOCAL_DATABASE_ADMIN_USER:-espacigo_admin}" | jq -sRr @uri):$(printf '%s' "$test_admin_password" | jq -sRr @uri)@${test_database_ip}:5432/${LOCAL_DATABASE_NAME:-espacigo_local}?sslmode=verify-ca&sslrootcert=$(printf '%s' "$test_ca_path" | jq -sRr @uri)"
go test ./internal/adapters/postgres/booking -run '^(TestDurableFakePaymentInboxDeduplicatesTimeoutAndRecoversAfterRestart|TestLocalFakeSettlementBoundaryTimeoutRestartAndConcurrentReplay)$' -count=1 -timeout=8m
go test ./internal/adapters/postgres/identity -run '^TestLocalSuppressionReplayAfterPgDumpRestore$' -count=1 -timeout=8m
echo 'PASS: alternate PostgreSQL verifies payment/settlement recovery and external suppression replay after restore; financial retries remain idempotent.'
