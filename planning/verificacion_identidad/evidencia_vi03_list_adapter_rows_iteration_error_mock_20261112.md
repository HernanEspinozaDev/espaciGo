# Evidencia VI-03 — error de iteración de filas en cola reviewer

- **Fecha:** 2026-11-12.
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Cambio:** test-only en `internal/adapters/postgres/documentverification/reviewer_list_adapter_scope_unit_test.go`; sin cambios productivos.

## Hueco y comportamiento caracterizado

La cobertura del adapter dejó sin cubrir el manejo de `rows.Err()` tras escanear candidatos. `TestListReviewerCasesDiscardsCandidatesWhenRowsIterationFails` ejecuta `Repository.ListReviewerCases` real con `pgx.Tx` fake y `pgx.Rows` sintético: el fake entrega una fila válida y luego reporta un error de iteración. El test comprueba retorno nil/error original, cierre de filas exactamente una vez, callback de rollback fake una vez, cero commit y ninguna relectura de caso, grants o archivos pese a existir una fila candidata parcial.

La caracterización pasó al primer intento; no se afirma RED/GREEN y no hubo cambio productivo. La medición package-only posterior informó `ListReviewerCases` en 69.6% (desde 67.4%) y el paquete en 72.8% (desde 72.7%). No se busca 100% por sí mismo; permanecen bloques no cubiertos.

## Verificación ejecutada

Con `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR"`, sin variables DB/PG ni flags opt-in:

- Test focal con `go test -race -count=1` — PASS.
- Tests fake relacionados del adapter y suites completas `-race` de dominio y HTTP — PASS.
- `go vet` de adapter/dominio/HTTP — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt -d` y verificación explícita de whitespace del test — limpios.

## Revisión

Revisión independiente `deleg_868e4a5d` devolvió `passed: true`, sin hallazgos. Confirmó fila válida seguida de error, retorno nil/error, `Close` una vez y ausencia de relecturas/grants/archivos. La aprobación se limita al fake y no acredita comportamiento pgx/PostgreSQL.

## Límites

La fila, error de iteración y cierre se simulan. La prueba no acredita comportamiento de pgx conectado, SQL, rollback/transacción PostgreSQL, persistencia, grants efectivos, locks, atomicidad o Cloud. V95/V96 no se aplicaron. Sin datos reales, commit, push, deploy o integración a `main`.
