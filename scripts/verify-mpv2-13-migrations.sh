#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
BASE="${MPV2_13_BASE_SHA:-c797f0eb734e2c7eda37eb70a50d4d509e82677d}"
cd "$ROOT_DIR"
git cat-file -e "$BASE^{commit}"

mapfile -t migration_files < <(find db/migrations -maxdepth 1 -type f -name 'V*.sql' -print | sort)
count="${#migration_files[@]}"
latest="$(printf '%s\n' "${migration_files[@]}" | sed -E 's#^.*/V0*([0-9]+)__.*#\1#' | sort -n | tail -1)"
if [[ "$count" != 57 || "$latest" != 57 ]]; then
  printf 'expected exactly V1-V57; found %s migration files, latest V%s\n' "$count" "${latest:-none}" >&2
  exit 1
fi
if ! git diff --quiet "$BASE" -- db/migrations; then
  printf 'db/migrations differs from MPV2-13 base %s\n' "$BASE" >&2
  git diff --stat "$BASE" -- db/migrations >&2
  exit 1
fi
if [[ -n "$(git ls-files --others --exclude-standard -- db/migrations/V*.sql)" ]]; then
  printf 'untracked migration files are present under db/migrations\n' >&2
  exit 1
fi

printf 'PASS: V1-V57 migration files are byte-identical to %s; no new migration was added.\n' "$BASE"
git ls-files -z 'db/migrations/V*.sql' | sort -z | xargs -0 sha256sum
