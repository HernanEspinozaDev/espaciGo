# GCP-03 origin observer IAM ordering repair

## Scope

This evidence accompanies the Terraform repair for the `g03o-1017` observer phase. The source is the merged GCP-03 bootstrap/observer definition at `6145b67a16a298855262ac05ddc70a5da08f6b42`. No migration, snapshot staging, import, restore, observer execution, or resource cleanup is part of this repair.

## Partial apply and read-only reconciliation

The reviewed observer plan `ba0e8fe29485501e1a764cdf1eb8b9005146b27bbab10fcafb4c35541c7ddfec` was applied once under release E. Terraform created the observer service account and its two instance-conditioned project bindings, then Cloud SQL rejected the IAM database-user request with HTTP 400: `Provided CLOUD_IAM_SERVICE_ACCOUNT: g03o-observer-g03o-1017@espacigo-testprod.iam.gserviceaccount.com, does not exist.` The apply ended with an error. No observer Job creation was reported, and no observer execution was started.

After the failure, read-only API checks confirmed:

- Terraform state lineage `4e81a953-7353-fbd4-93a8-e4b9a5019e95`, serial 7, and 11 managed resources.
- The service account `g03o-observer-g03o-1017@espacigo-testprod.iam.gserviceaccount.com` exists.
- `roles/cloudsql.client` and `roles/cloudsql.instanceUser` are present for that account, each conditioned on the exact run-scoped Cloud SQL instance.
- The only matching SQL user is absent; the only Cloud Run Job is the existing bootstrap Job.
- The Cloud SQL operations list has no observer IAM-user create operation. Its recent user operation is the earlier successful postgres `UPDATE_USER` at `2026-10-17T08:32:40Z`. A Cloud Audit Logs query for `sqladmin.googleapis.com` during `08:38–08:50Z` returned no entries.

The Terraform apply output showed the service account and SQL-user resources entering `Creating...` together. In the merged source, `google_sql_user.origin_observer.name` was built from local strings and therefore had no dependency edge on the service account. The source had no ordering guarantee that the account existed before the SQL Admin API request. The exact service-account visibility at the request instant cannot be recovered from the available operation/audit records, so propagation delay cannot be ruled in or out. The confirmed code defect is the missing dependency edge; this repair does not assert that propagation was the sole cause.

## Repair and verification

`google_sql_user.origin_observer.name` now derives from `google_service_account.origin_observer[0].email` using `trimsuffix(..., ".gserviceaccount.com")`. This retains the same Cloud SQL IAM login and gives Terraform an explicit service-account prerequisite. The observer Terraform test asserts that the database user name equals the trimmed service-account email.

Validation used Terraform 1.16.6 and the locked Google provider 8.7.0 in an isolated local directory with the GCS backend disabled and a mocked Google provider:

- `terraform test -no-color`: 5 passed, 0 failed.
- `terraform graph -type=plan` with the reviewed observer inputs contains `google_sql_user.origin_observer -> google_service_account.origin_observer`; the observer Job also depends on the SQL user. In Terraform's graph notation, the dependent points to its prerequisite.
- A new read-only plan against state serial 7 contains only two creates: the missing SQL IAM user and observer Job. The service account and two bindings are no-ops, and the bootstrap Job remains unchanged. This is a plan for review only, not approval to apply or execute.

The sanitized bootstrap evidence used for the observer gate has SHA-256 `429fce77b7b1a0890a42223112604f66343adfe4a3224dc94b77f62368adfb57`. No secret payload was read or recorded.

## Window and cost

The run began at `2026-10-17T07:55:05.478Z`. The reviewer extended the operating window through `2026-10-17T20:15:00Z`. At the reference Cloud SQL plus disk rate of USD 0.20221/hour, that is about USD 2.50 for 12 hours 20 minutes; the rate includes the configured disk and is not an invoice or a spend cap. It excludes logging, network, storage use, taxes, credits, and later Cloud Run execution. The extension permits preserving resources and preparing/reviewing plans; it does not authorize applying this repair plan or running the observer.

## Remaining limits

The partial resources remain in place and must not be deleted or recreated. No state import was performed. The observer plan requires a new review of its binary SHA-256 before any apply; the previous E/F releases stopped at the failed apply. Snapshot staging, origin restore, and acceptance of the rehearsal remain separate decisions.
