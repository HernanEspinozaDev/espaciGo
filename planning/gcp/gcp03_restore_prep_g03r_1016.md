# GCP-03 synthetic restore preparation — g03r-1016

**Status at this preparation snapshot:** foundation-only apply completed under the exact post-PR-#370 authorization. PR #369 remains frozen at the merged HEAD listed below. At that snapshot, no Cloud SQL connection, bootstrap/restore Job, secret version, or object upload had occurred. Subsequent execution is recorded below.

## Subsequent run status — 2026-10-17

The isolated run `g03r-1016` later completed the read-only diagnostic, synthetic restore, and verify phases. The diagnostic confirmed the empty baseline before restore; restore and verify each completed once and passed their nine fixed checks. The restored fixture was deliberately minimal: two synthetic users, one synthetic space, and two occupancy rows. It is not a snapshot, migration, or copy of the persistent source dataset.

The separately authorized `retire` Job was invoked once with no overrides. Cloud Run reported `Completed=True` and one successful task, but the executor emitted application failure evidence: `phase=retire`, `stage=phase_execute`, `error_code=PHASE_FAILED`, `commit_outcome_uncertain=false`. This is not successful retirement evidence. No retry was made, and this report does not infer that the database transaction rolled back. See [sanitized retire execution evidence](evidence/gcp03-restore-g03r-1016/retire-execution.json), [restore and verify evidence](evidence/gcp03-restore-g03r-1016/restore-verify.json), and the [phase-1 teardown plan](gcp03_retirement_teardown_phase1_g03r_1016.md).

No teardown, artifact deletion, secret deletion, or resource deletion has been applied. The prepared targeted phase-1 Terraform plan is private, deletes only the two run-scoped Cloud SQL IAM users, and is not applied. The four object generations are listed for exact conditional deletion in the teardown plan. GCP-03 remains partial: snapshot compatibility, the original acceptance criteria, and human acceptance remain open.

## Frozen baseline and local validation

- PR #369 was merged by `HernanEspinozaDev`: merge `1aaa2bd0e28b6f213f0db1e5465ed9d99ed6ea4e`; PR HEAD `1501135a17c1e69eeb747e436e891fa987214c69` remains unchanged.
- The clean merge commit was used as the source for the pushed restore executor OCI. The one-line `internal/gcp03scratch/phases.go` adapter correction in this follow-up branch fixes a compile error in the independent scratch rehearsal; it does not enter the restore image, whose Dockerfile copies only the restore executor packages.
- Synthetic rehearsal run `g03r-1016`: PostgreSQL 18.6 / PostGIS 3.6.4, network-isolated local container, 61 migrations, 2 synthetic users, 1 synthetic space, and 2 adjacent occupancy ranges. Ten checks passed: migration hashes, adjacency, overlap rejection, concurrent winner, Santiago DST, PostGIS function, object ownership, minimal NOLOGIN owner, safe SECURITY DEFINER path, and V60 trigger.
- The custom dump is 454,328 bytes, SHA-256 `282f406d77e341ca7b8630cd11414e60a940051d86d8c7e5806387717aa14998`. Fixture SHA-256: `e8c47dba25bf7261cd3e6c8cfc75337549b5e7a1fe120fc5f630dc1ec4883876`. Migration inventory SHA-256: `e4086e95c3ecf05f9262aabfaa4e536a72503b992cc806b4fcf610748c8fa8c4`.
- The reviewed TOC allowlist has 734 entries. It omits exactly seven entries for the three preinstalled extensions (`btree_gist`, `postgis`, `unaccent`) and PostGIS `spatial_ref_sys` data. The TOC hash is `4a2fddbdb08f636bc0f6cb7cfeeecfed621c9ad351dcb6b2f7fc5d2b0dc96244`; allowlist hash is `3c4595bd92c288f3189cee1f71f3c3f148b2f0cd8c3d27e68a335604abc38b93`. The reviewed allowlist and sealed manifest are included alongside this report; the exact raw `dump.toc` remains in `.local/testing` mode 0600 and is bound by the manifest SHA-256. The SQL fixture and custom dump are deliberately excluded.
- A second invocation through `scripts/test-gcp03-restore-rehearsal-postgres.sh g03r-1017` passed, confirming the script routes its fixture into its unique run directory.
- Focused Go packages passed: `internal/gcp03scratch`, `internal/gcp03restore`, `cmd/gcp03restore`, `internal/dbbootstrap`, and `internal/migrator`. Terraform `fmt -check`, `validate`, and architecture tests passed (2/2). `Dockerfile.gcp03restore` built from the frozen merge commit.

## Immutable images and inventory

