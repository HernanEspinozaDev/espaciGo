# Evidencia VI-03 — validación de entradas de descarga reviewer

**Fecha:** 2026-11-13
**Rama:** `codex/marketplace/fix/kyc-06-save-identity-first`
**HEAD:** `274f37e483a078890e92382a159fa49fd23c2fe8`

## Alcance

`TestReviewerDownloadRejectsInvalidInputsBeforeRepositoryOrStorage` cubre actor, case ID y file ID malformados con un `ReviewerAccessService` real y repository/store fakes; cada caso retorna `ErrInvalid` con documento vacío antes de leer el repository o el storage. `TestNilReviewerAccessServiceDownloadReturnsErrInvalid` verifica también el receptor nil sin panic. Los controles de descarga identity y space válidos conservan el comportamiento positivo.

Una medición focal previa de los tests reviewer seleccionados informó 95.0% para `ReviewerAccessService.Download` y cero ejecuciones del bloque de validación de entradas en `reviewer_access.go:252-254`; el perfil posterior confirma ese guard cubierto. La cobertura reportada se limita a esas selecciones y no representa la cobertura total del paquete.

## Verificación ejecutada

- Tests nuevos y controles positivos: `go test -race -count=20` — PASS.
- Perfil focal del guard de líneas 252–254 — cubierto.
- `go vet ./internal/documentverification`: PASS.
- `gofmt`: PASS.
- Caracterización test-only; primer run válido PASS. No se observó RED ni se modificó código productivo.

## Límites

Repository y storage son fakes. No demuestra grants efectivos, adapter PostgreSQL ni persistencia. No se ejecutaron SQL/DDL, acceso a DB persistente/Cloud, commit, push, deploy o integración a `main`.
