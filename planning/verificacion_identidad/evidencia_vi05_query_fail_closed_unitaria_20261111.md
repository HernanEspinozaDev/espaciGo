# Evidencia — VI-05, errores de lectura fail-closed

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; cambios ajenos preservados.

## Alcance

- `TestProcessClaimedRetentionJobStopsOnQueryFailureBeforeDeletion` inyecta un error sintético en seis lecturas, individualmente: caso inicial, lock de espacio, relectura del caso bloqueado, archivo elegible, lease/job y existencia de scan activo.
- Ejerce el método real del repository; cada error se propaga, no se invoca borrado ni se ejecuta Exec/commit, y ocurre el rollback diferido.
- Test-only characterization del fail-closed ante fallos de lectura; sin cambio productivo.

## Verificación local

- Test focal: PASS.
- Paquete completo `internal/adapters/postgres/documentverification`, normal y `-race`: PASS, con todas las variables `ESPACIGO_*`, `PG*` y `DATABASE_URL` retiradas.
- `go vet ./internal/adapters/postgres/documentverification ./internal/adapters/documentfs ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido el archivo nuevo: PASS.
- La corrida inicial reveló que el fragmento fake de la query bloqueada omitía la proyección `state,revision`; se acotó el selector a la tabla/`FOR UPDATE`, y la prueba focal pasó.

## Límites

Los errores de `Scan` y filas son sintéticos; no se conecta a PostgreSQL ni se valida SQL/locking/aislamiento. No demuestra persistencia, retries durables, GCS, backups o recuperación. Sin SQL/DDL, Cloud, proveedores, datos reales, commit, push, integración a `main` o despliegue. Revisión independiente `deleg_304ed8e7`: aprobada sin hallazgos; confirmó que los seis selectores alcanzan sus etapas, los errores detienen el borrado/Exec/commit y ocurre rollback.
