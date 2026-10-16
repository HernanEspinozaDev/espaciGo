# GCP-03 disposable scratch root

This root is independent of `../gcp` and uses a separate GCS state prefix. It reads the existing VPC/subnetwork and manages only resources named for one scratch `run_id`. It must never be initialized with the persistent root's state prefix. The reviewed database pairing is fixed to N4 `db-custom-N4-2-4096` plus 20 GiB Hyperdisk Balanced; confirm availability before a future apply.

Private backend configuration example, stored under `.local/` with directory mode 0700 and file mode 0600:

```hcl
bucket  = "espacigo-testprod-tfstate-447373903153-gcp"
prefix  = "espacigo/testprod/gcp03-scratch"
```

Use `terraform init -reconfigure -backend-config=<private-file>` for this root only. Set `project_id=espacigo-testprod`, `region=southamerica-west1`, and a unique run ID in private tfvars. Scratch/database and Job gates default to false. `enable_job` stays false until both a pushed digest in this run's Artifact Registry repository and a separately provisioned temporary secret version have been verified. Terraform never writes a SQL password or creates a SQL user.

Destroy procedure is deliberately two step and run-specific. First set `allow_scratch_destroy=true` and `confirm_destroy_run_id` to the exact `run_id` while keeping `enable_scratch=true` (and `enable_job=true` if the job exists), then review/apply only the protection changes. Next set the scratch/job gates false with the same run ID and review/apply the destroy plan for this root. The scratch-only secret container and its metadata are then deleted as well; its versions are deliberately never Terraform-managed. Never run `terraform destroy` against another root or state prefix.

The Terraform `plan` operation is the only cloud-side action permitted while preparing/reviewing this PR; Terraform may initialize an empty `default.tfstate` object at this new prefix, which must be preserved. No `apply`, Job execution, SQL, secret version, image upload, database user, restore, or import is part of the change.
