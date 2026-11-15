# Evidencia — VI-03, error de cola con resultados parciales

- **Fecha:** 2026-11-13.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron los cambios existentes.

## Brecha y alcance

La medición de la suite HTTP dejó sin cubrir el retorno de error de `ReviewerCasesHandler.ServeHTTP` cuando `List` falla. `TestReviewerQueueDoesNotSerializePartialResultsWhenListReturnsError` configura el fake de lista para devolver simultáneamente un snapshot sintético y `dv.ErrDocumentStorage`, situación distinta al caso cercano de cancelación tras una lista exitosa. Comprueba 503 genérico, sin ETag ni `items`/IDs en el cuerpo, cabeceras privadas y una única llamada con el actor autenticado. Test-only de caracterización, sin cambio productivo ni afirmación RED/GREEN.

## Verificación local

- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=1 ./internal/documentverification/transport/http -run '^TestReviewerQueueDoesNotSerializePartialResultsWhenListReturnsError$'`: PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=1 ./internal/documentverification/transport/http`: PASS.
- `go vet ./internal/documentverification/transport/http`, `gofmt` y `git diff --check`: PASS.
- Cobertura package-only de la suite HTTP: 87.1% total; `ReviewerCasesHandler.ServeHTTP`: 89.2%.

## Límites

La fuente entrega filas y error desde un fake en memoria. El test comprueba el comportamiento del handler ante ese contrato, no SQL/pgx, completitud de una consulta, grants efectivos, transacción, PostgreSQL, persistencia, ni Cloud. No se ejecutó SQL/DDL ni se accedió a bases persistentes o Cloud. Sin cambio productivo, commit, push, deploy o integración a `main`.

## Revisión

Revisión source-only independiente `deleg_2426b0ee`: PASS; consideró útil y no duplicada la regresión frente al caso de cancelación posterior a una lista exitosa. Confirmó el 503 neutral, la omisión de resultados parciales y el alcance del fake. Revisó el archivo local no rastreado junto al handler/tests del HEAD; no ejecutó pruebas ni editó archivos.
