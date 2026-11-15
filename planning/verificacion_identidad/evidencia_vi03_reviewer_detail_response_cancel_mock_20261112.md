# Evidencia VI-03 — cancelación antes de serializar detalle reviewer

- **Fecha:** 2026-11-12
- **Rama:** `codex/marketplace/fix/kyc-06-save-identity-first`
- **HEAD:** `274f37e483a078890e92382a159fa49fd23c2fe8`
- **Cambio:** `ReviewerCasesHandler.loadDetail` verifica el contexto después de `LoadDetail` y antes de validar/serializar el DTO. Un contexto cancelado produce el error genérico de almacenamiento (503), sin ETag ni declaración.

## TDD y comportamiento

`TestReviewerIdentityDetailHandlerDoesNotSerializeAfterCancellation` usa un servicio fake que cancela el contexto justo antes de devolver caso y declaración sintéticos, con nombre, apellido, RUT y fecha de nacimiento centinela.

RED observado antes del cambio: el handler respondía 200 y serializaba la declaración completa pese a la cancelación. GREEN después: 503 genérico, sin valores personales ni ETag; se conservan las cabeceras `private, no-store`, `nosniff` y `Vary: Authorization`.

## Verificación local

Con entorno mínimo sin variables DB/PG:

- Test focal `go test ./internal/documentverification/transport/http -run '^TestReviewerIdentityDetailHandlerDoesNotSerializeAfterCancellation$' -count=1`: PASS.
- `go test -race ./internal/documentverification/... -count=1`: PASS en dominio, inspección y transporte HTTP.
- `go vet ./internal/documentverification/...`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- `gofmt -d` y `git diff --check`: PASS.

## Límites

El test verifica la frontera del handler con un servicio fake; no prueba autorización/asignación integrada, persistencia ni transporte de red real. No ejecuté SQL/DDL, migraciones, consultas ni conexiones a bases persistentes o Cloud; no usé datos reales. Se preservó el worktree dirty existente. Sin commit, push, integración a `main` ni deploy.
