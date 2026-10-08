#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
STATE_ROOT="${XDG_STATE_HOME:-$HOME/.local/state}/espacigo/privacy-replay"
REGISTRY_DEFAULT="$STATE_ROOT/completed-suppressions-v1.json"
export LOCAL_UID="$(id -u)" LOCAL_GID="$(id -g)"
export LOCAL_M03_EVIDENCE_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/espacigo/m03-evidence"
export LOCAL_PRIVACY_REPLAY_DIR="$STATE_ROOT"
compose() { docker compose --project-directory "$ROOT_DIR" -f "$ROOT_DIR/compose.yaml" "$@"; }

usage() {
  cat >&2 <<'EOF'
Usage:
  scripts/privacy-local-replay.sh export [registry.json]
  scripts/privacy-local-replay.sh replay <restore-uuid> <active-admin-account-uuid> [registry.json]
  scripts/privacy-local-replay.sh purge

Export before creating a local backup. After restoring an older local backup,
replay the protected registry against the restored database.
EOF
}

command="${1:-}"
case "$command" in
  export)
    registry="${2:-$REGISTRY_DEFAULT}"
    registry_dir="$(dirname -- "$registry")"
    if [[ "$registry" == "$REGISTRY_DEFAULT" ]]; then
      [[ ! -L "$registry_dir" ]] || { echo 'Default registry parent cannot be a symlink.' >&2; exit 2; }
      mkdir -p "$registry_dir"
      chmod 0700 "$registry_dir"
    elif [[ ! -d "$registry_dir" || -L "$registry_dir" ]]; then
      echo 'Custom registry parent must already exist and cannot be a symlink.' >&2
      exit 2
    fi
    dir_mode="$(stat -c '%a' "$registry_dir")"
    (( (8#$dir_mode & 077) == 0 )) || { echo 'Registry parent must have mode 0700 or stricter.' >&2; exit 2; }
    dir_owner="$(stat -c '%u' "$registry_dir")"
    [[ "$dir_owner" == "$(id -u)" ]] || { echo 'Registry parent must be owned by the current user.' >&2; exit 2; }
    resolved="$(realpath -m -- "$registry")"
    case "$resolved" in "$ROOT_DIR"/*) echo 'Registry must be outside the repository.' >&2; exit 2;; esac
    [[ ! -L "$registry" ]] || { echo 'Registry symlinks are not accepted.' >&2; exit 2; }
    umask 077
    temporary="${registry}.tmp.$$"
    trap 'rm -f -- "$temporary"' EXIT
    compose exec -T backend /api local-privacy-replay-export > "$temporary"
    chmod 0600 "$temporary"
    mv -f -- "$temporary" "$registry"
    trap - EXIT
    echo "Replay registry exported with mode 0600: $registry"
    ;;
  replay)
    [[ $# -ge 3 && $# -le 4 ]] || { usage; exit 2; }
    restore_id="$2" actor_id="$3" registry="${4:-$REGISTRY_DEFAULT}"
    [[ "$restore_id" =~ ^[0-9a-fA-F-]{36}$ && "$actor_id" =~ ^[0-9a-fA-F-]{36}$ ]] || { echo 'Restore and administrator IDs must be UUIDs.' >&2; exit 2; }
    [[ -f "$registry" && ! -L "$registry" ]] || { echo 'Replay registry file is missing or unsafe.' >&2; exit 2; }
    mode="$(stat -c '%a' "$registry")"
    (( (8#$mode & 077) == 0 )) || { echo 'Replay registry must have mode 0600 or stricter.' >&2; exit 2; }
    compose exec -T backend /api local-privacy-replay-apply "$restore_id" "$actor_id" < "$registry"
    ;;
  purge)
    [[ $# -eq 1 ]] || { usage; exit 2; }
    compose exec -T backend /api local-privacy-retention-purge
    ;;
  *) usage; exit 2 ;;
esac
