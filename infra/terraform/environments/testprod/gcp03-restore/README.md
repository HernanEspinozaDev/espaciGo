# GCP-03 managed synthetic restore preparation

This independent Terraform root and state prefix describes a managed restore of the synthetic V1–V61 fixture. It creates a new private, zonal Cloud SQL instance and run-scoped artifact bucket. The exact bucket name is `espacigo-gcp03-restore-<run-id>` and is shared by Terraform and the executor's strict `GCP03_BUCKET` validation. Cloud SQL IAM database authentication is explicitly enabled with `cloudsql.iam_authentication=on`. It does not reference the persistent database, its volumes, or its files. The existing VPC and serverless subnet are read-only data sources. Every creation gate defaults off, and Terraform never invokes a Cloud Run Job.

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

All jobs are single-task, private VPC, zero-retry, and protected against deletion. They cannot be invoked by Terraform. The bootstrap gate creates `roles/run.invoker` only on its exact Job and `roles/iam.serviceAccountUser` only on the run admin service account for the one reviewed operator; both remain absent while the gate is off. The restore is non-idempotent: an uncertain result must stop for review, and a restore retry must never be automatic. Destroy protection is lowered only through the two-plan, run-ID-bound teardown described below.

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
- `restore` verifies the pinned manifest, exact dump bytes, TOC and allowlist, run/instance/bucket/database names, IAM identity and role memberships, preinstalled extensions, and empty-target preconditions. It uses `pg_restore --single-transaction --exit-on-error --no-owner --no-acl --role=<run owner> --use-list=<reviewed allowlist>`, restores runtime ACLs, grants the fixed retirement auditor `SELECT` on only `schema_migrations`, `usuario`, `espacio`, and `ocupacion` plus `DELETE` on `ocupacion`, then verifies the exact grants and runs post-restore checks. It explicitly excludes extension objects and the PostGIS `spatial_ref_sys` data from the reviewed allowlist because these are installed on the target already.
- `verify` uses a separate IAM verifier identity to recheck the migration inventory and V1–V61 checksums, synthetic row counts, adjacent slots, PostGIS, Santiago DST behavior, table/function ownership, SECURITY DEFINER search path, V60 trigger, overlap rejection, and concurrent one-winner behavior.
- `retire` removes only fixed synthetic verification rows and revokes the synthetic migrator's temporary database/schema ACLs. It verifies that each Cloud SQL IAM user remains login-capable with exactly its run-scoped database role; a later reviewed Terraform plan deletes those managed SQL users and clears their role bindings before removing the isolated instance. The executor does not issue SQL `REVOKE` against memberships managed by Cloud SQL and never changes Cloud-managed IAM users to `NOLOGIN`.

Successful phase evidence is a sanitized JSON record. Failure evidence is also sanitized and hash-bound to its phase, run ID, and pinned manifest; it includes an allowlisted stage/error code, an allowlisted PostgreSQL SQLSTATE when available, subprocess exit code, and commit-outcome uncertainty. It never emits database payloads, row values, credentials, tokens, or raw subprocess stderr. Each Cloud SQL Proxy sidecar exposes its fixed localhost-only admin endpoint on port 9091; the executor calls `/quitquitquit` on success and failure so a completed Cloud Run task does not leave the proxy process running.

The disposable integration command is `scripts/test-gcp03-restore-executor-postgres.sh`. It runs a network-isolated PostgreSQL 18/PostGIS container on a temporary Unix socket and synthetic V1–V61 fixture through bootstrap, restore, verify, and retire, plus preflight, corruption, restricted-audit, and rollback cases. The retirement call runs as a local `NOSUPERUSER` `postgres` database owner that does not own `ocupacion`; the fixed audit contract is also checked by a separate restricted login. The test control role removes only PostgreSQL's local creator ADMIN memberships after bootstrap so the existing V60 membership invariant remains unchanged; this setup does not prove Cloud SQL grants or Cloud SQL SQL-user cleanup behavior. In the local test, direct retirement revocation of the IAM role membership fails with SQLSTATE `42501`, so the executor retains exact login-role bindings and delegates account/binding cleanup to the later reviewed Terraform plan. Runtime behavior against Cloud SQL remains unverified. Cloud SQL's `cloudsqlsuperuser` is managed elevated access, not PostgreSQL `SUPERUSER`; local behavior does not prove Cloud SQL behavior.

