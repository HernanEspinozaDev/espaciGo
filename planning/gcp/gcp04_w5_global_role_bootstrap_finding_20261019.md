# GCP-04-W5: resultado de bootstrap del rol GADM

**Estado:** bloqueado por semántica PostgreSQL 18 bajo el actor solicitado. Evidencia local reproducible; sin conexión SQL ni cambios en Cloud.

## Alcance comprobado

Se probó el bootstrap global que V60 necesita fuera del archivo `pg_dump`: crear `espacigo_gadm_bootstrap` con atributos `NOLOGIN` mínimos y asignar el comentario exacto de V60 desde un actor `NOSUPERUSER CREATEROLE`. La prueba no ejecuta ni modifica V1–V64, no crea ledger y no accede a recursos persistentes.

El harness `scripts/test-gcp04-w5-global-role-bootstrap-postgres.sh` usa PostgreSQL 18/PostGIS fijado por digest en un Compose propio, `network_mode: none`, datos en tmpfs y un socket Unix efímero. El actor controlado es `NOSUPERUSER CREATEROLE`; no tiene password configurado. La ejecución local observó:

1. El actor puede crear el rol GADM y establecer el comentario exacto de V60.
2. PostgreSQL crea una membresía con `ADMIN OPTION` desde `gcp04_w5_actor` a `espacigo_gadm_bootstrap`, registrada con grantor `postgres`.
3. `REVOKE espacigo_gadm_bootstrap FROM gcp04_w5_actor` termina con advertencia porque el actor no otorgó esa arista; la membresía administrativa permanece.
4. `REVOKE ... GRANTED BY postgres` falla con `permission denied to revoke privileges granted by role "postgres"`; el actor no tiene privilegios del grantor.
5. Al fallar esa operación dentro de la transacción, el rollback comprobado elimina el rol temporal y su arista. El fixture termina sin dejar GADM.

Esto coincide con PostgreSQL: al crear roles con CREATEROLE se registra membresía administrativa; `REVOKE` solo puede retirar la concesión del grantor pertinente. Referencias: [CREATE ROLE, PostgreSQL 18](https://www.postgresql.org/docs/18/sql-createrole.html), [REVOKE, PostgreSQL 18](https://www.postgresql.org/docs/18/sql-revoke.html), [COMMENT, PostgreSQL 18](https://www.postgresql.org/docs/18/sql-comment.html).

## Consecuencia

No se agregó un helper que prometa satisfacer el postestado sin membresías: con un actor NOSUPERUSER CREATEROLE no se pudo quitar la arista automática del grantor bootstrap. La transacción puede fallar de forma segura, pero no completar la operación solicitada. Ejecutar la creación y limpieza en transacciones/identidades separadas no conserva atomicidad y deja una membresía temporal persistente entre ellas, por lo que no se adopta como solución.

V60 y las migraciones V1–V64 permanecen byte por byte intactas. El bootstrap de roles para el baseline lógico sigue sin estar resuelto para una autoridad sin SUPERUSER; no se declara compatible con Cloud SQL. No se ejecutó Cloud SQL, Cloud Run, IAM, Secret Manager ni Terraform.

La continuación requiere identificar y revisar una autoridad de base de datos que pueda crear/comentar el rol y retirar su arista —o un mecanismo administrado equivalente— sin conceder privilegios prohibidos ni dejar membresías. No se selecciona ni ejecuta una alternativa en este corte.

## Verificación local

- `bash -n scripts/test-gcp04-w5-global-role-bootstrap-postgres.sh`: PASS.
- `scripts/test-gcp04-w5-global-role-bootstrap-postgres.sh`: PASS como prueba del comportamiento/bloqueo; comprobó la advertencia, el error de permiso, rollback y ausencia final del rol.
- El fixture se retiró por su trap del propio proyecto Compose. Sin acceso TCP, sin volumen persistente y sin datos de ensayo retenidos.
