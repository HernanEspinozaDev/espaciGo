# GCP-03 role-comment Jobs plan gate — 2026-10-16

## Scope

This record describes a real, refreshed Terraform binary plan for the next review gate. It uses the existing remote scratch state in project `espacigo-testprod`, root `infra/terraform/environments/testprod/gcp03-scratch`, workspace `default`. The state contained 19 resources when planned and no role-comment Job resources.

The only non-no-op actions are exactly five creates:

1. `espacigo-gcp03-pilot01-role-comment-prepare` Cloud Run Job (`GCP03_PHASE=role_comment_prepare`).
2. `espacigo-gcp03-pilot01-role-comment-after-reconcile` Cloud Run Job (`GCP03_PHASE=role_comment_after_reconcile`). Terraform map keys and phase values retain underscores; Cloud Run resource IDs translate underscores to hyphens.
3. `roles/run.invoker` binding for each of those Jobs.
4. One `roles/iam.serviceAccountUser` (`actAs`) binding for the existing scratch migrator service account.

There are zero updates, replacements, or deletions. Existing credential roles and bindings are all `no-op`; their expiry remains `2026-10-16T15:26:16Z`. This PR and plan do not apply Terraform, create/execute Jobs, run SQL, reconcile database roles, or import data.

## Inputs verified by read-only preflight

- Project `espacigo-testprod`, region `southamerica-west1`, exact scratch database and instance.
- Login `espacigo_scratch_migrator` exists. Cloud SQL operation `d561ce54-e4b1-40e9-bccf-a7b00000004b` is `CREATE_USER`, `DONE`.
- The actual Secret Manager API resource is `projects/447373903153/secrets/espacigo-gcp03-pilot01-migrator-password/versions/3`, state `ENABLED`. Versions 1 and 2 are `DESTROYED`. The plan uses numeric version `3`, not an alias or assumed next number.
- Remote Artifact Registry readback confirms `sha256:3e5683bf557a009d0ac14f60575f1054f9c089239b9e65fae27ab8acadae3548` in the reviewed scratch repository.
- No Cloud Run Jobs existed before planning.

Both Jobs use the fixed reviewed image and numeric secret version, one task, parallelism 1, 600-second timeout, zero retries, 1 CPU, and 512 MiB. The migration permission gate remains false. The already applied credential-operator gate is kept true in plan inputs only so Terraform preserves its current IAM bindings and original expiry; it has no planned change.

## Plan evidence

- Configuration base: `050ad8828fc8a8eb1eb367dc264b8d9592358790` (merged PR #351).
- Terraform validation: passed.
- Real refresh plan summary: `5 to add, 0 to change, 0 to destroy`.
- Machine inspection: passed; exact five creates above, all other resources no-op, and exact image, secret version, task, timeout, retry, CPU, and memory settings verified.
- This revision supersedes the initial plan, whose names used invalid underscores and whose binary SHA-256 was `b0fa15c7bac1d3c75b75fa00497431e410ded7bdcdb82f0c78f8f985f1459ab0`. The initial plan was never applied and must not be applied.
- Corrected binary plan SHA-256: `d7646ed81cb7105b24cbdf841b1bb289409e2bf55693b02b11e5773c46493800`.
- Corrected JSON rendering SHA-256: `012440a277574bb7400ff499472cc395b3d0a6de77c217928a17888a2209fccb`.
- Both corrected Cloud Run Job requests passed Cloud Run API `validateOnly=true` with HTTP 200. A follow-up GET for each Job returned 404, confirming validation did not persist resources.
- Corrected binary, JSON, request/response evidence, text plan, tfvars and logs are private under `.local/testing/gcp03-role-probe-jobs-plan-20261016-v2/` with mode 0600. They are not committed or attached.

Before any separately reviewed apply, re-check state/drift, target, numeric version 3 still active, image digest, and remaining IAM expiry. If the expiry has passed or drift exists, stop and request a fresh plan/review; do not silently renew IAM. This plan is only the review gate for Job creation. It does not authorize a Job execution or SQL operation.
