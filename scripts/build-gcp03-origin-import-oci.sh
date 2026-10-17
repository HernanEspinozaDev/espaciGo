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
oci_archive="$output_dir/gcp03origin-import-linux-amd64.oci.tar"
manifest="$output_dir/gcp03origin-import-manifest.json"
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
python_image='python:3.13.16-slim-trixie@sha256:bf44cdfcb76cd3b41e879bc058fc37ec5872002ccfde7fcb765e218cde0cd79c'
postgres_image='postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
buildx_version="$(docker buildx version)"
docker_client_version="$(docker version --format '{{.Client.Version}}')"

docker buildx build \
  --platform linux/amd64 \
  --provenance=false \
  --sbom=false \
  --build-arg "SOURCE_DATE_EPOCH=$source_date_epoch" \
  --output "type=oci,dest=$oci_archive" \
  --file Dockerfile.gcp03origin-import \
  .

python3 - "$oci_archive" "$manifest" "$commit" "$python_image" "$postgres_image" "$source_date_epoch" "$buildx_version" "$docker_client_version" <<'PY'
import hashlib
import json
import pathlib
import sys
import tarfile

archive, manifest_path, commit, python_image, postgres_image, source_date_epoch, buildx_version, docker_client_version = sys.argv[1:]
with tarfile.open(archive, "r:") as image:
    index = json.load(image.extractfile("index.json"))
    manifests = index.get("manifests", [])
    if len(manifests) != 1 or manifests[0].get("platform") != {"architecture": "amd64", "os": "linux"}:
        raise SystemExit("expected exactly one linux/amd64 OCI manifest")
    digest = manifests[0]["digest"]
    algorithm, value = digest.split(":", 1)
    blob = image.extractfile(f"blobs/{algorithm}/{value}").read()
    if f"sha256:{hashlib.sha256(blob).hexdigest()}" != digest:
        raise SystemExit("OCI manifest digest did not match its blob")

record = {
    "schema_version": 1,
    "source_commit": commit,
    "python_base_image": python_image,
    "postgres_client_image": postgres_image,
    "python_dependencies_lock_sha256": hashlib.sha256(pathlib.Path("scripts/requirements-gcp03-origin-executor.txt").read_bytes()).hexdigest(),
    "source_date_epoch": int(source_date_epoch),
    "buildx_version": buildx_version,
    "docker_client_version": docker_client_version,
    "platform": "linux/amd64",
    "oci_manifest_digest": digest,
    "oci_archive_sha256": hashlib.sha256(pathlib.Path(archive).read_bytes()).hexdigest(),
    "executor_command": "scripts/gcp03_origin_cloud_import_executor.py",
    "executor_phases": ["authenticate", "import", "sanitize", "private-files-copy"],
    "target_confirmation": "espacigo-testprod:southamerica-west1:espacigo-gcp03-origin-g03o-1017-pg/espacigo_scratch",
    "execution_purpose": "one-shot GCP-03 origin import; no application services or workers",
}
path = pathlib.Path(manifest_path)
path.write_text(json.dumps(record, indent=2, sort_keys=True) + "\n")
path.chmod(0o600)
pathlib.Path(archive).chmod(0o600)
print(json.dumps({"oci_manifest_digest": digest, "oci_archive_sha256": record["oci_archive_sha256"], "source_commit": commit}))
PY
