# Evidencia VI-03 — error de consulta inicial de cola reviewer

- **Fecha:** 2026-11-12.
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Cambio:** test-only en `internal/adapters/postgres/documentverification/reviewer_list_adapter_scope_unit_test.go`; sin cambios productivos.

## Hueco y comportamiento caracterizado

La medición package-only previa informó `Repository.ListReviewerCases` en 65.2%. Se añadió `TestListReviewerCasesRollsBackWhenCandidateQueryFails`, usando el `Repository.ListReviewerCases` real y un `pgx.Tx` fake que devuelve un error centinela al consultar candidatos después del lock sintético del actor. El test comprueba que la llamada devuelve nil y preserva el error, invoca una vez `Rollback` del fake, no marca commit y no intenta reread de caso, grants ni archivos.

La caracterización pasó al primer intento; no se afirma RED/GREEN y no se modificó producción. La medición posterior `go test -coverprofile=... ./internal/adapters/postgres/documentverification` reportó `ListReviewerCases` en 67.4% y el paquete en 72.7% de statements (frente a 65.2% y 72.6%). La cobertura es package-only, no end-to-end.

## Verificación ejecutada

Con `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR"`, sin variables DB/PG ni flags opt-in de integración:

- Test focal con `go test -race -count=1` — PASS.
- Tests fake relacionados de adapter y suites completas `-race` de dominio y HTTP — PASS.
- `go vet` de adapter/dominio/HTTP — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt -d` y verificación explícita de whitespace del archivo candidato — limpios.

## Revisión

Revisión independiente `deleg_9c753f2b` devolvió `passed: true`, sin hallazgos de seguridad/lógica. Confirmó el orden del matcher de la query candidata, propagación del error por el adapter, retorno sin datos y observación del callback rollback del fake. La aprobación se limita al comportamiento sintético, no a SQL/transacción/DB.

## Límites

El error y el rollback son observaciones del fake. No se ejecuta ni valida SQL, consulta PostgreSQL, rollback real, persistencia, grants efectivos, locks, atomicidad, autorización real o Cloud. V95/V96 no se aplicaron. Sin datos reales, commit, push, deploy o integración a `main`.
