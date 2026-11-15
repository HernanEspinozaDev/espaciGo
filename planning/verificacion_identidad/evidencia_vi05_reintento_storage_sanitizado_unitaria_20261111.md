# Evidencia — VI-05, retry con error storage sanitizado (unitario)

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; cambios ajenos preservados.

## Alcance

- `TestProcessClaimedRetentionJobRequestsSanitizedRetryUpdateAfterStorageFailure` invoca el `Repository.ProcessClaimedRetentionJob` real con `pgx.Tx` sintético y lease exacta/vigente.
- El callback entrega un error sintético con marcador de ruta privada. El fake verifica una única actualización de retry, con IDs del job/file, demora suministrada, código allowlisted `retention_store_error` y lease exacta; no ve el texto del error crudo en los argumentos.
- Comprueba orden callback→retry update→commit observado por fake y ausencia de update de completion/insert de evento de borrado.
- Es caracterización del flujo del adapter usando filas/Exec fake; sin cambio de producción.

## Verificación local

- Focal de las tres pruebas `ProcessClaimedRetentionJob`: PASS.
- El mismo filtro con `-race`: PASS.
- `go vet ./internal/adapters/postgres/documentverification`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido archivo nuevo: PASS.
- Variables DB/PG y opt-in del integration gate retiradas de los comandos Go.

## Límites

La revisión independiente `deleg_95efeacf` aprobó sin hallazgos; confirmó que la evidencia no atribuye persistencia/atomicidad a PostgreSQL y reiteró que ausencia del error crudo en argumentos del fake no prueba logs ni otros sinks libres de datos sensibles. El fake solo observa los argumentos y la ruta de control: no acredita que PostgreSQL persista o haga commit la actualización, que el `WHERE`/lease sea aplicado por la base, ni retry durable, logs libres de PII, locks, almacenamiento remoto, backup/recuperación o alarmas. No representa una prueba de privacidad/logging. Solo fixtures sintéticos locales; sin SQL/DDL, conexión a bases, Cloud, proveedores, datos reales, commit, push, integración a `main` ni deploy.
