# Evidencia — VI-05, cleanup de copia space del propietario anterior

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y muy dirty; cambios preexistentes preservados.

## Alcance

- `TestRetentionProcessorDeletesOldOwnerSpaceCopyAndPreservesCurrentOwnerCopy` siembra dos objetos space sintéticos en `documentfs.Store` real y `t.TempDir()`: archivo superseded del propietario anterior y archivo vigente del nuevo propietario.
- Ejecuta `RetentionProcessor` real con claim fake acotado al propietario anterior, espacio, caso y file ID antiguos; verifica ausencia del path anterior y preservación de path y bytes del propietario vigente.
- El job/claim/completion es fake; no se cambia producción.

## Verificación local

- Test focal: PASS.
- `go test ./internal/adapters/postgres/documentverification -count=1`: PASS.
- `go test -race ./internal/adapters/postgres/documentverification -count=1`: PASS.
- `go vet ./internal/adapters/postgres/documentverification ./internal/adapters/documentfs ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido archivo nuevo: PASS.
- Variables DB/PG retiradas de comandos Go.

## Límites

Caracteriza limpieza local del path antiguo; no demuestra elegibilidad del job, cambio de titular persistido, ownership/locks PostgreSQL, GCS, recuperación o borrado forense. Revisión independiente `deleg_fea678df` aprobó sin hallazgos; confirmó que el claim fake elimina el scope anterior y conserva path/bytes del nuevo titular. Sin SQL/DDL, bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue.
