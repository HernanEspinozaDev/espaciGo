# VI-03 — descarte de declaración ante fallo del segundo commit scoped

Fecha: 2026-11-13 (America/Santiago). Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`; rama `codex/marketplace/fix/kyc-06-save-identity-first`; HEAD `274f37e483a078890e92382a159fa49fd23c2fe8`.

## Alcance

`TestReviewerIdentityDetailDiscardsDeclarationWhenSecondScopedCommitFails` recorre la ruta reviewer registrada, el handler, `ReviewerAccessService.LoadDetail` y `Repository.WithReviewerCaseForRead` reales. Una secuencia `pgx.Tx` fake permite que la primera lectura scoped de metadata termine y falle el `Commit` de la segunda transacción, después de que el reader de declaración devuelve valores privados sintéticos; un registro de eventos exige el orden `commit,declaration,commit`. Se comprueba 503 neutra sin identidad/diagnóstico/ETag/cabeceras de archivo, dos transacciones (sin iniciar la validación final), lectura de declaración una vez con subject/revision exactos, cero lecturas del store, ausencia de writes, commit exitoso solo en la primera transacción y una invocación de rollback fake por transacción.

## Verificación ejecutada

- `TestReviewerIdentityDetailDiscardsDeclarationWhenSecondScopedCommitFails`, `-race -count=20` — PASS.
- `go vet ./internal/adapters/postgres/documentverification` y `git diff --check` — PASS; gofmt aplicado y verificado.
- Revisión source-only independiente `deleg_77c493a1`: PASS, sin hallazgos; observó que el orden era inferible pero no explícito, por lo que se añadió `commit,declaration,commit` y se reejecutaron los gates. La revisión original no ejecutó pruebas.
- Revisión de seguimiento source-only `deleg_54846f4f`: PASS; confirmó la secuencia explícita, los commits/contadores y los límites fake-only. No ejecutó pruebas ni modificó archivos.
- Caracterización test-only, primer run PASS, sin afirmar RED/GREEN ni cambio productivo.

## Límites

Las transacciones, grants, lecturas, declaración, error y store son sintéticos; los `Rollback` fake son no-op y solo se cuenta su invocación. No se demuestra atomicidad, commit incierto, rollback durable, semántica PostgreSQL, locks/grants efectivos, persistencia, BFF, navegador ni producción. Sin SQL/DDL, acceso DB/Cloud, datos reales, commit, push, deploy o integración a `main`.
