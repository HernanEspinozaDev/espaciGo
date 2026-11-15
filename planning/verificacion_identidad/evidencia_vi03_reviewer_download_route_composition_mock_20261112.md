# Evidencia VI-03 — descarga identity a través de ruta registrada

- **Fecha:** 2026-11-12.
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Cambio:** test-only; el caso `TestReviewerDownloadHTTPUsesEncryptedStoreAndFailsClosedOnCancellation` ahora atraviesa `http.ServeMux` y `RegisterReviewerCasesRoutes`; no cambió código productivo.

## Comportamiento caracterizado

El control positivo solicita el contenido identity mediante la ruta registrada, usa `ReviewerAccessService` y el adapter AES-GCM reales con repository/BlobStore/auth fakes, y valida bytes recuperados, MIME, longitud, attachment y `Cache-Control: private, no-store`. El escenario de cancelación posterior a la lectura del BlobStore transita la misma ruta y verifica 503 sin bytes, `Content-Disposition`, `Content-Length` ni MIME de archivo; conserva cabeceras privadas. La caracterización preexistente pasó al primer intento al añadir montaje por mux; no se afirma RED/GREEN.

## Verificación ejecutada

Con `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR"`:

- Test focal con `go test -race -count=1` — PASS.
- Suites completas `-race` de `internal/documentverification` y `internal/documentverification/transport/http` — PASS.
- `go vet` de ambos paquetes — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt -d` y whitespace del archivo candidato — limpios.

## Revisión

Revisión independiente `deleg_77135ed8` aprobó (`passed: true`) sin hallazgos. Confirmó que el positivo y la cancelación tras lectura BlobStore pasan por mux/ruta registrada y que siguen las aserciones de cuerpo/cabeceras. El test separado de cancelación posterior a transacción continúa con invocación directa del handler; no fue parte de este delta revisado.

## Límites

El repository, auth/principal, grant y BlobStore son sintéticos; AES-GCM y el adapter son código real ejecutado en memoria. No demuestra grants/auth reales, GCS, SQL, PostgreSQL, persistencia o Cloud. Sin SQL/DDL, acceso a persistencia/Cloud, datos reales, commit, push, deploy o integración a `main`; cambios previos ajenos del worktree se conservaron.
