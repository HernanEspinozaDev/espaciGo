#!/usr/bin/env bash
set -Eeuo pipefail
umask 077

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
GCLOUD_BIN="${GCP06_GCLOUD_BIN:-gcloud}"
if [[ $# -ne 2 ]]; then
  echo 'usage: scripts/gcp06-upload-candidate-sql.sh .local/testing/gcp06-stage-<run-id>/data.sql <bucket>' >&2
  exit 2
fi
ARTIFACT="$1"
BUCKET="$2"
[[ "$ARTIFACT" =~ ^\.local/testing/gcp06-stage-[a-z0-9][a-z0-9-]{5,30}/data\.sql$ ]] || { echo 'artifact must be the private run-scoped data.sql.' >&2; exit 2; }
[[ "$BUCKET" =~ ^[a-z0-9][a-z0-9._-]{2,62}$ ]] || { echo 'invalid candidate bucket name.' >&2; exit 2; }
ARTIFACT_PATH="$ROOT_DIR/$ARTIFACT"
[[ -f "$ARTIFACT_PATH" && ! -L "$ARTIFACT_PATH" ]] || { echo 'private SQL artifact is missing or is a symlink.' >&2; exit 1; }
[[ "$(stat -c '%a' "$ARTIFACT_PATH")" == 600 ]] || { echo 'private SQL artifact must have mode 0600.' >&2; exit 1; }
RUN_ID="${ARTIFACT#*.local/testing/gcp06-stage-}"
RUN_ID="${RUN_ID%/data.sql}"
[[ "$RUN_ID" =~ ^[a-z0-9][a-z0-9-]{5,30}$ ]] || { echo 'invalid artifact run ID.' >&2; exit 2; }
OBJECT="gcp06/$RUN_ID/data.sql"
SHA256="$(sha256sum -- "$ARTIFACT_PATH" | cut -d ' ' -f 1)"
BYTES="$(stat -c '%s' "$ARTIFACT_PATH")"

"$GCLOUD_BIN" storage cp --if-generation-match=0 --custom-metadata="sha256=$SHA256" \
  "$ARTIFACT_PATH" "gs://$BUCKET/$OBJECT" >/dev/null
DESCRIPTION="$("$GCLOUD_BIN" storage objects describe "gs://$BUCKET/$OBJECT" --format=json 2>/dev/null)" || {
  echo 'uploaded object could not be verified; stop before Cloud SQL import.' >&2
  exit 1
}
python3 - "$DESCRIPTION" "$BUCKET" "$OBJECT" "$SHA256" "$BYTES" <<'PY'
import json,sys
try:
    obj=json.loads(sys.argv[1])
except (ValueError,IndexError):
    raise SystemExit('object metadata was invalid; stop before Cloud SQL import')
bucket,name,sha,size=sys.argv[2:]
metadata=obj.get('metadata') or {}
try:
    generation=int(obj.get('generation',0))
    remote_size=int(obj.get('size',-1))
except (TypeError,ValueError):
    raise SystemExit('object generation or size was invalid; stop before Cloud SQL import')
if obj.get('bucket') != bucket or obj.get('name') != name or generation < 1 or remote_size != int(size) or metadata.get('sha256') != sha:
    raise SystemExit('uploaded object does not match the local artifact; stop before Cloud SQL import')
PY
printf 'PASS: uploaded exact candidate object once with generation-zero precondition; generation, byte size, and SHA-256 verified. Import remains a separate authorized step.\n'
