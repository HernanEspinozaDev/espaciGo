# GCP-08 — guarded initial Cloud Run restore (2026-10-21)

## Purpose and boundary

This PR prepares a manual, preview-first path to create the three testprod Cloud Run services from digest-pinned images in a successful `main` publication. It does not deploy anything. Once all three services exist and a V67 baseline has been verified, the existing automatic image-only rollout remains available.

The initial restore accepts only creates of `google_cloud_run_v2_service.api[0]`, `google_cloud_run_v2_service.web["public"]`, and `google_cloud_run_v2_service.web["private"]`. It pins project `espacigo-testprod`, region `southamerica-west1`, the existing V67 Cloud SQL instance `espacigo-testprod-db`, database `espacigo_dms_stage_20261020`, login `espacigo_api_login`, role `espacigo_runtime`, and an explicitly versioned runtime password secret. The restore must not create or change IAM, Secret Manager, SQL, networking, or other resources. A plan containing any such change is rejected.

The V67 schema migration and baseline guard are already on `main` from PR #473. This change does not copy or rerun the migration. Releases whose schema baseline is V66 remain blocked by the existing manifest-scope check.

## Required reviewed setup

Before a preview can reach the privileged job, the testprod environment must already satisfy the rollout protection check, and operators must configure the reviewed V67 runtime password secret version, the SHA-256 of the runtime preflight evidence, and protected Terraform inputs that affirm runtime secret versions, runtime database bootstrap, and service enablement. The workflow forces the exact database target and disables managed-database creation and GCP-06 candidate selection. It never creates a password, secret, database login, or grant.

Preview artifacts contain only the allowlisted service addresses, actions, immutable images, source/release identifiers, mode, and fingerprint. Apply re-plans, validates the same policy, checks the approved preview and exact confirmation phrase, then applies only that job's private plan. Automatic rollouts remain image-only updates after the baseline gate.

## Observed blockers (reported 2026-10-21)

- Cloud Run inventory is empty, so the one-time create path is needed.
- Cloud SQL inventory shows only the `postgres` database; the named V67 database, login, grants, and runtime preflight have not been verified.
- The V67 database password secret is absent. No runtime credential is available or fabricated by this PR.
- The only configured GitHub repository variable is `GCP07C_PUBLISH_ENABLED`; rollout activation, V67 password version, and runtime preflight SHA are not configured.
- The protected Terraform variable secret `GCP07D_TFVARS_JSON` is absent.
- No IAM or Secret Manager mutation is authorized here. The deployer must already have read access to the existing target metadata; if the initial Terraform plan proposes IAM changes, policy rejects it and a separately reviewed authorization step is required.

These blockers mean the workflow must remain fail-closed until the existing database and secret are provisioned and verified by an authorized process, the required protected inputs are configured, and the exact preview is reviewed. This PR does not perform those operations or claim that runtime connectivity works.

## Validation and limits

Validation is local and offline: policy unit tests, Terraform formatting and tests with `-backend=false`, workflow lint when available, and `git diff --check`. No workflow dispatch, remote Terraform initialization/plan/apply, Cloud SQL query, Cloud Run operation, IAM change, Secret Manager change, or GitHub variable edit is part of this PR.
