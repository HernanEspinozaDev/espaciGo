# Evidencia — VI-05, scope de caso cambia antes del borrado (unitario)

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; cambios ajenos preservados.

## Alcance

- `TestProcessClaimedRetentionJobRejectsChangedCaseScopeBeforeDeletion` ejerce `Repository.ProcessClaimedRetentionJob` real con scope inicial coincidente con el claim; el fake devuelve propósito/titular/espacio cambiados solo en la lectura `FOR UPDATE` en tres escenarios separados.
- Cada escenario exige `dv.ErrConflict` tras la relectura protegida, sin callback de borrado, sin Exec/commit y con rollback diferido una vez. Aserciones adicionales limitan lecturas a 2 para propósito/titular y 3 para cambio de espacio (incluye lock-space fake).
- El lock-space se simula exitoso solo en el escenario de cambio de espacio; las respuestas son fixtures, no consultas a PostgreSQL.
- Caracterización de revalidación de scope en control de flujo Go; sin cambios productivos.

## Verificación local

- Test focal y suite focal de seis tests `ProcessClaimedRetentionJob`: PASS.
- Suite focal con `-race`: PASS; paquete completo `internal/adapters/postgres/documentverification`, normal y `-race`: PASS, con todas las variables `ESPACIGO_*`, `PG*` y `DATABASE_URL` retiradas del entorno.
- `go vet ./internal/adapters/postgres/documentverification ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check: PASS.
- Variables DB/PG y opt-in del integration gate retiradas en comandos Go.

## Límites

La revisión inicial read-only `deleg_88e20e46` aprobó sin hallazgos; recomendó opcionalmente afirmar lecturas/orden. Se incorporó el re-read diferenciado (scope del claim en la lectura inicial, valores cambiados en la lectura `FOR UPDATE`) y asserts de cantidad de lecturas; el follow-up `deleg_5ec86b50` aprobó la versión actual sin hallazgos, señalando únicamente como opcional registrar además la secuencia explícita. Los valores de caso y lock-space provienen del fake. No se prueba constraint/estado PostgreSQL, lock efectivo, transferencia concurrente, lease real ni query filtering. Tampoco prueba almacenamiento remoto, persistencia, borrado durable, backup/recuperación ni privacidad global. Solo fixtures sintéticos locales; sin SQL/DDL, conexión a bases, Cloud, proveedores, datos reales, commit, push, integración a `main` o despliegue.
