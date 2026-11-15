# Evidencia VI-03 — allowlist del formato declarado en detalle reviewer

- **Fecha:** 2026-11-12
- **Rama:** `codex/marketplace/fix/kyc-06-save-identity-first`
- **HEAD:** `274f37e483a078890e92382a159fa49fd23c2fe8`
- **Cambio:** el límite de serialización identity valida formato declarado contra `legacy`, `new` o `unsure`; `undetermined` es exclusivo de la observación del revisor y no se acepta como dato declarado.

## TDD y controles

Se agregó el escenario `undetermined is not a declared format` a `TestReviewerDetailRejectsOutOfScopeIdentityDeclarationBeforeSerialization`. RED observado antes del cambio: HTTP 200 con ETag y la declaración completa para `document_format_declared=undetermined`. GREEN: 404 neutral, sin PII ni ETag. `TestReviewerDetailAllowsLegacyAndUnsureDeclaredFormats` confirma que las opciones previas legítimas siguen entregando el detalle; los controles existentes cubren `new` y que un caso `space` permanezca minimizado.

## Verificación local

Con entorno mínimo sin variables DB/PG:

- Tests focales de formatos inválido/válidos y controles identity/space: PASS.
- `go test -race ./internal/documentverification/... -count=1`: PASS en dominio, inspección y transporte HTTP.
- `go vet ./internal/documentverification/...`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- `gofmt -d` y `git diff --check`: PASS.

## Límites

Son pruebas del handler con servicio fake; no demuestran semántica PostgreSQL, autorización/grants efectivos, ni fuente integrada del snapshot. No ejecuté SQL/DDL, migraciones, consultas o conexiones a bases persistentes o Cloud; no usé datos reales. Sin commit, push, integración a `main` ni deploy; worktree previo preservado.