## Applied foundation checkpoint: `g03r-1016`

The seven-resource foundation was applied under PR #370, then the exact three-resource bootstrap plan under PR #372. Terraform state is serial 5 with ten managed resources, and the post-apply refresh found no drift. The sanitized foundation readback is in `planning/gcp/evidence/gcp03-restore-g03r-1016/foundation-apply.json`; bootstrap apply and execution evidence are in `bootstrap-plan.json` and `bootstrap-execution.json`. The run-scoped SQL instance remains `RUNNABLE`, private-IP-only, connector-enforced, TLS `ENCRYPTED_ONLY`, IAM database authentication enabled, 20 GiB fixed disk, and deletion-protected. The exact bootstrap Job ran once and verified the empty target before it created the fixed roles and ensured extensions. No fixture rows were restored. SQL users remain limited to built-in `postgres`; the restore/verify IAM SQL users exist only in the unapplied plan. The private bucket has UBLA/public access prevention and no objects. Secret version `1` is enabled; its payload was never read back.

### Bootstrap execution checkpoint

The credential helper authorized for this run was invoked exactly once. Secret Manager version `1` is `ENABLED`; Cloud SQL Admin operation `1456827e-7085-4df6-a589-94760000004b` completed `DONE` without error. The secret payload was never read. The helper itself did not open a SQL connection.

The exact bootstrap plan from PR #372, SHA-256 `8a0c8ffe94451ef1673c515c6209de8033c4237b48eee4cdd027a1f64f3f1186`, was applied from its saved binary. Terraform reported three additions, zero changes, and zero destroys; state advanced from serial 4 to 5. A refreshed plan reported no drift. The bootstrap Job and the two scoped operator grants are now present.

The operator invoked `espacigo-gcp03-restore-g03r-1016-bootstrap` once, without overrides. Execution `espacigo-gcp03-restore-g03r-1016-bootstrap-9b25k` completed successfully with one succeeded task and zero failed tasks. Sanitized executor checks report `roles_created`, `extensions_ready`, and `target_empty` all `true`. The phase connected to the isolated database and ran its fixed bootstrap SQL; it did not restore data. Full execution JSON and logs remain private under `.local/testing`, mode 0600. Published sanitized details are in `planning/gcp/evidence/gcp03-restore-g03r-1016/bootstrap-execution.json`.

### Prepared restore plan (not applied)

The next plan uses the fixed sealed manifest SHA-256 `be91abf57aa34bae447b258353d11af107e1c56d625820ebc20a6c3567578317`, migration inventory SHA-256 `e4086e95c3ecf05f9262aabfaa4e536a72503b992cc806b4fcf610748c8fa8c4` (61 migrations), executor image digest `sha256:7cca0b4371c415ebb1f5b72516f1d3a119ba6f8d298ce46f07f2a93d12369bce`, proxy digest `sha256:076f128d5bdb9d345e253aa9bbf5f1f9a320caff21dfdbf07d582437e55e4f66`, and admin secret version `1`.

The current saved plan SHA-256 is `908c6fab4ecbd5a34da6fdce5d36585eec04761b00a472eefc577755e9c476c1`; its private full JSON SHA-256 is `922eac3cac2254d18409ba0c4d53b1991a50c0f4c7fe1517441e569f650fe565`. It proposes 17 creates, zero changes, and zero destroys: restore/verify/retire Jobs, their run-scoped service accounts and IAM bindings, the two Cloud SQL IAM users, and read-only access to the run bucket for the restore identity. It also grants the operator `roles/run.invoker` on each exact restore, verify, and retire Job, plus `roles/iam.serviceAccountUser` on only the restore and verify service accounts. No project-wide invoker or `run.jobs.runWithOverrides` grant is included. The binary plan, full JSON, and tfvars stay under `.local/testing` with mode 0600. Sanitized plan inventory is in `planning/gcp/evidence/gcp03-restore-g03r-1016/restore-plan.json`. This plan has not been applied; no restore, verify, retire, or artifact upload has occurred. The restore remains a separate non-idempotent release and must not be retried automatically.

