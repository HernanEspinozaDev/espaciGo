# GCP-03 local recovery review procedure

This recovery entrypoint is separate from the importer. It accepts only the already-restored `espacigo_scratch` database and the fixed V1–V61 normalization file. The recovery adapter exposes catalog reads and `psql` operations only; it does not invoke `pg_restore` or read `database.dump`. It decrypts the exact existing U2 private-files ciphertext in memory, then validates the tar inventory. The production diagnostic baseline is intentionally rejected when database identity, catalog inventory, migration history, role membership, or the reviewed artifact hashes differ.

## Review inputs

The reviewed anchors are:

- merge base `232be5e820d9273be6da883749045bb3688d5fbf`
- owner/ACL application report SHA-256 `3325288140b1f2de8bb944f7c4c1599fcf56f1a19531dc334bf171629f99391a`
- table inventory SHA-256 `5d78e5d55d9021a935b7210977395c3dae2540c983e4266107b2583619aaabe1` (96 relations)
- migration history SHA-256 `5dd344d805ecdf9c500eabac8e2013b0186a38066d3d17599d59ecef62c54cdd` (61 migrations)
- normalization SQL SHA-256 `865697a41f9ca70881522f7558093e8ac4107dfa5a4e58dbf749d5f670eff842`
- PostgreSQL major version 18, database `espacigo_scratch`, login `postgres`

The diagnostic Job reported Cloud Run task completion and a complete read-only application report. These are separate facts; `scripts/gcp03_job_evidence.py --phase recover` requires the recovery JSON report to set both `phase_success` and `report_complete` before accepting a successful execution.

## Catalog inspection needed for a production plan

The current saved diagnostic contains aggregate relation owners and sequence dependency counts, but it does not contain per-object function and text-search ownership. Before a real plan is reviewed, run the fixed read-only catalog query in `scripts/gcp03_origin_recovery_catalog.sql` against the already-restored copy and preserve its output privately. It selects database identity, role/membership flags, object names and owners, sequence-parent links, observer column privilege flags, and migration metadata. It reads no application row values and contains no write statement. The local preflight also queries each exact allowlisted normalization object to classify it as `postgres` or `espacigo_runtime`, and fails on missing, extra, ambiguous, or unknown facts.

No Cloud query or recovery mutation is authorized by this procedure. If the exact restored-copy catalog output is unavailable, the recovery plan remains unprepared; the PG18 fixture report is synthetic test evidence and cannot replace production state.

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

Terraform now defines a default-off recovery Job with separate plan and apply gates. Both modes pin the existing admin password and K_run secret versions to version 1, use the existing private SQL proxy and service account, and consume the receipt, manifest, normalization SQL, and private-files ciphertext through their exact hash/generation references. Apply additionally requires a reviewed plan SHA-256 and recalculates the current catalog-bound plan before any mutation. The private-file destination uses create-only writes in the existing private bucket; the flow creates no recovery receipt or support object.

The Cloud image builder creates only a private local OCI archive and manifest. No OCI publish, Cloud Run execution, production plan result, or recovery mutation has been performed in this preparation. The Terraform gates remain off. The real production plan still requires the fixed read-only catalog query above and reviewer inspection of its output; the synthetic PG18 fixture proves the test path and production-baseline rejection only.
