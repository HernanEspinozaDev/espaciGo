# GCP-04-W7 — prueba del executor efímero para observer

**Estado:** prueba local PostgreSQL 18 concluida; canal Cloud SQL y cierre de ciclo de vida siguen sin resolución. No hubo conexión ni mutaciones Cloud.

## Resultado

El patrón de W5 sí crea el grupo `espacigo_db_observer` con un rol intermediario `NOLOGIN CREATEROLE`, manteniendo separados los permisos:

1. Un actor SQL directo `LOGIN NOSUPERUSER NOCREATEDB CREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT` verifica `CONNECT`, `USAGE` y `SELECT` con `GRANT OPTION`.
2. El actor crea `espacigo_gcp04_w7_observer_executor`, le concede únicamente `SET` a su sesión y entra mediante `SET ROLE`.
3. El executor crea el grupo `NOLOGIN` mínimo. La sesión vuelve al actor original, quien concede al grupo solo `CONNECT`, `USAGE` y `SELECT` del ledger.
4. El executor permanece solo durante la ventana acotada de `users.insert` y verificación. En caso de fallo confirmado de API se compensa mientras aún existe; tras una membresía API exacta se retira. Retirarlo antes de resolver el resultado API dejaría al actor sin autoridad para limpiar el grupo si el alta falla.

La prueba local confirma también dos límites que impiden afirmar que este flujo ya es una solución completa:

- Si el executor concede la membresía del login, PostgreSQL lo registra como grantor. `DROP ROLE executor` falla con una dependencia sobre esa membresía. No se debe ejecutar ese `GRANT` con el executor.
- Si `users.insert` asigna el grupo con los flags PostgreSQL por defecto, el fixture tiene `ADMIN=false, INHERIT=true, SET=true` y el gate W7 lo rechaza. La documentación de Cloud SQL publica `databaseRoles` como nombres de roles, no las opciones de membresía. La prueba local elevó el fixture a superuser solo para verificar que el gate acepta el contrato exacto; esto **no** demuestra ni autoriza cambiar membresías en Cloud SQL.

Si el login llega con el contrato exacto y su membresía fue concedida por un grantor independiente del executor, el SQL candidato puede retirar el executor y comprobar que el vínculo del login se mantiene. Esa rama pasó en fixture. No es evidencia de que Cloud SQL lo haga así.

## Compensación y ciclo de vida

Si la llamada de alta API termina en fallo y una reconciliación confirma que el login no existe, el actor puede revocar los ACL que él concedió, entrar al executor, eliminar el grupo vacío y retirar el executor. Esta compensación pasó localmente. Un resultado API incierto requiere reconciliar primero; no se ejecuta compensación mientras el estado del login sea desconocido.

Después de un alta API exitosa, retirar el executor elimina su ADMIN sobre el grupo. Si más tarde `users.delete` retira al login, el actor `CREATEROLE` original ya no puede eliminar el grupo: PostgreSQL exige también `ADMIN OPTION` para `DROP ROLE`. El ensayo local reprodujo el rechazo.

Una opción concreta que conserva mínimo privilegio es tratar `espacigo_db_observer` como un rol de acceso durable: `NOLOGIN`, sin membresías al estar ocioso y con solo los ACL de lectura del ledger. Al borrar el login builtin vía API, se verifica que el grupo quedó sin miembros y se conserva el rol inerte para reutilizarlo en otro ciclo. Esta política evita dejar un ADMIN persistente y no requiere borrar el grant de grupo; necesita aceptación expresa del revisor/producto porque conserva el objeto/ACL. Si se exige borrar físicamente el rol, hace falta un steward ADMIN persistente y auditado u otra operación de administración demostrada. Ninguna opción se decide aquí.

## Decisiones y gates restantes

1. Confirmar, con documentación o una operación específicamente revisada, las opciones reales de membresía que deja Cloud SQL `users.insert` con `databaseRoles`. La fuente Admin API no especifica `ADMIN/INHERIT/SET`; no inferirlos de una prueba PostgreSQL local.
2. Resolver el actor real: sesión SQL directa, `CREATEROLE` y grant options exactas para la base, schema y ledger. `users.list` no prueba estas capacidades.
3. Decidir la política del rol tras `users.delete`: conservarlo inerte y sin miembros para reutilización, o exigir borrado físico y aprobar un steward `NOLOGIN NOCREATEROLE` con ADMIN restringido/auditable, u otra ruta demostrada. No se elige ninguna opción aquí.
4. Mantener `PUBLIC TEMPORARY=true` observado en W9 como diferencia remota pendiente. La preparación W7 requiere el contrato runtime local `PUBLIC TEMPORARY=false`, pero no cambia el ACL `PUBLIC` ni lo declara corregido en Cloud.

Hasta resolver esos gates, el runner remoto y Terraform permanecen OFF. No usar `postgres`, `cloudsqlsuperuser`, impersonación, lectura/reset de secretos ni nuevas mutaciones Cloud.

## Candidatos SQL

- [Preparar grupo y executor temporal](sql_candidates/gcp04_w7_observer_executor_prepare_candidate.sql)
- [Verificación de solo lectura tras users.insert](sql_candidates/gcp04_w7_observer_verify_api_membership_candidate.sql)
- [Retiro del executor si el API creó la membresía exacta](sql_candidates/gcp04_w7_observer_executor_retire_success_candidate.sql)
- [Compensación solo si el login API se confirmó ausente](sql_candidates/gcp04_w7_observer_compensate_candidate.sql)

Todos son candidatos de revisión local; ninguno es una liberación para Cloud.

## Prueba

Ejecutada mediante [harness PostgreSQL aislado](../../scripts/test-gcp04-w7-observer-executor-postgres.sh), con PostgreSQL 18/PostGIS fijado por digest, red deshabilitada, datos en tmpfs y socket Unix privado. Cubrió rechazo de `PUBLIC TEMP` y grant options ausentes, alta SQL exacta, compensación de login ausente, rechazo del grant hecho por executor, rechazo del `SET=true` por defecto, verificación positiva condicional, retiro del executor y fallo de retiro posterior del grupo sin ADMIN. Es una fixture SQL; la simulación de API no acredita compatibilidad Cloud SQL.

Referencias oficiales: [Cloud SQL: crear/administrar usuarios](https://docs.cloud.google.com/sql/docs/postgres/create-manage-users), [Cloud SQL `users.insert`](https://docs.cloud.google.com/sql/docs/postgres/admin-api/rest/v1beta4/users/insert), [Cloud SQL `users.delete`](https://docs.cloud.google.com/sql/docs/postgres/admin-api/rest/v1beta4/users/delete), [PostgreSQL 18: membresías](https://www.postgresql.org/docs/18/role-membership.html), [PostgreSQL 18: GRANT](https://www.postgresql.org/docs/18/sql-grant.html), [PostgreSQL 18: DROP ROLE](https://www.postgresql.org/docs/18/role-removal.html).
