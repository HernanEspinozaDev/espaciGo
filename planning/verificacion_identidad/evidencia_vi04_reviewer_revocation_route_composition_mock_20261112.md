# Evidencia VI-04 — revocación identity por ruta registrada

- **Fecha:** 2026-11-12.
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Cambio:** test-only; `TestReviewerRevokeTransitionsActualServiceAndReadsBackInMemoryState` ahora atraviesa `http.ServeMux` y `RegisterReviewerCasesRoutes`.

## Comportamiento caracterizado

La revocación identity pasa por la ruta registrada `/revoke`, `ReviewerCasesHandler` real y `ReviewerAccessService`/servicio de dominio reales. El fixture sintético usa revisión 7, grant identity exacto y autenticación fake. Se comprueba status 200/ETag 8, readback de `revocada`, hora de revocación y revisor, preservación de las marcas históricas de aprobación, evento de auditoría fake con motivo `solicitud_titular`, y una transición/lectura del repository fake.

Caracterización test-only al primer intento; no se afirma RED/GREEN ni cambio productivo.

## Verificación ejecutada

Con `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR"`:

- Test focal con `go test -race -count=1` — PASS.
- Suites completas `-race` de `internal/documentverification` y `internal/documentverification/transport/http` — PASS.
- `go vet` de ambos paquetes — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt -d` y whitespace del archivo candidato — limpios.

## Revisión

Revisión independiente read-only `deleg_b119b17d`: aprobada (`passed: true`), sin concerns ni errores lógicos. Confirmó dispatch por ruta registrada y conservación de aserciones de respuesta, ETag, estado/revisión, timestamps, auditoría fake y lectura del repository fake. La revisión limita expresamente la evidencia a auth/grant/repositorio sintéticos y estado en memoria; no acredita autorización ni persistencia reales.

## Límites

Autenticación, grant, repository/estado/auditoría son sintéticos/en memoria. No se prueba revocación durable, PostgreSQL, SQL, integración BFF→Go, Cloud ni autorización real. Sin SQL/DDL, acceso a persistencia/Cloud, datos reales, commit, push, deploy o integración a `main`; cambios previos ajenos del worktree se conservaron.
