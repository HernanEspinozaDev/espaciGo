# Evidencia — VI-03, error de capability al listar cola reviewer

- **Fecha:** 2026-11-13.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron los cambios existentes.

## Brecha y alcance

`TestReviewerQueueMapsCapabilityErrorToNeutralNotFound` recorre el `ReviewerCasesHandler` HTTP real con el fake de servicio compartido. Un control positivo devuelve el expediente identity sintético y comprueba la respuesta de cola; el escenario de error devuelve simultáneamente un snapshot parcial y `dv.ErrCapability`. El handler responde 404 neutral, sin ETag, `items` ni IDs del snapshot, conserva cabeceras privadas y hace una llamada `List` con el actor autenticado. Es una caracterización del mapeo HTTP; el fake no demuestra que el error venga del lock de actor del adapter ni de una asignación persistida. Test-only, sin cambio productivo ni afirmación RED/GREEN.

## Verificación local

- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=20 ./internal/documentverification/transport/http -run '^TestReviewerQueueMapsCapabilityErrorToNeutralNotFound$'`: PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go vet ./internal/documentverification/transport/http`: PASS.
- `gofmt -d` y chequeo explícito de whitespace/newline del test: PASS.

## Límites

El fake implementa el servicio HTTP y devuelve resultado parcial más error; no ejecuta adapter, SQL ni autorización real. El test caracteriza el error mapping y la ausencia de fuga en esa respuesta, no grants efectivos, consulta, PostgreSQL, persistencia, runtime BFF ni Cloud. No se ejecutó SQL/DDL ni se accedió a bases persistentes o Cloud. Sin cambios productivos, commit, push, deploy o integración a `main`.

## Revisión independiente

`deleg_3dd24741`: PASS source-only, sin concerns de seguridad ni errores lógicos. Confirmó el control positivo, el 404 neutral ante snapshot parcial más `ErrCapability`, ausencia de ETag/IDs/`items`, cabeceras y actor autenticado. No ejecutó pruebas. No encontró duplicación material con el caso cercano `ErrDocumentStorage`; esta revisión no verifica el origen del error ni amplía los límites fake-only.