- Executor image, built from commit `1aaa2bd0e28b6f213f0db1e5465ed9d99ed6ea4e` and pushed to the already-existing `gcp03-scratch-pilot01` Artifact Registry repository:
  `southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/gcp03restore@sha256:7cca0b4371c415ebb1f5b72516f1d3a119ba6f8d298ce46f07f2a93d12369bce` (`linux/amd64`). Registry readback confirmed the digest and unique tag `restore-g03r-1016-1aaa2bd`.
- Proxy pinned to `gcr.io/cloud-sql-connectors/cloud-sql-proxy@sha256:076f128d5bdb9d345e253aa9bbf5f1f9a320caff21dfdbf07d582437e55e4f66` (`linux/amd64`, version 2.26.0). The pulled pinned binary's `--help` exposes both `--quitquitquit` and `--exit-zero-on-sigterm`; upstream documents graceful exit on GET or POST `/quitquitquit`. No Cloud SQL connection or proxy session was started.
- Toolchain: Terraform 1.16.6, Google provider 8.7.0, Go build image `golang:1.27.1-alpine3.24` pinned by digest, runtime `postgis/postgis:18-3.6` pinned by digest, and local `pg_restore` 18.6. The inventory includes V1–V61 and hashes for all four `db/bootstrap/gcp03scratch/*.sql` files, `Dockerfile.gcp03restore`, `go.mod`, and `go.sum`.
- Artifact inventory SHA-256: `7ca53590aa8f5714a4b57b0a4d2c630d03e92e5d7f9f0513f1d6cb736a3efdb7`.

## Read-only project inventory

All GCP CLI requests used explicit `--project=espacigo-testprod --billing-project=espacigo-testprod`. Terraform used `GOOGLE_CLOUD_QUOTA_PROJECT=espacigo-testprod`, `GOOGLE_BILLING_PROJECT=espacigo-testprod`, and `USER_PROJECT_OVERRIDE=true`; it did not bill against the ADC file's other quota project.

- Project `espacigo-testprod`, region `southamerica-west1`; network `espacigo-testprod`; subnet `espacigo-testprod-serverless` `10.80.0.0/24` with Private Google Access; reserved service range `10.189.0.0/16`.
- Two existing Cloud SQL PostgreSQL 18 instances were listed project-wide. The baseline `espacigo-testprod-pg` and comparison `espacigo-gcp03-pilot01-pg` are both `db-custom-N4-2-4096`, zonal, 20 GiB Hyperdisk Balanced, private IP, `RUNNABLE`. The new plan would be a third project instance; no existing instance was modified.
- The existing regional Artifact Registry repository is standard Docker, `gcp03-scratch-pilot01`; it grew from the prior reported 58.735 MB to 310.258 MB after the authorized OCI push. No repository was created. Cloud Run listed no Jobs in Santiago. Logging has `_Default` at 30 days and locked `_Required` at 400 days, with only the default sinks.
- Compute project quotas included 24 `CPUS_ALL_REGIONS` (usage 0) and 5 network slots (2 used); those do not represent Cloud SQL instance quota. Cloud Run Service Quota readback showed 1,000 Jobs and 100 instances per project/region, with no regional Jobs currently listed. Cloud SQL Admin quotas returned API rate quotas; it did not expose the Enterprise instance ceiling in that response. Google documents 100 instances/project for old SQL network architecture and 1,000 for new architecture. The two existing instances use private IP on the existing VPC; the API output did not identify their architecture explicitly. The plan adds one instance, for three total, below the documented lower 100-instance ceiling, but regional physical capacity is not guaranteed.
- Cloud Billing API is disabled, and the active account cannot use it to read the project billing association. It was not enabled. The Cloud SQL and Cloud Storage list prices below are public Santiago rates; no account-specific discount, tax, currency conversion, or billing-account free-tier consumption is included.

## Foundation-only Terraform plan

A real, refreshed Terraform plan used the existing GCS state bucket and unique prefix `espacigo/testprod/gcp03-restore/g03r-1016`. Terraform initialized a 181-byte `default.tfstate` object in that prefix; readback confirmed state version 4, serial 1, and zero resources. This is backend state initialization for the requested remote plan, not an infrastructure resource or fixture upload; it is retained with the exact plan. The plan reads only the existing VPC/subnet as data sources and proposes:

1. one run-scoped private Cloud SQL PostgreSQL 18 instance;
2. its empty `espacigo_scratch` database;
3. one private run-scoped Standard bucket;
4. one admin service account;
5. Cloud SQL Client IAM for that account, scoped by resource condition;
6. empty Secret Manager metadata and its restricted accessor binding.

