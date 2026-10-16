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

A dedicated `allow_role_probe_job_destroy` gate now controls only the two probe Jobs. It requires an exact `pilot01` confirmation and requires `allow_scratch_destroy=false`. Tests assert that Cloud SQL deletion protection remains enabled.

Private plans under `.local/testing/gcp03-role-probe-cleanup-plans-20261016/` have mode 0600:

1. Stage 1 disables deletion protection on the two probe Jobs only: **0 add, 2 change, 0 destroy**. Binary SHA-256 `41a92b429a35dbe0918d7a3cbb787f970a2801dbcce8ce9eb7bbdd479b417dd5`; JSON SHA-256 `5b73dc3b7baa471d8d16c9a7dcb5ad23639320efe97549e6139342ba86d431fc`.
2. Stage 2 previews deletion of the two Jobs, two invoker bindings, one actAs binding, two temporary custom roles and two temporary credential IAM bindings: **0 add, 0 change, 9 destroy**. It changes no Cloud SQL, database or Secret Manager secret resources. Binary SHA-256 `521cc35714767ef50711226a3df43ec839e228a019f97bb85145c490cfb9ade0`; JSON SHA-256 `fbeaa4ea84a7973a434c0ebf352c6c6a690abdcb22d9a584bb703c58b3895c1a`.

Neither cleanup plan was applied. Stage 2 was generated against the pre-stage-1 state; it is a scope preview only and must not be applied. After a separately reviewed stage-1 apply, regenerate and re-review the stage-2 plan against the new state. The temporary credential IAM expiry remains `2026-10-16T15:26:16Z`; it was not renewed.

## Evidence and limits

The plan, apply logs, structured Job descriptions, execution summaries, filtered probe result lines, API state checks and cleanup plan artifacts are private under `.local/testing/` with directory mode 0700 and file mode 0600. Tokens, passwords, raw request bodies and full unfiltered logs are not committed. Evidence does not imply product acceptance or V60 compatibility.
