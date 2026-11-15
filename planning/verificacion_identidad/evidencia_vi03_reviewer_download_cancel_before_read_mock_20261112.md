# Evidencia VI-03 — cancelación antes de descargar evidencia privada

- **Fecha:** 2026-11-12
- **Rama:** `codex/marketplace/fix/kyc-06-save-identity-first`
- **HEAD:** `274f37e483a078890e92382a159fa49fd23c2fe8`
- **Cambio productivo:** `ReviewerAccessService.Download` corta con `ErrDocumentStorage` antes de solicitar el expediente si el contexto ya está cancelado y vuelve a verificar el contexto dentro del callback de lectura, inmediatamente antes de invocar el object store.

## TDD y comportamiento

Se añadieron dos pruebas en `internal/documentverification/reviewer_access_cancel_before_declaration_test.go`, con `ReviewerAccessService` real, grant/caso/archivo sintéticos y store fake:

1. `TestReviewerDownloadSkipsRepositoryReadWhenContextAlreadyCanceled` entrega un contexto precancelado y exige cero lecturas del repositorio y del store.
2. `TestReviewerDownloadSkipsStoreReadWhenContextCancelsBeforeReadCallback` cancela en el fake justo antes de ejecutar el callback autorizado; exige error genérico, cero bytes y cero lecturas del store.

RED observado en ambas antes del cambio: cada una alcanzó una lectura del repositorio y una lectura del store (`repository reads=1 storage reads=1`), aunque el servicio terminaba devolviendo el error por cancelación. GREEN: las dos pruebas pasan con cero lecturas privadas en los puntos cancelados.

## Verificación local

Con entorno mínimo sin variables DB/PG:

- Tests focales `go test ./internal/documentverification -run '^TestReviewerDownloadSkips' -count=1`: PASS.
- `go test -race ./internal/documentverification/... -count=1`: PASS en dominio, inspección y transporte HTTP.
- `go vet ./internal/documentverification/...`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- `gofmt -d` y `git diff --check`: PASS.

## Límites y worktree

La prueba usa fakes que ignoran deliberadamente el contexto para verificar que el servicio evite iniciar la lectura. No acredita cancelación dentro de una transacción PostgreSQL, autorización/asignación efectiva, GCS ni persistencia. No se ejecutó SQL/DDL, migración, consulta o conexión a bases persistentes o Cloud; no se usaron datos reales.

El checkout ya contenía numerosos cambios dirty. Se preservaron; este subcorte solo añadió guards en `reviewer_access.go`, pruebas al archivo de cancelación ya creado y esta evidencia/entrada de plan. Sin commit, push, integración a `main` ni deploy.
