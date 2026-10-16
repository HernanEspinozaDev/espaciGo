# GCP-03 disposable scratch root

This root is independent of `../gcp` and uses a separate GCS state prefix. It reads the existing VPC/subnetwork and manages only resources named for one scratch `run_id`. It must never be initialized with the persistent root's state prefix. The reviewed database pairing is fixed to N4 `db-custom-N4-2-4096` plus 20 GiB Hyperdisk Balanced; confirm availability before a future apply.

Private backend configuration example, stored under `.local/` with directory mode 0700 and file mode 0600:

```hcl
bucket  = "espacigo-testprod-tfstate-447373903153-gcp"
prefix  = "espacigo/testprod/gcp03-scratch"
```

Use `terraform init -reconfigure -backend-config=<private-file>` for this root only. Set `project_id=espacigo-testprod`, `region=southamerica-west1`, and a unique run ID in private tfvars. Scratch/database and Job gates default to false. Scratch `roles/cloudsql.client` grants use an IAM condition that limits access to the named disposable instance. `enable_job` stays false until the exact pushed digest and temporary secret versions are verified, the credential procedure is reviewed, and the V60 Cloud SQL permission blocker is resolved. Terraform never writes a SQL password or creates a SQL user.

Destroy procedure is deliberately two step and run-specific. First set `allow_scratch_destroy=true` and `confirm_destroy_run_id` to the exact `run_id` while keeping `enable_scratch=true` (and `enable_job=true` if the job exists), then review/apply only the protection changes. Next set the scratch/job gates false with the same run ID and review/apply the destroy plan for this root. The scratch-only secret container and its metadata are then deleted as well; its versions are deliberately never Terraform-managed. Never run `terraform destroy` against another root or state prefix.

The separate `enable_role_probe_credential_permissions` gate creates only the temporary credential-operator roles/bindings. Cloud SQL `users.*` permissions are treated as project-scoped: the custom role is narrowly limited to the exact user methods and has a fixed expiry, but can affect SQL users on any Cloud SQL instance in the project during that window. A separate confirmation variable makes this scope/risk explicit. Secret-version permissions attach only to the scratch migrator secret and expire at the same timestamp.

The `enable_role_probe_jobs` gate defines only `role_comment_prepare` and `role_comment_after_reconcile`; it requires the already-existing uploaded image digest and an already-existing active numeric secret version, and creates only Jobs plus per-Job invocation/actAs. It does not grant credential-creation permissions. Each gate stays off by default. See `planning/gcp/gcp03_v60_role_comment_probe_plan_20261016.md` for the current preflight, resource IDs, digest, separated plan sequence, permissions and remaining review decisions. No gate enables itself, and none authorizes SQL, migrations, imports, or Terraform apply without the separate reviewed operation plan.

Terraform planning is read-only against this root's existing state. A plan-only change does not authorize `apply`, Job execution, SQL, secret versions, database users, restore, or import. An OCI upload, when separately authorized, is a distinct action and must record the exact source commit, archive checksum, repository, tag, and verified remote digest.
