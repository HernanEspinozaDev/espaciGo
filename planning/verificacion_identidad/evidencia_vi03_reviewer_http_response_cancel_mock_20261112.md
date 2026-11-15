# Evidencia VI-03 — cancelación antes de serializar descarga reviewer

- **Fecha:** 2026-11-12
- **Rama:** `codex/marketplace/fix/kyc-06-save-identity-first`
- **HEAD:** `274f37e483a078890e92382a159fa49fd23c2fe8`
- **Cambio:** `ReviewerCasesHandler` vuelve a revisar el contexto después de que `Download` retorna y antes de preparar o escribir encabezados/cuerpo de archivo. Si fue cancelado, responde con el error genérico de almacenamiento (503), sin serializar el documento.

## TDD y comportamiento

`TestReviewerDownloadHandlerDoesNotWriteDocumentWhenCanceledAfterServiceReturns` usa un servicio fake que cancela el request context justo antes de devolver un documento sintético, simulando cancelación en la frontera entre servicio y handler.

RED observado antes del cambio: `status=200` y el cuerpo incluía `synthetic private identity evidence`. GREEN después: `503`, respuesta de texto genérica, sin bytes, `Content-Disposition` ni `Content-Length`; se mantienen `Cache-Control: private, no-store`, `X-Content-Type-Options: nosniff` y `Vary: Authorization`.

## Verificación local

Con entorno mínimo sin variables DB/PG:

- Test focal `go test ./internal/documentverification/transport/http -run '^TestReviewerDownloadHandlerDoesNotWriteDocumentWhenCanceledAfterServiceReturns$' -count=1`: PASS.
- `go test -race ./internal/documentverification/... -count=1`: PASS en dominio, inspección y transporte HTTP.
- `go vet ./internal/documentverification/...`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- `gofmt -d` y `git diff --check`: PASS.

## Límites

El test prueba el contrato del handler con un servicio fake; no acredita autorización integrada ni la respuesta de un servidor/proxy de red real. No ejecuté SQL/DDL, migraciones, consultas o conexiones a bases persistentes o Cloud; no usé datos reales. Se preservó el worktree preexistente. Sin commit, push, integración a `main` ni deploy.
