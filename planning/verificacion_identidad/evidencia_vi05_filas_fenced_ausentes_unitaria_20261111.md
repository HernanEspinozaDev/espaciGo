# Evidencia — VI-05, fila fenced ausente (unitaria sintética)

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; cambios ajenos preservados.

## Alcance

- `TestProcessClaimedRetentionJobStopsWhenFencedRowIsMissing` ejecuta el método real `Repository.ProcessClaimedRetentionJob` con una transacción `pgx.Tx` sintética.
- El fake devuelve `pgx.ErrNoRows` para tres lecturas independientes: caso, archivo elegible para retención y job de retención que coincide con el claim.
- En los tres escenarios espera `dv.ErrConflict`, ningún callback de borrado, ningún Exec/commit y un rollback diferido.
- Es una caracterización de manejo de error/flujo en Go; no cambia código productivo.

## Verificación local

- Test focal: PASS.
- Suite focal de seis tests `ProcessClaimedRetentionJob`: PASS; repetida con `-race`: PASS. Paquete completo `internal/adapters/postgres/documentverification`, normal y con `-race`: PASS, retirando del entorno todas las variables `ESPACIGO_*`, `PG*` y `DATABASE_URL`.
- `go vet ./internal/adapters/postgres/documentverification ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check (incluidos archivos nuevos): PASS.
- Variables DB/PG y opt-in del integration gate retiradas en comandos Go.

## Límites

La revisión independiente `deleg_8afb676a` aprobó sin hallazgos ni sugerencias; confirmó que cada `ErrNoRows` sintético llega a la rama prevista y que las aserciones y límites de evidencia son adecuados. El fake sintetiza `ErrNoRows` al interceptar cada lectura. No demuestra que las consultas SQL filtren por los predicados previstos, que PostgreSQL respete locks/aislamiento, ni que la elegibilidad por `retention_at` sea real. No acredita transacción durable, almacenamiento remoto, borrado/recuperación, backups, alertas ni privacidad global. Sin SQL/DDL, conexión a bases, Cloud, proveedores, datos reales, commit, push, integración a `main` o despliegue.
