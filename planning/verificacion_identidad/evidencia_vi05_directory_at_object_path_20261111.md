# Evidencia — VI-05, no eliminar recursivamente un path de objeto

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y muy dirty; cambios preexistentes preservados.

## Alcance

- `TestPrivateStoreIdentityDeleteDoesNotRecursivelyRemoveDirectory` crea objeto identity sintético en `t.TempDir()`, reemplaza el archivo final por un directorio no vacío con sentinel sintético, y ejecuta `DeleteIdentityDocument` del Store real.
- Espera `ErrStorage` y verifica que directorio y bytes del sentinel permanecen; el Store no realiza borrado recursivo ni declara éxito.

## Verificación local

- Test focal: PASS.
- `go test ./internal/adapters/documentfs -count=1`: PASS.
- `go test -race ./internal/adapters/documentfs -count=1`: PASS.
- `go vet ./internal/adapters/documentfs ./internal/adapters/postgres/documentverification ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluidos archivos nuevos de esta ronda: PASS.
- Variables DB/PG retiradas de comandos Go.

## Límites

Prueba secuencial sobre Store/filesystem temporal; no demuestra autorización, TOCTOU/concurrencia, GCS, reintento durable ni borrado forense. Revisión independiente `deleg_fa4cb175` aprobó sin defectos; confirmó que `os.Root.Remove` no elimina recursivamente y que sentinel/directorio permanecen. Sin SQL/DDL, bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue.
