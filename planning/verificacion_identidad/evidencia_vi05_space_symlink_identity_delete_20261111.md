# Evidencia — VI-05, symlink identity hacia objeto space

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y muy dirty; cambios preexistentes preservados.

## Alcance

- `TestPrivateStoreIdentityDeleteDoesNotFollowSymlinkIntoSpaceObject` usa `documentfs.Store` real en `t.TempDir()`, sustituye el path identity final por un symlink que apunta al path real del objeto space con IDs compartidos.
- La eliminación identity remueve el link final; `ReadSpaceDocument` confirma que el objeto space conserva sus bytes.

## Verificación local

- Test focal: PASS.
- `go test ./internal/adapters/documentfs -count=1`: PASS.
- `go test -race ./internal/adapters/documentfs -count=1`: PASS.
- `go vet ./internal/adapters/documentfs ./internal/adapters/postgres/documentverification ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido archivo nuevo: PASS.
- Variables `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `DATABASE_URL` y `PG*` retiradas de comandos Go.

## Límites

Prueba secuencial con Store y filesystem temporal; no demuestra autorización caller, TOCTOU/concurrencia, storage remoto, GCS, borrado durable o forense. Revisión independiente `deleg_7f9e48ea` aprobó sin hallazgos; acotó la cobertura a unlink del componente final y target space legible, sin parent symlinks o carreras. Sin SQL/DDL, bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue.
