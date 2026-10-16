# GCP-03 probe cleanup plan gate — 2026-10-16

Probe execution, exact-login retirement, and cleanup are recorded in [the execution evidence](gcp03_role_probe_execution_20261016.md). Stage 1 disabled deletion protection only on the two probe Jobs. Stage 2 has since removed both Jobs and their temporary IAM bindings/roles. The scratch Cloud SQL instance, database, Secret Manager secret resources, service accounts, Artifact Registry repository, and VPC remain intact. The migrator login is absent and Secret Manager version 3 is destroyed.

The Terraform root now has a dedicated `allow_role_probe_job_destroy` gate. It requires `confirm_role_probe_job_destroy_run_id == run_id`, is constrained to the fixed `pilot01` scratch run and `enable_job=false`, and requires `allow_scratch_destroy=false`. It changes only `google_cloud_run_v2_job.role_comment_probe[*].deletion_protection`; SQL instance/database and secret protections remain enabled.

## Staged cleanup plans

Private plan artifacts are in `.local/testing/gcp03-role-probe-cleanup-plans-20261016/` with file mode 0600.

- **Stage 1 — applied:** 0 add, 2 change, 0 destroy. Changed only `deletion_protection=true` to `false` on the two probe Jobs. Exact approved plan binary SHA-256 `41a92b429a35dbe0918d7a3cbb787f970a2801dbcce8ce9eb7bbdd479b417dd5`; apply completed; refreshed plan returned no changes. Cloud SQL instance `deletion_protection` and `settings.deletion_protection_enabled` remain true.
- **Stage 2 — applied from the exact reviewed post-stage-1 plan:** 0 add, 0 change, 9 destroy. It removed two Jobs, two `run.invoker` bindings, one `iam.serviceAccountUser` binding, two temporary custom roles and two temporary credential IAM bindings. It did not change Cloud SQL, the database or Secret Manager secret resources. Binary SHA-256 `81b64f3956b1e986b717bc7deb5b939e4cf11ead65e9ceead98263ea11ef1483`; JSON SHA-256 `6af3558b22ad63fbdf89656170fc15a9205d8ee0b62905dd6864087acae66196`.

The prior stage-2 preview hashes `521cc35714767ef50711226a3df43ec839e228a019f97bb85145c490cfb9ade0` (binary) and `fbeaa4ea84a7973a434c0ebf352c6c6a690abdcb22d9a584bb703c58b3895c1a` (JSON) are stale; they were generated before stage 1. Do not apply them.

The stage-2 apply completed after a fresh project/state preflight and exact SHA verification. Post-apply Cloud Run lookups returned `Cannot find job` for both Jobs; project and secret policies no longer contain either temporary role binding, and the migrator service-account policy no longer contains the operator `actAs` binding. Both custom-role lookups with `--show-deleted` returned NotFound/absent. Terraform state no longer contains any of the nine planned addresses, while a fresh plan with stage-2 variables returned no changes. Cloud SQL remains `RUNNABLE`, private, and protected; the scratch database and secret resources, runtime service accounts, Artifact Registry repository, and VPC/subnet remain present. Secret version 3 remains `DESTROYED`. Private validation output is under `.local/testing/gcp03-role-probe-cleanup-plans-20261016/stage2-apply-preflight-20261016/`, mode 0600.

Google custom-role deletion is soft-delete. Preserve the identifiers and deletion references from the private state/API evidence; do not recreate either role ID until retention/undelete behavior is known. The former credential IAM expiry was not renewed. The prior stage-2 preview generated before stage 1 remains stale and must not be applied.
