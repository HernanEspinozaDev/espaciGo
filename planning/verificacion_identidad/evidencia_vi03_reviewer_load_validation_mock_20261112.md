# Evidencia VI-03 — validación del actor en ReviewerAccessService.Load

- **Fecha:** 2026-11-12.
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Cambio:** test-only; nuevo `internal/documentverification/reviewer_access_load_validation_test.go`; sin cambios productivos.

## Comportamiento caracterizado

`ReviewerAccessService.Load` es consumido por `ReviewerCasesHandler.load` y por la ruta de decisión. `TestReviewerAccessLoadValidatesActorBeforeRepositoryRead` usa un control válido que devuelve el snapshot y verifica que actor/caso exactos se reenvían al `LoadReviewerCase` fake. Luego cambia solo el actor por `malformed-actor-id` y verifica `ErrInvalid`, snapshot cero y cero invocaciones/argumentos al reader.

La caracterización pasó al primer intento; no se afirma RED/GREEN. Cobertura package-only de `Load`: 70.0%→90.0%; `internal/documentverification`: 82.7%→83.1%.

## Verificación ejecutada

Con `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR"`:

- Test focal con `go test -race -count=1` — PASS.
- Suites completas `-race` de dominio y HTTP — PASS.
- `go vet` de ambos paquetes — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt -d` y verificación explícita de whitespace — limpios.

## Revisión

Revisión independiente `deleg_b09939f2` aprobó (`passed: true`) sin hallazgos. Confirmó el control válido, forwarding exacto actor/caso y rechazo del actor inválido antes del reader; el fake no acredita grants/auth/DB reales.

## Límites

El repository y el store son fakes. Se verifica el límite de entrada del servicio y el forwarding de actor/caso, no grant/autenticación efectivos, SQL, PostgreSQL, storage ni Cloud. Sin SQL/DDL, persistencia/Cloud, datos reales, commit, push, deploy o integración a `main`.
