# GCP-03 origin rehearsal foundation

This root implements **Foundation only** for a run-scoped Cloud SQL target. It can plan one private, zonal PostgreSQL 18 instance, an empty `espacigo_scratch` database on that new instance, one private artifact bucket, a bootstrap service account, empty Secret Manager secret metadata, and exact instance/secret IAM bindings. It does not declare bootstrap, observer, staging, restore, verify, or teardown jobs. Those phases are not implicitly enabled; their implementation is a later code change in a separate review.

The root is pinned to project `espacigo-testprod` and region `southamerica-west1`; it reads the existing VPC/subnet and never manages them. Each run uses a fresh state prefix, for example `espacigo/testprod/gcp03-origin/g03o-1017`, and resources prefixed `espacigo-gcp03-origin-g03o-1017`. The foundation flag defaults off and requires `confirm_origin_rehearsal=true`. Deletion protection is enabled in both Terraform and the Cloud SQL API. Lowering it requires a separate plan with `allow_origin_destroy=true` and `confirm_destroy_run_id` exactly matching `run_id`.

## Exact reviewed plan

Plan artifacts are kept outside Git under `.local/testing/gcp03-origin-foundation-20261017/` (directory mode 0700; files mode 0600). The saved binary plan and its SHA-256 are recorded in `planning/gcp/evidence/gcp03-origin-foundation-20261017/plan-summary.json`. The remote backend prefix had no state before initialization. `terraform init` created the remote `default.tfstate` metadata object; `terraform state list` returns zero managed resources and the state serial is 1. Plans use `-lock=false` to avoid writing a remote state lock; do not apply a saved plan until the reviewer releases this exact phase.

Reproduction commands, from this root (private config files are already prepared locally):

```sh
terraform init -reconfigure -backend-config=../../../../../.local/testing/gcp03-origin-foundation-20261017/backend.hcl
terraform plan -refresh=true -lock=false -var-file=../../../../../.local/testing/gcp03-origin-foundation-20261017/foundation.tfvars -out=../../../../../.local/testing/gcp03-origin-foundation-20261017/foundation.tfplan
```

The plan is expected to add exactly seven resources: SQL instance, SQL database, bucket, bootstrap service account, secret metadata, secret accessor binding on that secret, and a `roles/cloudsql.client` project grant conditioned on the exact instance resource. It must contain zero updates/deletes and no Cloud Run Jobs, IAM database users, secret versions, application data, or changes to the existing instances, network, subnet, Artifact Registry, or other shared resources. Recreate and compare the plan against current state immediately before any later apply.

Bucket soft-delete retention is explicitly zero and `force_destroy=false`; it has no automatic lifecycle deletion. The sealed source remains in its existing private local custody. Later teardown must preserve the sealed source and verified evidence, enumerate/delete only exact object generations staged by this run, verify the bucket is empty, and then remove the bucket with its separate teardown plan. Do not assume a bucket/object deletion removes soft-deleted data unless the current bucket policy confirms it.

## Cost and operating window

Rates were read on 2026-10-17 through the public Cloud Billing Catalog API (`GET /v1/services/9662-B51E-5089/skus` for Cloud SQL and `GET /v1/services/95FF-2EF5-5EA1/skus` for Cloud Storage). The Cloud SQL SKU prices have `effectiveTime=2026-10-16T07:00:00Z` and `serviceRegions=[southamerica-west1]`; the full regional rates and formulas are in the linked evidence JSON. They match the Santiago selection in Google's [Cloud SQL pricing page](https://cloud.google.com/sql/pricing). This is list pricing before taxes, credits, discounts or custom agreements.

For the configured N4 2-vCPU/4-GiB instance and 20-GiB Hyperdisk Balanced disk, the current fixed-rate estimate is about USD 0.20221 per hour, USD 1.62 for 8 hours, USD 4.85 for 24 hours, USD 145.59 for 720 hours, and USD 147.61 for 730 hours. The disk rate includes 1,000 IOPS above the 3,000 baseline and 30 MiB/s above the 140 MiB/s baseline. The existing root sets backups and PITR off; this estimate excludes data transfer, Cloud Logging, bucket operations, any uploaded-object capacity (Santiago Standard is USD 0.03/GiB-month), and all later Cloud Run tasks. An empty bucket, metadata-only secret and service accounts have no modeled capacity cost. See Google's [Cloud SQL storage options](https://docs.cloud.google.com/sql/docs/postgres/storage-options-overview) for the provisioned Hyperdisk behavior.

Operational limit for this rehearsal: start the 8-hour window at the Cloud SQL `createTime`, including setup/bootstrap/observation/restore/verification and planned cleanup. This is a run-duration guard, not a user budget or spend authorization. If work cannot finish within the window, stop advancing phases and obtain a separate review for an extension; do not assume automatic shutdown or teardown. The USD 1.62 figure is an informational instance-and-disk estimate for eight hours, not a hard billing cap.

Teardown must use two later plans. First keep Foundation enabled and set `allow_origin_destroy=true` plus exact `confirm_destroy_run_id` to lower protection; review/apply only that plan. Then disable Foundation with the same run-bound confirmation for the exact removal plan. The second plan must not lower protection and destroy resources in one step.

## Later phases

The next code delivery must add a separate minimal bootstrap job and a fixed read-only pre-restore observer against this same target. Only after those checks pass and snapshot staging receives its own release can a distinct origin restore executor be implemented. Preserve the synthetic restore root and its destructive fixture retirement unchanged. The future encryption envelope must use a fresh random data-encryption key (DEK), encrypt the sealed snapshot with authenticated encryption, and wrap the DEK with a new run-specific transport key held in Secret Manager or an equivalent short-lived secret channel; keep the original custody/master recovery key outside the cloud run and never upload it. Delete the transport key/version only through a separately reviewed closeout step after the result and evidence are retained. No key recovery, wrapping, upload, or object write is implemented by this Foundation root.
