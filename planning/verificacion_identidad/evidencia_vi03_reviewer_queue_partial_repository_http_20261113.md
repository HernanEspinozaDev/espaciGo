# Evidencia VI-03 — error parcial de cola reviewer en ruta registrada

**Fecha:** 2026-11-13
**Rama:** `codex/marketplace/fix/kyc-06-save-identity-first`
**HEAD:** `274f37e483a078890e92382a159fa49fd23c2fe8`

## Alcance

`TestReviewerQueueDiscardsPartialRepositoryResultsThroughRegisteredRoute` conecta la ruta HTTP registrada y el `ReviewerAccessService` reales a un `ReviewerCaseReader` sintético que devuelve un snapshot válido junto con un error diagnóstico. El endpoint responde `503 temporarily unavailable`, sin serializar ID, subject, `items` ni diagnóstico; mantiene `private, no-store`, `nosniff`, `Vary: Authorization` y ningún ETag. Se verifica el actor autenticado exacto, una lectura de lista, cero lecturas adicionales de casos y cero acceso a storage.

## Verificación ejecutada

- Test compuesto, control de handler con servicio fake y control de lista positiva: `-race -count=20` — PASS.
- `TestReviewerAccessListDiscardsPartialResultsOnRepositoryError` (servicio real y reader fake): `-race -count=20` — PASS.
- `go vet ./internal/documentverification/transport/http`: PASS.
- `gofmt`: PASS.
- Caracterización test-only; primer run válido PASS. No se observó RED ni se modificó código productivo.

## Límites

El reader, auth y storage son sintéticos; el test caracteriza la ruta HTTP→servicio y el descarte del valor parcial en memoria. No prueba la consulta SQL real, grants efectivos, BFF ni persistencia. No se ejecutaron SQL/DDL, acceso a DB persistente/Cloud, commit, push, deploy o integración a `main`.
