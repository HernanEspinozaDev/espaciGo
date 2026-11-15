# Evidencia VI-03 — revisión de ID de declaración malformado

- **Fecha:** 2026-11-12.
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Cambio:** test-only; nuevo `internal/documentverification/reviewer_access_invalid_declaration_revision_test.go`; sin cambios productivos.

## Hueco y comportamiento caracterizado

La cobertura inicial de `ReviewerAccessService.LoadDetail` marcó no cubierto el guard contra `IdentityDeclarationRevisionID` malformado. `TestReviewerIdentityDetailRejectsMalformedDeclarationRevisionBeforeRead` tiene un control sintético válido que accede a una declaración asociada y confirma la segunda lectura del caso. Con el mismo alcance y un revision ID malformado, el test exige `ErrDocumentStorage`, detalle cero, una sola lectura de caso, cero llamadas al declaration reader y cero lecturas de archivos identity/space.

La caracterización pasó al primer intento; no se afirma RED/GREEN y no hubo cambio productivo. Cobertura package-only posterior: `ReviewerAccessService.LoadDetail` 94.1%→97.1%; paquete `internal/documentverification` 82.4%→82.6%. No se busca 100% por sí mismo.

## Verificación ejecutada

Con `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR"`:

- Test focal con `go test -race -count=1` — PASS.
- Suites completas `-race` de `internal/documentverification` y `internal/documentverification/transport/http` — PASS.
- `go vet` de ambos paquetes — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt -d` y verificación explícita de whitespace — limpios.

## Revisión

Revisión independiente `deleg_233285e7` aprobó (`passed: true`) sin hallazgos. Confirmó control válido, rechazo del revision ID malformado antes del reader y cero lecturas de documentos; alcance limitado a fakes sintéticos.

## Límites

El repositorio, declaration reader y file store son fakes. La prueba acredita el guard del servicio en memoria, no la validación/persistencia de revision IDs en PostgreSQL, grants reales, transacción, storage, Cloud o autenticación. Sin SQL/DDL, acceso a persistencia/Cloud, datos reales, commit, push, deploy o integración a `main`.
