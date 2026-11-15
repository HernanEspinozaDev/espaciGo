# Evidencia VI-03 — cancelación durante la construcción de cambios

- **Fecha:** 2026-11-12
- **Rama:** `codex/marketplace/fix/kyc-06-save-identity-first`
- **HEAD base del corte:** `274f37e483a078890e92382a159fa49fd23c2fe8`
- **Cambios:** `Service.transact` ahora revisa el contexto también después de que el callback de dominio aplica el cambio y retorna éxito; un contexto cancelado convierte la salida del callback en `ctx.Err()`, para que el repositorio transaccional no confirme el snapshot. `Service.Start` hace la última revisión tras armar y validar el snapshot, antes de devolverlo al repositorio.

## TDD y comportamiento

- `TestRevokeDoesNotCommitWhenContextCancelsDuringDomainChange`: RED observado; el reloj controlado cancela el contexto dentro del callback de revocación y el fake confirmaba la mutación. GREEN: se propaga `context.Canceled`; el repositorio en memoria no confirma su copia ni cambia revisión/auditoría.
- `TestStartDoesNotPersistWhenContextCancelsWhileBuildingSnapshot`: RED observado; el reloj controlado cancela durante el armado del consentimiento y el fake creaba el caso. GREEN: se propaga `context.Canceled`; no cambia snapshot existente.

## Verificación local

Con entorno mínimo sin variables DB/PG:

- Tests focales de creación/transición y cancelación: PASS.
- `go test -race ./internal/documentverification/... -count=1`: PASS.
- `go vet ./internal/documentverification/...`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- `gofmt -d` y `git diff --check`: PASS.

## Límites

El repositorio fake modela no confirmar si el callback retorna error. Esto prueba el contrato del servicio y el fake, no demuestra rollback, commit ni locking de PostgreSQL. Sin SQL/DDL, consultas, migraciones ni conexiones a bases persistentes/Cloud; sin datos reales, commit, push, integración a `main` o deploy.
