# GCP-03 observer OCI and Job plan — 2026-10-16

## OCI published from the reviewed observer commit

- Reviewed source: PR #357 merge commit `cf209aa85372f675df3cc9837354f63cba01eee4`, merged by `HernanEspinozaDev` at `2026-10-16T15:09:14Z`.
- Exact existing Artifact Registry repository: `southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/dbmigrate`.
- New unique tag: `observe-cf209aa`; pushed at `2026-10-16T15:11:11Z`.
- Manifest digest returned by the build and read back from Artifact Registry: `sha256:d072139c9fbfd25476e30c6dd6ceb6552f34beaa5e960226e27ea2c55568a198`.
- Private OCI archive SHA-256: `671ebd0f3591984a3714b5a29717988fe30c905ea7ff5a1e4e76e27a1dd61eae`.
- Migration inventory: 61 files, SHA-256 `505f24b385ec8a98a6d27b0367178476a940bce4e45c8a5305fc0a64de72c4a9`.
- GCP-03 bootstrap SQL inventory SHA-256: `330a45a9be280f529748525a47fdfcdfb4118ade95bc603582253e5dbe1871e4`.
- Private archive, manifest, remote metadata and checksums: `.local/testing/gcp03-observer-image-20261016/` (directory 0700; files 0600).

The existing digest `sha256:3e5683bf557a009d0ac14f60575f1054f9c089239b9e65fae27ab8acadae3548` and all five applied migration Jobs were left unchanged. No OCI was pushed to another repository.

## Exact observer-only Terraform plan

- Root/workspace: `infra/terraform/environments/testprod/gcp03-scratch`, GCS prefix `espacigo/testprod/gcp03-scratch`, workspace `default`.
- Source commit for the plan: `cae5bc63e3b26074c940369ffb3b96d508b47f09`.
- Plan timestamp: `2026-10-16T15:16:36Z`; `terraform plan -refresh=true` result: **2 add, 0 change, 0 destroy**.
- The only creates are `google_cloud_run_v2_job.observer[0]` and `google_cloud_run_v2_job_iam_member.observer_operator[0]`.
- Existing migration Jobs `bootstrap`, `database_acl`, `migrate`, `finalize`, `retire`, their `run.invoker` bindings, the migrator `actAs`, Cloud SQL client and secret accessor grants were all `no-op`.
- New Job: `espacigo-gcp03-pilot01-observe`, fixed `observe` phase and exact `espacigo_scratch` target; image pinned to the read-back digest above; existing `gcp03-scratch-pilot01@espacigo-testprod.iam.gserviceaccount.com`; exact migrator secret version `4`; one task, parallelism one, zero retries, 1,800-second Cloud Run timeout, private-ranges-only VPC egress, and deletion protection enabled. Only the exact operator `user:pasteleriahijitos@gmail.com` receives the new per-Job `roles/run.invoker` binding.
- Private plan binary SHA-256: `bbef0971ea9e794f27e42390f34eec41f71382b63e69264b8ba16e9c16263925`.
- Private plan JSON SHA-256: `6d10cacbab06f0a9d0d23ec6df2433df88ff5dca4350db59d4b5b35401f73d31`.
- Private tfvars SHA-256: `9318dba1c16deac2f1edbb6109b0a6f4ced37b75d2dd8743b5cf84b1e0f63067`.
- Plan binary, JSON, rendered text and tfvars remain under `.local/testing/gcp03-observer-job-plan-20261016/` (directory 0700; files 0600); they are not committed or attached.

The observer gate defaults off. No plan was applied, no Cloud Run Job was updated or executed, and no Cloud SQL observer was run. The image contains only fixed `SELECT` observer queries. The local PostgreSQL 18/PostGIS integration explicitly enabled `default_transaction_read_only=on`; this plan does not establish that session parameter in Cloud SQL. The managed observer remains pending a separate review and execution. No persistent database, dataset, import, restore, or migration was touched in this step.
