# GCP-04-W3 — bootstrap runtime privado y preflight SQL

**Estado:** replay directo V1–V64 con rol restringido bloqueado por V60. Candidato de baseline lógico validado solo entre clústeres PostgreSQL locales efímeros; todavía no es un procedimiento autorizado ni probado en Cloud SQL. Este corte no acepta GCP-04 ni habilita API/worker.

## Inventario confirmado, solo metadata

El candidato único de este corte es la instancia existente `espacigo-testprod-pg` del proyecto `espacigo-testprod`: PostgreSQL 18, `RUNNABLE`, IP privada, región `southamerica-west1`, red `espacigo-testprod` y subred serverless de la misma región. En el inventario previo la API de Cloud SQL listó las bases `postgres` y `espacigo` y únicamente el usuario integrado `postgres`. Existen los secretos metadata de API y migrador, pero la lista de versiones no devolvió versiones activas. No se leyó ningún secreto ni se abrió sesión SQL.

El root Terraform correcto tiene lineage `95bafa42-5d1a-bef2-909b-7ce6b4a92096`, serial `11`. No se consultó ni usó state de GCP-03. La cuenta/password de `postgres`, `max_connections`, reservas, actividad, extensiones instaladas, ownership, ACL y membresías SQL siguen **sin observar**. El presupuesto de 24 conexiones es una hipótesis de W2, no una medición.

## Bloqueo de compatibilidad en V60

La inspección local de PostgreSQL 18 encontró una incompatibilidad que impide afirmar que el bootstrap V1–V64 es ejecutable en Cloud SQL:

