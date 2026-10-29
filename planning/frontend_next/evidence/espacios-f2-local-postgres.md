# Evidencia F2 — PostgreSQL nativo local

**Fecha:** 2026-10-28  
**Repositorio:** `/home/nandev/Documentos/projects/espaciGo`, rama `main`, HEAD base `3956d2f`; hay cambios locales sin commit que se preservaron.  
**Alcance:** exclusivamente PostgreSQL nativo local `espacigo_dms_stage_20261020`. No se tocó Cloud SQL/GCP, Terraform, IAM, despliegues ni volúmenes.

## Target y backup previo

- Instancia identificada: PostgreSQL `18.6 (Ubuntu 18.6-1.pgdg26.04+2)`, puerto 5432, socket Unix `/var/run/postgresql`, base exacta `espacigo_dms_stage_20261020`.
- La conexión de migración se ejecutó como OS/DB user `postgres`. No se leyeron `~/.pgpass` ni `~/.hermes/.env`, ni se imprimieron credenciales.
- Backup conservado fuera del repositorio: `/home/nandev/.hermes/cache/scratch/f2-espacigo-db-20261027-b81xPN`.
  - Formato custom; modo `0600`; tamaño `7,884,759` bytes.
  - SHA-256: `25ec1f73ae959a0042b0ea59887eaca783a3514bc94fff2f9a8f0a4387fada3e`.
  - `pg_restore --list` mostró 1,084 entradas y `pg_restore --exit-on-error --file=/dev/null` validó el archivo antes de aplicar DDL. El backup se mantiene intacto.

## Migración

- Se preparó/revisó el runner Go y V76; el plan previo era de solo lectura. La aplicación se hizo con `cmd/dbmigrate`, no con SQL manual.
- Salida del runner: `applied migrations: [68 69 70 71 72 73 74 75 76]`.
- Revalidación posterior con `dbmigrate -plan-only` como `postgres`: `database=espacigo_dms_stage_20261020`, `server_version=18.6`, `read_only=true`, `history_exists=true`, versiones V1–V76 presentes, checksums aceptados por el runner y `pending=[]`. Exit code 0.
- Inspección de solo lectura de V76: el trigger existente apunta a `img01b_enforce_gallery_slot()`, cuya función se reemplaza con `CREATE OR REPLACE FUNCTION` y permite `occupied < 30`; el CHECK persistente de posiciones se reemplaza por `1..30`. El guard de 10 en el backfill V68 es un bloque `DO` ejecutado al migrar, no una restricción permanente.

## Integración PostgreSQL sintética

- Test opt-in: `internal/adapters/postgres/media/repository_integration_test.go::TestSpacePhotoReplacementAtomic`.
- Ejecutado como OS user `postgres`, DB user `postgres`, por socket Unix; el guard del test limita el destino a la base exacta, puerto 5432 y host/socket permitido.
- Cinco ejecuciones consecutivas de la versión final: todas `PASS` (0.32–0.36 s cada una; exit code 0).
- Casos ejercitados contra la DB migrada: reemplazo preservando orden/metadata/caratula; CAS obsoleto sin escrituras parciales; rollback de transacción bajo bloqueo/cancelación; límite 29→30 con dos operaciones concurrentes (una gana y otra recibe límite), rechazo de la operación 31 y protección del trigger; claves válidas V1/V2 y rechazo de una clave inválida; estado/lease del asset retirado.
- Postcheck de solo lectura tras las cinco pasadas: `USERS|0`, `SPACES|0`, `ASSETS|0`, `INTENTS|0`, `SLOTS|0`, `ORDERS|0` para el patrón exclusivo de fixtures IMG01F. Una ejecución temprana anterior dejó fixtures sintéticos por un error de limpieza/FK; se inspeccionaron y eliminaron por sus identificadores/patrón exactos. Las pasadas finales verificaron cero restos y no se alteraron datos ajenos al fixture.

## Revisión independiente estática

- Revisión del test `TestSpacePhotoReplacementAtomic` y de sus dependencias V68, V70, V76, `media.Repository` y `spaceversion.Bump`: sin bloqueadores de seguridad o lógica; archivos sin cambios y sin acceso a DB.
- El revisor confirmó la guardia exacta de DSN antes de conectar/escribir, orden de limpieza compatible con FK, assertions de éxito y stale-CAS, rollback bajo lock, concurrencia 29→30 y rechazos 31 en app/DB.
- Observaciones no bloqueantes: la prueba de cancelación espera el lock hasta 5 s (posible flakiness en equipos muy cargados); la prueba V1 usa una clave representativa, no todas las formas aceptadas por el prefijo SQL. No se modificó el test por estas observaciones; las 5 pasadas finales reales contra PostgreSQL fueron verdes.
- Una revisión estática posterior planteó que V68–V75 conservarían un límite persistente de 10. La lectura de fuentes lo descarta para este esquema: V76 reemplaza la función llamada por los triggers existentes y su conteo activo permite reservar hasta el slot 30; el otro guard de 10 en V68 pertenece al bloque de backfill ejecutado durante la migración, no queda como restricción runtime. La búsqueda acotada de límites en V69–V75 no encontró otra barrera persistente. Además, las cinco pasadas de IMG01F contra la DB nativa ya ejercitaron 29→30 y el rechazo en 31. El revisor posterior no accedió a DB; no se repitió ninguna migración ni escritura para responderle. Esta conclusión aplica al target local probado, no a Cloud SQL.
- Cobertura no afirmada: IMG01F no ejercita dos reemplazos concurrentes contra el mismo slot ni un cambio de revisión ocurrido entre reserva y finalización; sus claves V1/V2 son representativas a nivel de constraint y no recorren `PlanVariant`/`RecordVariant` de extremo a extremo. La prueba sí cubre carrera de reservas append-only 29→30 y stale CAS en `Reserve`.

## Gates Go y límites

- `env -u TEST_DATABASE_URL -u DATABASE_URL -u IMG01C_TEST_DATABASE_URL -u IMG01D_TEST_DATABASE_URL -u IMG01F_TEST_DATABASE_URL go test -count=1 ./...` — PASS. Estas variables estaban retiradas deliberadamente; el resultado global no sustituye la prueba de PostgreSQL real arriba.
- `go vet ./...` — PASS.
- Formato documental: `prettier --check` pasó para este archivo, la evidencia UI y el plan. `git diff --check` pasó para cambios rastreados; los encabezados de metadatos conservan hard breaks Markdown intencionales.
- No se ejecutó browser→Next BFF→Go→PostgreSQL/storage. F2 verifica persistencia Go/SQL local, no cierra la UX/E2E de F4/F6.
- La prueba de integración usa el rol DB `postgres`, no el rol de runtime `espacigo_runtime`; no se cambiaron roles ni grants fuera de las migraciones autorizadas. La autorización y el alcance de este corte no se extienden a Cloud SQL ni a release/deploy.
