# VI-03 — descarte de declaración ante fallo del commit final scoped

Fecha: 2026-11-13 (America/Santiago). Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`; rama `codex/marketplace/fix/kyc-06-save-identity-first`; HEAD `274f37e483a078890e92382a159fa49fd23c2fe8`.

## Alcance

`TestReviewerIdentityDetailDiscardsDeclarationWhenFinalScopedCommitFails` recorre la ruta reviewer registrada, el handler, `ReviewerAccessService.LoadDetail` y `Repository.WithReviewerCaseForRead` reales con tres transacciones fake secuenciales. Las dos primeras lecturas scoped committean y el reader devuelve declaración privada sintética entre ellas; falla el commit de la tercera transacción, que revalida el caso antes de devolver el detalle. La traza exige `commit,declaration,commit,commit`. Se comprueba 503 neutra sin datos privados/diagnóstico/ETag/headers de archivo, tres transacciones exactas, un read de declaración con subject/revisión exactos, cero lecturas del store/escrituras, commits exitosos solo en las dos primeras y una invocación del rollback fake por transacción.

## Verificación ejecutada

- `TestReviewerIdentityDetailDiscardsDeclarationWhenFinalScopedCommitFails`, `-race -count=20` — PASS.
- `go vet ./internal/adapters/postgres/documentverification` y `git diff --check` — PASS; gofmt aplicado y verificado.
- Revisión source-only independiente `deleg_9302e270`: PASS; verificó tres transacciones fake secuenciales, commits exitosos en las dos primeras, lectura de declaración intermedia, fallo en commit final, traza explícita `commit,declaration,commit,commit`, 503 neutra sin PII, conteos exactos y límites fake-only. No ejecutó pruebas ni modificó archivos.
- Caracterización test-only, primer run PASS, sin afirmar RED/GREEN ni cambio productivo.

## Límites

Transacciones, grants, consultas, declaración, error y store son sintéticos; los `Rollback` son no-op y solo se cuenta su invocación. No se prueba atomicidad, commit incierto, rollback durable, semántica PostgreSQL, locks/grants efectivos, persistencia, BFF, navegador ni producción. Sin SQL/DDL, acceso DB/Cloud, datos reales, commit, push, deploy o integración a `main`.
