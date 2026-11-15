# Evidencia — VI-05, cancelación de borrado en adapter cifrado

- **Fecha:** 2026-11-12.
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y ampliamente dirty; se preservaron los demás cambios existentes.

## Cambio

`documentstorage.Store.remove` ahora devuelve `ctx.Err()` si `BlobStore.Delete` falla mientras el contexto de la operación ya fue cancelado. Los errores de backend no relacionados con cancelación continúan normalizados a `ErrStorage`.

`TestStoreDeletePropagatesCancellationReturnedByBlobStore` cancela el contexto desde el BlobStore fake durante `Delete` y simula que el backend retorna `context.Canceled`. RED previo: el adapter convertía el resultado a `ErrStorage`; GREEN: propaga `context.Canceled`.

## Verificación local

Con variables de DB/PG retiradas:

- Test focal — PASS después del cambio.
- `go test -race ./internal/adapters/documentstorage ./internal/adapters/postgres/documentverification ./internal/documentverification/... -count=1` — PASS en todos los paquetes listados.
- `go vet` de los paquetes listados — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt` — PASS.

## Límites

Fake-only; no prueba cancelación remota real, resultado ambiguo del backend, borrado GCS, durabilidad, retry/lease PostgreSQL ni worker productivo. No se ejecutó SQL/DDL, ni se accedió a bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue.
