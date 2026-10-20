#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
TEST_ROOT="$ROOT_DIR/.local/testing"
mkdir -p -m 0700 -- "$TEST_ROOT"
ARTIFACT_DIR="$(mktemp -d "$TEST_ROOT/gcp06-stage-test-XXXXXX")"
RUN_ID="${ARTIFACT_DIR##*/gcp06-stage-}"
RUN_ID="${RUN_ID,,}"
LOWER_ARTIFACT_DIR="$TEST_ROOT/gcp06-stage-$RUN_ID"
mv -- "$ARTIFACT_DIR" "$LOWER_ARTIFACT_DIR"
ARTIFACT_DIR="$LOWER_ARTIFACT_DIR"
ARTIFACT="$ARTIFACT_DIR/data.sql"
MOCK_BIN="$ARTIFACT_DIR/mock-gcloud"
trap 'rm -rf -- "$ARTIFACT_DIR"' EXIT
mkdir -p -m 0700 -- "$ARTIFACT_DIR"
printf 'synthetic candidate SQL fixture\n' > "$ARTIFACT"
chmod 0600 "$ARTIFACT"

cat > "$MOCK_BIN" <<'MOCK'
#!/usr/bin/env bash
set -Eeuo pipefail
if [[ "$1 $2" == 'storage cp' ]]; then
  [[ "$3" == --if-generation-match=0 ]]
  artifact="${@: -2:1}"
  target="${@: -1}"
  printf '%s\n' "$artifact" > "${MOCK_STATE_DIR}/artifact"
  printf '%s\n' "$target" > "${MOCK_STATE_DIR}/target"
  exit 0
fi
if [[ "$1 $2 $3" == 'storage objects describe' ]]; then
  target="$(cat "${MOCK_STATE_DIR}/target")"
  artifact="$(cat "${MOCK_STATE_DIR}/artifact")"
  bucket="${target#gs://}"
  bucket="${bucket%%/*}"
  name="${target#gs://$bucket/}"
  sha="$(sha256sum -- "$artifact" | cut -d ' ' -f 1)"
  size="$(stat -c '%s' "$artifact")"
  python3 - "$bucket" "$name" "$sha" "$size" "${GCP06_TEST_MUTATION:-}" <<'PY'
import json,sys
bucket,name,sha,size,mutation=sys.argv[1:]
obj={"bucket":bucket,"name":name,"generation":"1792490496223841","size":size,"custom_fields":{"sha256":sha}}
if mutation == "bucket": obj["bucket"]="wrong-bucket"
if mutation == "name": obj["name"]="wrong/object.sql"
if mutation == "generation": obj["generation"]="0"
if mutation == "size": obj["size"]=str(int(size)+1)
if mutation == "sha256": obj["custom_fields"]["sha256"]="0"*64
print(json.dumps(obj))
PY
  exit 0
fi
echo 'unexpected gcloud invocation' >&2
exit 2
MOCK
chmod 0700 "$MOCK_BIN"

export MOCK_STATE_DIR="$ARTIFACT_DIR"
export GCP06_GCLOUD_BIN="$MOCK_BIN"
bash "$ROOT_DIR/scripts/gcp06-upload-candidate-sql.sh" ".local/testing/gcp06-stage-$RUN_ID/data.sql" "synthetic-gcp06-test-bucket"

for mutation in bucket name generation size sha256; do
  if GCP06_TEST_MUTATION="$mutation" bash "$ROOT_DIR/scripts/gcp06-upload-candidate-sql.sh" \
    ".local/testing/gcp06-stage-$RUN_ID/data.sql" "synthetic-gcp06-test-bucket" >/dev/null 2>&1; then
    echo "expected verification failure for mutated $mutation" >&2
    exit 1
  fi
done
echo 'PASS: gcloud custom_fields JSON accepted; bucket, object, generation, size, and SHA-256 mismatches rejected.'
