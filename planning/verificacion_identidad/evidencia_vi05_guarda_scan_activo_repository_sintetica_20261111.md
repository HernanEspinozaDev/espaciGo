# Evidencia — VI-05, scan activo bloquea borrado (unitario)

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron los cambios previos.

## Alcance

- `TestProcessClaimedRetentionJobDoesNotDeleteWhileScanIsActive` ejerce `Repository.ProcessClaimedRetentionJob` real con `pgx.Tx` sintético y lease exacta/vigente.
- La fila fake indica un scan activo. Se espera `dv.ErrConflict`, callback de borrado nunca llamado, sin Exec/completion/commit y con rollback diferido.
- Caracterización de guard de control de flujo; la fila del scan y demás respuestas son prefijadas por el fake.
- Sin cambios productivos.

## Verificación local

- Test focal: PASS.
- Suite focal de los cuatro tests `ProcessClaimedRetentionJob`: PASS.
- Mismo filtro con `-race`: PASS.
- `go vet ./internal/adapters/postgres/documentverification`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido archivo nuevo: PASS.
- Variables DB/PG y opt-in del integration gate retiradas en comandos Go.

## Límites

La revisión independiente `deleg_f346fc4f` aprobó sin hallazgos de seguridad/lógica y confirmó la limitación del fake frente a PostgreSQL. Sugirió como mejora opcional explicitar el orden de rollback en la traza; el test ya verifica que el rollback diferido se ejecutó una vez antes de retornar. El fake no valida la consulta SQL, locks, timing, transacción ni comportamiento ante una carrera real con el scanner en PostgreSQL. Tampoco demuestra persistencia, borrado remoto/durable, retries, logging, backup/recuperación ni alertas. No se ejecutó SQL/DDL ni se conectó a bases/Cloud/proveedores; solo fixtures sintéticos locales.
