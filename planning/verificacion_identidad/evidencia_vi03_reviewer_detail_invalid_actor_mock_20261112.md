# Evidencia VI-03 — actor malformado antes de lectura reviewer

- **Fecha:** 2026-11-12.
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Cambio:** test-only; `TestReviewerIdentityDetailRejectsMalformedActorBeforeRepositoryRead` en `internal/documentverification/reviewer_access_invalid_declaration_revision_test.go`; sin cambios productivos.

## Comportamiento caracterizado

El test crea un control válido con identidad y asignación sintéticas que alcanza el declaration reader y retorna la declaración allowlisted. Cambia únicamente el actor de la llamada a `malformed-actor-id` y verifica `ErrInvalid`, detalle vacío y cero lecturas del repository, declaration reader o file store identity/space.

La caracterización pasó al primer intento; no se afirma RED/GREEN. Cobertura package-only posterior: `ReviewerAccessService.LoadDetail` 97.1%→100.0%; `internal/documentverification` 82.6%→82.7%.

## Verificación ejecutada

Con `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR"`:

- Tests focales de actor/revision malformados con `go test -race -count=1` — PASS.
- Suites completas `-race` de dominio y HTTP — PASS.
- `go vet` de ambos paquetes — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt -d` y verificación explícita de whitespace — limpios.

## Revisión

Revisión independiente `deleg_57c3c79a` aprobó (`passed: true`) sin hallazgos. Confirmó control válido y que el actor malformado es rechazado antes de cualquier lectura del repository/declaration/file store. Alcance de fakes explícito; no acredita auth o grants reales.

## Límites

Repositorio, declaration reader y file store son fakes. No se demuestra autenticación real, grants efectivos, PostgreSQL, SQL, storage o Cloud. Sin SQL/DDL, acceso a persistencia/Cloud, datos reales, commit, push, deploy o integración a `main`.
