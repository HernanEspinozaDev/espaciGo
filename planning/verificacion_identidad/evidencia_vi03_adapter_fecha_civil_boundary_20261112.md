# VI-03 — persistencia sintética del límite civil en el adapter

- **Fecha:** 2026-11-12.
- **Checkout/rama/HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- El worktree compartido estaba ampliamente dirty; se preservaron cambios preexistentes.

## Cobertura

Se amplió `TestTypedIdentityDecisionPersistsObservedAuditWithTransition` en `internal/adapters/postgres/documentverification/repository_test.go`. El fake `pgx.Tx` captura los argumentos de la transición identity del `Repository` real. La fixture usa el límite interno exclusivo correspondiente al día civil `2031-04-05` en `America/Santiago` (`2031-04-06T03:00:00Z`); el test exige que el mismo instante llegue al parámetro `document_expires_at` del UPDATE y a `document_expires_at_observed` del evento auditado, junto con el formato observado.

Es cobertura de caracterización con transacción fake; se amplió el test existente, sin cambio productivo ni afirmación de RED/GREEN. Complementa, en una capa distinta, el recorrido HTTP→servicio→repositorio en memoria documentado en `evidencia_vi03_aprobacion_http_fecha_civil_20261112.md`.

## Verificación local

Con variables de conexión/opt-in retiradas del test focal:

- Test focal sin race — PASS.
- Test focal con `-race` — PASS.
- `go vet ./internal/adapters/postgres/documentverification` — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt` y `git diff --check` del archivo — PASS.

## Límites

El fake captura llamadas y argumentos; no ejecuta SQL y no demuestra conversión del driver, semántica de columna, commit/auditoría atómica, locks, constraints, V96 ni persistencia PostgreSQL. No se ejecutó SQL/DDL, no se accedió a bases persistentes o Cloud, ni se usaron datos reales. Sin migraciones, commit, push, despliegue o integración a `main`.
