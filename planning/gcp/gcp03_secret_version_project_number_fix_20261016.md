# GCP-03 Secret Manager version namespace fix — 2026-10-16

## Incident and reconciliation record

PR #349 (`148dc31655e03bb8aa7bddb8da84c4a286bf27b3`, merged as part of `daddf9d852d42b5d4583b8d0b2fa3084aac2dd91`) authorized and applied the exact Terraform binary plan with SHA-256 `bad05c2acbc07a03ba3b9c5cdb84b81e72f7d3a0fe9ad00878ba9cb168c8ca9b`. Terraform reported four additions, zero changes, zero deletions. A fresh plan immediately after apply reported no changes. The temporary project-scoped Cloud SQL role, exact-secret Secret Manager role, and their expiring bindings remain in place until `2026-10-16T15:26:16Z`; do not renew them silently.

The one authorized invocation of `gcp03scratch-credential --apply` failed after Secret Manager created version 1 and before Cloud SQL user creation. The API returned canonical resource `projects/447373903153/secrets/espacigo-gcp03-pilot01-migrator-password/versions/1`; the CLI incorrectly accepted only the textual project ID. Read-only checks confirmed the migrator login was absent, version 1 was `ENABLED`, and no Cloud Run Jobs existed. Under the explicit reconciliation release, only that version was destroyed. Post-check metadata reports the same canonical resource as `DESTROYED`; no payload was read. No other secret version or secret container was modified.

No SQL statement or Cloud Run Job was executed. No Cloud SQL migrator login exists. The pre-existing project Owner grant remains unchanged, so these temporary bindings do not prove isolation from that broader identity.

Private pre/post metadata, the original approved plan and apply logs, CLI error output, user listing, and Job listing are retained with mode 0600 under `.local/testing/gcp03-credential-iam-plan-20261016/`.

## Fix

Secret version validation now accepts only the fixed migrator or database-owner secret, in the exact project by project ID or verified project number, with a canonical positive decimal version number. It rejects other projects, other secrets, `0`, aliases such as `latest`, suffixes, leading-zero variants, and extra path components. The original API resource name is preserved after validation so later read/destroy calls use the canonical name returned by Google.

The check is shared by Secret Manager `AddVersion`, scratch provisioning, owner-credential provisioning, and the fixed-version retirement guards. Unexpected responses stop before Cloud SQL user or owner-password calls. When an unexpected response is in the exact scratch namespace, the CLI returns a sanitized canonical reference and marks reconciliation required; references outside the allowed namespace are not printed or operated on.

## Verification

- `go test -count=1 ./internal/gcp03scratch ./cmd/gcp03scratch-credential`: passed.
- Tests cover realistic Secret Manager REST responses using both project ID and project number, wrong project and secret, aliases, zero, owner-version handling, and prevention of Cloud SQL creation for invalid responses.
- Existing Cloud SQL API adapter tests remain present and pass.
- `git diff --check`: passed.

This fix does not retry credential creation, create a new version or login, run SQL or Jobs, alter the Owner grant, or change the temporary IAM expiry. A new credential creation requires review of this fix and a separate explicit release; any next credential version must be read from the API and used by its numeric ID, never assumed to be version 1.
