# VI-03 — respuesta HTTP neutra ante fallo de commit de detalle reviewer

Fecha: 2026-11-13 (America/Santiago). Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`; rama `codex/marketplace/fix/kyc-06-save-identity-first`; HEAD `274f37e483a078890e92382a159fa49fd23c2fe8`.

## Alcance

`TestReviewerIdentityDetailKeepsCommitFailureNeutralAtRegisteredHTTPRoute` recorre la ruta reviewer registrada, `ReviewerCasesHandler` y `ReviewerAccessService.LoadDetail` reales hasta `Repository.WithReviewerCaseForRead`; el `pgx.Tx` fake devuelve error centinela en `Commit` tras las lecturas scoped de caso, grant, frescura y metadata de archivos. Comprueba HTTP 503 con cuerpo neutro, ausencia de ETag/headers de archivo/identificadores/diagnóstico, preservación de `private, no-store`, `Pragma: no-cache`, `Vary: Authorization`, `X-Content-Type-Options: nosniff` y `Content-Type: text/plain; charset=utf-8`, una invocación del rollback fake y cero escrituras, lecturas de declaración o accesos al store.

## Verificación ejecutada

- Test compuesto nuevo y controles `TestReviewerAccessLoadDiscardsSnapshotWhenReadCommitFails` y `TestReviewerAccessLoadDetailDiscardsResultWhenReadCommitFails`, `-race -count=20` — PASS.
- `go vet ./internal/adapters/postgres/documentverification` y `git diff --check` — PASS; gofmt aplicado y verificado.
- Revisión source-only `deleg_f180f1f1`: PASS, sin concerns/errores; sugirió fijar también `Pragma` y `Content-Type`, aserciones incorporadas y revalidadas localmente. La revisión no reejecutó pruebas ni editó archivos.
- Caracterización test-only del manejo existente; primer run PASS, sin afirmar RED/GREEN ni cambio productivo.

## Límites

Autenticador, transacción, consulta, grants, error de commit, declaración y store son sintéticos; el rollback base del fake es no-op. La prueba verifica la respuesta de la ruta local, no semántica PostgreSQL, autorización/grants/locks efectivos, commit incierto, rollback durable, persistencia, BFF real, navegador o producción. Sin SQL/DDL, DB/Cloud, datos reales, commit, push, deploy o integración a `main`.
