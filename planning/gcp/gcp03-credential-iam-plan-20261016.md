# GCP-03 credential IAM plan gate — 2026-10-16

## Scope

This is a review record for one real Terraform binary plan in project `espacigo-testprod`, workspace `default`, using the existing `gcp03-scratch` state. It prepares only temporary IAM permissions needed for the fixed credential probe. It does not authorize applying the plan or creating a Cloud SQL login, Secret Manager version, SQL data, or Cloud Run Job.

The plan contains exactly four creates:

1. Project custom role `gcp03roleprobe_sqlops` with `cloudsql.instances.get`, `cloudsql.users.create`, `cloudsql.users.delete`, `cloudsql.users.get`, and `cloudsql.users.update`.
2. Project IAM binding for `user:pasteleriahijitos@gmail.com` to that role, conditional until `2026-10-16T15:26:16Z`.
3. Project custom role `gcp03roleprobe_secretops` with `secretmanager.versions.add`, `secretmanager.versions.destroy`, `secretmanager.versions.get`, and `secretmanager.versions.list`.
4. IAM binding for the same principal to that role on the exact secret `espacigo-gcp03-pilot01-migrator-password`, with the same expiry.

Terraform's real plan summary is `4 to add, 0 to change, 0 to destroy`. JSON inspection confirmed those are the only non-no-op resource changes; all existing scratch resources are no-op. The binary plan was produced against the existing remote state (workspace `default`; 15 state resources). The plan is sensitive and is retained locally under `.local/testing/gcp03-credential-iam-plan-20261016/` with mode 0600; it is not committed or attached.

## Permission and scope evidence

Google IAM `queryTestablePermissions` returned all five Cloud SQL permissions as GA and custom-role supported for the target project, including `cloudsql.users.get`. This is important because the reviewed probe uses the Users.Get API method rather than relying on Users.List. The four Secret Manager version permissions were included in the custom role.

Cloud SQL Admin API permissions apply at project scope, not a lower instance scope. The Cloud SQL IAM Conditions documentation does not provide an instance-level condition for these `users.*` operations. The plan therefore binds a narrow role to the explicit operator principal with a short absolute expiry (less than two hours from plan generation) and records the project-scope limitation. No automatic renewal is configured.

References: [Cloud SQL IAM permissions](https://docs.cloud.google.com/sql/docs/postgres/iam-permissions?hl=en), [Cloud SQL IAM Conditions](https://docs.cloud.google.com/sql/docs/postgres/iam-conditions), [custom role permission support](https://cloud.google.com/iam/docs/custom-roles-permissions-support), and [Cloud SQL Users.Get](https://docs.cloud.google.com/sql/docs/postgres/admin-api/rest/v1/users/get).

## Plan evidence

- Code base before this documentation change: `1b04849e13404d0f6c6090949652e68de053d6ce` (merged PR #348).
- Target: `espacigo-testprod`, `southamerica-west1`, scratch database `espacigo_scratch`.
- Explicit operator principal: `user:pasteleriahijitos@gmail.com`.
- Absolute IAM expiry: `2026-10-16T15:26:16Z`.
- Binary plan SHA-256: `bad05c2acbc07a03ba3b9c5cdb84b81e72f7d3a0fe9ad00878ba9cb168c8ca9b`.
- Private JSON rendering SHA-256: `78ef775e8effe5be60b06bd55f4d2fbec29f9a0a421b304e11a275694031a1f2`.
- `terraform validate`: passed.
- Machine check of the saved plan: passed; exactly four creates, no updates, deletes, or replacements, and exact permissions, target secret, principal, and expiry verified.
- The binary plan, JSON, text rendering, tfvars, init log, plan log, and state-list evidence remain private in `.local/testing/gcp03-credential-iam-plan-20261016/` with restrictive permissions.

## Next gate

After review of this exact binary-plan hash, re-check state drift and expiry. Applying requires a separate explicit approval gate. Only after a reviewed apply may the fixed CLI create one credential; SQL, Jobs, and the later two/five-job plan remain separate gates. The expiry is absolute and must not be silently renewed; if it has passed, generate and review a new plan.
