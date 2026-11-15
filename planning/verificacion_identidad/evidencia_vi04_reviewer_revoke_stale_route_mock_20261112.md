# Evidencia VI-04 — conflicto CAS en revocación por ruta registrada

- **Fecha:** 2026-11-12.
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Cambio:** test-only; `TestReviewerRevokeStaleRevisionReturnsConflictWithoutReadback` ahora atraviesa `http.ServeMux` y `RegisterReviewerCasesRoutes`.

## Comportamiento caracterizado

El POST identity `/revoke` presenta `expected_revision=7` frente al snapshot fake autoritativo en revisión 8. La ruta registrada responde 409 con conflicto de revisión. El snapshot y auditoría fake permanecen idénticos al control previo; se observa una transición invocada y cero readbacks.

La caracterización pasó al primer intento; no se afirma RED/GREEN ni cambio productivo.

## Verificación ejecutada

Con `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR"`:

- Test focal con `go test -race -count=1` — PASS.
- Suites completas `-race` de `internal/documentverification` y `internal/documentverification/transport/http` — PASS.
- `go vet` de ambos paquetes — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt -d` y whitespace del archivo candidato — limpios.

## Revisión

Revisión independiente read-only `deleg_38f616bd`: aprobada (`passed: true`), sin concerns ni errores lógicos. Confirmó el 409/body, snapshot y auditoría fake sin cambio, una llamada de transición y cero readbacks, todo pasando por mux/ruta/handler real. El fake no demuestra CAS, autorización ni comportamiento de base reales.

## Límites

El CAS, snapshot y auditoría se simulan en repository fake; no demuestra concurrencia ni atomicidad PostgreSQL, ni auth/grants reales. Sin SQL/DDL, acceso a persistencia/Cloud, datos reales, commit, push, deploy o integración a `main`; cambios previos ajenos del worktree se conservaron.
