# Evidencia — VI-05, ausencia de objeto en el filesystem local tras limpieza

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y muy dirty; cambios preexistentes preservados.

## Alcance

- Se fortalecieron `TestRetentionProcessorDeletesIdentityFromPrivateStore` y `TestRetentionProcessorDeletesSpaceFromPrivateStore`.
- Tras procesar el claim con `RetentionProcessor` y `documentfs.Store` reales en `t.TempDir()`, cada test confirma tanto la lectura `os.ErrNotExist` como la ausencia del path exacto del envelope mediante `os.Lstat`.
- El test identity conserva legible el objeto space homónimo; el test space conserva legible el objeto identity homónimo.
- Solo test changes; no se alteró código productivo.

## Verificación local

- Tests focales de ambos propósitos: PASS.
- `go test ./internal/adapters/postgres/documentverification -count=1`: PASS.
- `go test -race ./internal/adapters/postgres/documentverification -count=1`: PASS.
- `go vet ./internal/adapters/postgres/documentverification ./internal/adapters/documentfs ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check: PASS.
- Variables `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `DATABASE_URL`, y `PG*` retiradas de comandos Go.

## Límites

Prueba ausencia del path local temporal, no borrado forense, bloques/backups, generaciones de object storage, GCS, persistencia del job o reinicio del host. Revisión independiente `deleg_6d2fb52f` aprobó sin hallazgos; confirmó paths exactos por propósito y conservación del objeto opuesto. Sin SQL/DDL, bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue.
