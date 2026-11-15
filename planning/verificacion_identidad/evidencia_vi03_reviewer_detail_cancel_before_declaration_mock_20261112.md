# Evidencia VI-03 — cancelación antes de leer la declaración del revisor

- **Fecha:** 2026-11-12
- **Rama:** `codex/marketplace/fix/kyc-06-save-identity-first`
- **HEAD:** `274f37e483a078890e92382a159fa49fd23c2fe8`
- **Cambio productivo:** `ReviewerAccessService.LoadDetail` devuelve error genérico de almacenamiento si el contexto ya está cancelado antes de iniciar la lectura del caso o si se cancela después de esa lectura y antes de consultar la declaración.

## TDD y comportamiento

Se añadieron dos pruebas en `internal/documentverification/reviewer_access_cancel_before_declaration_test.go`, ambas con `ReviewerAccessService` real y repositorio/lector sintéticos:

1. `TestReviewerIdentityDetailSkipsDeclarationReadWhenContextCancelsAfterCaseRead`: el fake de repositorio cancela el contexto al terminar la lectura inicial del caso. La versión previa falló porque el lector de declaración se invocó una vez pese a la cancelación. Después del guard entre la lectura del caso y el lector, pasa con cero lecturas privadas.
2. `TestReviewerIdentityDetailSkipsRepositoryReadWhenContextAlreadyCanceled`: entrega contexto ya cancelado. La versión previa falló porque inició una lectura del repositorio. Después del guard inicial, pasa con cero lecturas de repositorio y cero consultas de declaración.

Los dos RED ocurrieron por los efectos laterales que los tests pretendían impedir; no por errores de fixture. La respuesta es `ErrDocumentStorage` y detalle vacío. Ambos tests terminan sin consulta al lector de declaración; el caso precancelado tampoco inicia lectura del repositorio.

## Verificación local

Con entorno mínimo, sin variables DB/PG:

- Los dos tests focales: PASS.
- `go test -race ./internal/documentverification/... -count=1`: PASS en dominio, inspección y transporte HTTP.
- `go vet ./internal/documentverification/...`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- `gofmt -d` y `git diff --check`: PASS.

## Límites y worktree

El repositorio, grant y lector son sintéticos. La cobertura verifica el orden local de llamadas y el fail-closed ante cancelación; no demuestra cancelación dentro de PostgreSQL, límites de transacción, autorización efectiva, persistencia ni BFF→Go. No se ejecutó SQL/DDL, migración, consulta/conexión a bases persistentes o Cloud; no se usaron datos reales.

El checkout ya contenía numerosos cambios dirty. Se preservaron y se limitaron las ediciones productivas a `reviewer_access.go`, las pruebas nuevas a su archivo dedicado y esta documentación. Sin commit, push, integración a `main` ni deploy.
