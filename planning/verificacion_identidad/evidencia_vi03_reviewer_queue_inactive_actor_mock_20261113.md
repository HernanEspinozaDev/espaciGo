# Evidencia — VI-03, actor inactivo en cola reviewer

- **Fecha:** 2026-11-13.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron los cambios existentes.

## Brecha y alcance

`Repository.ListReviewerCases` llama `lockActor` antes de consultar candidatos. El test `TestListReviewerCasesChecksActorActivityBeforeListing` caracteriza la rama donde el lock sintético indica que el actor está inactivo: retorna `dv.ErrCapability` con lista nil, no consulta candidatos ni confirma, y se observa la llamada de rollback del fake. El control positivo con actor activo alcanza `Query`, recibe una cola vacía y confirma el fake. El fake comprueba el actor exacto en ambas llamadas y solo un fragmento de la forma de la consulta candidata; no valida el SQL completo. Test-only; no cambia código de producción ni afirma RED/GREEN.

## Verificación local

- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=20 ./internal/adapters/postgres/documentverification -run '^TestListReviewerCasesChecksActorActivityBeforeListing$'`: PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go vet ./internal/adapters/postgres/documentverification`: PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go build -o /dev/null ./cmd/api`: PASS.
- `gofmt -d` y chequeo explícito de whitespace/newline del test: PASS.

## Límites

El test ejecuta `Repository.ListReviewerCases` con un `pgx.Tx` fake; el fake reconoce consultas pero no las ejecuta. La respuesta de actividad, la cola y la llamada de rollback/commit son observaciones sintéticas. Esto no demuestra la función SQL, permisos/grants efectivos, locks, rollback durable, semántica PostgreSQL, persistencia ni Cloud. No se ejecutó SQL/DDL ni se accedió a bases persistentes o Cloud. No hay cambios de producción, commit, push, deploy o integración a `main`.

## Revisión independiente

`deleg_9e2d03d4`: PASS source-only, sin concerns de seguridad ni errores lógicos. El revisor confirmó que la rama inactiva falla antes de consultar candidatos y que el control activo llega a la consulta con el actor exacto; no ejecutó pruebas. Su observación de que el matcher comprueba solo un fragmento de SQL quedó explicitada arriba y en el plan. La revisión no amplía la evidencia fake-only.
