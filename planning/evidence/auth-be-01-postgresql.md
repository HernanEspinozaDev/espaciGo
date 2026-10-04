# AUTH-BE-01 — evidencia de dominio y repositorio

Fecha de ejecución: 2026-10-05. Fixtures sintéticos; no se usaron datos personales ni credenciales persistentes.

## Generación reproducible

- Configuración: `sqlc.yaml`, PostgreSQL y `pgx/v5`.
- Comando: `go generate ./internal/adapters/postgres/identity` (sqlc v1.31.1).
- Resultado: generación completada; consultas generadas consumidas por `internal/adapters/postgres/identity/repository.go`.

## Verificación

- `go test -race ./...` — PASS.
- `go vet ./...` — PASS.
- `git diff --check` — PASS.
- `TEST_DATABASE_URL=... go test ./internal/adapters/postgres/identity -count=1 -v` — PASS, 7 pruebas; instancia desechable PostgreSQL 18 de `postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677`.
- Cobertura integración: alta atómica de cuenta/rol/aceptaciones y rollback; conflicto/errores; estados de cuenta/sesión/términos; expiración idle/absoluta; reemplazo concurrente de token, intentos, consumo y conteo.
- PostgreSQL temporal retirado tras ejecución. DSN omitido deliberadamente; contraseña de prueba sintética, efímera y local.

## Alcance pendiente

AUTH-DB-02 está `done` tras merge del PR #15 (`0911b9c85643c8893950b381aec89b6249511706`). DB02-09 y RQF-213/-217/-218 continúan abiertos; no se añadió DDL para resolverlos. AUTH-BE-02 y AUTH-BE-03 permanecen bloqueadas hasta revisión y merge de AUTH-BE-01.
