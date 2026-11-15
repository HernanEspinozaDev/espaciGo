# Evidencia VI-03 — declaración reviewer solo en casos identity vigentes y ajenos

- **Fecha:** 2026-11-12
- **Rama:** `codex/marketplace/fix/kyc-06-save-identity-first`
- **HEAD:** `274f37e483a078890e92382a159fa49fd23c2fe8`
- **Cambio:** `ReviewerCasesHandler.loadDetail` vuelve a validar antes de serializar una declaración identity adjunta: propósito identity, titular con ID válido y distinto del revisor autenticado, revisión legal válida, formato declarado presente, caso en estado sometido/en revisión/requiere antecedentes, no revocado ni reemplazado, y cuatro campos allowlisted completos. Una inconsistencia devuelve 404 neutral sin ETag. Una declaración inesperada en un caso `space` se sigue omitiendo, conservando el DTO space existente.

## TDD y controles

`TestReviewerDetailRejectsOutOfScopeIdentityDeclarationBeforeSerialization` ejecuta el handler con snapshots y declaración sintéticos. RED observado: el código previo devolvía 200 con PII para caso aprobado, auto-revisión, caso revocado, reemplazado y revisión legal ausente. GREEN: esos cinco escenarios devuelven 404 sin PII/ETag; el caso space con dato inesperado sigue respondiendo 200 sin declaración. El control existente `TestAssignedIdentityDetailIncludesOnlyAllowlistedDeclaration` sigue confirmando el caso válido `identity`/`en_revision`.

La primera suite completa tras añadir el guard detectó que el caso space debe conservar su respuesta 200 minimizada; se ajustó el guard para validar adjuntos solo en propósito identity, y el control de space volvió a pasar.

## Verificación local

Con entorno mínimo sin variables DB/PG:

- Tests focales de rechazo, positivo identity y positivo space: PASS.
- `go test -race ./internal/documentverification/... -count=1`: PASS en dominio, inspección y transporte HTTP.
- `go vet ./internal/documentverification/...`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- `gofmt -d` y `git diff --check`: PASS.

## Límites

Los servicios/auth de las pruebas son fakes. El guard protege la frontera de serialización frente a una respuesta upstream inconsistente; no demuestra grant/asignación efectiva ni autorización integrada, persistencia, transacción o PostgreSQL. No ejecuté SQL/DDL, migraciones, consultas ni conexiones a bases persistentes o Cloud; no usé datos reales. Worktree preexistente preservado; sin commit, push, integración a `main` ni deploy.
