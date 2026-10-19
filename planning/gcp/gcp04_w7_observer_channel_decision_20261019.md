# GCP-04-W7 — decisión de canal para provisionar el observer

**Estado:** paquete local para revisión. No autoriza ni ejecuta mutaciones en Cloud SQL, Admin API, IAM, Secret Manager o Terraform. El gate remoto sigue OFF.

**Actualización posterior:** PR #429 fusionó el diagnóstico del creador directo. La prueba del patrón executor está en [hallazgo W7 executor](gcp04_w7_executor_membership_finding_20261019.md) y define el estado vigente. Los SQL directos de abajo y sus candidatos se conservan como historia de esa revisión; no ejecutar como solución actual.

## Hallazgo ACL por fase

W9 observó `PUBLIC CONNECT=true`, `TEMPORARY=true` en `espacigo` y `USAGE=true` en `public` antes del bootstrap runtime local. El contrato runtime que ya implementan `001_pre_migrate_roles.sql` y `002_local_restore_role_contract.sql` revoca `TEMPORARY` de `PUBLIC` y mantiene `TEMPORARY` solo para `espacigo_schema_owner`. El harness W6 ejecuta W4 antes y después del restore **después** de aplicar ese contrato.

W7 adopta explícitamente el estado más restrictivo del contrato runtime en ambas fases W4: `PUBLIC CONNECT=true`, `CREATE=false`, `TEMPORARY=false` en la base; `USAGE=true`, `CREATE=false` en `public`. La consulta W4 solo observa esos valores y falla cerrada ante una diferencia; nunca modifica ACL. W9 permanece como evidencia histórica pre-bootstrap, no como permiso para volver a conceder `TEMPORARY`.

## Capacidades necesarias por operación

| Operación | Capacidad PostgreSQL / Cloud | Precondición y límite |
| --- | --- | --- |
| Crear el grupo `espacigo_db_observer` | PostgreSQL `CREATEROLE` | El actor debe ser login directo, sin `SUPERUSER`, `CREATEDB`, `REPLICATION` ni `BYPASSRLS`. En PostgreSQL 18, crear un rol con un actor no superusuario `CREATEROLE` le deja ADMIN OPTION sobre el nuevo rol; el propio actor no puede quitar esa concesión automática. |
| Conceder `CONNECT` a la base | Propietario de la base o `CONNECT WITH GRANT OPTION` | Verificarlo en sesión SQL; que el actor tenga `CREATEROLE` no lo demuestra. |
| Conceder `USAGE` en `public` | Propietario del schema o `USAGE WITH GRANT OPTION` | Verificarlo en sesión SQL. No conceder `CREATE`. |
| Conceder `SELECT` a `public.schema_migrations` cuando exista | Propietario de la tabla o `SELECT WITH GRANT OPTION` | No conceder permisos a tablas de negocio ni grants por defecto. |
| Crear login builtin con rol personalizado | Cloud SQL Admin API `users.insert`, permiso IAM `cloudsql.users.create` | `databaseRoles` debe incluir el grupo preexistente; Google indica que omitir roles en un usuario builtin asigna `cloudsqlsuperuser`. El request documentado solo expone nombres de roles, no opciones de membresía PostgreSQL. No usar contraseña en argumentos, logs ni documento. |
| Corregir/verificar opciones de membresía | PostgreSQL `ADMIN OPTION` sobre `espacigo_db_observer` | La membresía final debe ser una sola: login W7 → grupo, `ADMIN=false`, `INHERIT=true`, `SET=false`. El endpoint de alta no documenta esos flags; se deben comprobar por catálogo y no inferir. |
| Retirar login | Cloud SQL Admin API `users.delete`, permiso IAM `cloudsql.users.delete` | Identidad/nombre exacto; solo después de verificar que fue creado para este corte. |
| Retirar grupo y ACL | Actor con capacidad de revocar los grants registrados y borrar el rol | Retirar primero el login; revocar solo ACL exactas del grupo y ejecutar `DROP ROLE`. Sin `DROP OWNED`, `CASCADE`, edición de `PUBLIC` ni borrado de entidades ajenas. |

