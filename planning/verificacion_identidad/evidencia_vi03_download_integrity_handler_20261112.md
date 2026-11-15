# Evidencia — VI-03, handler no entrega bytes con integridad fallida

- **Fecha:** 2026-11-12.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; demás cambios preservados.

## Alcance

`TestReviewerDownloadHandlerDoesNotReturnIntegrityMismatch` recorre el handler HTTP real y `ReviewerAccessService.Download` real con authenticator, repositorio y Store sintéticos. El expediente/grant identity son válidos y el store entrega bytes de igual tamaño, pero con SHA-256 distinto al metadata esperado. La respuesta es 503; no incluye bytes alterados ni MIME de descarga binaria (`image/*`, `application/pdf` o `application/octet-stream`), `Content-Disposition` o `Content-Length`. Se comprueban `private, no-store`, `nosniff` y `Vary: Authorization`; Store identity se lee una vez.

## Verificación local

- El test focal pasó antes y después de ampliar la aserción MIME; prueba test-only de caracterización, sin cambio productivo.
- `go test ./internal/documentverification/...`: PASS.
- `go test -race ./internal/documentverification/...`: PASS.
- `go vet ./internal/documentverification/...`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido archivo nuevo: PASS.
- Variables DB/PG retiradas de comandos Go.

## Límites

Servicio/handler reales con authenticator, repo y Store fake; no demuestra autenticación integrada, corrupción real, logging global, storage durable, PostgreSQL, BFF o producción. El 503 es el mapeo observado para este error sintético. La revisión independiente `deleg_977dc7b1` aprobó sin hallazgos de seguridad/lógica y sugirió generalizar `Content-Type`; se amplió el test para rechazar cualquier `image/*`, `application/pdf` y `application/octet-stream`. Sin SQL/DDL, bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue.
