# VI-03 — reviewer: owner lock y grant space en lectura de detalle

Fecha: 2026-11-13 (America/Santiago). Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`; rama `codex/marketplace/fix/kyc-06-save-identity-first`; HEAD `274f37e483a078890e92382a159fa49fd23c2fe8`.

## Alcance

Se añadió `TestWithReviewerCaseForReadRevalidatesSpaceOwnerBeforeCallback` a `internal/adapters/postgres/documentverification/reviewer_case_adapter_read_test.go`. Ejecuta el método real `Repository.WithReviewerCaseForRead` con expediente space, owner, grant y transacción sintéticos. Casos:

- Owner verificado y grant exacto: el callback recibe el snapshot space, `SpaceOwnerID` ligado al titular y el grant exacto; se leen archivos y se completa el commit fake.
- Owner discrepante: devuelve `ErrNotFound` antes de leer snapshot, grants o archivos y no invoca el callback.
- Error del lock de owner: propaga el error centinela, sin snapshot, grants, archivos ni callback.
- Grant space con scope incorrecto: no invoca el callback ni lee archivos.

Cada escenario comprueba una llamada al lock con el `space_id` y `subject_id` exactos. El helper existente mantiene su comportamiento identity por defecto; también lo usa la prueba sintética de revalidación de grants.

## Verificación ejecutada

- `go test -race -count=20 ./internal/adapters/postgres/documentverification -run '^(TestWithReviewerCaseForReadRevalidatesIdentityGrantScope|TestWithReviewerCaseForReadRevalidatesSpaceOwnerBeforeCallback|TestStaleIdentityDeclarationIsHiddenFromReviewerReadPaths|TestIdentityReviewerGrantRevokedAfterPreflightStopsBeforeFileRead)$'` — PASS.
- Cobertura focal de `WithReviewerCaseForRead`: 76,1% bajo esa selección; los bloques del lock/owner space fueron ejercitados.
- `go vet ./internal/adapters/postgres/documentverification`, `gofmt -d` y `git diff --check` — PASS.
- Caracterización test-only, sin cambio de producción ni afirmación RED/GREEN. El primer intento falló por tipo incorrecto en el valor sintético de `space_id`; se corrigió el fake y las ejecuciones posteriores pasaron.

## Límites

La función de ownership, el grant y la transacción son fakes. No se ejecutó SQL ni se demuestra lock/ownership efectivo, permisos PostgreSQL, aislamiento, concurrencia, rollback durable o persistencia. Sin SQL/DDL, DB/Cloud, datos reales, commit, push, deploy o integración a `main`.
