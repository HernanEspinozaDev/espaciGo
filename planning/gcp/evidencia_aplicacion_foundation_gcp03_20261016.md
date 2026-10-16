# GCP-03 foundation apply — `g03r-1016`

## Release and exact plan

PR #370 was approved and merged by `HernanEspinozaDev`. Its reviewed HEAD was `2fb4ad1ac87de5563f68f3fa843de362c0e04c22`; merge commit is `777fda96f10c99cb92a21d066da460583bdc61d5`. The authorization covered only the saved foundation plan below. It did not authorize secret creation, bootstrap, SQL access, object upload, or restore execution.

The exact saved plan was applied once from a clean worktree at the merge commit, using the dedicated Terraform root and existing unique backend prefix for run `g03r-1016`:

- Binary: `.local/testing/gcp03-restore-rehearsal-20261016/g03r-1016/foundation.tfplan`, mode `0600`, SHA-256 `074229bd5a4193f346ab9110ba95b0f2a365a39e954cb4550822dfab2ab74509`.
- Reviewed plan JSON: `planning/gcp/evidence/gcp03-restore-g03r-1016/foundation.plan.json`, SHA-256 `7f096eef52add77ed29d32909040b1c671d21f5c6072c793503bc4ca81a6df8c`.
- Private preflight record SHA-256: `18c622f76990ddcd769cacb36e07b9b88c0f64eb6fdb3540c8485f0ae7110e1b`.
- Apply exited `0`: seven resources added, zero changed, zero destroyed. Apply output is retained privately at `.local/testing/gcp03-restore-rehearsal-20261016/g03r-1016/foundation-apply.log` with mode `0600`.
- Remote state is serial `4`; it has seven managed resources and two existing-network/subnet data sources. The refreshed post-apply plan exited `0` at `2026-10-16T21:12:44.826357Z` with `No changes. Your infrastructure matches the configuration.`
- Sanitized machine-readable readback: [foundation-apply.json](evidence/gcp03-restore-g03r-1016/foundation-apply.json).

## Live readback

Read-only Cloud SQL, Storage, Secret Manager, Cloud Run, and IAM API requests used explicit project `espacigo-testprod`. No database connection, `gcloud sql connect`, SQL, secret payload read, or job invocation was issued.

| Resource | Verified state |
| --- | --- |
| Cloud SQL `espacigo-gcp03-restore-g03r-1016-pg` | `RUNNABLE`, PostgreSQL 18, private IP only, connector enforcement `REQUIRED`, SSL mode `ENCRYPTED_ONLY`, IAM auth `on`, zonal N4 2 vCPU / 4 GiB, 20 GiB Hyperdisk Balanced, disk autoresize off, deletion protection on. The only listed databases are built-in `postgres` and `espacigo_scratch`; the only listed SQL user is built-in `postgres`; no Cloud SQL IAM SQL users exist. |
| Instance lifetime | `createTime` `2026-10-16T21:08:13.708Z`; the requested eight-hour review/teardown target is `2026-10-17T05:08:13.708Z`. This is an operator deadline, not automatic stop/deletion. |
| Artifact bucket | `espacigo-gcp03-restore-g03r-1016`, `SOUTHAMERICA-WEST1`, uniform bucket-level access on, public access prevention enforced, three-day object lifecycle, zero objects. |
| Password secret | Run-scoped Secret Manager metadata exists with zero versions. No password was generated or read. |
| Admin service account | `g03-restore-admin-g03r-1016@espacigo-testprod.iam.gserviceaccount.com`; `roles/cloudsql.client` is conditioned to the one run-scoped SQL instance; `roles/secretmanager.secretAccessor` is bound on the one run-scoped password secret. |
| Jobs and unrelated resources | No Cloud Run Jobs in the region. The two pre-existing SQL instances remain listed and were not modified. Cloud Billing API remains disabled; no billing mode or quota changes were made. |

## Cost and cleanup window

The prior Santiago list-price reference for this SQL shape is `$1.39960544` per eight hours before tax, discounts, backup/network/log charges, or teardown delay. Cloud Billing API remained disabled, so account-specific pricing, credits, and bill association were not verified. The SQL instance remains billable until it is explicitly stopped/deleted through the separately reviewed, run-ID-bound teardown; neither the instance nor the deadline has an automatic cleanup mechanism. The three-day bucket lifecycle only applies to objects and does not clean up the infrastructure.

Do not touch the two existing instances. When a separately authorized teardown is due, use the documented two-plan process for this exact run ID. Do not remove resources with a project-wide or pattern-based cleanup.

## Remaining release gates

The four sealed synthetic objects remain local/not uploaded. Hashes are recorded in the Terraform README and sealed manifest. The bucket is empty. No bootstrap job exists, no credential exists, and `admin_password_secret_version` has no active version to reference.

The follow-up PR reviews the procedure and evidence. Once `HernanEspinozaDev` approves it, the user's standing authorization releases only the exact helper action documented in the Terraform README: generate 32 random bytes in process memory, store the resulting URL-safe password in the run-scoped Secret Manager secret, set the same bytes on the new scratch instance's built-in `postgres` account, and wait for the Cloud SQL operation to finish. The helper returns only the numeric secret version. That version can then be pinned in a new bootstrap-only Terraform plan with both image digests, `enable_bootstrap_job=true`, and `enable_restore_jobs=false`. Do not execute the job during Terraform apply. No credential or version was created by this PR.

The reviewed versioned helper is `cmd/gcp03restore-credential`, with fixed-target orchestration in `internal/gcp03restorecredential`. Local fake-API tests cover the complete success path, 403 rejection and 503 uncertainty classification, exact-target failures, same generated bytes reaching both APIs, quota-project header overriding unrelated ADC/test metadata, finite polling, and retention/cleanup behavior for definite versus uncertain outcomes. These tests made no live API requests.

For least privilege, grant the deploying principal `roles/iam.serviceAccountUser` on only the run admin service account if it needs `iam.serviceAccounts.actAs` to attach that identity. Grant the designated operator `roles/run.invoker` on only the reviewed bootstrap Job to execute it via CLI; do not grant project-wide invoker or `run.jobs.runWithOverrides`. These grants are not currently present for this run. See Google's [Cloud Run job execution roles](https://docs.cloud.google.com/run/docs/execute/jobs) and [service-account attachment permissions](https://docs.cloud.google.com/iam/docs/service-account-permissions). Bootstrap invocation, artifact upload, restore, verify, and retire each remain separate release gates. Restore is non-idempotent and must not be automatically retried.

## Verification limits

This verifies foundation provisioning and control-plane configuration only. It does not prove Cloud SQL login behavior, Secret Manager mount behavior, Cloud Run execution, restore behavior in Cloud SQL, or source-data restoration. The four inputs are synthetic fixtures; no persistent database or dataset was read or changed.
