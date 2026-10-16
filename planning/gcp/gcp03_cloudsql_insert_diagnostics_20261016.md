# GCP-03 Cloud SQL user insert diagnostics — 2026-10-16

## Read-only findings from the rejected attempt

At approximately `2026-10-16T13:37:04Z`, the single released CLI attempt added migrator Secret Manager version 2, then Cloud SQL rejected the user creation. The CLI treated the API response as a definitive 4xx and destroyed the exact version it had just created. Read-only checks confirmed versions 1 and 2 are `DESTROYED`, the fixed migrator login is absent, and no Cloud Run Jobs exist. No secret payload was read. Private metadata and sanitized logs are under `.local/testing/gcp03-credential-iam-plan-20261016/` with mode 0600.

The narrow Cloud Audit Logs query for the attempt window contains the Secret Manager `AddSecretVersion` and `DestroySecretVersion` entries, but no `sqladmin.users.insert` entry or status. The project IAM policy has no `auditConfigs` entry for `sqladmin.googleapis.com` or `allServices`. The Cloud SQL PostgreSQL log query returned two database log entries but no API error status. The existing CLI discarded the HTTP code and provider reason, so the exact 4xx cannot be recovered from retained logs. The concrete diagnostic blocker is therefore the missing SQL Admin audit record plus loss of the API error classification by the CLI.

Other read-only checks:

- `sqladmin.googleapis.com` and `secretmanager.googleapis.com` are enabled.
- The applied GA custom role contains `cloudsql.users.create`, and its temporary binding remains active until `2026-10-16T15:26:16Z`.
- The authenticated operator already had a project Owner grant before this probe; it was not changed. This means the probe does not demonstrate isolation from Owner.
- The instance is `RUNNABLE`, `POSTGRES_18`. The instance describe response exposed no `passwordValidationPolicy` or `databaseFlags`; this does not identify the API rejection reason.
- The recent Cloud SQL operation listing contained no user-create operation for this attempt.

## Contract check

The reviewed adapter calls `Users.Insert` for the fixed project and instance. It sends a JSON `User` with only the fixed migrator `name`, generated `password`, and `type=BUILT_IN`; it supplies no query parameters. The HTTP fake verifies this request shape without displaying the password. Google's PostgreSQL `v1beta4` reference defines `POST /sql/v1beta4/projects/{project}/instances/{instance}/users`, a `User` request body, and an `Operation` response. The `User` resource documents `name`, `password`, and `type`, and `BUILT_IN` is a supported type. The Cloud SQL Admin API is enabled, so the available evidence does not point to an inactive API or a mismatch with the documented request contract.

References: [Cloud SQL `users.insert` v1beta4](https://docs.cloud.google.com/sql/docs/postgres/admin-api/rest/v1beta4/users/insert), [PostgreSQL User resource](https://docs.cloud.google.com/sql/docs/postgres/admin-api/rest/v1beta4/users), [create and manage PostgreSQL users](https://docs.cloud.google.com/sql/docs/postgres/create-manage-users), and [Cloud SQL IAM permissions](https://docs.cloud.google.com/sql/docs/postgres/iam-permissions?hl=en).

## Safe diagnostics change

The CLI now preserves only a fixed service name, fixed method name, HTTP status code, and an allowlisted Google API `ErrorInfo.reason` or legacy `ErrorItem.Reason`. Unknown reasons become `UNCLASSIFIED`; transport errors report `http_status=none reason=TRANSPORT_UNCERTAIN`. The output never includes response bodies/messages, headers, request bodies, passwords, tokens, URLs, or arbitrary metadata. The original 4xx/408/409/429/5xx definitive classification is unchanged. Creation and owner-credential paths include the sanitized diagnostic while preserving their existing cleanup/reconcile behavior; retire and role-reconciliation behavior is unchanged.

The new HTTP fake tests cover 400, 403, 409, 429, 500, 503, transport failure, allowlisted and unrecognized reasons, and verify that response messages, metadata, headers, and request passwords do not appear in diagnostics. They also verify the documented fixed `users.insert` URL/body contract. Existing scratch credential, retirement, and reconciliation tests continue to run.

## Verification and limits

- `go test -count=1 ./cmd/gcp03scratch-credential ./internal/gcp03scratch`: passed.
- `git diff --check`: passed.
- No new user or secret version was created by this diagnostic work. No SQL or Jobs were run, and no Terraform or IAM change was made.
- Exact cause of the earlier Cloud SQL 4xx remains unknown because its status/reason were discarded and no matching SQL Admin audit entry is available. This PR enables safe diagnosis on a future separately released attempt; it does not retry the operation or authorize a credential, SQL, or Job action.
- The temporary IAM expiry remains the original absolute timestamp `2026-10-16T15:26:16Z`; it is not renewed here.
