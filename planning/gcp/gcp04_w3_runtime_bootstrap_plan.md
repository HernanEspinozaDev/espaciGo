# GCP-04-W3 — bootstrap runtime privado y preflight SQL

**Estado:** bloqueado para ejecución en Cloud SQL; propuesta y diagnóstico local preparados para revisión. No aplicada en Cloud SQL ni desplegada. Este corte no acepta GCP-04 ni habilita API/worker.

## Inventario confirmado, solo metadata

El candidato único de este corte es la instancia existente `espacigo-testprod-pg` del proyecto `espacigo-testprod`: PostgreSQL 18, `RUNNABLE`, IP privada, región `southamerica-west1`, red `espacigo-testprod` y subred serverless de la misma región. En el inventario previo la API de Cloud SQL listó las bases `postgres` y `espacigo` y únicamente el usuario integrado `postgres`. Existen los secretos metadata de API y migrador, pero la lista de versiones no devolvió versiones activas. No se leyó ningún secreto ni se abrió sesión SQL.

El root Terraform correcto tiene lineage `95bafa42-5d1a-bef2-909b-7ce6b4a92096`, serial `11`. No se consultó ni usó state de GCP-03. La cuenta/password de `postgres`, `max_connections`, reservas, actividad, extensiones instaladas, ownership, ACL y membresías SQL siguen **sin observar**. El presupuesto de 24 conexiones es una hipótesis de W2, no una medición.

## Bloqueo de compatibilidad en V60

La inspección local de PostgreSQL 18 encontró una incompatibilidad que impide afirmar que el bootstrap V1–V64 es ejecutable en Cloud SQL:

