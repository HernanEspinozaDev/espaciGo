# Evidencia local: baseline migrado y restore PostgreSQL 18

**Resultado:** pasó en entorno local descartable; no prueba compatibilidad con Cloud SQL.

**Fecha:** 2026-10-16.

**Commit fuente:** `352ecd84e982476144225bba55defb37eefa139c`.

**Script:** [`scripts/demo-gcp03-v60-migrated-baseline.sh`](../../scripts/demo-gcp03-v60-migrated-baseline.sh).

## Ensayo ejecutado

- Imagen local exacta `postgis/postgis:18-3.6`, image ID `sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677`.
- Dos contenedores PostgreSQL/PostGIS independientes, con `/var/lib/postgresql` en tmpfs (sin mounts de volumen) y puertos publicados solo en loopback; se retiraron junto con su red al terminar. El conteo global de volúmenes Docker quedó igual (20 antes y después). El script no toca Compose, `espacigo_pgdata` ni Cloud SQL.
- El runner existente aplicó V1–V61 en la base origen local. Historial de origen: 61 filas.
- Archive custom creado con `pg_dump --format=custom --no-owner --no-acl`.
- Roles destino `espacigo_owner`, `espacigo_runtime` y `espacigo_gadm_bootstrap` fueron aprovisionados separadamente como `NOLOGIN` y sin atributos elevados. El comentario aprobado de GADM se aprovisionó fuera del dump. Extensiones `postgis`, `btree_gist` y `unaccent` se preinstalaron en el destino.
- Se inspeccionó el TOC y se excluyeron las entradas precisas de creación/comentario para esas tres extensiones, y la fila de datos de `public.spatial_ref_sys`, administrada por PostGIS. El resto se restauró con `pg_restore --exit-on-error --no-owner --no-acl --role=espacigo_owner`, conectando como el administrador del clúster local. El restore no invocó el migrador.

## Comprobaciones

- Las 61 filas `schema_migrations` coinciden exactamente en `version`, `name`, `checksum`, `runner_checksum` y `applied_at` (`diff` vacío).
- Versiones de extensiones iguales entre bases: `btree_gist 1.8`, `postgis 3.6.4`, `unaccent 1.1`.
- Cero membresías entrantes o salientes del rol `espacigo_gadm_bootstrap`; el comentario de rol destino coincide con el texto provisionado aparte.
- Relaciones públicas de aplicación restauradas con owner `espacigo_owner`; los objetos administrados por PostGIS quedan excluidos de esa comprobación.
- `target_migrator_ran=no`, `target_v60_replayed=no`, sin acceso/SQL/lectura de datos Cloud SQL.
- La salida `PASS` y el `trap` de cleanup terminaron con exit code 0; no quedan contenedores ni red de este ensayo.

## Archivo privado

El archive, TOC, logs y comparaciones completos permanecen fuera de Git en:

`.local/planning/gcp/gcp03-v60-baseline-demo-20261016T075138Z-2502593/`

SHA-256 del archive custom:

`673deea6c9e0d9911e0a100cace555a3d0474310781136c90d4dab38be2ba6c1`

El directorio privado tiene permisos `0700`; sus archivos, `0600`. Los intentos locales previos fueron retirados. Un primer harness dejó volúmenes anónimos por el `VOLUME` declarado por la imagen; se identificaron por la ventana temporal exacta, se retiraron por sus IDs explícitos y el harness ahora usa tmpfs y valida que no haya mounts tipo volume. No se usó prune.

## Límites

El origen se migró como `postgres` superusuario local. El restore usó el administrador PostgreSQL local, con `SET ROLE` a `espacigo_owner`; el script preinstaló extensiones y omitió su carga/comentarios del TOC. Esto no demuestra que el usuario Cloud SQL permita ese procedimiento ni que las versiones de extensiones sean iguales. No se ha probado en Cloud SQL el replay de V1–V61, el restore, ownership/ACL, PostGIS 3.6.0, V60, Go/API, ni un cambio de criterio de instalación vacía. No se usaron datos de aplicación.

La decisión sobre baseline está pendiente: esta prueba solo aporta evidencia local para el ADR [`adr_gcp03_baseline_migrado_v60_20261016.md`](adr_gcp03_baseline_migrado_v60_20261016.md). No autoriza SQL, dump/restore, migración, conexión ni importación en Cloud SQL.
