# Evidencia — VI-05, cola de retención sin candidato

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; cambios ajenos preservados.

## Alcance

- `TestClaimReadyRetentionJobReturnsNoWorkWhenCandidateQueryIsEmpty` ejercita `Repository.ClaimReadyRetentionJob` con `pgx.ErrNoRows` sintético en la consulta candidata.
- Afirma resultado `(nil, nil)`, solo una consulta candidata, ningún Commit y un rollback diferido; traza además `candidate_query → rollback`, sin locks ni UPDATE.
- Characterization mock-only del caso de cola sin trabajo; sin cambio productivo ni RED observado.

## Verificación local

- Test focal: PASS.
- Paquete completo `internal/adapters/postgres/documentverification`, normal y `-race`: PASS con variables `ESPACIGO_*`, `PG*` y `DATABASE_URL` retiradas.
- `go vet ./internal/adapters/postgres/documentverification ./internal/adapters/documentfs ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido el archivo nuevo: PASS.

## Límites

El resultado `ErrNoRows` y la transacción son fake; no demuestra consulta, locks, aislamiento ni persistencia PostgreSQL. Sin SQL/DDL, conexión a bases, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue. La revisión inicial `deleg_9428ec23` y el follow-up `deleg_419782d5` aprobaron sin hallazgos; el follow-up confirmó la traza `candidate_query → rollback` y los límites sintéticos.
