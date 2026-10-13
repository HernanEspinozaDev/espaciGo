#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
for command in npm node; do
  command -v "$command" >/dev/null 2>&1 || { printf 'required Angular QA tool not found: %s\n' "$command" >&2; exit 2; }
done

printf '%s\n' '== Angular: install locked dependencies and run the consolidated frontend regression =='
npm --prefix frontend ci --ignore-scripts --no-audit --no-fund
npm --prefix frontend test -- --watch=false
npm --prefix frontend run api:check
npm --prefix frontend run build

printf '%s\n' '== Go/backend QA (independently runnable without Node/npm) =='
"$ROOT_DIR/scripts/test-local-qa-backend.sh"
