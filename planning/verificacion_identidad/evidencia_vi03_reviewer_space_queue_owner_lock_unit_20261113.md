# VI-03 — cola reviewer: revalidación sintética del titular de espacio

Fecha: 2026-11-13 (America/Santiago). Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`; rama `codex/marketplace/fix/kyc-06-save-identity-first`; HEAD `274f37e483a078890e92382a159fa49fd23c2fe8`.

## Alcance

Se añadió `TestListReviewerCasesRevalidatesSpaceOwnerBeforeReturningCandidate` en `internal/adapters/postgres/documentverification/reviewer_list_space_owner_unit_test.go`. El test ejecuta `Repository.ListReviewerCases` real usando un `pgx.Tx` fake y un expediente `space` sintético con grant exacto. Cubre tres resultados de la revalidación del owner:

- El fake devuelve owner coincidente: la cola devuelve el caso de espacio sintético, y las lecturas posteriores de caso, grant y archivos ocurren una vez.
- El fake devuelve owner no coincidente: la cola queda vacía y no hay relecturas de caso/grant ni lectura de archivos.
- El fake devuelve un error centinela al bloquear/verificar owner: no se devuelve candidato, se propaga el error y se invoca el rollback del fake.

En los tres escenarios se verifican una llamada a la consulta inicial y los argumentos exactos `space_id`/`subject_id` enviados a la comprobación del owner. El helper fake ampliado en `reviewer_list_adapter_scope_unit_test.go` conserva el fixture identity previo por defecto.

## Verificación ejecutada

- `go test -race -count=20 -coverprofile=/home/nandev/.hermes/cache/scratch/reviewer_list_space_owner_20261113.cover ./internal/adapters/postgres/documentverification -run '^TestListReviewerCases'` — PASS; `Repository.ListReviewerCases` reportó 88,2% bajo esta selección focal.
- `go vet ./internal/adapters/postgres/documentverification` — PASS.
- `gofmt -d` en los dos archivos de prueba — sin diferencias.
- La prueba es una caracterización test-only, pasó en su primera ejecución; no hubo RED/GREEN ni cambio de producción.

## Límites

Todos los resultados de ownership, transacción y filas proceden de fakes sintéticos. Esto caracteriza la ramificación de `ListReviewerCases`, no ejecuta la función SQL, no prueba la propiedad/lock efectivo, aislamiento ni rollback durable, ni grants efectivos o persistencia. No se ejecutó SQL/DDL, no se accedió a DB/Cloud ni a datos reales. Sin commit, push, deploy o integración a `main`.
