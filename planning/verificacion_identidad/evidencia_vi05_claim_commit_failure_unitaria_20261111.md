# Evidencia — VI-05, no entregar claim si falla su commit

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; cambios ajenos preservados.

## Alcance

- `TestClaimReadyRetentionJobDoesNotReturnClaimWhenCommitFails` ejecuta `Repository.ClaimReadyRetentionJob` con tx fake: candidato sintético, locks de caso/archivo, recheck sin scan activo, UPDATE `RETURNING` sintético y fallo al confirmar.
- Afirma que el error de Commit se propaga y el caller recibe claim nil; comprueba cinco lecturas/UPDATE en orden, argumentos del UPDATE (job ID, duración y filtro `readyBefore` nulo), una llamada a Commit y rollback diferido.
- Characterization de que un claim tentativo no se entrega cuando falla el Commit fake; sin cambio productivo ni RED observado.

## Verificación local

- Test focal: PASS.
- Paquete completo `internal/adapters/postgres/documentverification`, normal y `-race`: PASS con variables `ESPACIGO_*`, `PG*` y `DATABASE_URL` retiradas.
- `go vet ./internal/adapters/postgres/documentverification ./internal/adapters/documentfs ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido el archivo nuevo: PASS.

## Límites

El UPDATE/RETURNING y Commit son sintéticos; no se prueba estado durable ni resultado ambiguo de commit PostgreSQL, SQL, aislamiento, concurrencia o lease real. Sin SQL/DDL, conexión a bases, Cloud, proveedores, datos reales, commit, push, integración a `main` o despliegue. Revisión independiente `deleg_27a98bb9`: aprobada sin hallazgos; confirmó el camino sintético, y explicitó que los selectores por substring no validan SQL exacto ni durabilidad.
