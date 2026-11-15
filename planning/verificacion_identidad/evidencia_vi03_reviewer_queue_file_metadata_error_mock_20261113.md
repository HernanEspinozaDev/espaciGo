# Evidencia — VI-03, error al leer metadata de la cola reviewer

- **Fecha:** 2026-11-13.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron los cambios existentes.

## Brecha y alcance

`TestListReviewerCasesReturnsNoCandidatesWhenFileMetadataReadFails` recorre `Repository.ListReviewerCases` con un candidato identity sintético, grant exacto, lectura del caso y frescura válidos mediante fakes compartidos. El fake falla la consulta de metadata de archivos solo tras validar que recibió el ID de caso esperado. El adapter retorna lista nil y propaga el error; no alcanza commit, y el fake observa el callback de rollback y cierre de las filas de candidatos. El control positivo `TestListReviewerCasesRevalidatesExactGrantBeforeLoadingFiles` se ejecutó junto con la regresión. Caracterización test-only, sin cambio productivo ni afirmación RED/GREEN.

## Verificación local

- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=20 ./internal/adapters/postgres/documentverification -run '^(TestListReviewerCasesReturnsNoCandidatesWhenFileMetadataReadFails|TestListReviewerCasesRevalidatesExactGrantBeforeLoadingFiles)$'`: PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go vet ./internal/adapters/postgres/documentverification`: PASS.
- `gofmt -d` y chequeo explícito de whitespace/newline del test: PASS.

## Límites

El candidato, grant, declaración vigente, lectura metadata y error provienen de fakes; la capa adapter real solo decide cómo propagar el error. El test no demuestra query SQL, autorización/grants efectivos, PostgreSQL, semántica de transacción, rollback durable, persistencia ni Cloud. No se ejecutó SQL/DDL ni se accedió a bases persistentes o Cloud. Sin cambios de producción, commit, push, deploy o integración a `main`.

## Revisión y adjudicación

La revisión source-only `deleg_c42161cf` informó dos supuestos desajustes de contadores. El padre volvió a inspeccionar el candidato y repitió el test focal con `-race -count=1 -v`: PASS. La adjudicación source-only `deleg_c00f9c04` concluyó que el candidato no tiene defecto lógico: `reviewerListFileReadFailureTx` embebe el puntero al mismo `reviewerListAdapterTx`, no declara contadores sombra ni `Rollback`, y los campos/método promovidos operan sobre ese objeto. No se cambió código. Ninguno de los revisores ejecutó pruebas.
