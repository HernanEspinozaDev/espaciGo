# GCP-03 role-comment probe execution and retirement — 2026-10-16

## Change and apply

PR #352 was approved and merged by `HernanEspinozaDev` at HEAD `11efed484f99edaad41c712dcfc64254890b13f5`, merge commit `53fd28fecdd38ec20cd490a7e56f471bbe2937a7`. The approved Terraform binary plan SHA-256 was `d7646ed81cb7105b24cbdf841b1bb289409e2bf55693b02b11e5773c46493800`; its checksum was verified. Applying it produced exactly 5 additions, 0 changes and 0 deletions. A refreshed pre-apply plan showed the same five creates. A post-apply plan showed no changes.

The new resources were two probe Jobs, their two `roles/run.invoker` bindings, and one `roles/iam.serviceAccountUser` binding. Both Jobs were Ready, pinned to image digest `sha256:3e5683bf557a009d0ac14f60575f1054f9c089239b9e65fae27ab8acadae3548`, mounted the exact migrator secret version 3, and used one task, parallelism 1, a 600-second timeout, zero retries, 1 CPU and 512 MiB. No migration Job was created.

## Probe results

- `role_comment_prepare` ran exactly once as execution `espacigo-gcp03-pilot01-role-comment-prepare-jwwts`; completed successfully with one succeeded task. It reported PostgreSQL `180006`, a non-superuser `CREATEROLE` login, and the fixed `espacigo_gadm_bootstrap` role with LOGIN/SUPERUSER/CREATEDB/CREATEROLE/REPLICATION/BYPASSRLS/INHERIT all false. Membership counts were zero. The fixed role COMMENT succeeded; SQLSTATE was empty.
- The reviewed creator-membership reconcile operation `e928ae45-bf9a-4789-94d0-3c690000004b` (`UPDATE_USER`) completed `DONE`; Cloud SQL confirmed the migrator login had only `cloudsqlsuperuser`.
- `role_comment_after_reconcile` ran exactly once as execution `espacigo-gcp03-pilot01-role-comment-after-reconcile-js4ch`; completed successfully with one succeeded task. It reported the same safe role attributes and zero incoming/outgoing memberships. The fixed COMMENT succeeded; SQLSTATE was empty.
- Retirement operation `35e2e7b6-2da2-4b89-b429-78940000004b` (`DELETE_USER`) completed `DONE`. A subsequent Cloud SQL user list confirmed the migrator login absent; Secret Manager version 3 is `DESTROYED`.

The probe's fixed role creation and one fixed `COMMENT ON ROLE` per phase were the only SQL statements run. No V1–V61 migrations, application-data SQL, imports/restores, or database-owner credential operations were performed. The `42501` permission-denied result was an allowed experimental outcome, but this run returned `succeeded` with no SQLSTATE; neither outcome establishes V60 compatibility by itself.

## Cleanup review plans

A dedicated `allow_role_probe_job_destroy` gate controls only probe Job deletion protection and requires exact `pilot01` confirmation; `allow_scratch_destroy=false`. PR #353 was approved/merged at HEAD `9bdb8dd50d799c30b8fec9f1d203ef08940a36dc`, merge `01803d7c00111e160bd38807817844216ec20369`.

Stage 1 was separately authorized and applied from the exact reviewed plan after a current no-drift preflight. The apply produced **0 add, 2 change, 0 destroy**, and changed only `deletion_protection=true` to `false` on the two probe Jobs. Cloud SQL instance protection remained enabled. A refreshed plan with the stage-1 variables returned no changes.

The post-stage-1 stage-2 plan is private under `.local/testing/gcp03-role-probe-cleanup-plans-20261016/`, mode 0600:

- Exact delta: **0 add, 0 change, 9 destroy** — two Jobs, two `roles/run.invoker` bindings, one `roles/iam.serviceAccountUser` binding, two custom roles and two temporary credential IAM bindings. Both Job state objects already have `deletion_protection=false`. No SQL instance, database or Secret Manager secret resource changes are planned.
- Binary SHA-256: `81b64f3956b1e986b717bc7deb5b939e4cf11ead65e9ceead98263ea11ef1483`.
- JSON SHA-256: `6af3558b22ad63fbdf89656170fc15a9205d8ee0b62905dd6864087acae66196`.
- The older stage-2 preview (`521cc357...`) was generated before stage 1 and is stale; do not apply it.

The exact fresh stage-2 binary plan above was applied after confirming the explicit active project, scratch state and no-drift, both probe executions complete, migrator login absent, secret version 3 destroyed, SQL deletion protection enabled, and the exact plan/JSON hashes. It completed **0 add, 0 change, 9 destroy**. Post-apply Cloud Run lookups returned `Cannot find job` for both probe Jobs; project and secret IAM policy reads showed no binding to either temporary custom role, and the migrator service-account policy had no operator `actAs` binding. Both custom-role queries with `--show-deleted` returned NotFound/absent. A fresh Terraform plan with the stage-2 variables returned no changes, and Terraform state no longer contains the nine deleted addresses. The scratch instance remained `RUNNABLE`, private and protected; the database, metadata-only secret resources, both runtime service accounts, Artifact Registry repository, VPC/subnet, and destroyed state of secret version 3 were preserved.

The apply log, SHA checks, API readbacks and Terraform no-drift output remain private in `.local/testing/gcp03-role-probe-cleanup-plans-20261016/stage2-apply-preflight-20261016/` with directory mode 0700 and file mode 0600. No SQL, migration Job, new credential/version, import or restore was performed. Google custom-role deletion is soft-delete; both exact role IDs were checked absent with `--show-deleted`; preserve their identifiers and do not reuse them until retention/undelete behavior is known. The former credential IAM expiry was not renewed.

## Evidence and limits

The plan, apply logs, structured Job descriptions, execution summaries, filtered probe result lines, API state checks and cleanup plan artifacts are private under `.local/testing/` with directory mode 0700 and file mode 0600. Tokens, passwords, raw request bodies and full unfiltered logs are not committed. Evidence does not imply product acceptance or V60 compatibility.
