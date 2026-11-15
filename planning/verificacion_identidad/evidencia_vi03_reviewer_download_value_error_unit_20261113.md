# Evidencia VI-03 — descarga parcial de storage acompañada de error

Fecha: 2026-11-13 (hora local `-03:00`)

## Alcance

Se añadió `TestReviewerDownloadDiscardsContentReturnedWithStoreError` en `internal/documentverification/reviewer_download_partial_error_test.go`. Llama a `ReviewerAccessService.Download` real con un caso, archivo limpio, grant y digest sintéticos en memoria. El storage fake entrega bytes privados sintéticos junto con un error.

Se comprueba que el servicio responda `ErrDocumentStorage` y `DownloadedDocument{}` sin contenido ni MIME, y que la única lectura use el owner/case/file exactos por el puerto identity (cero lecturas space). El caso control existente `TestReviewerDownloadRejectsContentWithMismatchedSizeOrDigest` verifica la ruta de bytes con error nil y metadata no coincidente.

La nueva prueba pasó en su primer run: caracterización del comportamiento existente, sin RED ni cambio de producción. La autorización y el repository son sintéticos; no se demuestra una concesión efectiva.

## Verificación ejecutada

- Preflight de `internal/documentverification`: sin `TestMain` ni `init`; el test usa solo repository/store fakes y el servicio de acceso real.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test ./internal/documentverification -run '^TestReviewerDownloadDiscardsContentReturnedWithStoreError$' -count=1 -v` — PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=20 ./internal/documentverification -run '^(TestReviewerDownloadDiscardsContentReturnedWithStoreError|TestReviewerDownloadRejectsContentWithMismatchedSizeOrDigest)$'` — PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go vet ./internal/documentverification` — PASS.
- `gofmt -d internal/documentverification/reviewer_download_partial_error_test.go` — sin diferencias.
- `git diff --check -- internal/documentverification/reviewer_download_partial_error_test.go` — PASS.

## Límites

No se ejecutó la suite completa. El storage fake no prueba errores de adapters reales, entrega parcial de GCS/filesystem, cifrado, transacción, SQL/DDL, PostgreSQL, persistencia/reinicio, Cloud ni proveedores externos. La prueba HTTP existente con servicio fake y esta prueba de acceso real no son un recorrido BFF→Go ni un E2E productivo.

- Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`
- Rama: `codex/marketplace/fix/kyc-06-save-identity-first`
- HEAD observado: `274f37e483a078890e92382a159fa49fd23c2fe8`

Sin SQL/DDL, acceso a DB/Cloud, datos reales, commit, push, deploy ni integración a `main`; se preservó el estado previo del worktree.