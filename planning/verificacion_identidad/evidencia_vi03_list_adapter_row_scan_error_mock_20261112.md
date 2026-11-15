# Evidencia VI-03 — error al escanear candidato de cola reviewer

- **Fecha:** 2026-11-12.
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Cambio:** test-only en `internal/adapters/postgres/documentverification/reviewer_list_adapter_scope_unit_test.go`; sin cambios productivos.

## Hueco y comportamiento caracterizado

La cobertura de `Repository.ListReviewerCases` marcó el error de `scanSnapshot` al recorrer candidatos como no cubierto. `TestListReviewerCasesDiscardsCandidatesWhenRowScanFails` usa el método real con `pgx.Tx`/`pgx.Rows` fakes; el iterator presenta una fila candidata y `Scan` devuelve un error centinela. El test verifica nil/error original, cierre de filas una vez, observación del callback rollback fake una vez, cero commit y ninguna relectura de caso, grants o archivos.

La caracterización pasó al primer intento; no se afirma RED/GREEN y no se cambió producción. La medición package-only posterior informó `ListReviewerCases` en 73.9% (desde 69.6%) y el paquete en 73.0% (desde 72.8%). Permanecen bloques no cubiertos; no se persigue 100% por sí mismo.

## Verificación ejecutada

Con `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR"`, sin variables DB/PG ni flags opt-in:

- Test focal `go test -race -count=1` — PASS.
- Tests fake relacionados del adapter y suites completas `-race` de dominio y HTTP — PASS.
- `go vet` de adapter/dominio/HTTP — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt -d` y verificación explícita de whitespace del test — limpios.

## Revisión

Revisión independiente `deleg_8c69aa1d` devolvió `passed: true`, sin hallazgos. Confirmó error de `Scan`, resultado nil, cierre único, rollback fake y ausencia de lecturas posteriores; el alcance permanece explícitamente fake-backed, sin prueba de pgx/PostgreSQL.

## Límites

El iterator y error de `Scan` son sintéticos. La prueba no valida pgx conectado, SQL, rollback/transacción PostgreSQL, persistencia, grants efectivos, locks, atomicidad o Cloud. V95/V96 no se aplicaron. Sin datos reales, commit, push, deploy o integración a `main`.
