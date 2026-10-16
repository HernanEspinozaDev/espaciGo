# GCP-03 managed synthetic restore preparation

This independent Terraform root and state prefix describes a managed restore of the synthetic V1–V61 fixture. It creates a new private, zonal Cloud SQL instance and run-scoped artifact bucket. It does not reference the persistent database, its volumes, or its files. The existing VPC and serverless subnet are read-only data sources. Every creation gate defaults off, and Terraform never invokes a Cloud Run Job.

The root is a staged proposal. Local validation, mock-provider tests, and an unpublished local executor-image build passed; there has been no live Terraform plan, cloud inventory, apply, image push, artifact upload, Cloud Run invocation, or database access from this work. A later exact review is required before each cloud operation.

## Stages and state

Use a private backend config in `.local/` with mode 0600 and a unique state prefix:

```hcl
bucket = "espacigo-testprod-tfstate-447373903153-gcp"
prefix = "espacigo/testprod/gcp03-restore/<run-id>"
```

Initialize only this root with `terraform init -reconfigure -backend-config=<private-file>`. Never share the prefix with `gcp03-scratch` or the persistent environment.

The gates are independent and ordered:

1. `enable_foundation` plus `confirm_isolated_restore` describes the new private SQL instance, its empty `espacigo_scratch` database, run-scoped bucket, admin identity, and Secret Manager secret metadata. It does not create a secret payload.
2. `enable_bootstrap_job` additionally requires an existing numeric admin secret version and reviewed executor and proxy digests. The bootstrap job uses the mounted `postgres` password, preinstalls extensions and creates only the fixed run roles and ACLs.
3. `enable_restore_jobs` requires separate confirmation that bootstrap evidence passed, the exact manifest and migration hashes, and pinned images. It creates restore and verify IAM SQL users plus restore, verify, and retire jobs. PostgreSQL IAM usernames use the service-account email prefix ending in `@<project>.iam`; Cloud SQL omits only `.gserviceaccount.com`. The admin service account can read the password secret; restore can read only the run bucket; verify has no bucket access.

All jobs are single-task, private VPC, zero-retry, and protected against deletion. They cannot be invoked by Terraform. No `run.invoker` grant is created here. The restore is non-idempotent: an uncertain result must stop for review, and a restore retry must never be automatic. Destroy protection is lowered only through the two-plan, run-ID-bound teardown described below.

## Sealed synthetic artifacts

The executor accepts exactly four run-scoped objects: `manifest.json`, `synthetic-v1-v61.dump`, `dump.toc`, and `allowlist.toc`. The separately reviewed Terraform input pins the manifest SHA-256; the manifest binds dump bytes, the raw `pg_restore --list` output, the reduced allowlist, and the V1–V61 migration inventory. The sealer checks the local synthetic fixture and requires a manually reviewed, unique allowlist that is a subset of the actual dump TOC. It refuses to overwrite a sealed run and writes mode-0600 files under `.local/testing`.

```sh
python3 scripts/gcp03-restore-seal.py \
  --run-id <run-id> \
  --evidence-dir .local/testing/gcp03-restore-rehearsal-20261016/<local-run> \
  --allowlist .local/testing/gcp03-restore-rehearsal-20261016/<local-run>/allowlist.toc
```

Review every selected TOC entry against the exact dump. Upload is a distinct, separately reviewed step and is not part of this preparation. Never seal or upload source-dataset or persistent-database data through this procedure.

## Executor phases

The executor implements four fixed phases:

- `bootstrap` checks an empty scratch target, refuses existing application/run roles, installs `btree_gist`, `postgis`, and `unaccent`, then creates the fixed `NOLOGIN` owner, verifier, runtime, GADM bootstrap, and synthetic migrator roles and baseline ACLs in one transaction.
- `restore` verifies the pinned manifest, exact dump bytes, TOC and allowlist, run/instance/bucket/database names, IAM identity and role memberships, preinstalled extensions, and empty-target preconditions. It uses `pg_restore --single-transaction --exit-on-error --no-owner --no-acl --role=<run owner> --use-list=<reviewed allowlist>`, restores runtime ACLs, and runs post-restore checks.
- `verify` uses a separate IAM verifier identity to recheck the migration inventory and V1–V61 checksums, synthetic row counts, adjacent slots, PostGIS, Santiago DST behavior, table/function ownership, SECURITY DEFINER search path, V60 trigger, overlap rejection, and concurrent one-winner behavior.
- `retire` removes only fixed synthetic verification rows and revokes exact owner/verifier memberships while the Cloud SQL IAM users remain login-capable. A later reviewed Terraform plan deletes those IAM SQL users; the executor never changes Cloud-managed IAM users to `NOLOGIN`.

Evidence is one sanitized JSON object per successful phase. The implementation deliberately emits fixed error messages and does not expose database or subprocess output. Runtime behavior against Cloud SQL remains unverified. In particular, role membership revoke and bootstrap permissions must be checked against the selected Cloud SQL version before any job is run. Cloud SQL's `cloudsqlsuperuser` is managed elevated access, not PostgreSQL `SUPERUSER`; local superuser tests do not prove Cloud SQL behavior.

The custom-format archive is restored by `pg_restore` in a private Cloud Run Job rather than Cloud SQL's SQL-import API. The local sealer emits `cloud-restore-manifest.json`; a future, separately reviewed upload must place those exact bytes at the run object path `manifest.json`. Google documents custom/directory archives with `pg_restore`, removal of extension entries where extensions are preinstalled, and post-import verification: [Cloud SQL pg_dump/pg_restore guidance](https://cloud.google.com/sql/docs/postgres/import-export/import-export-dmp?hl=es), [import/export best practices](https://docs.cloud.google.com/sql/docs/postgres/import-export), and [IAM service-account database usernames](https://docs.cloud.google.com/sql/docs/postgres/add-manage-iam-users?hl=en).

## Cost and teardown

The proposed resource shape is a zonal N4 2-vCPU/4-GiB Cloud SQL PostgreSQL 18 instance, 20 GiB Hyperdisk Balanced, a small regional Standard bucket, and short Cloud Run tasks. A rough eight-hour lower-bound reference calculation is compute `8 × (2 × $0.0413 + 4 × $0.007) = $0.885` and disk capacity `8 × 20 × $0.000157534 = $0.025`, plus Cloud Run, storage requests, and applicable regional/network charges. This is not a Santiago quote. Verify current `southamerica-west1` SKUs and billing in the pricing calculator before approval: [Cloud SQL pricing](https://cloud.google.com/sql/pricing) and [Cloud Storage pricing](https://cloud.google.com/storage/pricing?hl=es).

The bucket uses uniform access, public access prevention, `force_destroy=false`, no soft-delete retention, and a three-day object lifecycle. Teardown requires two separately reviewed plans: first set `allow_restore_destroy=true` and the exact `confirm_destroy_run_id` while keeping the resources enabled to lower protections; then disable the relevant creation gates in a new plan to remove only the run-scoped resources. Do not use global prune. Remove temporary credentials when the run is retired and retain only sanitized, hash-bound evidence under the approved retention policy.

GCP-03 remains partial. This change is synthetic-only; it does not restore a Cloud SQL snapshot or source dataset and does not complete GCP-04. No cloud plan, apply, upload, or job execution has been performed.
