# Evidencia — VI-05, cancelación de escritura en adapter cifrado

- **Fecha:** 2026-11-12.
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y ampliamente dirty; se preservaron los demás cambios existentes.

## Cambio

`documentstorage.Store.put` ahora propaga `ctx.Err()` cuando `BlobStore.Put` falla durante la cancelación del caller; los errores de backend sin cancelación continúan normalizados a `ErrStorage`.

`TestStorePutPropagatesCancellationReturnedByBlobStore` cancela el contexto dentro del BlobStore fake, que devuelve `context.Canceled`; RED previo: el adapter respondía `ErrStorage`. GREEN: propaga la cancelación y el fake confirma que no dejó objeto.

## Verificación local

Con variables DB/PG/opt-in de integración retiradas:

- Test focal — PASS después del cambio.
- `go test -race ./internal/adapters/documentstorage ./internal/adapters/postgres/documentverification ./internal/documentverification/... -count=1` — PASS en todos los paquetes listados.
- `go vet` de los paquetes listados — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt` — PASS.

## Límites

Fake-only; no prueba resultado ambiguo ni escritura remota, GCS, persistencia, retry/lease durable o worker integrado. No se ejecutó SQL/DDL ni se accedió a bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue.
