#!/usr/bin/env bash
# Execute the actual versioned-attestation gate in its pinned SDK image, offline.
set -euo pipefail
ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/espacigo-schema-sdk.XXXXXX")"
cleanup() { python3 -c 'import shutil,sys; shutil.rmtree(sys.argv[1])' "$TMP_DIR"; }
trap cleanup EXIT

python3 - "$TMP_DIR" <<'PY'
from datetime import datetime, timedelta, timezone
import hashlib
import json
import pathlib
import re
import shutil
import sys
import yaml

sys.path.insert(0, 'scripts')
from test_img01_schema_release_evidence import valid_report
from resolve_img01_schema_release_attestation import ATTESTATION_FORMAT

tmp=pathlib.Path(sys.argv[1])
config=yaml.safe_load(pathlib.Path('cloudbuild.yaml').read_text())
step=next(s for s in config['steps'] if s['id']=='acquire-and-validate-deploy-lock')
image=step['name']
assert re.fullmatch(r'gcr.io/google.com/cloudsdktool/google-cloud-cli:stable@sha256:[a-f0-9]{64}', image), image
script=step['args'][-1]
start=script.index('# BEGIN schema-attestation gate') + len('# BEGIN schema-attestation gate')
end=script.index('# END schema-attestation gate', start)
gate=script[start:end].replace('$$','$').strip() + '\n'
(tmp/'image').write_text(image)
(tmp/'gate.sh').write_text(gate)
workspace=tmp/'workspace'
(workspace/'scripts').mkdir(parents=True)
(workspace/'db/schema-evidence').mkdir(parents=True)
(workspace/'db/query').mkdir(parents=True)
(workspace/'db/migrations').mkdir(parents=True)
for name in ('resolve_img01_schema_release_attestation.py','verify_img01_schema_release_evidence.py'):
    shutil.copy(pathlib.Path('scripts')/name, workspace/'scripts'/name)
shutil.copy(pathlib.Path('db/query/img01_schema_release_evidence.sql'), workspace/'db/query/img01_schema_release_evidence.sql')
for path in pathlib.Path('db/migrations').glob('*.sql'):
    shutil.copy(path, workspace/'db/migrations'/path.name)

report=valid_report()
report['captured_at']=(datetime.now(timezone.utc)-timedelta(days=30)).isoformat()
reference=json.loads(pathlib.Path('db/schema-evidence/img01-release-attestation.json').read_text())
reference['captured_at']=report['captured_at']
reference['validation_source_commit']='9c00127d5a07145757892e21e72336756c325ef2'
report_bytes=(json.dumps(report,separators=(',',':'))+'\n').encode()
digest=hashlib.sha256(report_bytes).hexdigest()
reference['evidence_sha256']=digest
reference['evidence_uri']=f"gs://espacigo-testprod-cloud-build-447373903153/img01-schema-evidence/{digest}.json"
reference['evidence_generation']='1234567890123456'
(workspace/'db/schema-evidence/img01-release-attestation.json').write_text(json.dumps(reference))
(tmp/'valid-manifest.json').write_text(json.dumps(reference))
(tmp/'report.json').write_bytes(report_bytes)
(tmp/'generation').write_text(reference['evidence_generation'])
digest_mismatch=json.loads(json.dumps(reference))
digest_mismatch['evidence_sha256']='a'*64
digest_mismatch['evidence_uri']=f"gs://espacigo-testprod-cloud-build-447373903153/img01-schema-evidence/{digest_mismatch['evidence_sha256']}.json"
(tmp/'digest-mismatch-manifest.json').write_text(json.dumps(digest_mismatch))
generation_mismatch=json.loads(json.dumps(reference))
generation_mismatch['evidence_generation']='1234567890123457'
(tmp/'generation-mismatch-manifest.json').write_text(json.dumps(generation_mismatch))

invalid=json.loads(json.dumps(report))
invalid['migration_ledger']['entries'][67]['checksum']='0'*64
invalid_bytes=(json.dumps(invalid,separators=(',',':'))+'\n').encode()
invalid_digest=hashlib.sha256(invalid_bytes).hexdigest()
reference['evidence_sha256']=invalid_digest
reference['evidence_uri']=f"gs://espacigo-testprod-cloud-build-447373903153/img01-schema-evidence/{invalid_digest}.json"
(tmp/'invalid-manifest.json').write_text(json.dumps(reference))
(tmp/'invalid.json').write_bytes(invalid_bytes)

