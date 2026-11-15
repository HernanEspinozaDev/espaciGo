# Evidencia — VI-03, revalidación del vínculo de declaración del revisor

- **Fecha:** 2026-11-12.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y ampliamente dirty; se preservaron los cambios ajenos.

## Alcance

`TestReviewerIdentityDetailSuppressesDeclarationWhenLinkedRevisionChangesDuringRead` caracteriza `ReviewerAccessService.LoadDetail`: el repositorio fake entrega primero un caso identity válido y asignado; el lector fake devuelve una declaración sintética para la revisión original, pero cambia el vínculo en el snapshot autoritativo antes de la segunda lectura. El servicio devuelve `ErrNotFound` y un `ReviewerCaseDetail` completamente zero-valued (incluido Snapshot), llama al reader una vez y repite la lectura del caso dos veces. El test pasó en su primera ejecución: es caracterización de la revalidación ya implementada, sin cambios productivos.

## Verificación local

Con variables DB/PG retiradas:

- Test focal `TestReviewerIdentityDetailSuppressesDeclarationWhenLinkedRevisionChangesDuringRead` — PASS; incluye aserción de salida completamente zero-valued solicitada por la revisión independiente `deleg_e7d9af96`. El reviewer reejecutó solo el focal; suite normal/`-race`, vet y build se repitieron localmente en esta sesión.
- `go test ./internal/documentverification/...` — PASS.
- `go test -race ./internal/documentverification/...` — PASS.
- `go vet ./internal/documentverification/...` — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt` y `git diff --check` — PASS.

## Límites

El repositorio y el lector son fakes: no prueba locking/transacción PostgreSQL, cambio concurrente durable, autorización integrada, descifrado real ni persistencia. No se ejecutó SQL/DDL, base persistente, Cloud, proveedor externo ni dato real. Sin commit, push, integración a `main` o despliegue; VI-03 continúa parcial.