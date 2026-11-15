# Evidencia VI-03 — composición adapter→ReviewerAccessService con fake

- **Fecha:** 2026-11-12.
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Cambio:** test-only en `internal/adapters/postgres/documentverification/reviewer_list_access_service_composition_test.go`; sin cambios productivos.

## Comportamiento caracterizado

`TestReviewerAccessListUsesRepositoryExactGrantResult` ejecuta el `Repository.ListReviewerCases` real mediante `ReviewerAccessService.List` real. El `pgx.Tx` fake entrega el candidato y materializa grants sintéticos. El grant identity exacto devuelve el caso; scope, reviewer o capability distintos devuelven cola vacía y no leen archivos. Se comprueban el actor/caso enviados por el adapter, una llamada por etapa, commit del fake, número de lecturas de metadatos de archivos y cero lecturas del store de documentos durante la lista.

El test pasó en su primera ejecución; es una caracterización, no se afirma RED/GREEN ni hubo cambio productivo.

## Verificación ejecutada

Con `env -i`, sin heredar variables DB/PG:

- Test focal `go test -race -count=1 ./internal/adapters/postgres/documentverification -run '^TestReviewerAccessListUsesRepositoryExactGrantResult$' -v` — PASS, 4 escenarios.
- Suites `go test -race -count=1 ./internal/adapters/postgres/documentverification ./internal/documentverification ./internal/documentverification/transport/http` — PASS.
- `go vet` en los tres paquetes — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt -d` y verificación explícita de whitespace en los tests/evidencias/plan — limpios.

## Revisión

El usuario, como revisor autorizado, validó el 2026-11-12 el gate local de código y fixtures sintéticos y autorizó continuar. Revisión independiente read-only `deleg_d5e8ff17` devolvió `passed: true`, sin hallazgos; confirmó que la composición ejecuta el adapter y `ReviewerAccessService.List` reales. El límite de que candidatos/grants son fakes y no prueban SQL/persistencia permanece explícito.

## Límites

El test compone el adapter y el servicio, pero el fake simula la consulta de candidatos y el resultado de grants. No ejecuta SQL ni acredita predicados PostgreSQL, autorización persistida, expiración/revocación real, locks, aislamiento, atomicidad, almacenamiento o Cloud. No se ejecutó SQL/DDL ni se conectó a bases persistentes/Cloud; V95/V96 siguen sin aplicar. Sin datos reales, commit, push, deploy o integración a `main`.