The four already sealed artifacts are review-bound by these SHA-256 values; their local secret-free counterparts remain under the repo evidence directory, while the raw dump and raw TOC remain private mode-0600 files in `.local/testing`. None has been uploaded:

| Object | SHA-256 |
| --- | --- |
| `runs/g03r-1016/manifest.json` | `be91abf57aa34bae447b258353d11af107e1c56d625820ebc20a6c3567578317` |
| `runs/g03r-1016/synthetic-v1-v61.dump` | `282f406d77e341ca7b8630cd11414e60a940051d86d8c7e5806387717aa14998` |
| `runs/g03r-1016/dump.toc` | `4a2fddbdb08f636bc0f6cb7cfeeecfed621c9ad351dcb6b2f7fc5d2b0dc96244` |
| `runs/g03r-1016/allowlist.toc` | `3c4595bd92c288f3189cee1f71f3c3f148b2f0cd8c3d27e68a335604abc38b93` |

The eight-hour review/teardown target starts at the Cloud SQL instance `createTime`; for this run it is `2026-10-17T05:08:13.708Z`. It is an operator deadline, not an automatic stop or deletion. Cleanup must use the two reviewed, run-ID-bound Terraform teardown plans above; do not disable deletion protection or delete resources by name/pattern outside that procedure.

The custom-format archive is restored by `pg_restore` in a private Cloud Run Job rather than Cloud SQL's SQL-import API. The local sealer emits `cloud-restore-manifest.json`; a future, separately reviewed upload must place those exact bytes at the run object path `manifest.json`. Google documents custom/directory archives with `pg_restore`, removal of extension entries where extensions are preinstalled, and post-import verification: [Cloud SQL pg_dump/pg_restore guidance](https://cloud.google.com/sql/docs/postgres/import-export/import-export-dmp?hl=es), [import/export best practices](https://docs.cloud.google.com/sql/docs/postgres/import-export), and [IAM service-account database usernames](https://docs.cloud.google.com/sql/docs/postgres/add-manage-iam-users?hl=en).

## Cost and teardown

The proposed resource shape is a zonal N4 2-vCPU/4-GiB Cloud SQL PostgreSQL 18 instance, 20 GiB Hyperdisk Balanced, a small regional Standard bucket, and short Cloud Run tasks. A rough eight-hour lower-bound reference calculation is compute `8 × (2 × $0.0413 + 4 × $0.007) = $0.885` and disk capacity `8 × 20 × $0.000157534 = $0.025`, plus Cloud Run, storage requests, and applicable regional/network charges. This is not a Santiago quote. Verify current `southamerica-west1` SKUs and billing in the pricing calculator before approval: [Cloud SQL pricing](https://cloud.google.com/sql/pricing) and [Cloud Storage pricing](https://cloud.google.com/storage/pricing?hl=es).

The bucket uses uniform access, public access prevention, `force_destroy=false`, no soft-delete retention, and a three-day object lifecycle. Teardown requires two separately reviewed plans: first set `allow_restore_destroy=true` and the exact `confirm_destroy_run_id` while keeping the resources enabled to lower protections; then disable the relevant creation gates in a new plan to remove only the run-scoped resources. Do not use global prune. Remove temporary credentials when the run is retired and retain only sanitized, hash-bound evidence under the approved retention policy.

GCP-03 remains partial. The foundation-only plan has been applied for run `g03r-1016`; this is synthetic-only and does not restore a Cloud SQL snapshot or source dataset or complete GCP-04. The bucket remains empty, the password secret has zero versions, and no Cloud Run job has been created or executed. The Terraform root still does not invoke jobs. Future credential, bootstrap, upload, restore, verify, and retire actions require their separate reviewed release gates above.
