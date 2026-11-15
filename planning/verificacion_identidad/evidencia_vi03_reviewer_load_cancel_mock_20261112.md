# Evidencia VI-03 — cancelación de lectura de caso reviewer

- **Fecha:** 2026-11-12
- **Rama:** `codex/marketplace/fix/kyc-06-save-identity-first`
- **HEAD base:** `274f37e483a078890e92382a159fa49fd23c2fe8`
- **Archivos del subcorte:** `internal/documentverification/reviewer_access.go`, `internal/documentverification/reviewer_access_load_cancel_test.go`.

## Cambio

`ReviewerAccessService.Load` ahora rechaza un contexto precancelado antes de llamar a `LoadReviewerCase` y descarta el snapshot si el contexto se cancela mientras el repositorio devuelve. En ambos casos entrega `ErrDocumentStorage` y snapshot vacío, conservando la respuesta genérica. El repositorio fake cubre ambos límites con un caso identity sintético.

## TDD

Los dos tests nuevos fallaron antes del cambio por el resultado esperado: el caso precancelado llegaba al repositorio y se devolvía el snapshot; si el fake cancelaba durante la lectura, el snapshot también se devolvía con error nil. Tras añadir las comprobaciones, ambos pasan.

## Verificación

Ejecutado con `env -i`, sin variables/configuración DB/PG:

- Test focal `TestReviewerAccessLoad(SkipsRepositoryWhenContextAlreadyCanceled|DiscardsSnapshotWhenContextCancelsDuringRead)`: PASS.
- `go test -race ./internal/documentverification/... -count=1`: PASS.
- `go vet ./internal/documentverification/...`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- `gofmt -d` y `git diff --check`: PASS.

## Límites

El test usa repositorio fake en memoria. No acredita cancelación efectiva del driver, autorización/asignación real, transacción, PostgreSQL, persistencia ni concurrencia. No se ejecutó SQL/DDL, no hubo acceso a bases persistentes/Cloud, ni commit, push o deploy.
