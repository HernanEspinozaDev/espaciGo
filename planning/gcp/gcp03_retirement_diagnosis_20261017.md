# GCP-03 retirement diagnosis execution — 2026-10-17

The read-only retirement diagnosis ran once after PR [#379](https://github.com/HernanEspinozaDev/espaciGo/pull/379) merged at `9fc68ceee93d7a04d0d595ba072bb88f23341c77`. The saved Terraform plan was verified against the current backend lineage and serial before application.

## Apply and execution

- Saved plan SHA-256: `dcafb7f89987dd6cb176d7220331742c1b0a5725e2164158ffb44bde334a8f70`.
- State lineage matched; serial was 9 before apply and 10 after. Terraform added the diagnosis Job and its job-scoped invoker only: 2 added, 0 changed, 0 destroyed.
- Job: `espacigo-gcp03-restore-g03r-1016-retire-diagnose`, configured with the reviewed image digest, existing admin service account, secret version 1, fixed Cloud SQL proxy target and private network, phase `retire-diagnose`, one task, no retries.
- Execution: `espacigo-gcp03-restore-g03r-1016-retire-diagnose-bwspn` (`uid` `5f42de9e-1c96-481a-9b2e-ae67784e7a65`). Cloud Run reported `Completed=True`, `succeededCount=1`; the task and both containers completed with exit code 0. The task API still returned an empty `lastAttemptResult.status`.
- The corrected validator accepted exactly one success payload whose execution label matched the API execution name. Result: `success`.

## Read-only diagnosis

The application asserted `transaction_read_only=true`; the code runs catalog and fixture-count queries in a `REPEATABLE READ READ ONLY` transaction. No retirement SQL, grants, or data mutations ran.

| Check | Result |
| --- | --- |
| Fixture rows in `public.usuario` / `public.espacio` / `public.ocupacion` | 2 / 1 / 2 |
| Migration history | Valid; inventory SHA-256 `e4086e95c3ecf05f9262aabfaa4e536a72503b992cc806b4fcf610748c8fa8c4` |
| Restore login | Exists, login capable, not superuser; expected group membership true; 2 direct memberships |
| Verify login | Exists, login capable, not superuser; expected group membership true; 2 direct memberships |
| Runtime current ACL contract | Pass |
| Runtime retired ACL contract | Fail |
| Retired database ACL contract | Fail |
| Retired migrator contract | Fail |
| Exact audit ACL contract | Pass |

The state is not fully retired: active runtime grants remain and the retired-state ACL checks do not pass. This confirms the cleanup did not complete, but does not identify the cause or substep of the earlier `PHASE_FAILED`; its saved failure payload had no substep. The diagnostic reports membership counts and contract flags, not individual extra membership names or ACL rows, so it is insufficient to attribute that failure to a specific grant.

## Limits and evidence

No retire, restore, verify, teardown, user deletion, object deletion, secret deletion, or cleanup ran. The diagnosis Job, invoker, Cloud SQL instance, service account and secret remain provisioned. No recovery or cleanup plan is claimed from this result.

The sanitized execution record, exact execution identity, diagnosis, validation result and evidence hashes are in [retirement-diagnosis-execution.json](evidence/gcp03-restore-g03r-1016/retirement-diagnosis-execution.json). Raw API JSON and logs remain private under `.local/testing/gcp03-1017/` with mode `0600`.
