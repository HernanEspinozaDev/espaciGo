# Evidencia VI-03 — cancelar transición antes de mutar el caso

- **Fecha:** 2026-11-12
- **Rama:** `codex/marketplace/fix/kyc-06-save-identity-first`
- **HEAD base del corte:** `274f37e483a078890e92382a159fa49fd23c2fe8`
- **Cambio:** `Service.transact` retorna `ctx.Err()` si el contexto ya está cancelado antes de invocar el repositorio; vuelve a comprobarlo al entrar al callback de transición y justo antes de ejecutar la mutación de dominio.

## TDD y resultado

- `TestRevokeDoesNotReachRepositoryWhenContextAlreadyCanceled`: RED observado; el fake que ignora el contexto recibía la transición, revocaba y aumentaba revisión/auditoría. GREEN: retorna `context.Canceled`, cero llamadas al repositorio y estado/auditoría iguales.
- `TestRevokeStopsWhenContextCancelsBeforeTransitionCallback`: el repositorio sintético cancela antes de invocar el callback. RED observado; el servicio aceptaba y guardaba la revocación. GREEN: retorna `context.Canceled`; el fake transaccional conserva estado/revisión/auditoría.
- El error propagado conserva `context.Canceled` (no se convierte a error genérico de storage), preservando el contrato de compensación de `Upload`.

## Verificación local

Con entorno mínimo sin variables DB/PG:

- Tests focales: PASS tras el cambio.
- `go test -race ./internal/documentverification/... -count=1`: PASS.
- `go vet ./internal/documentverification/...`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- `gofmt -d` y `git diff --check`: PASS.

Una ejecución inicial de la suite tras introducir la guardia señaló que `Upload` espera preservar `context.Canceled` durante compensación; se corrigió el servicio para retornar el error concreto del contexto y se repitieron las gates, todas PASS.

## Límites

La transición/fake es en memoria. Esto prueba el guard de dominio y que el callback no muta con contexto cancelado, pero no demuestra locking, rollback, commit ni cancelación real PostgreSQL. Sin SQL/DDL, consultas, migraciones o conexiones a bases persistentes/Cloud; sin datos reales, commit, push, integración a `main` o deploy.
