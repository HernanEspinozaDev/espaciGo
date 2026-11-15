# Evidencia — ventanas de autoridad V95/V96 en PostgreSQL desechable

- **Fecha:** 2026-11-14.
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama / HEAD base:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`; worktree compartido y dirty. El HEAD no contiene estos cambios sin commit.
- **Alcance:** scripts versionados, harness y ejecución únicamente en PostgreSQL desechable fijado a 18.7. No se consultaron filas de negocio ni secretos.

## Cambios del corte

Se añadieron `db/repair/prepare_native_kyc_v96.sql` y `db/repair/finalize_native_kyc_v96.sql`, manteniendo los scripts V95 ya existentes. La preparación V96 exige el ledger continuo V1–V95 y checksums exactos V94/V95; valida identidad peer local, base/OID/puerto/engine, roles y membresías, ownership/indexes/secuencia, grants runtime y grantors, ACL exacta de `public` y del ledger, estado V95 y ausencia de esquema V96 parcial. Solo abre el `CREATE` temporal separado de V96. El finalizador cierra ese grant y lee de vuelta ACL, owners, grants/grantors y el límite del ledger V95 o V96 con sus checksums; no reescribe el ledger.

`internal/migrator/native_kyc_v95_v96_authority_window_test.go` ahora cubre target incorrecto sin DDL para ambas ventanas; prepara y finaliza V95/V96; aplica el SQL fuente V95 y V96 en sesiones sintéticas `espacigo_migrator_login` → `SET ROLE espacigo_schema_owner`; comprueba los límites de ledger, ACL/owner y cleanup/reintento. El scope `authority-window` exige los cinco resultados del test en `scripts/gcp/check_migration_ci.py` y su test unitario.

Checksums de SQL fuente consultados con `sha256sum`: V95 `351a9bbdbd890e34b3b5db46ecffa971d49b1f5d4519d9bf7f2b1eae1e897275`; V96 `a206e74995db47112c6b382c33ffab5badcab2ec9b9859af0fbd018478cc7ca5`.

## Ejecución observada

`TMPDIR="$HOME/.hermes/cache/scratch" scripts/test-migration-runner-postgres.sh authority-window` — **PASS**, cinco resultados requeridos, cero omitidos/faltantes. El runner usó `postgres@sha256:c7fedce47eff6a78cb35dad6b627f0d684107f75ab15c0870e40c093766ba452`, red `none`, sin puertos ni volúmenes, `PGDATA` temporal; verificó la retirada de su propio contenedor. Se ejecutaron migraciones V95/V96 reales solo en las bases sintéticas del contenedor.

También pasaron:

- `python3 -m unittest -v test_check_migration_ci` — 8/8.
- `python3 -m py_compile check_migration_ci.py test_check_migration_ci.py`.
- `env -u DATABASE_URL -u DATABASE_SET_ROLE -u PGPASSWORD -u PGPASSFILE -u PGOPTIONS -u PGSERVICE -u PGSERVICEFILE -u TEST_DATABASE_URL -u TEST_MIGRATOR_CI_ISOLATED go test -race -count=1 ./internal/migrator`.
- `go vet ./internal/migrator`, `gofmt -d` del test del corte y `git diff --check`.

La revisión independiente read-only del candidato está en curso (`deleg_7d6b9262`); sus hallazgos no están incorporados aún.

## Límites y gates pendientes

Esto demuestra comportamiento en PostgreSQL 18.7 desechable, no durabilidad, estado actual ni autorización efectiva del PostgreSQL nativo, Cloud SQL o producto. El preflight nativo del 2026-11-14 sigue reportando V1–V94, owners `postgres` para las dos tablas y `public CREATE=false`; **no se ejecutó DDL persistente**. No hay backup fresco validado para esta ventana: el backup histórico del 2026-11-09 no satisface ese gate. No se repitió el preflight nativo después de esta prueba, no se verificó el esquema actual de Cloud SQL y no se creó/ejecutó nada en GCP.

La aplicación nativa V95 está bloqueada hasta revisión independiente aprobada, backup privado fresco validado y preflight exacto inmediatamente anterior. Entre V95 y V96 se requieren cierre/readback de `CREATE`, ledger/postcheck y un backup fresco validado separado. No commit, push, integración a `main` ni despliegue. Los gates de privacidad/legal, MFA, operadores reales, GCS/CMEK y aceptación de producto permanecen separados.
