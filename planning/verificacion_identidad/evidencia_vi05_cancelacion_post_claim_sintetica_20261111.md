# Evidencia — VI-05, cancelación después del claim no inicia borrado (sintético)

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron los cambios previos.

## Alcance

- `internal/adapters/postgres/documentverification/retention_processor_test.go` añade `TestRetentionProcessorCancellationAfterClaimSkipsDeletion`.
- El fake de jobs cancela el contexto luego de recibir la solicitud de claim y antes de devolver el claim identity sintético. El `RetentionProcessor` devuelve `context.Canceled` en su chequeo post-claim.
- Se verifica `ctx.Err()==context.Canceled`, que se pasó lease de un minuto, no se invocó la operación fenced fake, no se confirmó ni reintentó el job y no se llamó ningún borrado identity/space.
- Caracterización de comportamiento existente; primera ejecución PASS, sin RED/GREEN ni cambio productivo. La cancelación ocurre dentro del fake; no demuestra release o expiración de lease real.

## Verificación

- Test focal `go test ./internal/adapters/postgres/documentverification -run '^TestRetentionProcessorCancellationAfterClaimSkipsDeletion$' -count=1` — PASS; repetido después de agregar aserción explícita de cancelación del fake.
- Suite unitaria filtrada `go test -race ./internal/adapters/postgres/documentverification -run '^TestRetentionProcessor' -count=1` — PASS.
- `go vet ./internal/adapters/postgres/documentverification`, gofmt focal y `git diff --check` — PASS.
- Los procesos Go corrieron sin variables de conexión DB/PG.

## Límites

La revisión independiente read-only `deleg_acdfe6c9` aprobó sin hallazgos de seguridad/lógica; sugirió confirmar explícitamente que el hook cancela el contexto. Se añadió la aserción `ctx.Err()==context.Canceled` y se reejecutaron focal, suite filtrada `-race`, vet, gofmt y diff-check — PASS. No demuestra claim durable, lease, release/recovery al cancelar, borrado físico, idempotencia, retry/commit, PostgreSQL, GCS, observabilidad ni backup. No se ejecutaron SQL/DDL, migraciones, consultas/conexiones a bases, Cloud, proveedores ni datos reales. Sin commit, push, integración a `main` o despliegue.