- El harness aislado ejecuta V1–V59 como `espacigo_schema_owner` y confirma que el ledger queda bajo ese rol.
- V60 ejecuta `COMMENT ON ROLE espacigo_gadm_bootstrap`. PostgreSQL requiere `CREATEROLE` y `ADMIN OPTION` sobre el rol comentado para este comando ([documentación PostgreSQL 18](https://www.postgresql.org/docs/18/sql-comment.html)). El rol de Cloud SQL `postgres` no es superusuario PostgreSQL; la documentación de Cloud SQL limita sus atributos y privilegios ([usuarios de Cloud SQL](https://docs.cloud.google.com/sql/docs/postgres/users)).
- En PostgreSQL 16+, un usuario con `CREATEROLE` que crea otro rol recibe una membresía `ADMIN OPTION` sobre ese rol ([CREATE ROLE](https://www.postgresql.org/docs/current/sql-createrole.html)). La migración V60 rechaza cualquier membresía del rol GADM. Por eso dar temporalmente `CREATEROLE` al schema owner no basta: la ejecución local se detiene en V60 con `permission denied`; permitirle crear el rol produciría la membresía que V60 prohíbe.
- El preflight y la prueba local reproducen ambos comportamientos. No hubo conexión SQL con Cloud SQL, por lo que esto es compatibilidad deducida de la migración versionada y del comportamiento PostgreSQL 18, no una observación sobre el estado SQL del candidato.

V1–V64 son inmutables según las reglas del repositorio. No hay una ruta de replay completa que este corte pueda recomendar sin una decisión arquitectónica explícita sobre V60. Los archivos de bootstrap de esta propuesta son únicamente diagnósticos locales y no deben ejecutarse en Cloud SQL. Resolverlo probablemente requiere un corte autorizado con migración nueva o un mecanismo de bootstrap compatible acordado por revisión; no se presupone ni implementa esa decisión aquí.

## Roles y secuencia propuesta (no ejecutable aún)

El archivo `001_pre_migrate_roles.sql` es un fixture local, con guardas de base de datos. En Cloud SQL su control de membresías puede abortar y revertir la transacción al detectar el edge admin que crea PostgreSQL. No se propone aún crear grupos en la instancia.

La arquitectura de `DATABASE_SET_ROLE` y el preflight SELECT se implementaron como código/propuesta para revisión, pero el replay no supera V60 y no se presenta como listo para desplegar. El harness aplica V1–V59 localmente, verifica la detención esperada en V60 y prueba el SELECT observer local.

Si una decisión revisada resuelve V60, después deberá diseñarse de nuevo el aprovisionamiento de logins y permisos; este PR no incluye los scripts previos de binding ni recomienda ejecutar esa secuencia. Como referencia de diseño no aprobada, Cloud SQL permite asignar roles de base de datos personalizados al crear usuarios. [Crear usuarios con roles de base de datos](https://docs.cloud.google.com/sql/docs/postgres/create-manage-users#create) y [usuarios y roles de Cloud SQL](https://docs.cloud.google.com/sql/docs/postgres/users).

El API y el migrator ahora aceptan `DATABASE_SET_ROLE` y realizan `SET ROLE` por conexión. La configuración Terraform del Job migrator fija `espacigo_schema_owner`. El worker ya definía su rol. Estas piezas de código no habilitan despliegue: falta un replay válido, identidades con permisos revisados y preflight remoto.

## SELECT privado de capacidad y estado

`db/query/gcp04_runtime_private_preflight.sql` abre una transacción `READ ONLY`; solo consulta settings, conteos agregados de backends, propietarios/ACL, atributos y memberships de los seis roles, extensiones y el historial de migraciones. No devuelve direcciones de cliente, PID, SQL ejecutado, filas de negocio, contraseñas ni hashes de credenciales. Se ejecutaría con el login `espacigo_observer_login`, conectado al grupo de solo lectura.

La vía concreta es un Job de una sola tarea en Cloud Run, región `southamerica-west1`, Direct VPC egress a la red/subred existentes y destino Cloud SQL privado usando el Go Connector con `CLOUD_SQL_PRIVATE_IP=true`; no se habilita IP pública. El service account recibe `roles/cloudsql.client` condicionado a esta instancia y `secretAccessor` solo sobre el secreto del observer. El Job monta la versión numérica fijada como archivo, ejecuta únicamente el preflight, conserva salida saneada y no expone endpoint. Cloud Run permite Direct VPC egress para Jobs; Cloud SQL requiere usar la IP privada y la misma VPC. [Direct VPC para Cloud Run Jobs](https://docs.cloud.google.com/run/docs/configuring/vpc-direct-vpc) y [conexión privada desde Cloud Run a Cloud SQL](https://docs.cloud.google.com/sql/docs/postgres/connect-run).

El resultado debe demostrar `max_connections`, `reserved_connections`, `superuser_reserved_connections`, número de client backends visibles y slots ordinarios actualmente libres. Antes de cualquier activación se compara la capacidad libre con el presupuesto revisado de 24. Si la visibilidad del catálogo o el margen es insuficiente, se detiene el rollout; no se infiere capacidad. La query no mide conexiones futuras ni evita una segunda consulta tras cambiar cargas.

## Operación pendiente de decisión y revisión independiente

No ejecutar como parte de este PR: cambiar contraseña de `postgres`, crear usuarios, añadir versiones de Secret Manager, ejecutar SQL, publicar imagen, crear/ejecutar Jobs, modificar state, conceder IAM o iniciar worker/API. Antes de cualquier operación Cloud se necesita resolver y revisar la incompatibilidad de V60. Los secretos API/migrador inventariados no tienen versión activa observada y no se presume que exista una credencial de administrador válida.

Cuando el propietario revise y libere la operación concreta, el orden sería:

1. Releer metadata/state del candidato y verificar que instancia, base, VPC y subred coinciden con este corte. Determinar por canal seguro cómo obtener una credencial administrativa; Cloud SQL no crea password para su usuario `postgres` por defecto en una instancia nueva. No reutilizar credenciales de otros ensayos.
2. Tras resolver V60 en un corte revisado, definir el bootstrap compatible y recién entonces considerar crear roles o ejecutar replay.
3. En una autorización posterior, revalidar identidad/metadata/state, credenciales, SQL y orden de provisión; ninguno de los pasos heredados de una propuesta no aprobada se ejecuta automáticamente.
4. Worker Pools siguen Beta/Pre-GA y con costo pendiente; este corte no activa worker. El gate de API sigue cerrado mientras falten las demás familias de background.

## Pruebas y límites

El harness aislado `scripts/test-gcp04-w3-role-bootstrap-postgres.sh` crea roles desechables, prueba `SET ROLE`, aplica V1–V59, verifica el fallo esperado de V60 y corre el preflight con el observer. También comprueba el edge `ADMIN OPTION` en PostgreSQL 18. El fixture usa un contenedor efímero PostGIS, red `none`, sin puertos, base en tmpfs y socket Unix privado; retira solo su proyecto y volumen Compose propios.

La preparación local no constituye una ejecución SQL contra el candidato. El presupuesto de 24 sigue sin confirmar hasta el SELECT privado. La base persistente, sus secretos, recursos, estado y todas las pruebas/evidencias ajenas al run permanecen preservados.
