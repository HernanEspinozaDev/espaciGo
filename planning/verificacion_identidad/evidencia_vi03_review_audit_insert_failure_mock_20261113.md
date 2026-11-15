# Evidencia VI-03 — fallo sintético al insertar auditoría de revisión

**Fecha:** 2026-11-13
**Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`
**Rama / HEAD base:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`

## Alcance

Se añadió `internal/adapters/postgres/documentverification/identity_review_audit_failure_unit_test.go`, una prueba de caracterización del límite transaccional en `Repository.Transition`. La prueba usa el adapter real y un `pgx.Tx` fake que permite el `UPDATE` de caso y devuelve un error sintético al intentar el `INSERT` de `document_verification_event`. Verifica que se invoca el callback de decisión, que las escrituras se intentan en orden, que el adapter propaga el error, no invoca `Commit` y llama una vez a `Rollback`.

El control positivo existente `TestTypedIdentityDecisionPersistsObservedAuditWithTransition` se ejecuta junto al caso de fallo para retener cobertura del camino exitoso. La prueba pasó desde su primera ejecución: no se afirma RED/GREEN ni se cambió código de producción.

## Verificación local

- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=20 ./internal/adapters/postgres/documentverification -run '^(TestIdentityReviewDoesNotCommitWhenAuditInsertFails|TestTypedIdentityDecisionPersistsObservedAuditWithTransition)$'` — PASS.
- `go vet ./internal/adapters/postgres/documentverification` — PASS.
- `gofmt` para el nuevo test y `git diff --check` — PASS.
- No existe `TestMain` ni `init` en el paquete que ejecute conexiones como parte de esta prueba; el selector ejecutó solo los dos tests indicados, con entorno mínimo y sin variables PG/DATABASE.

## Revisión independiente

`deleg_fad6ef3f` revisó el test, `Repository.Transition` y el control positivo: `passed=true`, sin hallazgos de seguridad ni errores lógicos. Sugirió opcionalmente comprobar explícitamente que el callback se ejecutó; se añadió esa aserción y se repitieron el test focal con race, `go vet`, `gofmt` y `git diff --check` — PASS. El revisor no ejecutó SQL/DDL ni conectó a una base.

## Límites y autorización

El fake comprueba el orden de llamadas, el error y que el adapter no intente confirmar la transacción. Su callback `Rollback` no ejecuta PostgreSQL ni revierte estado real; esto no demuestra atomicidad durable, semántica del motor, auditoría persistida ni recuperación tras reinicio. Es evidencia de unit test con fake, no de integración.

Por instrucción vigente del usuario, no se ejecutó SQL/DDL contra base persistente o Cloud, no se accedió a datos reales ni proveedores externos. No hubo cambios de producción, commit, push, despliegue ni integración a `main`. El worktree ya contenía cambios compartidos previos; esta evidencia no se los atribuye.
