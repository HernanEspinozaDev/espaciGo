# Evidencia del replay GCP-03 V1–V61 — 2026-10-16

## Alcance

El ensayo se ejecutó en la base aislada `espacigo-testprod:southamerica-west1:espacigo-gcp03-pilot01-pg/espacigo_scratch`. No se usó la base persistente, datos de aplicación, datasets, imports ni restores. La instancia Cloud SQL se conservó. Se usó el OCI revisado `sha256:3e5683bf557a009d0ac14f60575f1054f9c089239b9e65fae27ab8acadae3548`, sin overrides de fase, target, imagen, credencial o reintentos.

El plan de cinco Jobs aplicado el 2026-10-16 a las 14:43:21Z tuvo 12 adiciones, cero cambios y cero eliminaciones. Hashes privados del plan binario y JSON: `bc1a4241622027a3981cf2068ef832b0339f97628cc501b0ea382fa6edc786c7` y `9f1bf8338b179be0aabd46592042c5351de2ec0f5da1c530ab6469558919935e`. El `terraform plan -refresh` posterior no encontró drift. Migrator v4 y owner v1 estaban `ENABLED`; sus payloads nunca se leyeron. Jobs, invocaciones y `actAs` se limitaron a los principals planeados. Las cinco fases tenían una tarea, cero reintentos, timeout de cuatro horas y el mismo digest de imagen.

## Ejecuciones

| Paso | Operación/ejecución | Resultado comprobado |
| --- | --- | --- |
| `bootstrap` | `espacigo-gcp03-pilot01-bootstrap-8799x` | Cloud Run `Completed=True`, una tarea exitosa. Instaló/validó `btree_gist=1.8`, `postgis=3.6.4`, `unaccent=1.1`. |
| Reconcile | Cloud SQL `UPDATE_USER` `47c146bb-4880-4129-96fe-7f530000004b` | `DONE`; el CLI revisado leyó después las membresías y confirmó únicamente `cloudsqlsuperuser` para el login migrador. |
| `database_acl` | `espacigo-gcp03-pilot01-database-acl-qcfw6` | Cloud Run `Completed=True`, una tarea exitosa; el handler completó las verificaciones fijas de ACL. |
| `migrate` | `espacigo-gcp03-pilot01-migrate-rhsmc` | Cloud Run `Completed=True`, una tarea exitosa, con preflight fijo de bootstrap y ACL. Corrió la fase migratoria inmutable V1–V61. |
| `finalize` | `espacigo-gcp03-pilot01-finalize-n6st7` | Cloud Run `Completed=True`, una tarea exitosa. El handler exige `VerifyAllApplied` para 61 migraciones/checksums, aplica grants runtime y exige las 16 comprobaciones runtime. El log terminó en `GCP-03 scratch phase finalize completed`. |

Cada Job se ejecutó una vez, después de revisar target, digest, fase, identidad, versión numérica habilitada del secreto, policy de invocación y ausencia de ejecuciones anteriores. Las listas posteriores muestran una ejecución para cada fase completada. No hubo override ni reintento.

## Límites y estado posterior

El observador de extensiones, ownership, funciones `SECURITY DEFINER`, ACL y operaciones espaciales no está en el digest usado. El PR #357 agrega la implementación y la salida NDJSON acotada (registros ≤32 KiB, manifiesto con conteos/hash, y reconstrucción que rechaza datos incompletos o corruptos). La regresión local supera 256 KiB y verifica reconstrucción/hash. Una integración PostgreSQL 18/PostGIS 3.6 desechable ejecutó V1–V61 y el observador como migrador con sesión `transaction_read_only=on`; pasaron inventario, ACL, 16 checks runtime, PostGIS y el índice GIST esperado. Esta prueba local no demuestra la membresía administrada `cloudsqlsuperuser` ni reemplaza la observación administrada. El cambio continúa pendiente de revisión; no se ha compilado/publicado una nueva OCI, agregado/aplicado un Job ni ejecutado el observador en Cloud SQL. Por ello el ensayo **no declara compatibilidad total**.

`retire` no se ejecutó. Migrator v4 y owner v1 siguen habilitados a la espera del cierre separado: validar el observador, ejecutar y revisar `retire` como owner, revocar los roles temporales del migrador y destruir solo su versión exacta, y rotar `postgres` y destruir solo la versión owner exacta tras confirmar `DONE`. Los bindings IAM temporales expiran a las 16:00Z; esa expiración no equivale a caducidad automática de las contraseñas ni de las versiones secretas.

Los logs saneados, respuestas estructuradas, estados de Jobs/ejecuciones, versiones metadata-only y hashes permanecen en `.local/testing/gcp03-replay-credential-iam-plan-20261016/` con directorio 0700 y archivos 0600. No se guardaron payloads de secretos ni credenciales en Git.
