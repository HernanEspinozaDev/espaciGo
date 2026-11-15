# VI-03 — descarte del detalle si falla el commit de lectura

Fecha: 2026-11-13 (America/Santiago). Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`; rama `codex/marketplace/fix/kyc-06-save-identity-first`; HEAD `274f37e483a078890e92382a159fa49fd23c2fe8`.

## Alcance

`TestReviewerAccessLoadDetailDiscardsResultWhenReadCommitFails` conecta `Repository.WithReviewerCaseForRead` real y `ReviewerAccessService.LoadDetail` real con caso identity/grant sintéticos y `pgx.Tx` fake cuyo `Commit` devuelve un error centinela. El callback llega después de la lectura scoped de caso, grant, frescura y metadata; el fallo de commit se propaga y `LoadDetail` devuelve detalle vacío. La prueba confirma cero llamadas al lector de declaración y al store, un intento de commit, ausencia de commit exitoso/escrituras y una invocación del rollback fake.

## Verificación ejecutada

- Test focal `-race -count=1` — PASS.
- Selección de ocho tests reviewer (owner/grant space, auto-revisión, callback-error, commit-error/readback, stale declaration y grant revocado) con `-race -count=20` — PASS.
- Cobertura en esa selección: `WithReviewerCaseForRead` 78,3%; `LoadReviewerCase` 71,4%.
- `go vet ./internal/adapters/postgres/documentverification`, gofmt y `git diff --check` — PASS.
- Revisión source-only independiente `deleg_7b31b71f`: PASS, sin hallazgos ni sugerencias; no ejecutó pruebas ni modificó archivos.
- Caracterización test-only del error de lectura existente; no se observó RED/GREEN ni se cambió producción. Las fixtures de identidad no incluyen datos personales reales.

## Límites

El commit error, grants, consultas, callback y rollback son sintéticos; el `Rollback` base del fake es un no-op y solo se cuenta su invocación diferida. Esto prueba que el servicio descarta su resultado cuando el adaptador devuelve el error, no atomicidad, resultado de commit incierto real, rollback durable, locks/grants efectivos, semántica PostgreSQL o persistencia. Sin SQL/DDL, acceso a DB/Cloud, datos reales, commit, push, deploy o integración a `main`.
