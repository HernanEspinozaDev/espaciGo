# GCP-03 local recovery review procedure

This recovery entrypoint is separate from the importer. It accepts only the already-restored `espacigo_scratch` database and the fixed V1–V61 normalization file. The recovery adapter exposes catalog reads and `psql` operations only; it does not invoke `pg_restore` or read `database.dump`. It decrypts the exact existing U2 private-files ciphertext in memory, then validates the tar inventory. The production diagnostic baseline is intentionally rejected when database identity, catalog inventory, migration history, role membership, or the reviewed artifact hashes differ.

## Review inputs

The reviewed anchors are:

- merge base `232be5e820d9273be6da883749045bb3688d5fbf`
- owner/ACL application report SHA-256 `3325288140b1f2de8bb944f7c4c1599fcf56f1a19531dc334bf171629f99391a`
- table inventory SHA-256 `5d78e5d55d9021a935b7210977395c3dae2540c983e4266107b2583619aaabe1` (96 relations)
- migration history SHA-256 `5dd344d805ecdf9c500eabac8e2013b0186a38066d3d17599d59ecef62c54cdd` (61 migrations)
- U2 manifest SHA-256 `c5fc1964175244e998742e34dd4c4376b2d822b27574416b61228b912641ec30`
- U2 normalization SQL SHA-256 `2ca53ad3e92e078c30375c691ca6259197be54f49c4682750ed73dd41d6820b2`
- U2 receipt SHA-256 `42f817e89382af2df3e0f452b594311936e91839fd92cc251f133bd4d867377c`, generation `1792273080133875`, 1892 bytes
- PostgreSQL major version 18, database `espacigo_scratch`, login `postgres`

The diagnostic Job reported Cloud Run task completion and a complete read-only application report. These are separate facts; `scripts/gcp03_job_evidence.py --phase recover` requires the recovery JSON report to set both `phase_success` and `report_complete` before accepting a successful execution.

## Catalog inspection needed for a production plan

The read-only Cloud plan Job performs the per-object catalog inspection itself, including function and text-search ownership. After the plan Job is authorized and run, review its catalog facts hash, current inventory, pending action counts, and complete plan SHA-256. `scripts/gcp03_origin_recovery_catalog.sql` is available for an independent manual catalog snapshot if the Job needs diagnosis; that query selects no application row values and makes no changes.

The branch does not run the Cloud plan Job. The PG18 fixture report is synthetic test evidence and cannot replace the production plan output.

## Local fixture

Use the pinned PG18 image `postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677`. Start a task-owned container with `--network none`, a tmpfs data directory, the image init scripts hidden, and only a private Unix-socket bind mount. Use `fixture_control` as the bootstrap superuser and create a separate `postgres` login with `NOSUPERUSER CREATEROLE CREATEDB`; grant the runtime role with `SET TRUE, INHERIT FALSE, ADMIN FALSE`. Keep that container and its evidence until the preservation cutoff. `scripts/test-gcp03-origin-recovery-postgres.sh` verifies the container name, digest, running state and isolated network, runs only the synthetic integration test, and never starts or removes a container.

The fixture includes a serial-owned sequence, a standalone sequence, a function, a text-search dictionary and configuration, plus the runtime membership and observer ACL. The test confirms table ownership transfer cascades to its serial sequence, skips the redundant `ALTER SEQUENCE`, and still explicitly transfers the standalone sequence. It does not restore the origin dump or simulate the 96-table production database.

## Recovery gates

1. Validate the historic diagnostic report and the current copy’s exact identity, inventory, migration hash, role membership, schema privileges and observer privileges.
2. Read the exact reviewed normalization SQL and private-files archive, validate their hashes and archive inventory, then create a plan whose hash binds those artifacts and the current catalog facts. The plan summary contains statement hashes and actions, never SQL text or object names.
3. Review the exact plan hash and preserve the plan file mode `0600`.
4. Immediately before apply, rebuild the plan from the current catalog and compare it with the approved hash. Any mismatch stops before mutation.
5. Apply only pending owner/ACL statements in one transaction. A failure is never retried; an uncertain transaction outcome stops the sequence.
6. Check relation, routine, text-search and owned-sequence owners, observer write privileges and runtime `public` CREATE before sanitation.
7. Sanitize with the existing fixed quarantine SQL. Check all four quarantine counts are zero and runtime login is disabled.
8. Compare all private-file references against the fully validated archive. Copy files only with create-only semantics; a collision or copy failure stops without retry.
9. Preserve the complete application report separately from Cloud Run task status. Do not accept `Completed=True` as proof of recovery.

Terraform now defines a default-off recovery Job with separate plan and apply gates. Both modes pin the existing admin password and K_run secret versions to version 1, use the existing private SQL proxy and shared admin service account, and consume the existing U2 receipt, manifest, normalization SQL, and private-files ciphertext through their exact hash/generation references. The shared service account already has conditional `roles/storage.objectCreator` on the exact private run prefix because the existing importer uses it. Plan mode makes no call to the writer, sanitizer, or recovery executor; tests assert this path. Apply additionally requires a reviewed plan SHA-256 and recalculates the current catalog-bound plan before any mutation. The private-file destination uses create-only writes in the existing private bucket; the flow creates no recovery receipt or support object.

The local preflight CLI also supports the separate normalization artifact hash `865697a41f9ca70881522f7558093e8ac4107dfa5a4e58dbf749d5f670eff842` used by the local reviewed copy. Cloud recovery does not use that local constant: it pins and validates the existing U2 normalization hash `2ca53ad3e92e078c30375c691ca6259197be54f49c4682750ed73dd41d6820b2` from the receipt and manifest. The Terraform plan gate enforces the exact U2 manifest and normalization hashes.

The Cloud image builder creates only a private local OCI archive and manifest. No OCI publish, Cloud Run execution, production plan result, or recovery mutation has been performed in this preparation. The Terraform gates remain off. The plan Job should run once after review of this branch; its own read-only catalog scan produces the production-bound plan for separate review. The synthetic PG18 fixture proves the test path and production-baseline rejection only.
