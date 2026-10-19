# GCP-04-W5: bootstrap global del rol GADM

**Estado:** candidato local verificado en PostgreSQL 18; compatibilidad con Cloud SQL pendiente. No hubo conexión SQL ni cambios en Cloud.

## Resultado

Se probó el bootstrap global que requiere el baseline lógico, fuera del archivo `pg_dump`, con un actor directo `LOGIN NOSUPERUSER NOCREATEDB CREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT`. El helper `db/bootstrap/gcp04-runtime/004_gadm_global_role.sql` crea y comenta `espacigo_gadm_bootstrap` usando un rol intermediario transaccional `NOLOGIN CREATEROLE`, sin editar ni repetir V1–V64.

La secuencia completa dentro de una transacción es:

1. Validar atributos mínimos del actor. Rechazar si ya existe el nombre reservado del intermediario.
2. Si GADM ya existe, aceptar solo su contrato exacto; en ese caso el helper no muta nada. Atributos, comentario, membresías, ownership de objetos y settings inesperados hacen abortar.
3. Si falta GADM, crear el intermediario `NOLOGIN CREATEROLE`. PostgreSQL concede al creador una membresía `ADMIN OPTION` que no permite `SET ROLE` en este caso. El actor añade una segunda membresía a sí mismo con `ADMIN FALSE, INHERIT FALSE, SET TRUE`.
4. Ejecutar `SET ROLE` al intermediario, crear GADM con atributos `NOLOGIN` mínimos y fijar el comentario exacto conservado por V60.
5. Ejecutar `RESET ROLE`, eliminar el intermediario y comprobar GADM sin membresías entrantes/salientes, objetos propios ni configuración por rol. Solo entonces hacer `COMMIT`.

La prueba inicial también conserva el diagnóstico de revocación directa: el `CREATEROLE` actor puede crear y comentar GADM, pero la membresía automática queda otorgada por `postgres`; el `REVOKE` normal no la elimina y `REVOKE ... GRANTED BY postgres` falla por permisos. El intermediario resuelve ese borde porque se elimina entero dentro de la misma transacción, retirando sus membresías en ambos extremos. La regresión comprueba ambas rutas.

## Evidencia y alcance

`scripts/test-gcp04-w5-global-role-bootstrap-postgres.sh` corre en un Compose exclusivo, con PostgreSQL 18/PostGIS fijado por digest, red `none`, datos en tmpfs y socket Unix efímero. Probó:

- Creación/comentario con el actor CREATEROLE y cero membresías GADM al finalizar.
- Eliminación del intermediario y conservación de las propiedades del actor.
- Rerun exacto como no-op, sin nuevas membresías ni crecimiento de permisos.
- Fallo sin reparación ante atributos, comentario, membresía entrante/saliente y ownership inesperados.
- Rollback limpio de la ruta de revocación directa fallida.
- Teardown del contenedor y volumen propios.

El contrato global corresponde a roles y comentarios fuera de un `pg_dump`; no cambia cómo se ejecutan migraciones. V60 conserva `COMMENT ON ROLE` y su guardia, byte por byte. Un replay directo de V1–V64 como `espacigo_schema_owner` aún falla en V60; esta prueba no altera ni pretende evitar ese replay. La posible aplicación del helper antes de un restore lógico de baseline necesita revisión de secuencia/privilegios y prueba adicional del destino.

No se validó Cloud SQL ni el grantor/atributos efectivos de sus usuarios. Por tanto, este resultado demuestra PostgreSQL 18 local, no compatibilidad con el candidato Cloud SQL. No se ejecutó Cloud SQL, Cloud Run, IAM, Secret Manager ni Terraform; no se creó ledger ni se usaron datos persistentes.

Referencias PostgreSQL: [CREATE ROLE](https://www.postgresql.org/docs/18/sql-createrole.html), [GRANT/REVOKE de membresías](https://www.postgresql.org/docs/18/sql-revoke.html), [COMMENT](https://www.postgresql.org/docs/18/sql-comment.html), [DROP ROLE](https://www.postgresql.org/docs/18/sql-droprole.html).
