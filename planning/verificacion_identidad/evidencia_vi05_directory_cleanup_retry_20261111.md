# Evidencia — VI-05, reintento ante directorio no vacío en path del objeto

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y muy dirty; cambios preexistentes preservados.

## Alcance

- `TestRetentionProcessorRetriesWhenObjectPathIsNonEmptyDirectory` reemplaza un objeto identity sintético por un directorio con sentinel bajo `t.TempDir()`.
- Processor y Store reales ejercen el fallo de borrado; job fake registra un retry `retention_store_error` con la demora base y cero completion, y el sentinel permanece.
- Sin cambios productivos; el estado del job lo observa solamente el fake.

## Verificación local

- Test focal: PASS.
- `go test ./internal/adapters/postgres/documentverification -count=1`: PASS.
- `go test -race ./internal/adapters/postgres/documentverification -count=1`: PASS.
- `go test ./internal/adapters/documentfs -count=1`: PASS.
- `go test -race ./internal/adapters/documentfs -count=1`: PASS.
- `go vet ./internal/adapters/postgres/documentverification ./internal/adapters/documentfs ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido archivo nuevo: PASS.
- Variables DB/PG retiradas de comandos Go.

## Límites

Retry/completion son observación fake; no prueba retry/lease durable, elegibilidad, PostgreSQL, GCS, recuperación o borrado forense. Revisión independiente `deleg_671b2210` aprobó sin defectos materiales ni claims sobrestimados; confirmó clasificación y demora del retry fake y preservación del sentinel. Sin SQL/DDL, bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue.
