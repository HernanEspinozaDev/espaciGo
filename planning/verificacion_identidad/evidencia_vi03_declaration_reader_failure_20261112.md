# Evidencia — VI-03, fallo del lector de declaración

- **Fecha:** 2026-11-12.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y muy dirty; se preservaron los cambios preexistentes y ajenos.

## Alcance

`TestReviewerIdentityDetailFailsClosedWhenDeclarationReaderErrors` caracteriza `ReviewerAccessService.LoadDetail` con caso/grant sintéticos válidos. Primero exige que el control con lector exitoso devuelva la declaración fixture y complete ambas lecturas de autorización. Después el reader fake devuelve una declaración sintética parcial junto con error: se exige `ErrDocumentStorage`, respuesta `ReviewerCaseDetail{}` completa, un único intento de lectura, sin revalidación posterior ni lecturas de stores. Ambos paths verifican el scope exacto subject/revision. El test pasó en primera ejecución: es caracterización de comportamiento existente, sin cambio productivo.

## Verificación local

Con variables `DATABASE_URL`, `DATABASE_DSN`, `DB_URL`, `ESPACIGO_DATABASE_URL`, `PGHOST`, `PGPORT`, `PGUSER`, `PGPASSWORD`, `PGDATABASE` y `PGPASSFILE` retiradas:

- Test focal: PASS.
- `go test ./internal/documentverification/...`: PASS.
- `go test -race ./internal/documentverification/...`: PASS.
- `go vet ./internal/documentverification/...`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y `git diff --check`: PASS.

## Revisión independiente

`deleg_ce296afe` aprobó la prueba sin hallazgos de seguridad/lógica. Aclaró que el control demuestra lectura de declaración bajo un grant exacto, no una decisión de aprobación ni integración con base de datos.

## Límites

El reader y repositorio son fakes; el control y el fallo no demuestran descifrado, autorización integrada, BFF, acceso PostgreSQL, transacción, persistencia o reinicio. No ejecuté SQL/DDL ni pruebas conectadas a bases, ni accedí a Cloud, proveedores o datos reales. No hay commit, push, integración a `main` o despliegue.
