# Evidencia — VI-03, campos obligatorios de declaración

- **Fecha:** 2026-11-12.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y muy dirty; se preservaron los cambios preexistentes y ajenos.

## Alcance

`TestReviewerIdentityDetailRejectsIncompleteDeclarationFields` caracteriza el guard de `ReviewerAccessService.LoadDetail` para los cuatro campos necesarios de la declaración: `FirstName`, `LastName`, `RUT` y `DateOfBirth`. Un control primero confirma que el snapshot/grant sintéticos válidos y la declaración completa permiten leer el detalle. Luego cada subcaso omite exactamente un campo; exige `ErrDocumentStorage`, `ReviewerCaseDetail{}` exacto, scope subject/revision correcto, una llamada al reader, una lectura del repositorio (sin revalidación tras el fallo) y cero lecturas a stores. Pasó en la primera ejecución; es test de caracterización, sin cambio productivo.

## Verificación local

Con variables `DATABASE_URL`, `DATABASE_DSN`, `DB_URL`, `ESPACIGO_DATABASE_URL`, `PGHOST`, `PGPORT`, `PGUSER`, `PGPASSWORD`, `PGDATABASE` y `PGPASSFILE` retiradas:

- Test focal: PASS.
- `go test ./internal/documentverification/...`: PASS.
- `go test -race ./internal/documentverification/...`: PASS.
- `go vet ./internal/documentverification/...`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y `git diff --check`: PASS.

## Revisión independiente

`deleg_f08cde4e` aprobó la prueba en el checkout y HEAD indicados, sin preocupaciones de seguridad ni errores lógicos. Confirmó que el control válido permite leer la declaración, cada caso omite exactamente uno de los cuatro campos y atraviesa `LoadDetail`, y los asserts exigen el error fail-closed, detalle vacío, scope y ausencia de lecturas a stores. El revisor reportó también el test focal PASS. Es evidencia de unidad con fixtures sintéticos, no de integración.

## Límites

Reader, repositorio y stores son fakes; el test no demuestra descifrado, autorización integrada, BFF, PostgreSQL, transacción, persistencia o reinicio. Comprueba campos vacíos, no valida semántica/normalización de RUT, nombres o fecha de nacimiento. Sin SQL/DDL, bases persistentes, Cloud, proveedores, datos reales, commit, push, integración a `main` o despliegue.
