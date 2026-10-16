# GCP-03 scratch migrator runner: infrastructure and package

Status: **code and local plan preparation only**. This root is isolated from the persistent GCP state. No scratch resources, image uploads, secret values/versions, SQL users, connections, SQL statements, migrations, restore, or imports were created or executed by this change.

## Isolated root and state

Terraform root: `infra/terraform/environments/testprod/gcp03-scratch`. Configure the existing state bucket with a new private backend prefix `espacigo/testprod/gcp03-scratch`; do not reuse the persistent root prefix and do not import its state. The root reads the already existing VPC/subnet, but does not manage them or create a new PSA range/peering. Every scratch-owned resource has a unique `run_id` name. `enable_scratch` and `enable_job` default to false. Project, inventory, image, secret-version and destruction gates are explicit.

Scratch resources are limited to a disposable PostgreSQL 18 instance and database, a dedicated runner service account, project-scoped `roles/cloudsql.client`, a new scratch-only Secret Manager metadata container with accessor IAM on that exact secret, an Artifact Registry repository, and a one-task Cloud Run Job. The runtime identity receives no persistent migrator/API secret or private files bucket access. Terraform creates no SQL user and no secret version/value. The Job image is accepted only as `@sha256:<64 hex>`, reads the password file from the scratch-only secret, has no retries, a four-hour timeout, 2 vCPU/4 GiB, Direct VPC egress `PRIVATE_RANGES_ONLY`, and no ingress configuration. The named operator is the only principal bound to `roles/run.invoker` and `roles/iam.serviceAccountUser` for the scratch job identity.

Cloud SQL has no public IPv4, requires the Cloud SQL connector, enforces encrypted connections, and attaches only to the existing private VPC. It is zonal, starts with 20 GiB PD-SSD, disables backup/PITR for this disposable empty rehearsal, and has native/Terraform deletion protection. Teardown requires a two-step exact `run_id` gate: first lower deletion protection while keeping resources enabled, then disable this root's resources. This root never targets the persistent `espacigo-testprod-pg` instance. The SQL client role is project-scoped; the runner still has no persistent database password or API credential, and the future plan must recheck the effective IAM and instance inventory.

## Reproducible migrator package

`Dockerfile.dbmigrate` builds only `cmd/dbmigrate` and the immutable V1–V61 migration files using Go 1.27.1 from a digest-pinned linux/amd64 builder, then emits a non-root scratch image with the pinned builder CA bundle. `scripts/build-dbmigrate-oci.sh` refuses dirty worktrees, requires exactly V1–V61, emits an OCI archive, verifies the embedded manifest digest and writes a private JSON manifest with commit, toolchain, builder image, platform, per-file migration hashes, aggregate migration inventory hash, extension names, and archive hash. The output directory and files are mode 0700/0600. It does not push to Artifact Registry.

The Terraform job gate remains off until a later approved step has pushed the exact image and verified the registry digest, and provisioned a temporary password version outside Terraform. An empty secret container alone cannot start a job. No mutable tag is accepted.

## SQL/bootstrap and future order

`db/bootstrap/gcp03-scratch-bootstrap.sql` and `gcp03-scratch-verify.sql` are review-only. The bootstrap is intended only for a separately approved disposable database: it checks the database name, creates supported extensions, creates a restricted `espacigo_runtime NOLOGIN` role if missing, and grants CONNECT. The verification checks extension presence/version output and the V60 `espacigo_gadm_bootstrap` and runtime role attributes. Neither file was run. Extension versions must be compared with the migration source and captured Cloud SQL availability before any real compatibility claim. The migration login and its password are created separately after scope review; no credential is embedded in SQL/Terraform.

Future sequencing after this PR merges and after a separately reviewed authorization: refresh explicit project/network/Cloud SQL/organization policy inventory; exact plan only for the new scratch root; review/apply exact scratch infra; build from the reviewed commit and push the OCI digest; create the temporary SQL role/secret out of band; run bootstrap only against the scratch database; verify V1–V61, extension versions, V60 and runtime grants; only then enable and invoke the named job. Restore/import requires a separate approved source and destination procedure. Preserve the local database and existing persistent database throughout.

## Cost and limits

The existing Santiago Cloud SQL model is an estimate, not a quoted scratch bill. Current defaults (`db-custom-N4-2-4096`, PD-SSD, 20 GiB) and a one-shot 4-hour Cloud Run Job are retained for comparability; the exact regional Cloud Run Job SKU is not yet selected or verified. Do not treat the Iowa/default published job rate as a Santiago quote. The job and instance remain off in default Terraform variables. No apply, image upload, secret version, SQL execution, user creation, restore, import, or Paid-mode change is authorized by this preparation PR.
