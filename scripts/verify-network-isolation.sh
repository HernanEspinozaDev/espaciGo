#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
export LOCAL_UID="$(id -u)"
export LOCAL_GID="$(id -g)"
export LOCAL_M03_EVIDENCE_DIR="${ROOT_DIR}/.local/testing/evidence"
export LOCAL_PRIVACY_REPLAY_DIR="${ROOT_DIR}/.local/testing/privacy"
mkdir -p "$LOCAL_M03_EVIDENCE_DIR" "$LOCAL_PRIVACY_REPLAY_DIR"

CONFIG_JSON="$(docker compose -f "${ROOT_DIR}/compose.yaml" config --format json)"

python3 -c 'import json, sys
data = json.loads(sys.argv[1])
services = data.get("services", {})

frontend_services = ["angular-frontend", "frontend-next"]
found_frontends = 0

for svc_name in frontend_services:
    svc = services.get(svc_name)
    if not svc:
        continue
    found_frontends += 1
    networks = svc.get("networks", {})
    net_names = list(networks.keys()) if isinstance(networks, dict) else (networks or [])

    if "data" in net_names:
        print(f"FAIL: {svc_name} must NOT be attached to data network", file=sys.stderr)
        sys.exit(1)

    if "api" not in net_names:
        print(f"FAIL: {svc_name} must be attached to api network", file=sys.stderr)
        sys.exit(1)

    env = svc.get("environment", {})
    env_keys = list(env.keys()) if isinstance(env, dict) else [e.split("=")[0] for e in env if "=" in e]
    for key in env_keys:
        if any(bad in key.upper() for bad in ["DATABASE", "POSTGRES", "DB_PASS", "DB_USER"]):
            print(f"FAIL: {svc_name} contains database environment variable: {key}", file=sys.stderr)
            sys.exit(1)

if found_frontends == 0:
    print("FAIL: No frontend services found in compose configuration", file=sys.stderr)
    sys.exit(1)

db = services.get("database")
if db:
    db_nets = list(db.get("networks", {}).keys()) if isinstance(db.get("networks"), dict) else db.get("networks", [])
    if "api" in db_nets or "public" in db_nets:
        print("FAIL: database service must NOT be on public/api network", file=sys.stderr)
        sys.exit(1)

backend = services.get("backend")
if backend:
    b_nets = list(backend.get("networks", {}).keys()) if isinstance(backend.get("networks"), dict) else backend.get("networks", [])
    if "data" not in b_nets or "api" not in b_nets:
        print("FAIL: backend must be on both data and api networks", file=sys.stderr)
        sys.exit(1)

print("PASS: Network topology and frontend isolation verified successfully")
' "$CONFIG_JSON"