The plan is **7 creates, 0 updates, 0 deletes**. `enable_bootstrap_job=false` and `enable_restore_jobs=false`: no Cloud Run Job and no `google_sql_user` is present. Terraform did not execute SQL. Full exact JSON is `planning/gcp/evidence/gcp03-restore-g03r-1016/foundation.plan.json`, SHA-256 `7f096eef52add77ed29d32909040b1c671d21f5c6072c793503bc4ca81a6df8c`. The exact binary plan remains in `.local/testing/gcp03-restore-rehearsal-20261016/g03r-1016/foundation.tfplan`, mode 0600, SHA-256 `074229bd5a4193f346ab9110ba95b0f2a365a39e954cb4550822dfab2ab74509`. JSON inspection found no secret payload: the provider marks `root_password` null and `server_ca_cert` unknown/sensitive. The original plan JSON and binary remain mode 0600 locally.

## Santiago cost reference for an eight-hour window

Cloud SQL's current Santiago N4 rates are $0.0537 per vCPU-hour and $0.0161 per GiB-memory-hour; Cloud SQL Hyperdisk Balanced capacity is $0.000157534 per GiB-hour. For the planned zonal 2-vCPU/4-GiB/20-GiB instance:

- compute: `(2 × 0.0537 + 4 × 0.0161) × 8 = $1.3744`;
- disk: `20 × 0.000157534 × 8 = $0.02520544`;
- subtotal: **$1.39960544 per 8 hours** before tax, discount, backups, network, logs, and delayed teardown.

The existing `pilot01` has the same SKU and disk, so its list-price reference is also $1.39960544 per eight hours; the planned instance is an additional instance of that shape. The planned bucket is empty, so it has no object-storage usage now. Four future objects matching this synthetic manifest would total about 0.57 MB; at the published Santiago Standard rate of $0.000027397/GiB-hour, eight hours of object storage is negligible (about $0.00000012), excluding operations and any transfer. The Cloud Run job gates are off, so current plan execution incurs no job usage. The pushed image adds roughly 0.25 GiB to Artifact Registry; at $0.000136986/GiB-hour that is about $0.00027 for eight hours or about $0.025/month before the account-wide 0.5 GiB-month free tier. The billing API limitation prevents determining whether that free tier is already consumed.

Sources: [Cloud SQL pricing](https://cloud.google.com/sql/pricing?hl=en), [Cloud Storage pricing](https://cloud.google.com/storage/pricing), [Artifact Registry pricing](https://cloud.google.com/artifact-registry/pricing), [Cloud Run pricing](https://cloud.google.com/run/pricing), [Cloud SQL quotas and limits](https://docs.cloud.google.com/sql/docs/quotas), and [Cloud Run quotas and limits](https://docs.cloud.google.com/run/quotas).

## Release boundary and teardown order

The exact foundation plan has now been applied once, after PR #370 merged. See [foundation apply evidence](evidence/gcp03-restore-g03r-1016/foundation-apply.json) and [the sanitized apply/next-stage procedure](evidencia_aplicacion_foundation_gcp03_20261016.md). Terraform created only the seven reviewed foundation resources. No credential was generated, the secret still has zero versions, and no SQL login/connection, Cloud Run Job, SQL IAM user, artifact upload, bootstrap, restore, or job log was performed. Only the executor OCI was published to the already-existing, user-authorized Artifact Registry repository. No persistent SQL/data/files were read or changed.

The review in PR #370 released only the exact foundation plan. For the next release boundary, the user has standing authorization for this isolated run; approval of the follow-up PR by `HernanEspinozaDev` is the technical release to run the exact versioned password helper described in the Terraform README. It sets the same in-memory generated password in the empty run-scoped Secret Manager secret and the built-in `postgres` account on only this new scratch instance, then waits for the Cloud SQL operation to finish. That approval does not release bootstrap invocation, artifact upload, restore, verify, retire, or teardown. A future bootstrap plan must pin the executor and proxy digests, set only the bootstrap gate, name the reviewed numeric secret version, and keep restore jobs disabled. No credential, upload, or bootstrap action is included in this follow-up PR.

If later authorized and the restore stages complete, cleanup must first delete the run-scoped Cloud SQL IAM SQL users via the Cloud SQL Delete API (Terraform's managed `google_sql_user` resources), allowing the API to clear their role bindings. Do not issue SQL `REVOKE` for IAM-managed memberships or set those users `NOLOGIN`. Only after user deletion should the run service accounts, jobs, and instance be removed with the two reviewed, run-ID-bound teardown plans. Keep `LOGIN` intact until Delete API completion. Retain only sanitized hash-bound evidence; remove local credentials when that later test is retired. The private bucket lifecycle is three days, but it is not a substitute for reviewed teardown.
