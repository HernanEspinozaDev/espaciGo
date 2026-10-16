# GCP-03 revoke-retired-membership: OCI and exact plan gate — 2026-10-16

## Source artifact

The Linux/amd64 OCI was built from the reviewed and merged PR #363 commit `d8a9d128b30f707886ea71f03ab1ea147f02b2a7`. Its tag is `revoke-retired-membership-d8a9d12`; the immutable Artifact Registry reference is:

`southamerica-west1-docker.pkg.dev/espacigo-testprod/gcp03-scratch-pilot01/dbmigrate@sha256:9bd186ce3eb0d83e58f825d7af29579d08b68f02f56c0f0f0227adcffa75918f`

The local OCI archive SHA-256 is `1601be7e36075fd7d176de767cbfa83edefc34709782cc5ef45be36149ee74fe`. The remote Artifact Registry read returned the same digest as the local OCI manifest and Docker push. The builder was `golang:1.27.1-alpine3.24@sha256:cd9a32216aee5667f957a62d13a10032a63fd58e14b3f3d9cc8c2122f501e95e`, Go 1.27.1, Buildx v0.38.0, and Docker 29.9.1.

The packaged source inventory contains 61 migrations with inventory SHA-256 `505f24b385ec8a98a6d27b0367178476a940bce4e45c8a5305fc0a64de72c4a9`, plus four bootstrap SQL files with inventory SHA-256 `356f10069cbce6cdbb909e5cc45515c91bc14003cf644187118a2c7fbd2fd85a`. The fixed phase `revoke_retired_membership` is present in the OCI manifest. No migration or bootstrap SQL was changed for this cut.

## Exact refreshed Terraform plan

Terraform 1.16.6 and Google provider 8.7.0 initialized the existing GCS backend bucket `espacigo-testprod-tfstate-447373903153-gcp`, prefix `espacigo/testprod/gcp03-scratch`, workspace `default`. The refreshed plan has **2 creates, 0 changes, 0 destroys**:

- `google_cloud_run_v2_job.retired_membership_revoke[0]`
- `google_cloud_run_v2_job_iam_member.retired_membership_revoke_operator[0]`

The Job uses the immutable digest above and fixed phase, exact target `espacigo-testprod:southamerica-west1:espacigo-gcp03-pilot01-pg/espacigo_scratch`, existing service account `gcp03-scratch-acl-pilot01@espacigo-testprod.iam.gserviceaccount.com`, owner secret version `1`, private VPC egress, one task, parallelism one, zero retries, and deletion protection. Its only planned invoker is `user:pasteleriahijitos@gmail.com`.

All 35 other managed resource actions are no-op, including all eight existing Cloud Run Jobs and the three temporary IAM bindings whose expiration remains `2026-10-16T18:00:00Z`. The plan does not renew those bindings or change existing resources.

Private plan files and OCI archive are retained under `.local/testing/gcp03-revoke-retired-plan-20261016/` with restrictive permissions. SHA-256 of the binary plan is `dce4ae0215341af73329cd0fb97b6af1f1f4c04c061c4fe9e4d6944a1b32bc8b`; SHA-256 of its JSON rendering is `54511667830793e03dc026a385de9ef7d2e84321739cdc7e6664a0953ba9a15f`.

## Checks and limits

The artifact was read back from the existing scratch registry and its digest matched. The plan JSON was checked for the exact resource actions, target, phase, digest, service account, secret version, operator, task/retry settings, and protected Job. No `terraform apply`, Cloud Run Job execution, manual SQL, secret payload read, IAM renewal, login reactivation, or privilege grant was performed. The local PostgreSQL fixture confirms transaction/rollback mechanics only and does not establish Cloud SQL authority.

This PR is a review gate for the OCI and exact plan. Applying it and executing the Job require separate review and authorization. Later secret version destruction, owner credential rotation, and cleanup also require separate review.
