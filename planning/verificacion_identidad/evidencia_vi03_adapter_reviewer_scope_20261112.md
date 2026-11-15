# VI-03 — lectura del adapter con scope reviewer sintético

- Fecha: 2026-11-12.
- Checkout/rama/HEAD: `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- El worktree compartido estaba ampliamente dirty; se preservaron cambios ajenos.

## Cobertura

Se añadió `TestWithReviewerCaseForReadRevalidatesIdentityGrantScope`, que ejecuta el método real del adapter `Repository.WithReviewerCaseForRead` sobre `pgx.Tx` fake. Con grant identity exacto, el callback recibe el snapshot y el grant asignado y se cargan los archivos. Con scope distinto, el adapter devuelve `ErrNotFound` antes de cargar archivos, llamar al callback o confirmar. La tabla usa fixtures identity sintéticos; el helper fake ya usado por las pruebas del repositorio proporciona las filas de caso/grant.

Es caracterización de control/argumentos del adapter con transacción mockeada; no ejecuta SQL real ni acredita la semántica, aislamiento, lock, expiración o revocación de PostgreSQL. No hubo cambio productivo.

## Verificación

Se retiraron del proceso las variables PostgreSQL y los opt-ins locales de integración:

- `go test -race ./internal/adapters/postgres/documentverification -count=1` — PASS.
- `go vet ./internal/adapters/postgres/documentverification` — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt` y `git diff --check` — PASS.

El comando de pruebas no usó conexiones: `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, passfiles y banderas de integración estaban unset.

## Límites

No prueba consultas SQL, aislamiento/locks, grant activo, autorización real, transacción durable, V96, persistencia, BFF/UI ni despliegue. No se ejecutó SQL/DDL ni se accedió a bases persistentes o Cloud; sin migraciones, datos reales, commit, push o integración a `main`.
