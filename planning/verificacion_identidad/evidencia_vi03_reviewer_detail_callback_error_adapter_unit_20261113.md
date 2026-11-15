# VI-03 — error del callback en lectura de expediente reviewer

Fecha: 2026-11-13 (America/Santiago). Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`; rama `codex/marketplace/fix/kyc-06-save-identity-first`; HEAD `274f37e483a078890e92382a159fa49fd23c2fe8`.

## Alcance

`TestWithReviewerCaseForReadPropagatesCallbackErrorWithoutCommit` caracteriza `Repository.WithReviewerCaseForRead` con un caso identity, grant exacto y `pgx.Tx` fake. El recorrido obtiene el caso, valida declaración y carga metadata sintética antes de invocar una vez el callback, que devuelve un error centinela.

La prueba verifica que el error original se propaga, no se llama a `Commit`, se invoca el `Rollback` diferido del fake una vez, no hay escrituras y el conteo de lecturas scoped de caso/grant/frescura/archivos es 1/1/1/1.

## Verificación ejecutada

- Test focal `-race -count=1` — PASS.
- Selección reviewer con `-race -count=20` que incluye owner/grant space, auto-revisión, declaración stale y revocación de grant — PASS.
- Cobertura de `WithReviewerCaseForRead`: 78,3% en esa selección; `LoadReviewerCase`: 71,4%.
- `go vet ./internal/adapters/postgres/documentverification`, `gofmt -d` y `git diff --check` — PASS.
- Revisión source-only independiente final `deleg_6c004510`: PASS, sin hallazgos ni sugerencias; no ejecutó pruebas ni modificó archivos. La revisión anterior `deleg_a2adb5d8` se descartó por informar una edición no autorizada del plan; la inspección del checkout confirmó que permanece una sola entrada prevista y el candidato fue revisado de nuevo.
- Caracterización test-only de comportamiento existente; pasó en primera ejecución válida. No se afirma RED/GREEN ni hubo cambio de producción.

## Límites

El callback, grant, consultas y transacción son sintéticos. La llamada a rollback solo se observa en el fake; no demuestra locks, commit/rollback efectivos, aislamiento, comportamiento PostgreSQL ni durabilidad/persistencia. Sin SQL/DDL, acceso a DB/Cloud, datos reales, commit, push, deploy o integración a `main`.
