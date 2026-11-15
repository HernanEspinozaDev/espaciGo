# Evidencia VI-03 — cierre diferido por fallo de grant en lista adapter

- **Fecha:** 2026-11-12.
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Cambio:** test-only en `internal/adapters/postgres/documentverification/reviewer_list_adapter_scope_unit_test.go`; sin cambios productivos.

## Comportamiento caracterizado

Se añadió `TestListReviewerCasesRollsBackWhenGrantReadFails`. El fake `pgx.Tx` entrega un candidato identity, permite la relectura sintética del caso y falla la consulta fake de grants con un error centinela. El test comprueba que `ListReviewerCases` devuelve el error original y ningún resultado, que el defer llama una vez a `Rollback`, que no se llama `Commit` ni se leen archivos, y que las etapas anteriores ocurrieron una vez. El test de primer intento pasó; es una caracterización, no se afirma RED/GREEN ni se modificó producción.

## Verificación ejecutada

Con `env -i` para no heredar variables de conexión/opt-in:

- Tests focales `go test -race -count=1 ./internal/adapters/postgres/documentverification -run '^TestListReviewerCases(RevalidatesExactGrantBeforeLoadingFiles|RollsBackWhenGrantReadFails)$' -v` — PASS.
- Suites `go test -race -count=1 ./internal/adapters/postgres/documentverification ./internal/documentverification ./internal/documentverification/transport/http` — PASS.
- `go vet` en los tres paquetes — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt -d` y verificación explícita de whitespace en test/evidencia/plan — limpios.

## Revisión

Revisión independiente `deleg_02a649ba` devolvió `passed: true`, sin hallazgos de seguridad/lógica. Confirmó que el test alcanza el método real con `pgx.Tx` fake, propaga el centinela y observa una llamada a `Rollback` sin commit ni lectura de archivos. Su sugerencia fue conservar el límite de que el contador fake no prueba rollback/atomicidad PostgreSQL, ya explícito arriba. No modificó archivos ni ejecutó los gates del padre.

## Límites

El contador solo observa la invocación de `Rollback` en el fake; no prueba semántica PostgreSQL, descarta cambios ya confirmados ni verifica rollback/atomicidad reales. El fake también simula candidatos y grants; no se ejecutó SQL/DDL, persistencia ni Cloud. Sin datos reales, commit, push, deploy o integración a `main`; V95/V96 siguen sin aplicar.
