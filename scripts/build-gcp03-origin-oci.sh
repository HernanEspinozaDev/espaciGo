#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

if [[ $# -ne 1 ]]; then
  echo "usage: $0 PRIVATE_OUTPUT_DIRECTORY" >&2
  exit 2
fi

output_dir="$1"
umask 077
mkdir -p "$output_dir"
chmod 0700 "$output_dir"
output_dir="$(cd "$output_dir" && pwd)"
oci_archive="$output_dir/gcp03origin-linux-amd64.oci.tar"
manifest="$output_dir/gcp03origin-manifest.json"
if [[ -e "$oci_archive" || -e "$manifest" ]]; then
  echo "refusing to overwrite an existing private OCI artifact or manifest" >&2
  exit 1
fi

commit="$(git rev-parse HEAD)"
if [[ -n "$(git status --porcelain --untracked-files=normal)" ]]; then
  echo "refusing to package a dirty worktree; commit the reviewed source first" >&2
  exit 1
fi

source_date_epoch="$(git show -s --format=%ct "$commit")"
builder_image='golang:1.27.1-alpine3.24@sha256:cd9a32216aee5667f957a62d13a10032a63fd58e14b3f3d9cc8c2122f501e95e'
buildx_version="$(docker buildx version)"
docker_client_version="$(docker version --format '{{.Client.Version}}')"
go_version="$(sed -n 's/^go //p' go.mod)"
[[ "$go_version" == "1.27.1" ]] || { echo "unexpected Go toolchain: $go_version" >&2; exit 1; }

docker buildx build \
  --platform linux/amd64 \
  --provenance=false \
  --sbom=false \
  --build-arg "SOURCE_DATE_EPOCH=$source_date_epoch" \
  --output "type=oci,dest=$oci_archive" \
  --file Dockerfile.gcp03origin \
  .

python3 - "$oci_archive" "$manifest" "$commit" "$go_version" "$builder_image" "$source_date_epoch" "$buildx_version" "$docker_client_version" <<'PY'
import hashlib
import json
import pathlib
import sys
import tarfile

archive, manifest_path, commit, go_version, builder_image, source_date_epoch, buildx_version, docker_client_version = sys.argv[1:]
with tarfile.open(archive, "r:") as image:
    index = json.load(image.extractfile("index.json"))
    manifests = index.get("manifests", [])
    if len(manifests) != 1 or manifests[0].get("platform") != {"architecture": "amd64", "os": "linux"}:
        raise SystemExit("expected exactly one linux/amd64 OCI manifest")
    digest = manifests[0]["digest"]
    algorithm, value = digest.split(":", 1)
    manifest_blob = image.extractfile(f"blobs/{algorithm}/{value}").read()
    if f"sha256:{hashlib.sha256(manifest_blob).hexdigest()}" != digest:
        raise SystemExit("OCI manifest digest did not match its blob")

sql_target = "espacigo-testprod:southamerica-west1:espacigo-gcp03-origin-g03o-1017-pg/espacigo_scratch"
record = {
    "schema_version": 1,
    "source_commit": commit,
    "go_toolchain": go_version,
    "builder_image": builder_image,
    "source_date_epoch": int(source_date_epoch),
    "buildx_version": buildx_version,
    "docker_client_version": docker_client_version,
    "platform": "linux/amd64",
    "oci_manifest_digest": digest,
    "oci_archive_sha256": hashlib.sha256(pathlib.Path(archive).read_bytes()).hexdigest(),
    "executor_command": "cmd/gcp03origin",
    "executor_phases": ["bootstrap", "observe"],
    "target_confirmation": sql_target,
    "execution_purpose": "GCP-03 origin bootstrap and read-only pre-restore observation; no snapshot staging or restore",
}
path = pathlib.Path(manifest_path)
path.write_text(json.dumps(record, indent=2, sort_keys=True) + "\n")
path.chmod(0o600)
pathlib.Path(archive).chmod(0o600)
print(json.dumps({"oci_manifest_digest": digest, "oci_archive_sha256": record["oci_archive_sha256"], "source_commit": commit}))
PY
