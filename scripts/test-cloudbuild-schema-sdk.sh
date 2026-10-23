#!/usr/bin/env bash
# Execute the real gate command in its pinned SDK image, offline/no credentials.
set -euo pipefail
ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/espacigo-schema-sdk.XXXXXX")"
cleanup() { python3 -c 'import shutil,sys; shutil.rmtree(sys.argv[1])' "$TMP_DIR"; }
trap cleanup EXIT
python3 - "$TMP_DIR" <<'PY'
import json,pathlib,re,sys,yaml
sys.path.insert(0,'scripts')
from test_img01_schema_release_evidence import valid_report
p=pathlib.Path(sys.argv[1])
config=yaml.safe_load(pathlib.Path('cloudbuild.yaml').read_text())
step=next(s for s in config['steps'] if s['id']=='acquire-and-validate-deploy-lock')
image=step['name']
assert re.fullmatch(r'gcr.io/google.com/cloudsdktool/google-cloud-cli:stable@sha256:[a-f0-9]{64}',image), image
(p/'image').write_text(image)
script=step['args'][-1]
start=script.index('schema_python=')
end=script.index('\n',script.index('scripts/verify_img01_schema_release_evidence.py',start))
command=script[start:end].replace('$$','$').replace('/workspace/img01-schema-evidence.json','/fixtures/report.json')
(p/'gate.sh').write_text(command+'\n')
report=valid_report()
(p/'valid.json').write_text(json.dumps(report))
report['migration_ledger']['entries'][67]['checksum']='0'*64
(p/'invalid.json').write_text(json.dumps(report))
PY
IMAGE="$(cat "$TMP_DIR/image")"
run_gate() {
  docker run --rm --network none --read-only --tmpfs /tmp:rw,size=128m \
    --env CLOUDSDK_CONFIG=/tmp/gcloud --env CLOUDSDK_CORE_DISABLE_USAGE_REPORTING=true \
    --env CLOUDSDK_CORE_DISABLE_PROMPTS=1 --env PYTHONDONTWRITEBYTECODE=1 \
    --mount "type=bind,src=$ROOT_DIR,dst=/workspace,readonly" \
    --mount "type=bind,src=$TMP_DIR,dst=/fixtures,readonly" \
    --workdir /workspace --entrypoint bash "$IMAGE" -ceu \
    'source /fixtures/gate.sh'
}
cp "$TMP_DIR/valid.json" "$TMP_DIR/report.json"
run_gate
cp "$TMP_DIR/invalid.json" "$TMP_DIR/report.json"
if run_gate >"$TMP_DIR/invalid.log" 2>&1; then
  echo 'SDK gate accepted a corrupt migration checksum' >&2; exit 1
fi
python3 - "$TMP_DIR/invalid.log" <<'PYERROR'
import pathlib,sys
message=pathlib.Path(sys.argv[1]).read_text()
if 'database migration ledger has a gap, rename, or checksum drift' not in message:
    print(message,file=sys.stderr)
    raise SystemExit('SDK gate failed for a reason other than the expected ledger drift')
PYERROR
printf 'PASS: actual pinned SDK gate accepts valid evidence and rejects ledger drift without network or credentials.\n'
