# Evidencia VI-03 — propósito no soportado en detalle reviewer

- **Fecha:** 2026-11-12
- **Rama:** `codex/marketplace/fix/kyc-06-save-identity-first`
- **HEAD:** `274f37e483a078890e92382a159fa49fd23c2fe8`
- **Cambio de este subcorte:** se agregó una prueba en `internal/documentverification/reviewer_access_test.go`; no se modificó código productivo.

## Comportamiento cubierto

`TestReviewerDetailRejectsUnsupportedPurposeBeforeDeclarationRead` invoca el `ReviewerAccessService.LoadDetail` real con un repositorio sintético que retorna un caso de propósito no soportado, pero con IDs/formato/declaración válidos y grant identity plausible. El servicio responde `ErrNotFound`, devuelve detalle vacío y no llama al lector privado de declaración. El fake registra una sola lectura scoped del caso.

La prueba nació de cobertura de `internal/documentverification`: `reviewerCanReadCase` tenía 87.5% y la rama que rechaza propósitos distintos de identity/space no estaba cubierta. El perfil focal registra ahora el bloque `reviewer_access.go:155.3,156.1` con conteo 1. Es una caracterización del guard existente; el test pasó en la primera ejecución, sin RED/GREEN ni cambio de producción.

## Verificación local

Con entorno mínimo sin variables DB/PG:

- Test focal `go test ./internal/documentverification -run '^TestReviewerDetailRejectsUnsupportedPurposeBeforeDeclarationRead$' -count=1`: PASS.
- Perfil focal: rama `reviewer_access.go:155` ejecutada (`1`).
- `go test -race ./internal/documentverification/... -count=1`: PASS en dominio, inspección y transporte HTTP.
- `go vet ./internal/documentverification/...`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- `gofmt -d` y `git diff --check`: PASS.

## Límites y worktree

La prueba ejecuta servicio y política de lectura reales, pero repositorio, autorización/grant y lector son sintéticos; no acredita asignación efectiva, transacción/aislamiento, autorización PostgreSQL, persistencia ni integración HTTP/BFF. No se ejecutó SQL/DDL, migración, consulta o conexión a bases persistentes/Cloud; no se usaron datos reales.

`reviewer_access.go`, `reviewer_access_test.go` y el plan ya estaban dirty antes de este subcorte. Se preservaron los cambios compartidos y solo se agregó esta prueba y su recibo/entrada de plan. Sin commit, push, integración a `main` ni deploy.
