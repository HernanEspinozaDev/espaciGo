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
oci_archive="$output_dir/gcp03scratch-linux-amd64.oci.tar"
manifest="$output_dir/gcp03scratch-manifest.json"

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
  --file Dockerfile.dbmigrate \
  .

python3 - "$oci_archive" "$manifest" "$commit" "$go_version" "$builder_image" "$source_date_epoch" "$buildx_version" "$docker_client_version" <<'PY'
import hashlib
import json
import pathlib
import sys
import tarfile

archive, manifest_path, commit, go_version, builder_image, source_date_epoch, buildx_version, docker_client_version = sys.argv[1:]
migrations = sorted(pathlib.Path("db/migrations").glob("V*.sql"))
if len(migrations) != 61:
    raise SystemExit(f"expected exactly V1-V61, found {len(migrations)} migration files")
expected = [f"V{i:06d}__" for i in range(1, 62)]
for path, prefix in zip(migrations, expected, strict=True):
    if not path.name.startswith(prefix):
        raise SystemExit(f"migration inventory mismatch at {path.name}, expected prefix {prefix}")

migration_hashes = {
    path.as_posix(): hashlib.sha256(path.read_bytes()).hexdigest()
    for path in migrations
}
aggregate_hash = hashlib.sha256()
for path, digest in migration_hashes.items():
    aggregate_hash.update(f"{digest}  {path}\n".encode())
aggregate = aggregate_hash.hexdigest()
bootstrap_sql = sorted(pathlib.Path("db/bootstrap/gcp03scratch").glob("*.sql"))
if [path.name for path in bootstrap_sql] != ["001_pre_migrate.sql", "002_retire_migrator.sql", "003_database_acl.sql"]:
    raise SystemExit("unexpected GCP-03 scratch SQL phase inventory")
bootstrap_hashes = {
    path.as_posix(): hashlib.sha256(path.read_bytes()).hexdigest()
    for path in bootstrap_sql
}
bootstrap_hash = hashlib.sha256()
for path, digest in bootstrap_hashes.items():
    bootstrap_hash.update(f"{digest}  {path}\n".encode())
bootstrap_aggregate = bootstrap_hash.hexdigest()
with tarfile.open(archive, "r:") as image:
    index = json.load(image.extractfile("index.json"))
    manifests = index.get("manifests", [])
    if len(manifests) != 1:
        raise SystemExit(f"expected one OCI manifest, found {len(manifests)}")
    if manifests[0].get("platform") != {"architecture": "amd64", "os": "linux"}:
        raise SystemExit(f"unexpected OCI platform: {manifests[0].get('platform')}")
    digest = manifests[0]["digest"]
    algorithm, value = digest.split(":", 1)
    blob_path = f"blobs/{algorithm}/{value}"
    manifest_blob = image.extractfile(blob_path).read()
    calculated = f"sha256:{hashlib.sha256(manifest_blob).hexdigest()}"
    if calculated != digest:
        raise SystemExit("OCI manifest digest did not match its blob")

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
    "executor_command": "cmd/gcp03scratch",
    "executor_phase_names": ["bootstrap", "database_acl", "migrate", "finalize", "retire", "role_comment_prepare", "role_comment_after_reconcile"],
    "scratch_target_confirmation": "espacigo-testprod:southamerica-west1:espacigo-gcp03-pilot01-pg/espacigo_scratch",
    "migration_count": len(migrations),
    "migration_inventory_sha256": aggregate,
    "migration_sha256": migration_hashes,
    "extensions_expected": ["btree_gist", "postgis", "unaccent"],
    "bootstrap_sql_sha256": bootstrap_hashes,
    "bootstrap_sql_inventory_sha256": bootstrap_aggregate,
    "execution_purpose": "GCP-03 isolated Cloud SQL scratch compatibility rehearsal; no production data",
}
path = pathlib.Path(manifest_path)
path.write_text(json.dumps(record, indent=2, sort_keys=True) + "\n")
path.chmod(0o600)
pathlib.Path(archive).chmod(0o600)
print(json.dumps({"oci_manifest_digest": digest, "migration_count": len(migrations), "migration_inventory_sha256": aggregate, "bootstrap_sql_inventory_sha256": bootstrap_aggregate}))
PY
