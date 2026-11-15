# Evidencia VI-03 — descarga reviewer con error y bytes parciales en HTTP

**Fecha:** 2026-11-13
**Rama:** `codex/marketplace/fix/kyc-06-save-identity-first`
**HEAD:** `274f37e483a078890e92382a159fa49fd23c2fe8`

## Alcance

`TestReviewerIdentityDownloadPartialStorageErrorStaysNeutralAtHTTPBoundary` compone la ruta HTTP registrada y el `ReviewerAccessService` reales con un repositorio de autorización in-memory y un store sintético. El store identity devuelve bytes privados de fixture junto con un error; la prueba verifica que la respuesta sea `503 temporarily unavailable`, sin bytes, diagnóstico, `Content-Disposition` ni longitud de archivo. Comprueba además `private, no-store`, `Vary: Authorization`, un único read scoped al reviewer/caso solicitado y cero lecturas space.

## Verificación ejecutada

- Test focal con `-race -count=20`: PASS.
- `go vet ./internal/documentverification/transport/http`: PASS.
- `gofmt` y `git diff --check` del test: PASS.
- Caracterización test-only; primer run válido PASS. No se observó RED ni se modificó código productivo.

## Límites

El repositorio y storage son fakes sintéticos. Esto caracteriza el flujo registrado HTTP→servicio→puerto, no un adapter real que devuelva bytes junto con error, la autorización/grants persistidos, BFF, almacenamiento/GCS ni persistencia. No se ejecutaron SQL/DDL, acceso a DB persistente/Cloud, commit, push, deploy o integración a `main`.
