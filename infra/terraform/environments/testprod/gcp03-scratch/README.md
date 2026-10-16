# GCP-03 disposable scratch root

This root is independent of `../gcp` and uses a separate GCS state prefix. It reads the existing VPC/subnetwork and manages only resources named for one scratch `run_id`. It must never be initialized with the persistent root's state prefix. The reviewed database pairing is fixed to N4 `db-custom-N4-2-4096` plus 20 GiB Hyperdisk Balanced; confirm availability before a future apply.

Private backend configuration example, stored under `.local/` with directory mode 0700 and file mode 0600:

```hcl
bucket  = "espacigo-testprod-tfstate-447373903153-gcp"
prefix  = "espacigo/testprod/gcp03-scratch"
```

Use `terraform init -reconfigure -backend-config=<private-file>` for this root only. Set `project_id=espacigo-testprod`, `region=southamerica-west1`, and a unique run ID in private tfvars. Scratch/database and Job gates default to false. Scratch `roles/cloudsql.client` grants use an IAM condition that limits access to the named disposable instance. `enable_job` stays false until the exact pushed digest and temporary secret versions are verified, the credential procedure is reviewed, and the V60 Cloud SQL permission blocker is resolved. Terraform never writes a SQL password or creates a SQL user.

Destroy procedure is deliberately two step and run-specific. First set `allow_scratch_destroy=true` and `confirm_destroy_run_id` to the exact `run_id` while keeping `enable_scratch=true` (and `enable_job=true` if the job exists), then review/apply only the protection changes. Next set the scratch/job gates false with the same run ID and review/apply the destroy plan for this root. The scratch-only secret container and its metadata are then deleted as well; its versions are deliberately never Terraform-managed. Never run `terraform destroy` against another root or state prefix.

The separate `enable_role_probe_jobs` gate defines only the fixed `role_comment_prepare` and `role_comment_after_reconcile` phases. It requires a digest-pinned uploaded image, an explicit numeric secret version, an operator and an expiry for temporary IAM. The review plan is in `planning/gcp/gcp03_v60_role_comment_probe_plan_20261016.md`. Cloud SQL user-method IAM Conditions must be proven effective for the exact instance before granting or running these Jobs. This gate remains off by default and does not authorize a SQL execution, migration, image upload, or Terraform apply.

Terraform planning is read-only against this root's existing state. A plan-only change does not authorize `apply`, Job execution, SQL, secret versions, database users, restore, or import. An OCI upload, when separately authorized, is a distinct action and must record the exact source commit, archive checksum, repository, tag, and verified remote digest.
