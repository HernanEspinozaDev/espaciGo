# Evidencia — VI-03, metadata de archivos con digest malformado en cola reviewer

- **Fecha:** 2026-11-13.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron cambios ajenos al subcorte.

## Brecha y alcance

`TestListReviewerCasesRejectsMalformedFileDigestWithoutReturningPartialCandidate` recorre el `Repository.ListReviewerCases` real con `pgx.Tx`/rows fakes y un candidato identity sintético con grant exacto, lectura vigente de declaración y dos filas sintéticas de metadata. El control positivo comprueba que se devuelven ambas filas y sus hashes SHA-256 de 32 bytes exactos. En el negativo, la segunda fila contiene 31 bytes: `loadFiles` propaga el error de longitud, el adapter retorna `cases == nil`, no confirma y ejecuta el callback fake de rollback. Ambos conjuntos de filas se cierran. Caracterización test-only que pasó en su primera ejecución válida; no se afirma RED/GREEN ni hay cambio productivo.

## Verificación local

- Focal inicial con `-race -count=1 -v`: PASS en control positivo y rechazo del digest malformado.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=20 ./internal/adapters/postgres/documentverification -run '^(TestListReviewerCasesRejectsMalformedFileDigestWithoutReturningPartialCandidate|TestListReviewerCasesReturnsNoCandidatesWhenFileMetadataReadFails|TestListReviewerCasesRevalidatesExactGrantBeforeLoadingFiles)$'`: PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go vet ./internal/adapters/postgres/documentverification`: PASS.
- `gofmt -d` y chequeo explícito de whitespace/newline del test: PASS.
- Revisión source-only independiente `deleg_be586256`: PASS, sin concerns ni errores lógicos; confirmó el ruteo del fake, control positivo, rechazo del digest de 31 bytes, cierre de ambos row sets y límites fake-only. La revisión posterior `deleg_df743d25` volvió a revisar este caso junto al error de `Rows.Err` y también PASS. Ningún revisor ejecutó pruebas.

## Límites

La consulta, grant, candidato, filas, hashes y transacción son sintéticos. La prueba caracteriza propagación y cierre con el adapter y sus fakes; no demuestra SQL/driver, grants efectivos, PostgreSQL, semántica o rollback durable, persistencia, storage/GCS, runtime ni Cloud. No se ejecutó SQL/DDL ni se accedió a bases persistentes o Cloud. Sin commit, push, deploy o integración a `main`.
