#!/usr/bin/env bash
set -euo pipefail

: "${TF_STATE_BUCKET:?Set TF_STATE_BUCKET to the canonical GCS backend bucket.}"

root="infra/terraform/environments/testprod/gcp"
work_dir="$(mktemp -d "${TMPDIR:-/tmp}/gcp07-state-backend.XXXXXX")"
chmod 700 "$work_dir"
trap 'rm -rf "$work_dir"' EXIT

export TF_DATA_DIR="$work_dir/terraform-data"
export TF_IN_AUTOMATION=true
export TF_INPUT=false

if ! terraform -chdir="$root" init \
  -backend-config="bucket=$TF_STATE_BUCKET" \
  -backend-config="prefix=espacigo/testprod/gcp" \
  -input=false -lockfile=readonly >"$work_dir/init.log" 2>&1; then
  echo "GCP-07 state backend initialization failed; command output was suppressed." >&2
  exit 1
fi

if ! workspaces="$(terraform -chdir="$root" workspace list -no-color 2>/dev/null)"; then
  echo "GCP-07 state workspace listing failed; command output was suppressed." >&2
  exit 1
fi

workspace_lines="$(printf '%s\n' "$workspaces" | sed '/^[[:space:]]*$/d' | sed 's/^[*[:space:]]*//;s/[[:space:]]*$//')"
if [[ "$workspace_lines" != "default" ]]; then
  echo "GCP-07 state backend is reachable, but the expected default workspace was not the only listed workspace." >&2
  exit 1
fi

echo "GCP-07 state backend initialization and default workspace listing passed; state contents were not displayed."
