# VI-03 — descarte del snapshot si falla el commit de lectura

Fecha: 2026-11-13 (America/Santiago). Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`; rama `codex/marketplace/fix/kyc-06-save-identity-first`; HEAD `274f37e483a078890e92382a159fa49fd23c2fe8`.

## Alcance

`TestReviewerAccessLoadDiscardsSnapshotWhenReadCommitFails` compone `Repository.LoadReviewerCase` y `ReviewerAccessService.Load` reales con caso identity/grant sintéticos y un `pgx.Tx` fake cuyo `Commit` falla tras completar lecturas de actor, scope/caso, grant, frescura de declaración y archivos. Comprueba que el error centinela se propaga con snapshot vacío, que no se marca commit exitoso ni se registran escrituras, que se invoca el rollback diferido fake y que el store no se lee en la ruta metadata-only.

## Verificación ejecutada

- `TestReviewerAccessLoadDiscardsSnapshotWhenReadCommitFails` y control `TestReviewerAccessLoadDetailDiscardsResultWhenReadCommitFails`, `-race -count=20` — PASS.
- Selección de cinco tests de lectura reviewer (`Load`/`LoadDetail`, owner/grant, auto-revisión y callback-error), `-race -count=20` — PASS.
- En esa selección focal: `LoadReviewerCase` 71,4%; `WithReviewerCaseForRead` 58,7% (la selección no cubre todos los escenarios de la función).
- `go vet ./internal/adapters/postgres/documentverification`, gofmt y `git diff --check` — PASS.
- Revisión source-only independiente `deleg_b57a470b`: PASS, sin concerns ni errores lógicos; sugirió aclarar que no reejecutó gates, lo que ya queda delimitado aquí.
- Caracterización test-only del camino de error existente; primer run del test nuevo pasó, sin afirmar RED/GREEN ni cambio productivo.

## Límites

El caso, grants, consultas y error de commit son sintéticos; el `Rollback` base del fake es no-op y solo se cuenta la invocación diferida. No demuestra commit incierto real, rollback durable, SQL, locks/grants efectivos, semántica PostgreSQL ni persistencia. Sin SQL/DDL, acceso a DB/Cloud, datos reales, commit, push, deploy o integración a `main`.