Fuentes oficiales: [Cloud SQL: crear y administrar usuarios](https://docs.cloud.google.com/sql/docs/postgres/create-manage-users), [Cloud SQL: `users.insert`](https://docs.cloud.google.com/sql/docs/postgres/admin-api/rest/v1beta4/users/insert), [permisos Cloud SQL para métodos Admin API](https://docs.cloud.google.com/sql/docs/postgres/iam-permissions), [PostgreSQL 18: atributos de roles](https://www.postgresql.org/docs/18/role-attributes.html), [PostgreSQL 18: GRANT](https://www.postgresql.org/docs/18/sql-grant.html), [PostgreSQL 18: REVOKE](https://www.postgresql.org/docs/18/sql-revoke.html).

La documentación Admin API no demuestra las opciones que quedan en `pg_auth_members` al asignar un `databaseRoles[]`; eso es una brecha del contrato documentado, no una prueba de que el valor por defecto falle. La validación W7 exige evidencia del catálogo después del alta.

## Transacción candidata para el grupo

El SQL siguiente muestra las guardas y ACL mínimas que el actor autorizado tendría que poder ejecutar. Es un **candidato de revisión**, no un script autorizado. La precondición rechaza un rol previo y cualquier `PUBLIC TEMP`; no cambia ACL públicas.

El SQL fuente de alta, ejecutado por el harness local, es [gcp04_w7_observer_group_bootstrap_candidate.sql](sql_candidates/gcp04_w7_observer_group_bootstrap_candidate.sql). La retirada SQL está en [gcp04_w7_observer_group_retirement_candidate.sql](sql_candidates/gcp04_w7_observer_group_retirement_candidate.sql).

```sql
BEGIN;

DO $guard$
DECLARE
    actor pg_catalog.pg_roles%ROWTYPE;
BEGIN
    IF current_database() <> 'espacigo' OR current_user <> session_user THEN
        RAISE EXCEPTION 'W7 requires a direct session on database espacigo';
    END IF;

    SELECT * INTO actor FROM pg_catalog.pg_roles WHERE rolname = session_user;
    IF NOT FOUND OR NOT actor.rolcanlogin OR NOT actor.rolcreaterole
       OR actor.rolsuper OR actor.rolcreatedb OR actor.rolreplication
       OR actor.rolbypassrls OR actor.rolinherit THEN
        RAISE EXCEPTION 'W7 requires a direct LOGIN CREATEROLE actor with elevated attributes disabled';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname IN
        ('espacigo_db_observer','espacigo_gcp04_w7_observer_executor')) THEN
        RAISE EXCEPTION 'W7 role collision; reconcile without adopting or changing existing roles';
    END IF;
    IF has_database_privilege('public', current_database(), 'TEMP')
       OR NOT has_database_privilege(session_user, current_database(), 'CONNECT WITH GRANT OPTION')
       OR NOT has_schema_privilege(session_user, 'public', 'USAGE WITH GRANT OPTION') THEN
        RAISE EXCEPTION 'W7 PUBLIC baseline or database/schema grant authority is not satisfied';
    END IF;
    IF to_regclass('public.schema_migrations') IS NOT NULL
       AND NOT has_table_privilege(session_user, 'public.schema_migrations', 'SELECT WITH GRANT OPTION') THEN
        RAISE EXCEPTION 'W7 actor lacks ledger SELECT grant authority';
    END IF;
END
$guard$;

CREATE ROLE espacigo_db_observer
    NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE
    NOREPLICATION NOBYPASSRLS NOINHERIT;

GRANT CONNECT ON DATABASE espacigo TO espacigo_db_observer;
GRANT USAGE ON SCHEMA public TO espacigo_db_observer;

DO $ledger$
BEGIN
    IF to_regclass('public.schema_migrations') IS NOT NULL THEN
        EXECUTE 'GRANT SELECT ON TABLE public.schema_migrations TO espacigo_db_observer';
    END IF;
END
$ledger$;

COMMIT;
```

The candidate has a blocker: PostgreSQL 18 automatically leaves ADMIN OPTION for the non-superuser `CREATEROLE` actor that creates the group. That actor can then grant the group to another login, including itself with `SET` or `INHERIT`; the ACL is not read-only in the sense of an unchangeable steward. The PostgreSQL documentation says the actor cannot revoke or alter that automatic membership. The W7 query now rejects any membership into `espacigo_db_observer` except the single target login, so this direct-creator result cannot pass the gate. The local W5 helper pattern avoids a lasting ADMIN edge by dropping its temporary executor, but also removes that executor's ability to manage the group later. A way to attach the API-created login with exact `INHERIT/SET` options and retire all provisioning authority has not yet been demonstrated.

## Alta, inspección y retirada

Una ruta candidata de login es `users.insert` con el usuario builtin exacto y `databaseRoles=["espacigo_db_observer"]`. Debe usar un flujo de secreto separado, versionado y protegido; este paquete no lee ni crea secretos. La API requiere el grupo preexistente. Al terminar, solo una consulta SQL de solo lectura puede comprobar que el login tiene atributos exactos, una sola membresía al grupo con `ADMIN=false`, `INHERIT=true`, `SET=false`, y no tiene membresías adicionales. Si cualquier condición falla, no ejecutar el runner.

La verificación del estado final puede hacerse con:

```sql
BEGIN READ ONLY;

SELECT r.rolname, r.rolcanlogin, r.rolsuper, r.rolcreatedb,
       r.rolcreaterole, r.rolreplication, r.rolbypassrls, r.rolinherit
FROM pg_catalog.pg_roles r
WHERE r.rolname IN ('espacigo_db_observer', 'espacigo_gcp04_w7_observer');

SELECT granted.rolname AS granted_role, member.rolname AS member_role,
       m.admin_option, m.inherit_option, m.set_option
FROM pg_catalog.pg_auth_members m
JOIN pg_catalog.pg_roles granted ON granted.oid = m.roleid
JOIN pg_catalog.pg_roles member ON member.oid = m.member
WHERE granted.rolname = 'espacigo_db_observer'
   OR member.rolname = 'espacigo_gcp04_w7_observer';

SELECT has_database_privilege('public', 'espacigo', 'CONNECT') AS public_connect,
       has_database_privilege('public', 'espacigo', 'CREATE') AS public_create,
       has_database_privilege('public', 'espacigo', 'TEMP') AS public_temporary,
       has_schema_privilege('public', 'public', 'USAGE') AS public_schema_usage,
       has_schema_privilege('public', 'public', 'CREATE') AS public_schema_create;

COMMIT;
```

Retirada candidata, solo tras la decisión y liberación separadas: borrar únicamente el login W7 con `users.delete`; verificar que ya no existe el login API; después, un actor autorizado ejecuta la transacción de abajo. Acepta solo la membresía ADMIN residual propia del actor que creó directamente el grupo, y aborta ante cualquier otro miembro/owner. Los grants se revocan con el mismo actor/grantor de alta. Cualquier dependencia inesperada hace fallar `DROP ROLE` y revierte la transacción; no se usa `CASCADE` ni `DROP OWNED`. No revocar ACL de `PUBLIC`.

```sql
BEGIN;

DO $retire_guard$
DECLARE
    actor pg_catalog.pg_roles%ROWTYPE;
    observer_oid oid;
BEGIN
    IF current_database() <> 'espacigo' OR current_user <> session_user THEN
        RAISE EXCEPTION 'W7 retirement requires a direct session on database espacigo';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = 'espacigo_gcp04_w7_observer') THEN
        RAISE EXCEPTION 'delete the exact W7 Cloud SQL login through users.delete first';
    END IF;

    SELECT * INTO actor FROM pg_catalog.pg_roles WHERE rolname = session_user;
    SELECT oid INTO observer_oid FROM pg_catalog.pg_roles WHERE rolname = 'espacigo_db_observer';
    IF NOT FOUND OR observer_oid IS NULL OR NOT actor.rolcanlogin OR NOT actor.rolcreaterole
       OR actor.rolsuper OR actor.rolcreatedb OR actor.rolreplication OR actor.rolbypassrls THEN
        RAISE EXCEPTION 'retirement actor or observer group is not the reviewed direct CREATEROLE context';
    END IF;
    IF EXISTS (
        SELECT 1 FROM pg_catalog.pg_auth_members m
        JOIN pg_catalog.pg_roles member ON member.oid = m.member
        WHERE m.roleid = observer_oid
          AND (member.rolname <> session_user OR NOT m.admin_option OR m.inherit_option OR m.set_option)
    ) THEN
        RAISE EXCEPTION 'observer group has an unexpected member; reconcile without CASCADE';
    END IF;
    IF EXISTS (
        SELECT 1 FROM pg_catalog.pg_shdepend d
        WHERE d.refclassid = 'pg_catalog.pg_authid'::regclass
          AND d.refobjid = observer_oid AND d.deptype = 'o'
    ) THEN
        RAISE EXCEPTION 'observer group owns objects; stop for explicit reconciliation';
    END IF;
    IF NOT has_database_privilege(session_user, current_database(), 'CONNECT WITH GRANT OPTION')
       OR NOT has_schema_privilege(session_user, 'public', 'USAGE WITH GRANT OPTION') THEN
        RAISE EXCEPTION 'retirement actor no longer holds the original database/schema grant options';
    END IF;
    IF to_regclass('public.schema_migrations') IS NOT NULL
       AND NOT has_table_privilege(session_user, 'public.schema_migrations', 'SELECT WITH GRANT OPTION') THEN
        RAISE EXCEPTION 'retirement actor no longer holds the original ledger grant option';
    END IF;
END
$retire_guard$;

REVOKE CONNECT ON DATABASE espacigo FROM espacigo_db_observer;
REVOKE USAGE ON SCHEMA public FROM espacigo_db_observer;

DO $ledger_revoke$
BEGIN
    IF to_regclass('public.schema_migrations') IS NOT NULL THEN
        EXECUTE 'REVOKE SELECT ON TABLE public.schema_migrations FROM espacigo_db_observer';
    END IF;
END
$ledger_revoke$;

DROP ROLE espacigo_db_observer;
COMMIT;
```

## Actor comprobado y decisión requerida

No hay evidencia disponible de una identidad SQL existente que cumpla `CREATEROLE` más los grant options de base/schema/ledger. El `users.list` de W9 solo reportó `postgres`; eso no informa atributos SQL de actores ocultos ni capacidad de conceder ACL, así que no se usa para afirmar presencia o ausencia. El login W9 con `pg_read_all_settings` fue retirado y no se reutiliza. `postgres`, `cloudsqlsuperuser`, impersonación y lectura/reset de secretos están excluidos.

La fixture W6 demuestra que un actor `CREATEROLE` puede preparar roles en PostgreSQL 18 local y que el contrato W7 se satisface luego del bootstrap runtime con `PUBLIC TEMPORARY=false`. La nueva prueba local adicional registra que crear directamente un grupo con ese actor deja una membresía ADMIN persistente. Esto evidencia que hace falta resolver el stewardship del grupo antes de Cloud: aceptar y revisar ese administrador, designar un provisioner `NOLOGIN` dedicado con una asignación controlada, o aprobar otra ruta que demuestre alta y membresía exactas sin privilegio persistente. Ninguna de esas decisiones se presupone aquí.

Hasta que exista evidencia SQL de un actor y se elija el stewardship, el estado correcto es **bloqueado antes del alta remota**, con el runner y Terraform OFF. Ningún recurso Cloud, rol, usuario, secreto, IAM, Job o state se creó o alteró para este paquete.
