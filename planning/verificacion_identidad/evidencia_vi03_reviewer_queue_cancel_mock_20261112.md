# Evidencia VI-03 — cancelación de cola antes de devolver resultados

- **Fecha:** 2026-11-12
- **Rama:** `codex/marketplace/fix/kyc-06-save-identity-first`
- **HEAD:** `274f37e483a078890e92382a159fa49fd23c2fe8`
- **Cambio:** `ReviewerAccessService.List` rechaza el contexto precancelado antes de consultar el repositorio y descarta la cola si la cancelación ocurre durante la consulta. `ReviewerCasesHandler` vuelve a comprobar el contexto antes de convertir/serializar la cola.

## TDD y comportamiento

- `TestReviewerAccessListSkipsRepositoryWhenContextAlreadyCanceled`: RED observado; el servicio fake recibía una lectura y devolvía el caso aun con contexto precancelado. GREEN: `ErrDocumentStorage`, cero casos y cero llamadas al repositorio.
- `TestReviewerAccessListReturnsNoCasesWhenContextCancelsAfterRead`: RED observado; el repositorio fake cancelaba y aun así la cola contenía el snapshot. GREEN: error genérico y cero casos.
- `TestReviewerQueueHandlerDoesNotSerializeAfterCancellation`: RED observado; el handler respondía 200 con el identificador del caso después de la cancelación al retorno del servicio. GREEN: 503 genérico, sin lista/ID, conservando cabeceras de privacidad.

## Verificación local

Con entorno mínimo sin variables DB/PG:

- Los tres tests focales: PASS.
- `go test -race ./internal/documentverification/... -count=1`: PASS en dominio, inspección y transporte HTTP.
- `go vet ./internal/documentverification/...`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- `gofmt -d` y `git diff --check`: PASS.

## Límites

Los repositorios/servicios de cancelación son sintéticos. No acreditan grants/asignación efectiva, cancelación de una consulta PostgreSQL real, ni transporte HTTP/red. La cola excluida es metadato de revisión, no declaración o imagen, pero se mantiene privada y no se devuelve tras cancelar. No ejecuté SQL/DDL, migraciones, consultas o conexiones a bases persistentes o Cloud; no usé datos reales. Sin commit, push, integración a `main` ni deploy; worktree preexistente preservado.
