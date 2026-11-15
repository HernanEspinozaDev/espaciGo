# Evidencia — VI-03, handler de descarga no filtra bytes ante error

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y muy dirty; cambios ajenos preservados.

## Alcance

- `TestReviewerDownloadFailureDoesNotReturnDocumentBytes` ejerce handler HTTP real con autenticador y servicio fake para 404 `ErrNotFound`, 503 `ErrDocumentStorage` y error interno sintético.
- Aunque el fake incluye bytes privados junto con el error, todas las respuestas omiten cuerpo y `Content-Disposition`; el diagnóstico interno no aparece. Comprueba que se llama con el actor autenticado y que se conservan `private, no-store`, `nosniff` y `Vary: Authorization`.
- Test-only; no prueba transporte/servicio de producción.

## Verificación local

- Test focal: PASS.
- `go test ./internal/documentverification/... -count=1`: PASS.
- `go test -race ./internal/documentverification/... -count=1`: PASS.
- `go vet ./internal/documentverification/... ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido archivo nuevo: PASS.
- Variables DB/PG retiradas de comandos Go.

## Límites

La comprobación de no filtración usa sentinels sintéticos concretos; no prueba ausencia de cualquier posible dato/diagnóstico ni fija body, Content-Type o Content-Length exactos. Revisión independiente `deleg_c5698d6e` aprobó sin hallazgos de lógica o seguridad y confirmó que los errores se mapean antes de escribir headers/cuerpo de descarga. Handler real + autenticador/servicio fake; no demuestra autenticación integrada, autorización de PostgreSQL, BFF, storage durable o logs globales. Sin SQL/DDL, bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue.
