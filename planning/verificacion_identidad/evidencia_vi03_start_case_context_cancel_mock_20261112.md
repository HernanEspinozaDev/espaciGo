# Evidencia VI-03 — cancelación al crear caso de identidad

- **Fecha:** 2026-11-12
- **Rama:** `codex/marketplace/fix/kyc-06-save-identity-first`
- **HEAD base del corte:** `274f37e483a078890e92382a159fa49fd23c2fe8`
- **Cambio:** `Service.Start` preserva el error exacto del contexto: corta antes de llamar `CreateCase`, y revisa nuevamente al entrar al callback de construcción/autorización para evitar crear snapshot si el contexto se canceló en tránsito.

## TDD y resultado

- `TestStartDoesNotReachRepositoryWhenContextAlreadyCanceled`: RED observado; el fake ignoraba el contexto precancelado y creaba un caso. GREEN: devuelve `context.Canceled` antes de llamar al repositorio.
- `TestStartStopsWhenContextCancelsBeforeCreateCaseCallback`: RED observado; el fake cancelaba antes de llamar al callback y el servicio aún construía/guardaba el caso. GREEN: devuelve `context.Canceled` y no cambia estado/revisión.

## Verificación local

Con entorno mínimo sin variables DB/PG:

- Tests focales Start y transacción/revocación: PASS.
- `go test -race ./internal/documentverification/... -count=1`: PASS.
- `go vet ./internal/documentverification/...`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- `gofmt -d` y `git diff --check`: PASS.

## Límites

`CreateCase` usa fixture en memoria que deliberadamente ignora cancelación; el test acredita guardas del servicio, no atomicidad ni persistencia. No ejecuté SQL/DDL, consultas, migraciones ni conexiones a bases persistentes/Cloud; sin datos reales, commit, push, integración a `main` o deploy.