- El harness aislado ejecuta V1–V59 como `espacigo_schema_owner` y confirma que el ledger queda bajo ese rol.
- V60 ejecuta `COMMENT ON ROLE espacigo_gadm_bootstrap`. PostgreSQL requiere `CREATEROLE` y `ADMIN OPTION` sobre el rol comentado para este comando ([documentación PostgreSQL 18](https://www.postgresql.org/docs/18/sql-comment.html)). El rol de Cloud SQL `postgres` no es superusuario PostgreSQL; la documentación de Cloud SQL limita sus atributos y privilegios ([usuarios de Cloud SQL](https://docs.cloud.google.com/sql/docs/postgres/users)).
- En PostgreSQL 16+, un usuario con `CREATEROLE` que crea otro rol recibe una membresía `ADMIN OPTION` sobre ese rol ([CREATE ROLE](https://www.postgresql.org/docs/current/sql-createrole.html)). La migración V60 rechaza cualquier membresía del rol GADM. Por eso dar temporalmente `CREATEROLE` al schema owner no basta: la ejecución local se detiene en V60 con `permission denied`; permitirle crear el rol produciría la membresía que V60 prohíbe.
- El preflight y la prueba local reproducen ambos comportamientos. No hubo conexión SQL con Cloud SQL, por lo que esto es compatibilidad deducida de la migración versionada y del comportamiento PostgreSQL 18, no una observación sobre el estado SQL del candidato.

### Resultado GCP-04-W5 (2026-10-19)

La prueba dedicada `scripts/test-gcp04-w5-global-role-bootstrap-postgres.sh` confirmó el límite que faltaba medir: un actor directo `NOSUPERUSER CREATEROLE` puede crear GADM y establecer el comentario V60, pero PostgreSQL 18 registra la membresía `ADMIN OPTION` con grantor `postgres`. El `REVOKE` ordinario no la elimina (advertencia; la arista permanece) y `REVOKE ... GRANTED BY postgres` falla por falta de privilegios del grantor. El rollback transaccional retira el rol temporal y evita residuos, pero no completa el bootstrap.

No hay helper transaccional funcional bajo ese actor que cumpla el requisito de no dejar membresías. No se añadió una secuencia de dos transacciones porque deja una arista global persistente entre operaciones y pierde atomicidad. El resultado reproducible y sus límites constan en [GCP-04-W5](gcp04_w5_global_role_bootstrap_finding_20261019.md). V1–V64 y V60 siguen inmutables. La compatibilidad SQL administrada continúa sin demostrarse; no se hizo operación Cloud.

V1–V64 son inmutables según las reglas del repositorio. Un replay directo con el rol de esquema en Cloud SQL sigue sin ruta compatible por la guarda de V60. El ensayo local completó las migraciones originales sin filtrar statements en una fuente efímera administrada localmente, y después probó un candidato de traslado lógico. No se presupone que Cloud SQL pueda aceptar ese artefacto ni se propone ejecutarlo allí antes de una decisión/revisión del procedimiento operativo.

## Roles y secuencia propuesta (no ejecutable aún)

El archivo `001_pre_migrate_roles.sql` es un fixture local, con guardas de base de datos. En Cloud SQL su control de membresías puede abortar y revertir la transacción al detectar el edge admin que crea PostgreSQL. No se propone aún crear grupos en la instancia. El contrato de roles del destino del ensayo está fuera del dump y mantiene los grupos NOLOGIN; ningún rol miembro de `espacigo_gadm_bootstrap` se crea.

La arquitectura de `DATABASE_SET_ROLE` y el preflight SELECT se implementaron como código/propuesta para revisión. Además de la prueba de que un replay directo como rol restringido se detiene en V60, el harness `scripts/test-gcp04-w3-logical-baseline-postgres.sh` valida el candidato de baseline local:

- Crea dos clústeres PostgreSQL 18/PostGIS efímeros, sin red ni puertos TCP, y espera el servidor final comprobando `SHOW listen_addresses = '*'` (no se basa solo en `pg_isready`).
- En el primer clúster vacío, ejecuta con `cmd/dbmigrate` V1–V64 byte por byte y conserva los datos semilla/ledger reales producidos por las migraciones. Solo después de terminar el replay, un fixture local normaliza owners/grants para el diseño de roles.
- Genera un custom archive nativo con `pg_dump`. Roles/membresías globales no viajan en el archive: el segundo clúster precrea los grupos NOLOGIN y los cuatro logins con atributos mínimos. No hay `ADMIN OPTION` ni membresías a/desde `espacigo_gadm_bootstrap`.
- La TOC filtrada se limita a extensiones ya instaladas en el destino, sus objetos de extensión y sus datos; ACL de base/schema provienen del contrato SQL local explícito. El resto de ACL se extrae del archive y aplica como `SET ROLE espacigo_schema_owner` con el login migrator no-superuser. Los filtros, entradas omitidas y hashes quedan en el manifiesto efímero.
- El `pg_restore` nativo se ejecuta como `espacigo_migrator_login` con `SET ROLE espacigo_schema_owner`. Después, el migrator verifica V1–V64/checksums desde los archivos canónicos y reporta `sin cambios`; una divergencia sintética de checksum V64 hace abortar antes de DDL.
- El ensayo compara hash de datos JSON por tabla para 98 tablas públicas, schema dump estructural, roles/membresías, ACL expandidas, owners, extensiones y funciones `SECURITY DEFINER`. Los hashes source/destination deben coincidir. Los ACL explícitos del owner se documentan aparte porque el destino puede expresar los mismos privilegios implícitamente por ownership. Las diferencias de paréntesis de `pg_dump` permitidas se inventarían por nombre y hash, verificando igualdad de la definición de catálogo.
- Prueba LOGIN/SET ROLE de API, worker y migrator, observer por membresía heredada sin `SET ROLE`, permisos esperados y denegaciones reales. Ejecuta el preflight read-only en el destino restaurado.

El baseline source/restore pasó localmente. El artefacto custom, TOC original/filtrada, manifiesto y checksums se usan solo dentro del run y se retiran al terminar. Esto demuestra portabilidad/restricción en el fixture local; no valida Cloud SQL, cuentas IAM, la ruta de acceso a artefactos, Cloud Run, Secret Manager ni los permisos efectivos del usuario `postgres` del candidato.

Antes de cualquier ejecución Cloud, la revisión debe decidir si acepta el baseline lógico como mecanismo operativo y definir cómo generar/retener/revisar el artefacto, cómo preaprovisionar roles/owners/ACL/extensions con mínimo privilegio, y cómo ejecutar un restore privado verificable en Cloud SQL. El ensayo local no incluye el comando de operación Cloud ni la provisión. Como referencia de diseño no aprobada, Cloud SQL permite asignar roles de base de datos personalizados al crear usuarios. [Crear usuarios con roles de base de datos](https://docs.cloud.google.com/sql/docs/postgres/create-manage-users#create) y [usuarios y roles de Cloud SQL](https://docs.cloud.google.com/sql/docs/postgres/users).

El API y el migrator ahora aceptan `DATABASE_SET_ROLE` y realizan `SET ROLE` por conexión. La configuración Terraform del Job migrator fija `espacigo_schema_owner`. El worker ya definía su rol. Estas piezas de código no habilitan despliegue: falta definir y revisar un procedimiento Cloud que use el baseline lógico validado localmente, identidades con permisos revisados y ejecutar el preflight remoto.

## SELECT privado de capacidad y estado

`db/query/gcp04_runtime_private_preflight.sql` abre una transacción `READ ONLY`; solo consulta settings, conteos agregados de actividad/roles, propietarios/ACL, extensiones y el historial de migraciones. Incluye el login `espacigo_privacy_worker_login`, que coincide con Terraform y el worker. La consulta de capacidad informa backends de cliente clasificados, filas de actividad con `backend_type` oculto y un límite inferior conservador de slots libres: resta todas las filas no clasificables como si consumieran una conexión cliente. PostgreSQL puede ocultar ese campo para otros usuarios sin `pg_read_all_stats`; el preflight no solicita ese privilegio amplio. Si la visibilidad es parcial, la cota puede ser menor que la capacidad real, nunca se debe tratar el contador clasificado como total. No devuelve direcciones de cliente, PID, SQL ejecutado, filas de negocio, contraseñas ni hashes de credenciales. Se ejecutaría con el login `espacigo_observer_login`, conectado al grupo de solo lectura.

La vía concreta es un Job de una sola tarea en Cloud Run, región `southamerica-west1`, Direct VPC egress a la red/subred existentes y destino Cloud SQL privado usando el Go Connector con `CLOUD_SQL_PRIVATE_IP=true`; no se habilita IP pública. El service account recibe `roles/cloudsql.client` condicionado a esta instancia y `secretAccessor` solo sobre el secreto del observer. El Job monta la versión numérica fijada como archivo, ejecuta únicamente el preflight, conserva salida saneada y no expone endpoint. Cloud Run permite Direct VPC egress para Jobs; Cloud SQL requiere usar la IP privada y la misma VPC. [Direct VPC para Cloud Run Jobs](https://docs.cloud.google.com/run/docs/configuring/vpc-direct-vpc) y [conexión privada desde Cloud Run a Cloud SQL](https://docs.cloud.google.com/sql/docs/postgres/connect-run).

El resultado debe demostrar `max_connections`, `reserved_connections`, `superuser_reserved_connections`, client backends visibles, filas no clasificables y el límite inferior de slots libres. Antes de cualquier activación se compara esa cota con el presupuesto revisado de 24; si no alcanza o la salida no permite calcularla, se detiene el rollout. La query no mide conexiones futuras ni evita una segunda consulta tras cambiar cargas. La regresión PG18 mantiene otros logins conectados durante el preflight y confirma que la cota queda por debajo de 24 cuando la capacidad disponible no basta.

## Operación pendiente de decisión y revisión independiente

No ejecutar como parte de este PR: cambiar contraseña de `postgres`, crear usuarios, añadir versiones de Secret Manager, ejecutar SQL, publicar imagen, crear/ejecutar Jobs, modificar state, conceder IAM o iniciar worker/API. Antes de cualquier operación Cloud se necesita resolver y revisar la incompatibilidad de V60. Los secretos API/migrador inventariados no tienen versión activa observada y no se presume que exista una credencial de administrador válida.

Cuando el propietario revise y libere la operación concreta, el orden sería:

1. Releer metadata/state del candidato y verificar que instancia, base, VPC y subred coinciden con este corte. Determinar por canal seguro cómo obtener una credencial administrativa; Cloud SQL no crea password para su usuario `postgres` por defecto en una instancia nueva. No reutilizar credenciales de otros ensayos.
2. Revisar el candidato local de baseline y definir un procedimiento Cloud separado que preserve V1–V64/checksums/datos, roles NOLOGIN, grants y verificación de destino sin requerir el `COMMENT ON ROLE` directo.
3. En una autorización posterior, revalidar identidad/metadata/state, credenciales, SQL y orden de provisión; ninguno de los pasos heredados de una propuesta no aprobada se ejecuta automáticamente.
4. Worker Pools siguen Beta/Pre-GA y con costo pendiente; este corte no activa worker. El gate de API sigue cerrado mientras falten las demás familias de background.

## Pruebas y límites

El harness aislado `scripts/test-gcp04-w3-role-bootstrap-postgres.sh` crea roles desechables, prueba `SET ROLE`, aplica V1–V59, verifica el fallo esperado de V60 y corre el preflight con el observer. También comprueba el edge `ADMIN OPTION` en PostgreSQL 18. El fixture usa un contenedor efímero PostGIS, red `none`, sin puertos, base en tmpfs y socket Unix privado; retira solo su proyecto y volumen Compose propios.

La preparación local no constituye una ejecución SQL contra el candidato. El presupuesto de 24 sigue sin confirmar hasta el SELECT privado. La base persistente, sus secretos, recursos, estado y todas las pruebas/evidencias ajenas al run permanecen preservados.
