# AUTH-DB-02 — Evidencia PostgreSQL M01

Fecha de ejecución: 2026-10-04. Rama: `feat/auth-db-02-migrations`; base verificada: `bd8144e611105306ca4163180ea3bb04530322fb` (merge de PR #14). Todos los datos de prueba son sintéticos (`.invalid`); no se usaron datos personales reales ni se registraron credenciales/DSN.

## Instancia descartable

- Imagen fijada: `postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677`.
- PostgreSQL: `18.6 (Debian 18.6-1.pgdg13+2)`; PostGIS disponible `3.6.4`.
- Docker Engine: `29.8.2`; Go: `1.27.1 linux/amd64`.
- Contenedor de prueba `auth-db02-pg-9CVYGX`: `--network none`, sin publicación/bindings de puertos (`{}`), datos en tmpfs `/var/lib/postgresql` (512 MiB) y conexión solo por socket Unix dentro de `~/.hermes/cache/scratch/`.
- `listen_addresses` vacío. El estado previo a limpieza se inspeccionó desde Docker; no se usó la instancia PostgreSQL persistente del host.

## Migración y repetición

Migración descubierta: exactamente `db/migrations/V000001__m01_identity.sql`.

- La prueba de integración creó una base temporal vacía, ejecutó `migrator.Run` y obtuvo versión aplicada `[1]`; la repetición devolvió cero versiones aplicadas. El historial quedó en una sola fila y conservó nombre/checksum de la migración. La suite existente también verificó checksum exacto del runner.
- Prueba de CLI contra la base descartable vacía, con `DATABASE_URL` limitado al proceso y socket temporal:
  - Primera ejecución: `applied migrations: [1]`.
  - Segunda ejecución: `sin cambios`.
  - Lectura PostgreSQL posterior: una fila en `schema_migrations`, checksum de 64 caracteres y tres filas de fixtures sintéticos.
- El total de tablas públicas excluyendo `schema_migrations` fue siete: seis tablas M01 más `spatial_ref_sys`, propiedad de PostGIS. La migración no crea esa tabla.

## Esquema, persistencia e integridad verificados

`TestM01MigrationPersistsAuthorizedSchemaAndConstraints` verificó las seis tablas M01, columnas/tipos, fixtures y ausencia de tablas fuera de alcance (`perfil_usuario`, `notificacion`, `historial_clave`, `preferencia_uso`). Comprobó índices únicos/no únicos y predicados parciales esperados; encontró seis FK y ninguna acción de borrado distinta de `RESTRICT`/`NO ACTION`.

La prueba hizo round-trip de campos sintéticos de cuenta (correo original y clave canónica, estado, contador/bloqueo), tiempos de sesión, campos de token incluyendo estados separados de consumo e invalidación, y aceptación de términos/canal. Inserciones negativas produjeron los SQLSTATE esperados: `23514` para checks inválidos, `23505` para valores duplicados y `23503` para FK inexistente; la eliminación de una cuenta referenciada también fue rechazada. Dos inserciones concurrentes con la misma `correo_normalizado` produjeron exactamente una aceptación y un rechazo por unicidad. `TestM01FKIntegrityRejectsUnknownUser` pasó.

Los fixtures de términos están etiquetados expresamente como sintéticos; sus hashes corresponden a etiquetas de fixture, no a textos legales publicables. DB02-09 sigue pendiente: no se añadió DDL para preferencia de uso, historial de claves ni notificación.

## Comandos y resultados

- `env -u DATABASE_URL TEST_DATABASE_URL=[REDACTED: URI de socket Unix temporal] go test ./internal/migrator -run '^(TestM01MigrationPersistsAuthorizedSchemaAndConstraints|TestM01FKIntegrityRejectsUnknownUser)$' -count=1 -v` — PASS.
- `env -u DATABASE_URL TEST_DATABASE_URL=[REDACTED: URI de socket Unix temporal] go test -race -count=1 ./...` — PASS en todos los paquetes, sin omitir las pruebas de integración.
- `go vet ./...` — PASS.
- `git diff --check` — PASS.
- CLI `go run ./cmd/dbmigrate -dir db/migrations` en la base temporal: primera pasada `[1]`, segunda `sin cambios`.

## Limpieza comprobada

- Después de la suite, consulta de `pg_database` encontró cero bases `migrator_test_*`; cada prueba elimina su base temporal.
- Se eliminó el contenedor de PostgreSQL y se comprobó que no quedaba en `docker ps -a`; tampoco quedan contenedores activos de esa imagen por esta prueba.
- Se eliminaron el socket Unix y su archivo `.lock`; se comprobó que el directorio de scratch específico ya no existe (`scratch_exists=False`).
