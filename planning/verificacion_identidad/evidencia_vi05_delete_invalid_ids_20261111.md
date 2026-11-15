# Evidencia — VI-05, borrado identity rechaza IDs inválidos

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y muy dirty; cambios preexistentes preservados.

## Alcance

- Test-only characterization: `TestPrivateStoreIdentityDeleteRejectsInvalidIDsWithoutTouchingValidObject` crea un objeto identity válido en `t.TempDir()` y prueba owner, case y file IDs con `../escape`.
- Cada borrado inválido devuelve `ErrInvalid`; readback confirma que el objeto válido mantiene exactamente sus bytes y `os.Stat` confirma su path.
- No cambia el path resolver ni código productivo.

## Verificación local

- Test focal: PASS.
- `go test ./internal/adapters/documentfs -count=1`: PASS.
- `go test -race ./internal/adapters/documentfs -count=1`: PASS.
- `go vet ./internal/adapters/documentfs ./internal/adapters/postgres/documentverification ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido archivo nuevo: PASS.
- Variables `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `DATABASE_URL`, y `PG*` retiradas de comandos Go.

## Límites

Caracteriza únicamente validación de IDs antes del borrado en filesystem temporal; no acredita aislamiento/concurrencia, autorización del caller, SQL, persistencia, GCS ni borrado forense. Revisión independiente `deleg_410428fb` aprobó sin hallazgos; confirmó rechazo de los tres IDs traversal-like y conservación de bytes/path válidos. Sin SQL/DDL, bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue.
