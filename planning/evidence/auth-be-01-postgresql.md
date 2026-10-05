# AUTH-BE-01 — evidencia de dominio y repositorio

Verificación reproducible re-ejecutada el 2026-10-05 sobre el commit `ac9bf06d24f015806dfa12e6ddca194d40644e59`, en worktree temporal separado. Fixtures sintéticos; no se usaron datos personales ni credenciales persistentes.

## sqlc y verificaciones re-ejecutadas

- `go generate ./internal/adapters/postgres/identity` (sqlc v1.31.1) — PASS; el diff de los archivos generados quedó limpio, confirmando reproducibilidad.
- `go test -race ./...` — PASS, ejecutado nuevamente con `TEST_DATABASE_URL` apuntando al PostgreSQL desechable.
- `go test -count=1 -v ./internal/adapters/postgres/identity` — PASS; 7 pruebas de integración, ejecutadas nuevamente sin caché.
- `go vet ./...` — PASS, ejecutado nuevamente.
- `git diff --check` — PASS, ejecutado nuevamente.
- PostgreSQL 18 desechable: imagen `postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677`. Los tests crearon y retiraron sus bases aleatorias; al finalizar no quedaron bases `auth_be01_test_*` ni sesiones de cliente.
- Cobertura de integración observada: alta atómica de cuenta/rol/aceptaciones y rollback; conflicto/errores; estados de cuenta/sesión/términos; expiración idle/absoluta; reemplazo concurrente de token, intentos, consumo y conteo.

## Corrección de limpieza

La nota de la ejecución anterior afirmaba que PostgreSQL se había retirado, pero la inspección del 2026-10-05 encontró el contenedor `auth-be01-pg-20261004T225310` aún en ejecución desde el 2026-10-04. Se comprobó que era el recurso desechable autorizado: imagen indicada arriba, red `none`, almacenamiento de datos en `tmpfs`, solo el socket temporal montado, y ningún cliente activo ni base de prueba remanente. Tras re-ejecutar las pruebas, el contenedor y su directorio de socket fueron retirados; se verificó que ambos ya no existen. La afirmación anterior queda corregida. DSN omitido deliberadamente; fixtures y credenciales de prueba fueron efímeros y locales.

## Alcance pendiente

AUTH-DB-02 está integrada en `main` por el PR #15 (`0911b9c85643c8893950b381aec89b6249511706`). DB02-09 y RQF-213/-217/-218 continúan abiertos; no se añadió DDL para resolverlos. AUTH-BE-01 sigue pendiente de revisión e integración del PR #16; AUTH-BE-02 y AUTH-BE-03 permanecen bloqueadas hasta que esa entrega cumpla aceptación y se fusione.
