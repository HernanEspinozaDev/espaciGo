# Evidencia VI-03 — revalidación del grant en la lista del adapter (fake)

- **Fecha:** 2026-11-12.
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Cambio:** test-only en `internal/adapters/postgres/documentverification/reviewer_list_adapter_scope_unit_test.go`; no se modificó código productivo.

## Comportamiento caracterizado

`TestListReviewerCasesRevalidatesExactGrantBeforeLoadingFiles` ejecuta el método Go real `Repository.ListReviewerCases` usando un `pgx.Tx` fake. El fake entrega un candidato identity sintético al paso inicial, luego devuelve el grant re-leído configurado por cada escenario. Así se caracteriza que el actor y el caso se pasan con el scope esperado, que un grant identity exacto permite el resultado y la lectura de archivos, y que un grant con otro `ScopeID` descarta el candidato antes de leer archivos. El fake registra el commit y no implementa conexión a base.

La primera corrida del test encontró un defecto de enrutamiento del fake: el SQL de candidatos contiene una subconsulta a grants y coincidía antes con el matcher de grant. Se priorizó el matcher de tabla/candidatos en el fake. No se atribuye ese fallo a una regresión productiva ni se presenta como RED/GREEN del comportamiento.

## Verificación ejecutada

Con entorno reducido `env -i` (sin variables de conexión/opt-in):

- Test focal `go test -race -count=1 ./internal/adapters/postgres/documentverification -run '^TestListReviewerCasesRevalidatesExactGrantBeforeLoadingFiles$' -v` — PASS, 2 escenarios.
- Suites `go test -race -count=1 ./internal/adapters/postgres/documentverification ./internal/documentverification ./internal/documentverification/transport/http` — PASS.
- `go vet` en los tres paquetes — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt -d` y `git diff --check` del test — limpios.

## Revisión independiente

La revisión read-only `deleg_9ffeb9da` devolvió `passed: true`, sin hallazgos de seguridad ni lógica. Confirmó que el test ejecuta `ListReviewerCases` real y diferencia scope exacto de scope ajeno antes de leer archivos. Recomendó conservar explícito que el query de candidatos se simula y que no hay aserción de rollback; ambas limitaciones quedan registradas. El revisor no ejecutó pruebas ni modificó archivos; los gates de arriba fueron ejecutados por el parent sobre este estado.

## Límites

El test no ejecuta el SQL del adapter. El fake simula la primera consulta de candidatos y entrega grants ya materializados; no demuestra predicados SQL, expiración/revocación real del grant, aislamiento/locks/commit de PostgreSQL, autorización efectiva ni persistencia. Solo se afirma el commit del camino exitoso; los errores/rollback no están cubiertos por esta prueba. Tampoco prueba storage real, BFF, Cloud o datos reales. El alcance es la ruta Go de relectura/matching y el orden de lectura de archivos bajo un fake `pgx.Tx`. No se ejecutó SQL/DDL ni se conectó a bases persistentes o Cloud; V95/V96 siguen sin aplicar. Sin commit, push, despliegue o integración a `main`.
