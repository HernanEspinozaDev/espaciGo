# Evidencia — VI-05, no tomar claim con scan activo al revalidar

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; cambios ajenos preservados.

## Alcance

- `TestClaimReadyRetentionJobDeclinesClaimWhenScanAppearsBeforeRecheck` ejerce `Repository.ClaimReadyRetentionJob` con transaction fake. El fake entrega un candidato sintético y, al llegar a la segunda comprobación de scan activo tras las lecturas/locks de caso y archivo, devuelve `true`.
- Afirma que no se devuelve claim, se hacen solo cuatro lecturas en el orden candidato→caso→archivo→scan, no se intenta el UPDATE de claim y la transacción de no-trabajo se confirma.
- Caracterización sintética de la revalidación de scan en el camino de claim; sin cambio productivo ni RED observado.

## Verificación local

- Test focal: PASS.
- Paquete completo `internal/adapters/postgres/documentverification`, normal y `-race`: PASS, con variables `ESPACIGO_*`, `PG*` y `DATABASE_URL` retiradas.
- `go vet ./internal/adapters/postgres/documentverification ./internal/adapters/documentfs ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido el archivo nuevo: PASS.

## Límites

Las lecturas/scan/tx son fake; no se ejecuta SQL ni se demuestra aislamiento/concurrencia real, locks PostgreSQL o persistencia. No se accedió a bases, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue. Revisión independiente `deleg_a8c1665c`: aprobada sin hallazgos; confirmó las cuatro lecturas en orden, ausencia del UPDATE de claim y commit no-productivo.
