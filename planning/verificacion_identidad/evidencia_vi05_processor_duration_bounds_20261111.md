# Evidencia — VI-05, límites de duración del processor

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; cambios ajenos preservados.

## Alcance

- Test-only: `TestNewRetentionProcessorValidatesDurationBounds` caracteriza ocho valores de configuración: mínimo positivo y máximos aceptados (lease de 5 min, retry de 24 h), más lease/retry cero, negativos y cada valor por encima del máximo.
- Verifica cuándo el constructor devuelve processor no nil o nil. No ejecuta `ProcessOne`/`Run`, no cambia código productivo ni accede a storage o DB.

## Verificación local

- Test focal `go test ./internal/adapters/postgres/documentverification -run '^TestNewRetentionProcessorValidatesDurationBounds$' -count=1`: PASS.
- Suite `go test ./internal/adapters/postgres/documentverification -count=1`: PASS.
- Suite `go test -race ./internal/adapters/postgres/documentverification -count=1`: PASS.
- `go vet ./internal/adapters/postgres/documentverification ./internal/adapters/documentfs ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido el archivo nuevo: PASS.
- Variables `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `DATABASE_URL`, y `PG*` retiradas de comandos Go.

## Límites

Characterization sintética del constructor; no acredita política de configuración en producción ni worker operativo. Revisión independiente `deleg_4ecf62a2` aprobó sin hallazgos y confirmó límites inclusivos/casos fuera de rango frente a las comparaciones del constructor. Sin SQL/DDL, bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue.
