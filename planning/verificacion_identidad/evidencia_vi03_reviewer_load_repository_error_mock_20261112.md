# Evidencia VI-03 — propagación de error en ReviewerAccessService.Load

- **Fecha:** 2026-11-12.
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Cambio:** test-only; extensión de `internal/documentverification/reviewer_access_load_validation_test.go`; sin cambios productivos.

## Comportamiento caracterizado

`TestReviewerAccessLoadPropagatesRepositoryErrorWithoutSnapshot` primero ejecuta el control válido para demostrar que actor/caso llegan al fake reader y que el snapshot es retornado. Luego el mismo reader devuelve un error sentinel sintético; el servicio propaga ese error, retorna snapshot cero, y el fake registra una sola llamada con el actor y caso exactos.

Caracterización test-only al primer intento; no se afirma RED/GREEN. Cobertura package-only de `ReviewerAccessService.Load`: 90.0%→100.0%; paquete `internal/documentverification`: 83.1%→83.2%.

## Verificación ejecutada

Con `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR"`:

- Tests focales de Load con `-race` — PASS.
- Suites completas `-race` de dominio y HTTP — PASS.
- `go vet` de ambos paquetes — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt -d` y verificación de whitespace del archivo candidato — limpios.

## Revisión

Revisión independiente `deleg_eed95529` aprobó (`passed: true`) sin hallazgos. Confirmó el control exitoso, propagación del sentinel original y snapshot cero al fallar; fake-only, sin prueba de DB/auth/grants reales.

## Límites

Repository fake; el error sentinel no representa una falla real de DB. Esto caracteriza el fail-closed del servicio, no SQL, PostgreSQL, grants/auth reales, storage ni Cloud. Sin SQL/DDL, acceso a persistencia/Cloud, datos reales, commit, push, deploy o integración a `main`.
