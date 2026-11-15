# Evidencia VI-03 — decisiones identity por rutas registradas

- **Fecha:** 2026-11-12.
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Cambio:** test-only; `TestReviewerIdentityNeedsEvidenceHTTPUsesActualServiceAndReadsBack` y `TestReviewerIdentityRejectHTTPUsesActualServiceAndReadsBack` ahora montan el handler por `http.ServeMux` + `RegisterReviewerCasesRoutes`.

## Comportamiento caracterizado

Los dos POST recorren la ruta registrada `/review`, `ReviewerCasesHandler` real, `ReviewerAccessService` real y servicio de dominio real, con autenticación, repositorio/estado/auditoría sintéticos. El escenario `needs_evidence` valida revisión CAS 7→8, readback del estado/formato y auditoría con motivo/checklist; el escenario `reject` valida readback de rechazo, revisión 8, motivo/formato/checklist y ausencia de campos de aprobación. Ambos comprueban que el actor sintético exacto llega a las lecturas/transición del repository fake.

Caracterización test-only al primer intento; no se afirma RED/GREEN. No cambió producción.

## Verificación ejecutada

Con `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR"`:

- Tests focales con `go test -race -count=1` — PASS.
- Suites completas `-race` de `internal/documentverification` y `internal/documentverification/transport/http` — PASS.
- `go vet` de ambos paquetes — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt -d` y whitespace de los candidatos — limpios.

## Revisión

Revisión independiente read-only `deleg_b8156226`: aprobada (`passed: true`), sin concerns ni errores lógicos. Confirmó el registro real del handler/rutas en mux y que permanecen las aserciones de estado/revisión/auditoría/readback. La revisión es de composición HTTP con fakes; no amplía los límites de evidencia descritos abajo.

## Límites

Autenticación, grant, estado del caso y auditoría son sintéticos/en memoria. El test no acredita autenticación/grants efectivos, atomicidad PostgreSQL, SQL, persistencia, BFF→Go, Cloud ni aprobación humana real. Sin SQL/DDL, acceso a persistencia/Cloud, datos reales, commit, push, deploy o integración a `main`; cambios previos ajenos del worktree se conservaron.
