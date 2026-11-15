# Evidencia — VI-03, integridad de descarga de evidencia

- **Fecha:** 2026-11-12.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservan los demás cambios.

## Alcance

`TestReviewerDownloadRejectsContentWithMismatchedSizeOrDigest` usa `ReviewerAccessService` real, grant identity exacto y Snapshot sintético válido. Comprueba por separado (1) bytes con longitud incorrecta y (2) bytes de la misma longitud pero digest distinto; asserts explícitos comprueban que las dos fixtures satisfacen sus condiciones antes de llamar al servicio. Ambos producen `ErrDocumentStorage`, sin retornar bytes/MIME; el Store identity se lee una vez y el Store space no se consulta. Test-only; primera ejecución pasó como caracterización del guard existente (sin RED ni cambio productivo). Revisión independiente `deleg_d5d42fb0` aprobó sin hallazgos; rerun del revisor fue solo test focal normal. Se añadieron asserts explícitos de tamaño/digest y se repitió test focal normal/`-race`: PASS.

## Verificación local

- Test focal normal y `-race`: PASS.
- `go test ./internal/documentverification/... -count=1`: PASS.
- `go test -race ./internal/documentverification/... -count=1`: PASS.
- `go vet ./internal/documentverification/... ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido archivo nuevo: PASS.
- Variables DB/PG retiradas de comandos Go.

## Límites

Repositorio y Store fake; no prueba alteración/corrupción en filesystem/GCS, autorización integrada, atomicidad, PostgreSQL, persistencia o transporte HTTP. No implica que un mismatch sea automáticamente corregible ni borra el objeto. Sin SQL/DDL, bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue.
