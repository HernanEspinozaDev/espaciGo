# Evidencia VI-03 — descarte de declaración parcial con error

Fecha: 2026-11-13 (hora local `-03:00`)

## Alcance

Se añadió `TestReviewerIdentityDetailDiscardsDeclarationReturnedWithError` en `internal/documentverification/reviewer_access_detail_error_test.go`. El `ReviewerAccessService.LoadDetail` real usa repository y store en memoria; el lector sintético devuelve una declaración completa con datos ficticios y un error al mismo tiempo.

Se comprueba que el servicio mapee el fallo a `ErrDocumentStorage`, devuelva un `ReviewerCaseDetail{}` vacío, no ejecute lecturas de almacenamiento y haga una sola lectura de declaración con el account ID y revisión legal exactos. El fake repository registra dos comprobaciones de caso (preflight y la lectura ligada a la declaración); no ocurre el read final después del error.

El test pasó en su primera ejecución: caracterización de manejo fail-closed existente, sin RED ni cambio de producción. Complementa la prueba HTTP de detalle poblado + error: esta cubre la capa de acceso real; la HTTP prueba que un handler no serializa ningún valor parcial si recibe un error.

## Verificación ejecutada

- Preflight del paquete: sin `TestMain` ni `init`; el test usa solo `reviewerAccessRepo`, `reviewerAccessStore`, un reader fake y el servicio real.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test ./internal/documentverification -run '^TestReviewerIdentityDetailDiscardsDeclarationReturnedWithError$' -count=1 -v` — PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=20 ./internal/documentverification -run '^(TestReviewerIdentityDetailDiscardsDeclarationReturnedWithError|TestReviewerIdentityDetailRejectsIncompleteDeclarationFields)$'` — PASS (prueba nueva más control existente).
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go vet ./internal/documentverification` — PASS.
- `gofmt -d internal/documentverification/reviewer_access_detail_error_test.go` — sin diferencias.
- `git diff --check -- internal/documentverification/reviewer_access_detail_error_test.go` — PASS.

## Límites

No se ejecutó la suite completa. El valor+error procede de un reader fake; no prueba que un adapter real lo produzca, ni prueba grants efectivos, DB, SQL/DDL, atomicidad durable, persistencia/reinicio, storage real, GCS, Cloud o proveedores.

- Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`
- Rama: `codex/marketplace/fix/kyc-06-save-identity-first`
- HEAD observado: `274f37e483a078890e92382a159fa49fd23c2fe8`

Sin SQL/DDL, acceso a DB/Cloud, datos reales, commit, push, deploy ni integración a `main`; se preservó el estado previo del worktree.