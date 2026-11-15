# Evidencia — VI-03, acceso del revisor al vencer la retención

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y muy dirty; cambios ajenos preservados.

## Alcance

- `TestReviewerDownloadRejectsExpiredRetentionBeforeReadingStore` fija un reloj sintético y cubre `RetentionAt` anterior al instante actual y exactamente igual al instante actual.
- Con caso/reviewer grant válidos, ambos escenarios devuelven `ErrNotFound`, sin contenido/MIME y con cero lecturas del Store fake.
- Test-only; no cambia la semántica productiva ni fija vencimientos civiles/documentales.

## Verificación local

- Test focal: PASS.
- `go test ./internal/documentverification/... -count=1`: PASS.
- `go test -race ./internal/documentverification/... -count=1`: PASS.
- `go vet ./internal/documentverification/... ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluidos archivos nuevos de esta ronda: PASS.
- Variables DB/PG retiradas de comandos Go.

## Límites

Caracteriza el guard de acceso contra fakes y reloj controlado; no prueba reloj/estado PostgreSQL, tarea de limpieza durable, storage, transacción ni lectura concurrente. Revisión independiente `deleg_7ada9c9f` aprobó; confirmó los casos `RetentionAt < now` y `== now`, cero lecturas de Store y separación respecto de vencimiento civil/documental. Sin SQL/DDL, bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue.
