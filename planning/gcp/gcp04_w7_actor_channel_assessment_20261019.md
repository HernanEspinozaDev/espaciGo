# GCP-04-W7 — actor SQL y opciones de membresía

**Estado:** metadatos de control plane leídos; no se abrió una sesión SQL remota ni se mutó Cloud. El corte no habilita alta remota.

## Actor y canal que muestran los metadatos

Para el proyecto explícito `espacigo-testprod`, el principal activo de `gcloud` tiene `roles/owner` en el proyecto. `projects.testIamPermissions` confirmó para ese principal `cloudsql.instances.connect`, `cloudsql.instances.get`, `cloudsql.users.create` y `cloudsql.users.delete`. Son permisos de IAM/control plane. No atribuyen `CREATEROLE`, propiedad de objetos ni `GRANT OPTION` en PostgreSQL.

`gcloud sql instances list` mostró `espacigo-testprod-pg` RUNNABLE en `southamerica-west1`, PostgreSQL 18. `gcloud sql users list` para esa instancia mostró solo el usuario builtin `postgres`. La documentación de Cloud SQL aclara que la lista de usuarios no muestra todos los usuarios del sistema; tampoco contiene atributos ni membresías del catálogo PostgreSQL. Por eso no se concluye que no exista otro rol SQL: los metadatos disponibles no identifican un actor SQL directo permitido con `CREATEROLE` y los grant options requeridos. El `postgres` listado no se usa por la restricción vigente.

La evidencia privada saneada está en `.local/testing/gcp04-w7-actor-channel-metadata-20261019.json` (0600); no versiona tokens ni respuestas brutas de IAM. Las lecturas usaron proyecto explícito y no cambiaron configuración global.

## Permiso faltante y siguiente decisión concreta

El control plane permite a la identidad activa administrar usuarios de Cloud SQL, pero `users.insert` requiere que el custom database role exista antes; no hay una API de Cloud SQL documentada que cree ese rol custom. Para crear el grupo faltan evidencias SQL directas de un login permitido con `CREATEROLE`, y `CONNECT`, `USAGE` y `SELECT` con `GRANT OPTION` en los objetos exactos. Un `users.list` o una unión IAM no demuestra ninguna de esas capacidades.

El siguiente input operativo debe ser un actor PostgreSQL **existente y nombrado** cuya conexión directa y catálogo se puedan verificar en modo read-only, o una decisión explícita de detener W7 porque no hay actor admisible. Si nadie puede identificar ese actor, la capacidad concreta que falta es la sesión SQL autorizada; no se resuelve leyendo/resetando el secreto de `postgres`, usando `cloudsqlsuperuser`, impersonando ni creando una cuenta con roles de fallback. No se hizo ninguna de esas acciones.

Para un actor candidato, la verificación read-only debe confirmar `LOGIN`, `NOSUPERUSER`, `NOCREATEDB`, `CREATEROLE`, `NOREPLICATION`, `NOBYPASSRLS`, contexto directo y los grant options de `CONNECT` en `espacigo`, `USAGE` en `public` y `SELECT` en `public.schema_migrations` si existe. El grantor de cada ACL se conserva para el plan de compensación. No se autoriza aquí conectarse a Cloud SQL ni cambiar ACL.

## `SET=true` frente a `SET=false`

El harness local comparó dos logins con membresía `INHERIT=true`, el mismo grupo observer y los mismos ACL: `ADMIN=false, SET=true` frente a `ADMIN=false, SET=false`. Ambos reciben los mismos privilegios efectivos de base/schema/ledger: `CONNECT`, `USAGE` y solo `SELECT` del ledger; ambos son rechazados al intentar insertar. `pg_has_role(...,'USAGE')` coincide. `pg_has_role(...,'SET')` solo permite ejecutar `SET ROLE` para el login `SET=true`.

Inmediatamente después de conectarse, ambos logins conservaron `session_user=current_user=login`. `SET=true` no ejecuta `SET ROLE` automáticamente. Solo al emitir explícitamente `SET ROLE espacigo_db_observer`, `session_user` siguió identificando el login y `current_user` pasó a ser el grupo; el login `SET=false` recibió rechazo. La consulta W4 reporta `current_user` en su campo `database.user` ([consulta W4](../../db/query/gcp04_w4_target_preflight.sql)), pero no ejecuta `SET ROLE`. Por tanto, no se observó una alteración automática de identidad ni expansión de privilegios de objetos en el runner. El gate actual conserva `SET=false` como contrato más restrictivo; este corte no lo relaja ni atribuye a `SET=true` un cambio de identidad que requiere una acción explícita. Cualquier revisión futura debe decidir por separado si se permite esa capacidad y qué identidad se valida/audita.

La prueba cubre PostgreSQL 18 local, no Cloud SQL ni los flags reales que asigna `databaseRoles`. Google documenta la lista `databaseRoles` como nombres de roles y no especifica `ADMIN/INHERIT/SET`; la membresía real sigue pendiente de inspección aprobada. `PUBLIC TEMPORARY=true` observado en W9 permanece una diferencia remota sin cambio.

## Verificación

El harness [test-gcp04-w7-observer-executor-postgres.sh](../../scripts/test-gcp04-w7-observer-executor-postgres.sh) ahora contrasta los dos flags de forma aislada junto con las ramas de verificación y retiro del executor. El ensayo mantiene PostgreSQL 18/PostGIS fijado por digest, red `none`, datos tmpfs y socket Unix privado; elimina recursos propios al salir.

Fuentes oficiales: [Cloud SQL crear/administrar usuarios](https://docs.cloud.google.com/sql/docs/postgres/create-manage-users), [Cloud SQL permisos IAM](https://docs.cloud.google.com/sql/docs/postgres/iam-permissions), [Resource Manager `projects.testIamPermissions`](https://docs.cloud.google.com/resource-manager/reference/rest/v3/projects/testIamPermissions), [PostgreSQL 18 membresías](https://www.postgresql.org/docs/18/role-membership.html), [PostgreSQL 18 `pg_has_role`](https://www.postgresql.org/docs/18/functions-info.html).
