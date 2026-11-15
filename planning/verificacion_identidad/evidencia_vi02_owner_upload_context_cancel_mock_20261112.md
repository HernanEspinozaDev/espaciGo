# Evidencia VI-02 — carga owner y cancelación del request

- **Fecha:** 2026-11-12
- **Rama:** `codex/marketplace/fix/kyc-06-save-identity-first`
- **HEAD base del corte:** `274f37e483a078890e92382a159fa49fd23c2fe8`
- **Cambio:** la ruta HTTP de carga vuelve a verificar el contexto tras parsear el multipart y antes de invocar `Upload`; también comprueba después de un resultado exitoso y antes de fijar ETag/serializar receipt.

## TDD y resultado

`TestOwnerUploadDoesNotStartOrReturnAfterCancellation` observa dos límites:

- Request precancelado: RED observado; el handler llamaba al servicio y devolvía HTTP 201/ETag/receipt. GREEN: 503 neutral sin llamar al servicio.
- El servicio fake finaliza el upload y cancela antes de retornar receipt: RED observado; el handler devolvía HTTP 201 con ID/estado de archivo y ETag. GREEN: 503 neutral sin receipt ni ETag. La llamada de servicio ocurrió una vez; el estado del upload real se debe reconciliar mediante GET separado.

## Verificación local

Con entorno mínimo sin variables DB/PG:

- Tests focales de cancelación, contrato de receipt y compatibilidad de upload space sin role: PASS.
- `go test -race ./internal/documentverification/... -count=1`: PASS.
- `go vet ./internal/documentverification/...`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- `gofmt -d` y `git diff --check`: PASS.

## Límites

El servicio, auth, multipart y receipt son sintéticos; el test caracteriza la frontera HTTP, no el almacenamiento ni la operación del request body en sockets reales. No demuestra integración Go→DB/GCS, commit, autorización efectiva o cancelación del driver. Sin SQL/DDL, consultas, migraciones ni conexiones a bases persistentes/Cloud; sin datos reales, commit, push, integración a `main` o deploy.
