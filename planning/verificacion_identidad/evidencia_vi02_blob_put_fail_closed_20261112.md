# Evidencia — VI-02, fallos seguros de lectura y escritura cifrada

- **Fecha:** 2026-11-12.
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8` (HEAD consultado antes de los cambios).
- **Estado:** worktree compartido y ampliamente dirty; se tocaron únicamente `internal/adapters/documentstorage/store.go`, `internal/adapters/documentstorage/store_test.go`, este archivo y la referencia correspondiente en el plan.

## Alcance y resultado

Se caracterizaron fallos seguros de escritura y se corrigió un caso de cancelación durante lectura del adapter de documentos cifrados:

- `TestStorePutCancellationAndBackendFailureAreFailClosed`: un contexto ya cancelado no invoca `BlobStore.Put`; un fallo de escritura que contiene una ruta privada se normaliza a `ErrStorage`, sin filtrar ni encadenar el error interno y sin dejar objeto en el fake.
- `TestStoreReadDoesNotReturnPlaintextWhenContextCancelsDuringBlobRead`: un BlobStore sintético cancela el contexto durante `Get` y aun así devuelve el envelope. RED observado antes del cambio: la lectura devolvía `nil` error y exponía plaintext. En `Store.read`, ahora se revalida `ctx.Err()` inmediatamente después de `Get` y después del descifrado; si hay cancelación se devuelve `nil` contenido y el error de contexto.
- `TestStoreReadPropagatesCancellationReturnedByBlobStore`: el fake cancela el contexto y devuelve `context.Canceled`. RED observado antes del cambio: el adapter lo convertía en `ErrStorage`; ahora prioriza `ctx.Err()` cuando el `Get` falla para preservar la cancelación del caller sin contenido.

La corrección limita la ventana de entrega plaintext tras cancelación; no pretende controlar el backend ni cancelar operaciones ya completadas. Los datos y backend fueron sintéticos.

## Verificación local

Con variables de conexión y opt-in de integración retiradas:

- Ambos tests de cancelación: RED observados por los resultados incorrectos indicados arriba; pasan después de la corrección.
- `go test -race ./internal/adapters/documentstorage -count=1` — PASS.
- `go vet ./internal/adapters/documentstorage` — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt` — PASS.

## Límites

No se ejecutó SQL/DDL, se accedió a bases persistentes, Cloud, proveedor externo ni datos reales. No se probó GCS, persistencia, atomicidad, IAM, key-version/CMEK ni wiring runtime. No hubo commit, push, integración a `main` o despliegue. VI-02 y el plan general siguen parciales.
