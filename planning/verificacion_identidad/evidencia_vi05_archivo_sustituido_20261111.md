# Evidencia — VI-05, retención de archivo de identidad sustituido

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y muy dirty; cambios preexistentes preservados.

## Alcance

- `TestRetentionProcessorDeletesSupersededIdentityFileAndPreservesCurrent` usa `RetentionProcessor` y `documentfs.Store` reales en `t.TempDir()`, con dos archivos identity cifrados de IDs distintos y contenido sintético bajo mismo titular/caso.
- Un claim fake apunta al archivo sustituido; se verifica completion en memoria, path antiguo físicamente ausente mediante `os.Lstat`, path actual presente y lectura del contenido actual sin cambios.
- Characterization del procesamiento local con storage temporal y job fake. No prueba que un flujo de reemplazo real cree esos objetos/claims ni que persista su vínculo.

## Verificación local

- Test focal: PASS.
- `go test ./internal/adapters/postgres/documentverification -count=1`: PASS.
- `go test -race ./internal/adapters/postgres/documentverification -count=1`: PASS.
- `go vet ./internal/adapters/postgres/documentverification ./internal/adapters/documentfs ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check: PASS.
- Variables `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `DATABASE_URL`, y `PG*` retiradas de comandos Go.

## Límites

No prueba transacción de reemplazo, elegibilidad del job, PostgreSQL, generaciones GCS, borrado forense, backup o recuperación. Revisión independiente `deleg_53615c85` aprobó sin hallazgos; confirmó los límites del claim fake y Store temporal; reejecutó solo el test focal. Sin SQL/DDL, bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue.
