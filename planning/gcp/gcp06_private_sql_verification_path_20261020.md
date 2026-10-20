# GCP-06: preparación de verificación SQL privada

Fecha de inspección: 2026-10-20. Este registro separa las observaciones de control plane, los datos comunicados por revisión y lo que todavía no se ha probado. No se abrió una sesión SQL, no se creó un Cloud Run Job, no se cambiaron IAM/Secret Manager, flags, usuarios, contraseñas, red ni recursos Cloud.

## Importación reportada

PR #457 fue aprobado por `HernanEspinozaDev` y fusionado por esa misma cuenta. HEAD revisado: `41354e57c965ae66ced9aad3c44fd2cbdb0c5978`; merge SHA: `716fb073c344d939a24b39f170ff6408aaeab2d2`; check `candidate` SUCCESS, run `39405422064`.

El revisor reportó la instancia `espacigo-gcp06-20261020-gcp06a-pg`, PostgreSQL 18, estado `RUNNABLE`, sin IP pública; base `espacigo_gcp06_candidate_20261020-gcp06a`; importación `DONE`, operación `adc28794-0335-4b4c-9245-15370000004b`. El dump importado corresponde al objeto `gs://espacigo-testprod-gcp06-20261020-gcp06a/gcp06/20261020-gcp06a/data.sql`, generación `1792490496223841`, 16.620.453 bytes, SHA-256 `bb35a8709ae74d51e73b69d0dd75d7b4b761822508353dab9dddcd631cec9c11`. Los bindings y roles IAM temporales de carga/importación fueron retirados, según revisión. No se repitió carga ni importación en este seguimiento.

Importación `DONE` solo acredita el resultado de la operación reportada. El revisor aún no pudo consultar por SQL el ledger, conteos, extensiones, ACL, ownership o acceso runtime por falta de una credencial DB disponible en Cloud SQL Studio. La aplicación no está conectada a esta base candidata.

## Ruta de red observada (solo lectura)

- Cloud SQL responde por la IP privada `10.189.0.13`, adjunta a `projects/espacigo-testprod/global/networks/espacigo-testprod`; `ipv4Enabled=false`, SSL `ENCRYPTED_ONLY`, tier `db-custom-1-3840`.
- La red `espacigo-testprod` tiene la conexión de Private Services Access `servicenetworking-googleapis-com`, rango reservado `10.189.0.0/16`; la IP de Cloud SQL cae dentro de ese rango.
- Cloud Run `espacigo-test-api` usa Direct VPC egress mediante `espacigo-testprod-api-serverless` (`10.81.0.0/26`) con `private-ranges-only`. Los servicios `espacigo-test-public` y `espacigo-test-private` usan `espacigo-testprod-serverless` (`10.80.0.0/24`), también `private-ranges-only`. En el metadata observado los tres muestran ingress `all`; el ingreso existente de la web privada conserva su capa de IAP. No se alteró ingress.
- La consulta local `ip route get 10.189.0.13` devuelve ruta por gateway LAN `192.168.1.1` desde `192.168.1.9`, no por la VPC del proyecto. El revisor había reportado TCP 5432 sin respuesta desde este host. Por eso la máquina local no es un cliente de la red privada. La configuración de Cloud Run indica una ruta de egress a rangos privados dentro de la VPC, pero no demuestra por sí sola una conexión exitosa a este candidato.
- Cloud SQL Admin API lista únicamente el usuario integrado `postgres`; no revela una contraseña. Google documenta que Cloud SQL crea ese usuario al crear una instancia, pero no establece su contraseña por defecto; la autenticación con ese usuario requiere fijarla. La descripción de esta instancia no muestra `cloudsql.iam_authentication` habilitado. No se consultaron payloads de Secret Manager ni contraseñas/hash de usuarios.

Estas lecturas comprueban configuración y metadatos solamente; no hicieron una conexión TCP desde Cloud Run ni una conexión SQL. Google documenta que Cloud Run puede conectar por IP privada cuando egress usa Direct VPC o VPC Connector hacia la misma red. Los servicios actuales usan su configuración y credenciales de runtime para la instancia compartida `espacigo-testprod-pg/espacigo`; no se cambió su destino. Fuentes: [Conectar Cloud Run a Cloud SQL PostgreSQL](https://docs.cloud.google.com/sql/docs/postgres/connect-run) y [configuración de IP privada de Cloud SQL](https://docs.cloud.google.com/sql/docs/postgres/configure-private-ip).

## Bloqueo y opción mínima

El camino de red privado ya está configurado para cargas Cloud Run dentro de la misma VPC. El bloqueo actual es de autenticación SQL, no una razón para habilitar IP pública, Data API, NAT o cambiar ingress. Cloud SQL Studio solicita usuario/contraseña y no hay una credencial de base disponible para la verificación.

La opción con menos cambios de infraestructura es abrir Cloud SQL Studio para la instancia privada existente y ejecutar solo consultas `SELECT` con una credencial DB ya autorizada, sin cambiar la contraseña de `postgres`. Esta ruta usa el acceso administrado de Cloud SQL y no requiere Cloud Run Job, VPC/IAM/Secret Manager changes ni IP pública. La credencial debe provenir del propietario por un canal seguro fuera de este chat; no se ha supuesto ni recuperado.

Para reducir privilegio más adelante, se puede revisar separadamente un usuario observador temporal con permisos limitados a `public.schema_migrations`, metadatos de extensiones y un verificador de conteos agregados sin devolver filas. Crear ese principal y sus grants requiere un actor DB administrador; no hay hoy credencial disponible para bootstrap. No crear usuario ni grants hasta aprobar el SQL exacto y su revocación. Si se elige IAM DB authentication, hará falta revisar expresamente el flag de instancia y los permisos IAM; ambos quedan fuera de la autorización actual. No crear el Job que ejecute psql en este gate.

Una vez disponible una autenticación aprobada, la secuencia de lectura será:

1. Comprobar `public.schema_migrations`: 64 filas distintas y contiguas, versión mínima 1 y máxima 64.
2. Obtener conteos exactos por tabla y contrastarlos con el manifiesto sanitizado local, sin exportar filas ni identificadores.
3. Obtener `extname`/`extversion` de `pg_catalog.pg_extension` y comparar con el contrato del destino PostgreSQL 18/PostGIS.
4. Registrar solo resultados agregados, instancia/base, usuario de lectura, hora UTC y limitaciones. No conectar la aplicación.

No hay camino ejecutable ahora que respete simultáneamente la falta de credenciales y la prohibición de crear/resetear usuarios o habilitar IAM DB auth. Siguiente gate humano: disponer por canal seguro de una credencial DB ya válida, o aprobar por separado un diseño de observador y su bootstrap. No cambiar Secret Manager, IAM, flags, ingress, password de `postgres`, recursos ni ejecución Cloud Run antes de ese gate y revisión de su plan exacto.
