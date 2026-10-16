# GCP-03 probe cleanup plan gate — 2026-10-16

Probe execution and exact-login retirement are recorded in [the execution evidence](gcp03_role_probe_execution_20261016.md). The two Cloud Run probe Jobs and their IAM bindings remain deployed pending separate cleanup approval. The migrator login is absent and Secret Manager version 3 is destroyed.

The Terraform root now has a dedicated `allow_role_probe_job_destroy` gate. It requires `confirm_role_probe_job_destroy_run_id == run_id`, is constrained to the fixed `pilot01` scratch run and `enable_job=false`, and requires `allow_scratch_destroy=false`. It changes only `google_cloud_run_v2_job.role_comment_probe[*].deletion_protection`; SQL instance/database and secret protections remain enabled.

## Staged cleanup plans

Private binary and JSON plans are stored with mode 0600 under `.local/testing/gcp03-role-probe-cleanup-plans-20261016/`.

- **Stage 1 — protection transition:** 0 add, 2 change, 0 destroy; changes only the two probe Jobs from deletion protection enabled to disabled. Binary SHA-256 `41a92b429a35dbe0918d7a3cbb787f970a2801dbcce8ce9eb7bbdd479b417dd5`; JSON SHA-256 `5b73dc3b7baa471d8d16c9a7dcb5ad23639320efe97549e6139342ba86d431fc`.
- **Stage 2 — remove probe Jobs and temporary IAM:** preview shows 0 add, 0 change, 9 destroy: two Jobs, two `run.invoker` bindings, one `iam.serviceAccountUser` binding, two temporary custom roles, and two temporary credential IAM bindings. It does not plan Cloud SQL, database or Secret Manager secret changes. Binary SHA-256 `521cc35714767ef50711226a3df43ec839e228a019f97bb85145c490cfb9ade0`; JSON SHA-256 `fbeaa4ea84a7973a434c0ebf352c6c6a690abdcb22d9a584bb703c58b3895c1a`.

The stage-2 binary is a review preview against the current state, where Job deletion protection is still enabled. **Do not apply it.** If stage 1 is separately reviewed and applied, generate a fresh stage-2 plan against that resulting state and review its exact diff and SHA before any cleanup apply. Neither cleanup stage was applied in this work.
