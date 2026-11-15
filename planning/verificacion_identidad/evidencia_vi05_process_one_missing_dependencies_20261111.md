# Evidencia — VI-05, dependencias ausentes en ProcessOne

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; cambios ajenos preservados.

## Alcance

- Test-only: `TestRetentionProcessorProcessOneRejectsMissingDependenciesBeforeClaiming` cubre processor nil, puerto de jobs nil y puerto de documentos nil.
- En cada caso afirma `dv.ErrInvalid` antes de claim/proceso o borrado mediante contadores fake.
- Characterization del guard existente; no altera código productivo ni ejecuta almacenamiento/DB.

## Verificación local

- Test focal `go test ./internal/adapters/postgres/documentverification -run '^TestRetentionProcessorProcessOneRejectsMissingDependenciesBeforeClaiming$' -count=1`: PASS.
- Suite `go test ./internal/adapters/postgres/documentverification -count=1`: PASS.
- Suite `go test -race ./internal/adapters/postgres/documentverification -count=1`: PASS.
- `go vet ./internal/adapters/postgres/documentverification ./internal/adapters/documentfs ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido archivo nuevo: PASS.
- Variables `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `DATABASE_URL`, y `PG*` retiradas de comandos Go.

## Límites

Fakes y filesystem no intervienen en estas entradas inválidas; la cobertura acredita solo el guard Go antes de reclamar. No acredita worker productivo, persistencia o DB. Revisión independiente `deleg_70717310` aprobó sin hallazgos bloqueantes y confirmó que los tres casos cortan antes del claim/proceso. Su sugerencia opcional de reforzar contadores directos ya queda cubierta por las aserciones actuales de lease/claim, proceso y borrado. Sin SQL/DDL, bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue.
