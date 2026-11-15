# Evidencia — VI-05, symlink space hacia objeto identity

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y muy dirty; cambios preexistentes preservados.

## Alcance

- `TestPrivateStoreSpaceDeleteDoesNotFollowSymlinkIntoIdentityObject` usa `documentfs.Store` real en `t.TempDir()`, sustituye el path space final por un symlink al objeto identity real con IDs compartidos.
- `DeleteSpaceDocument` quita el link final; `ReadIdentityDocument` confirma que el objeto identity conserva bytes originales.

## Verificación local

- Test focal: PASS.
- `go test ./internal/adapters/documentfs -count=1`: PASS.
- `go test -race ./internal/adapters/documentfs -count=1`: PASS.
- `go vet ./internal/adapters/documentfs ./internal/adapters/postgres/documentverification ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido archivo nuevo: PASS.
- Variables DB/PG retiradas de comandos Go.

## Límites

Prueba secuencial con Store/filesystem temporal; no demuestra autorización, TOCTOU/concurrencia, symlink intermedio, GCS ni borrado durable/forense. Revisión `deleg_7f4a46fa` aprobó sin hallazgos; limita el caso a unlink del symlink final y bytes identity intactos, sin carreras o symlinks de componentes padre. Sin SQL/DDL, bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue.
