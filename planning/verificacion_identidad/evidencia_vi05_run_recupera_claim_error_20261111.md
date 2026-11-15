# Evidencia — VI-05, Run se recupera de un error transitorio de claim

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y muy dirty; se preservaron cambios preexistentes.

## Alcance

- Test-only: `TestRetentionProcessorRunContinuesAfterClaimErrorUntilNextClaimSucceeds` usa un jobs fake secuencial y storage fake.
- El primer claim devuelve un error transitorio; el callback lo recibe y el worker continúa tras el intervalo de poll. El segundo claim devuelve un job identity; se verifica el claim exacto, lease, demora de retry derivada, borrado con scope de identidad esperado y cierre graceful al cancelar el contexto después del procesamiento.
- No hay cambios de producción; la prueba caracteriza el ciclo `Run` con dependencias sintéticas.

## Verificación local

- Test focal: PASS.
- Repetición focal `-count=50`: PASS.
- `go test ./internal/adapters/postgres/documentverification -count=1`: PASS.
- `go test -race ./internal/adapters/postgres/documentverification -count=1`: PASS.
- `go vet ./internal/adapters/postgres/documentverification ./internal/adapters/documentfs ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido archivo nuevo: PASS.
- Variables `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `DATABASE_URL`, y `PG*` retiradas de comandos Go.

## Límites

Fake-only: no acredita ticker en producción, claim/lease durable, SQL, transacciones, filesystem real ni worker/API integrada. Revisión independiente `deleg_9ceee0a4` aprobó sin hallazgos; confirmó que la secuencia es estable (repetición focal `-count=50`) y que el test/evidencia no sobredeclaran cobertura. Sin SQL/DDL, bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue.