stub=tmp/'bin/gcloud'
stub.parent.mkdir()
stub.write_text('''#!/usr/bin/env bash
set -euo pipefail
case "$1 $2" in
  "info --format=value(basic.python_location)") printf '%s\\n' "$ATTESTATION_TEST_PYTHON" ;;
  "storage objects")
    printf 'describe\\n' >> "${ATTESTATION_TEST_CALL_LOG:?}"
    [[ "$3" == describe ]]
    cat "${ATTESTATION_TEST_GENERATION_FILE:?}"
    ;;
  "storage cp")
    printf 'cp\\n' >> "${ATTESTATION_TEST_CALL_LOG:?}"
    last="${!#}"
    cp "${ATTESTATION_TEST_REPORT_FILE:?}" "$last"
    ;;
  *) echo "unexpected gcloud invocation in offline gate test" >&2; exit 64 ;;
esac
''')
stub.chmod(0o755)
PY

IMAGE="$(cat "$TMP_DIR/image")"
SOURCE_COMMIT="$(git rev-parse HEAD)"
run_gate() {
  local report_file="$1" manifest_file="$2" log_file="$3"
  local workspace_manifest="$TMP_DIR/workspace/db/schema-evidence/img01-release-attestation.json"
  : > "$TMP_DIR/workspace/gcloud-calls.log"
  if [[ "$manifest_file" != "$workspace_manifest" ]]; then
    cp "$manifest_file" "$workspace_manifest"
  fi
  if docker run --rm --network none --read-only --tmpfs /tmp:rw,size=128m \
    --env PATH="/fixtures/bin:/usr/local/bin:/usr/bin:/bin" \
    --env PYTHONDONTWRITEBYTECODE=1 \
    --env COMMIT_SHA="$SOURCE_COMMIT" \
    --env ATTESTATION_TEST_PYTHON=/usr/lib/google-cloud-sdk/platform/bundledpythonunix/bin/python3 \
    --env ATTESTATION_TEST_GENERATION_FILE=/fixtures/generation \
    --env ATTESTATION_TEST_CALL_LOG=/workspace/gcloud-calls.log \
    --env ATTESTATION_TEST_REPORT_FILE="/fixtures/$(basename "$report_file")" \
    --mount "type=bind,src=$TMP_DIR/workspace,dst=/workspace" \
    --mount "type=bind,src=$TMP_DIR,dst=/fixtures,readonly" \
    --workdir /workspace --entrypoint bash "$IMAGE" -ceu 'source /fixtures/gate.sh' >"$log_file" 2>&1; then
    return 0
  else
    cat "$log_file" >&2
    return 1
  fi
}

run_gate "$TMP_DIR/report.json" "$TMP_DIR/workspace/db/schema-evidence/img01-release-attestation.json" "$TMP_DIR/valid.log"
workspace_manifest="$TMP_DIR/workspace/db/schema-evidence/img01-release-attestation.json"
rm "$workspace_manifest"
if run_gate "$TMP_DIR/report.json" "$workspace_manifest" "$TMP_DIR/missing-manifest.log"; then
  echo 'SDK gate accepted a missing attestation manifest' >&2; exit 1
fi
if [[ -s "$TMP_DIR/workspace/gcloud-calls.log" ]]; then
  echo 'SDK gate accessed GCS without a valid attestation manifest' >&2; exit 1
fi
cp "$TMP_DIR/valid-manifest.json" "$workspace_manifest"
if run_gate "$TMP_DIR/report.json" "$TMP_DIR/generation-mismatch-manifest.json" "$TMP_DIR/generation-mismatch.log"; then
  echo 'SDK gate accepted a mismatched GCS generation' >&2; exit 1
fi
if grep -q '^cp$' "$TMP_DIR/workspace/gcloud-calls.log"; then
  echo 'SDK gate downloaded evidence before rejecting its GCS generation' >&2; exit 1
fi
if run_gate "$TMP_DIR/report.json" "$TMP_DIR/digest-mismatch-manifest.json" "$TMP_DIR/digest-mismatch.log"; then
  echo 'SDK gate accepted a GCS body with the wrong digest' >&2; exit 1
fi
if ! grep -q '^cp$' "$TMP_DIR/workspace/gcloud-calls.log"; then
  echo 'SDK gate did not download the object before validating its body digest' >&2; exit 1
fi
run_gate "$TMP_DIR/invalid.json" "$TMP_DIR/invalid-manifest.json" "$TMP_DIR/invalid.log" && {
  echo 'SDK gate accepted a corrupt migration checksum' >&2; exit 1;
}
python3 - "$TMP_DIR/invalid.log" <<'PYERROR'
import pathlib,sys
message=pathlib.Path(sys.argv[1]).read_text()
if 'database migration ledger has a gap, rename, or checksum drift' not in message:
    print(message,file=sys.stderr)
    raise SystemExit('SDK gate failed for a reason other than the expected ledger drift')
PYERROR
printf 'PASS: automatic manifest resolution passes without evidence flags after 30 days, fails closed on missing manifest, generation/body hash mismatch and ledger drift, offline in the pinned SDK image.\n'
