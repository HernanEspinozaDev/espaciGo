#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"
output_dir="$repo_root/.local/testing/gcp04-w7-b1/runtime-build"
install -d -m 0700 "$output_dir" "$output_dir/exported"
docker buildx build --platform linux/amd64 --file Dockerfile.gcp04-w7-b1-runtime --target binary-export --output "type=local,dest=$output_dir/exported" .
find "$output_dir/exported" -type d -exec chmod 0700 {} +
find "$output_dir/exported" -type f -exec chmod 0600 {} +
chmod 0700 "$output_dir/exported"
sha256sum "$output_dir/exported/gcp04-w7-b1-probe" > "$output_dir/binary-sha256.txt"
chmod 0600 "$output_dir/binary-sha256.txt"
docker buildx build \
  --platform linux/amd64 \
  --file Dockerfile.gcp04-w7-b1-runtime \
  --tag espacigo-gcp04-w7-b1-probe:local \
  --load \
  .
docker image save --output "$output_dir/gcp04-w7-b1-probe.tar" espacigo-gcp04-w7-b1-probe:local
chmod 0600 "$output_dir/gcp04-w7-b1-probe.tar"
docker image inspect espacigo-gcp04-w7-b1-probe:local --format 'local_image_id={{.Id}}' > "$output_dir/local-image-id.txt"
sha256sum Dockerfile.gcp04-w7-b1-runtime cmd/gcp04w7b1probe/main.go db/query/gcp04_w7_b1_actor_probe.sql > "$output_dir/source-sha256.txt"
chmod 0600 "$output_dir/local-image-id.txt" "$output_dir/source-sha256.txt"
printf 'Local image saved under .local/testing/gcp04-w7-b1/runtime-build; it was not pushed.\n'
