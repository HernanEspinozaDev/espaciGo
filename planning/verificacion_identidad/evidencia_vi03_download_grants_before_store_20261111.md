# Evidencia — VI-03, descargas denegadas antes del Store

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y muy dirty; cambios ajenos preservados.

## Alcance

- `TestReviewerDownloadRejectsInvalidGrantsBeforeReadingStore` cubre seis fixtures: grant ausente, grant ligado a otro reviewer, scope incorrecto, capability incorrecta, combinación de capabilities y auto-revisión.
- Para cada caso, `ReviewerAccessService` real y repositorio/store fakes retornan denegación neutral `ErrNotFound`, sin bytes/MIME y con cero lecturas de ambos stores.
- Test-only; sin cambios productivos.

## Verificación local

- Test focal: PASS.
- `go test ./internal/documentverification/... -count=1`: PASS.
- `go test -race ./internal/documentverification/... -count=1`: PASS.
- `go vet ./internal/documentverification/... ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido archivo nuevo: PASS.
- Variables `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `DATABASE_URL` y `PG*` retiradas de comandos Go.

## Límites

Caracteriza autorización/ruteo del servicio contra repositorio y stores fake; no prueba grant real, endpoint/BFF integrado, autorización transaccional PostgreSQL, almacenamiento durable ni auditoría persistente. Revisión independiente `deleg_c5885298` aprobó sin hallazgos; confirmó los seis casos y cero lecturas del Store fake. Sin SQL/DDL, bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue.
