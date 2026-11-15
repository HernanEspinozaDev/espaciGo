# Evidencia — VI-05, fallos de lectura durante claim

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; cambios ajenos preservados.

## Alcance

- `TestClaimReadyRetentionJobFailsClosedOnQueryErrors` ejerce `Repository.ClaimReadyRetentionJob` con errores sintéticos en seis etapas: query candidata, lock del espacio, lock del caso, lock del archivo, recheck de scan y UPDATE condicional `RETURNING`.
- En cada etapa el error se propaga, no se devuelve claim, no ocurre Commit y se registra el rollback diferido. Los selectores confirman la query de falla esperada y el fake aborta en esa etapa.
- Characterization mock-only; sin cambio productivo ni RED observado.

## Verificación local

- Test focal: PASS.
- Paquete completo `internal/adapters/postgres/documentverification`, normal y `-race`: PASS con variables `ESPACIGO_*`, `PG*` y `DATABASE_URL` retiradas.
- `go vet ./internal/adapters/postgres/documentverification ./internal/adapters/documentfs ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido el archivo nuevo: PASS.

## Límites

Los errores y filas son sintéticos; no se ejecuta SQL ni se valida la semántica PostgreSQL, locks, rows afectados, persistencia o transacción real. Sin conexión a bases, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue. Revisión independiente `deleg_60ce6dc5`: aprobada sin hallazgos; confirmó que los seis fragmentos alcanzan sus etapas y cada error produce nil claim, cero commits y rollback diferido.
