# Evidencia — VI-03, error de commit al listar cola reviewer

- **Fecha:** 2026-11-13.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron los cambios existentes.

## Brecha y alcance

`TestListReviewerCasesReturnsNoCandidatesWhenCommitFails` inyecta un error sintético en el commit del adapter real `Repository.ListReviewerCases`, después de recorrer un candidato identity válido, releer el caso y grant exactos, validar frescura y cargar metadata de archivos con fakes. El adapter retorna lista nil y propaga el error; el fake observa una llamada a `Commit`, una llamada posterior al callback `Rollback`, y ningún `committed` exitoso. El control positivo existente `TestListReviewerCasesRevalidatesExactGrantBeforeLoadingFiles` se ejecutó junto con el test nuevo. Caracterización test-only, sin cambio productivo ni afirmación RED/GREEN.

## Verificación local

- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=20 ./internal/adapters/postgres/documentverification -run '^(TestListReviewerCasesReturnsNoCandidatesWhenCommitFails|TestListReviewerCasesRevalidatesExactGrantBeforeLoadingFiles)$'`: PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go vet ./internal/adapters/postgres/documentverification`: PASS.
- `gofmt -d` y chequeo explícito de whitespace/newline del test: PASS.

## Límites

El adapter se ejecuta contra `pgx.Tx`/rows fakes compartidos del test package. El candidato, grant, frescura, lectura de archivos, error de commit y callbacks de commit/rollback son sintéticos. No demuestra transacción o rollback durable, locks, atomicidad, semántica PostgreSQL, grants efectivos ni persistencia. No se ejecutó SQL/DDL ni se accedió a bases persistentes o Cloud. Sin cambios de producción, commit, push, deploy o integración a `main`.

## Revisión independiente

`deleg_00f86fbf`: PASS source-only, sin concerns de seguridad ni errores lógicos. Confirmó que el test alcanza la etapa de commit después de la ruta identity sintética, retorna error original/lista nil y observa rollback únicamente mediante el fake. No ejecutó pruebas. El test comparte helpers del mismo package en `reviewer_list_adapter_scope_unit_test.go`; la revisión consideró válida esa dependencia y mantuvo explícito el límite de semántica PostgreSQL.
