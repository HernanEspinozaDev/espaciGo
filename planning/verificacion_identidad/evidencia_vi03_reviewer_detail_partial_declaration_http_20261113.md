# Evidencia VI-03 — declaración reviewer parcial con error por ruta HTTP

**Fecha:** 2026-11-13
**Rama:** `codex/marketplace/fix/kyc-06-save-identity-first`
**HEAD:** `274f37e483a078890e92382a159fa49fd23c2fe8`

## Alcance

`TestReviewerIdentityDetailDiscardsPartialDeclarationThroughRegisteredRoute` compone la ruta reviewer registrada, el `ReviewerAccessService` y el handler HTTP reales con auth, repository y store sintéticos. El lector de declaración devuelve nombre, apellido, RUT y fecha de nacimiento fixture junto con un error diagnóstico. La ruta responde `503 temporarily unavailable`, sin serializar campos, datos parciales, diagnóstico ni ETag; conserva `private, no-store`/`Vary: Authorization`. Se verifican el actor/caso en cada lectura del repository, el owner/revision exactos usados por el reader y cero lecturas de evidencia.

## Verificación ejecutada

- Test compuesto, control HTTP parcial y control de detalle HTTP positivo: `-race -count=20` — PASS.
- `TestReviewerIdentityDetailDiscardsDeclarationReturnedWithError` (servicio real, reader fake): `-race -count=20` — PASS.
- `go vet ./internal/documentverification/transport/http`: PASS.
- `gofmt`: PASS.
- Caracterización test-only; primer run válido PASS. No se observó RED ni se modificó código productivo.

## Límites

El reader, repository y store son sintéticos. Esto verifica el flujo registrado HTTP→servicio y descarte de valor+error en memoria; no prueba readers/adapters reales, grants persistidos, BFF, PostgreSQL ni persistencia. No se ejecutaron SQL/DDL, acceso a DB persistente/Cloud, commit, push, deploy o integración a `main`.
