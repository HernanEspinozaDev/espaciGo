# Evidencia — VI-03, error de Scan de metadata de archivos en cola reviewer

- **Fecha:** 2026-11-13.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron cambios ajenos al subcorte.

## Brecha y alcance

`TestListReviewerCasesReturnsNoCandidatesWhenFileMetadataScanFails` recorre `Repository.ListReviewerCases` real con un candidato identity, grant exacto, declaración vigente y una fila válida de metadata. El control positivo permite que el scan termine y llega al commit fake con digest de 32 bytes. En el negativo, el fake de `Rows.Scan` devuelve un error centinela; el adapter retorna lista nil/error, no confirma, observa el callback fake de rollback y cierra los row sets de metadata y candidatos. Caracterización test-only que pasó en su primera ejecución válida; no se afirma RED/GREEN ni hay cambio productivo.

## Verificación local

- Focal inicial con `-race -count=1 -v`: PASS en control positivo y fallo de Scan.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=20 ./internal/adapters/postgres/documentverification -run '^(TestListReviewerCasesRejectsMalformedFileDigestWithoutReturningPartialCandidate|TestListReviewerCasesDiscardsCandidateWhenFileMetadataRowsIterationFails|TestListReviewerCasesReturnsNoCandidatesWhenFileMetadataScanFails|TestListReviewerCasesReturnsNoCandidatesWhenFileMetadataReadFails|TestListReviewerCasesRevalidatesExactGrantBeforeLoadingFiles)$'`: PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go vet ./internal/adapters/postgres/documentverification`: PASS.
- `gofmt -d` y chequeo explícito de whitespace/newline del archivo Go: PASS.
- Revisión source-only independiente `deleg_ed8a32d0`: PASS, sin concerns ni errores lógicos. Confirmó que el `Scan` falla una vez, se propaga con lista nil, no hay commit y se cierran ambos row sets; no ejecutó pruebas. La sugerencia no bloqueante de explicitar lista nil/error antes de revisar rollback ya está satisfecha por las aserciones `cases == nil` y `errors.Is(err, wantErr)`.

## Límites

El error de Scan, candidato, grant, declaración, filas y transacción son sintéticos. El test caracteriza propagación/cierre con el adapter y fakes; no demuestra error de driver real, SQL, grant efectivo, PostgreSQL, rollback durable, persistencia, storage/GCS, runtime ni Cloud. No se ejecutó SQL/DDL ni se accedió a bases persistentes o Cloud. Sin commit, push, deploy o integración a `main`.
