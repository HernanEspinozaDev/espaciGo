# Evidencia — VI-03, fallo de iteración en metadata de archivos de cola reviewer

- **Fecha:** 2026-11-13.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron cambios ajenos al subcorte.

## Brecha y alcance

`TestListReviewerCasesDiscardsCandidateWhenFileMetadataRowsIterationFails` recorre `Repository.ListReviewerCases` real con candidato identity, grant exacto, declaración vigente y una fila de metadata sintética válida. Después de procesar esa fila, el fake devuelve un error centinela desde `Rows.Err`; el adapter retorna lista nil/error, no confirma, observa el callback fake de rollback y cierra los rows de candidato y metadata. Es una caracterización test-only del error posterior a la iteración, distinta del error de consulta y del digest malformado. No se afirma RED/GREEN ni hay cambio productivo.

## Verificación local

- Focal inicial `-race -count=1 -v` para esta prueba y la regresión de digest: PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=20 ./internal/adapters/postgres/documentverification -run '^(TestListReviewerCasesRejectsMalformedFileDigestWithoutReturningPartialCandidate|TestListReviewerCasesDiscardsCandidateWhenFileMetadataRowsIterationFails|TestListReviewerCasesReturnsNoCandidatesWhenFileMetadataReadFails|TestListReviewerCasesRevalidatesExactGrantBeforeLoadingFiles)$'`: PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go vet ./internal/adapters/postgres/documentverification`: PASS.
- `gofmt -d` y chequeo explícito de whitespace/newline del archivo Go: PASS.
- Revisión source-only independiente `deleg_df743d25`: PASS, sin concerns ni errores lógicos; revisó conjuntamente el caso de digest y el de `Rows.Err`, confirmó el ruteo/delegación y el alcance fake-only, sin ejecutar pruebas.

## Límites

El error `Rows.Err`, el candidato, el grant, la declaración, los archivos y la transacción son sintéticos. El test comprueba flujo del adapter con fakes; no demuestra errores reales de pgx/driver, SQL, autorización efectiva, PostgreSQL, rollback durable, persistencia, storage/GCS, runtime ni Cloud. No se ejecutó SQL/DDL ni se accedió a bases persistentes o Cloud. Sin commit, push, deploy o integración a `main`.
