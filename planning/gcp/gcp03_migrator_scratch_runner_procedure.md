# GCP-03: procedimiento propuesto para migrador y scratch privado

Este documento es preparación técnica, no un despliegue. No hay Cloud Run Job, bucket de staging, usuario SQL migrador, versión de secreto ni conexión activa creados por este procedimiento. Cualquier ejecución posterior requiere revisión y autorización para el recurso y los datos destino concretos.

## Conexión y ejecución futura

Los controles aplicados a `espacigo-testprod-pg` requieren Cloud SQL Auth Proxy o un Cloud SQL Language Connector y TLS. El conector autentica/cifra el túnel, pero no crea una ruta de red; el cliente necesita conectividad VPC privada hacia la instancia. La API ya usa `cloud.google.com/go/cloudsqlconn` con `WithPrivateIP()` en `cmd/api/database_pool.go`. En cambio, `cmd/dbmigrate` y `internal/migrator.Run` usan `DATABASE_URL` y `pgx.Connect` directo. Antes de migrar, desarrollar y revisar integración del conector/DialFunc en el migrador, o documentar un Proxy administrado en un runner privado. No relajar `connector_enforcement` para acomodar conexiones directas.

Propuesta para un futuro Cloud Run Job de una sola ejecución: región `southamerica-west1`, 1 tarea, paralelismo 1, reintentos 0, límite 4 horas, 2 vCPU y 4 GiB; Direct VPC egress con `PRIVATE_RANGES_ONLY`, VPC `espacigo-testprod` y subnet `espacigo-testprod-serverless` (`10.80.0.0/24`), sin IP pública ni ingress. Usaría la cuenta de servicio migradora existente `espacigo-test-migrator` con `roles/cloudsql.client`. La cuenta tiene acceso al contenedor `database-migrator-password`, pero no existe versión del secreto. Confirmar todo el IAM efectivo antes de usarlo.

Para un dump de ensayo, crear solo tras autorización un bucket GCS regional temporal con acceso uniforme, prevención de acceso público, retención/limpieza explícita y `roles/storage.objectViewer` limitado a la cuenta migradora. No reutilizar el bucket privado de archivos de la aplicación. Evitar guardar credenciales o valores secretos en Terraform, argumentos, logs o state.

## Roles y migraciones

Separar el login de migración de `espacigo_runtime`. Cloud SQL PostgreSQL no ofrece `SUPERUSER` real; la documentación del proveedor indica que `cloudsqlsuperuser` permite capacidades elevadas (incluye `CREATEROLE`, `CREATEDB`, `LOGIN`) y que ciertas extensiones requieren un usuario miembro de ese rol. La ruta propuesta es un login temporal y dedicado, con membresía elevada solo si la matriz de extensiones y las migraciones V1–V61 lo requieren; retirar/deshabilitar ese acceso al terminar. Verificar primero si el conjunto cabe con privilegios más estrechos en un PostgreSQL 18 descartable. No asignar `cloudsqlsuperuser` al runtime.

El rol `espacigo_runtime` debe ser un rol restringido, sin `SUPERUSER`, `CREATEDB` ni `CREATEROLE`, con grants explícitos equivalentes a `internal/dbbootstrap.GrantRuntimePermissions` luego de migrar. `scripts/postgres/init-runtime-user.sh` sirve como referencia local, no demuestra por sí solo compatibilidad con roles administrados de Cloud SQL. V60 crea `espacigo_gadm_bootstrap` como rol restringido `NOLOGIN`; la cuenta migradora necesita validar su capacidad de crear ese rol. No crear logins GADM ni activar GADM en Cloud SQL.

## Pruebas de compatibilidad en scratch

Mantener dos ensayos separados:

1. Base vacía PostgreSQL 18: aplicar V1–V61 sin cambios, verificar checksums, extensiones, ownership, tablas, índices, restricciones, triggers, funciones, secuencias y consultas espaciales.
2. Restauración: medir el dump de origen con una lectura aprobada, guardar tamaño/hash e inventario inmutable, ejecutar `pg_restore -l`, diseñar una allowlist TOC revisada para excluir ownership/ACL incompatibles, precrear extensiones soportadas y restaurar únicamente en la base descartable. Comparar conteos, secuencias, constraints, índices, checksums y consultas de negocio.

No borrar entradas TOC en bloque ni editar migraciones para hacer pasar el ensayo. El tamaño real del dump aún no está medido en este alcance; por tanto, almacenamiento, backups y transferencia siguen siendo variables. No restaurar ni importar en la base persistente sin autorización específica posterior.

## Costos orientativos

El Cloud Run Job se factura por tiempo de instancia y tiene un mínimo de un minuto. La tabla oficial de Jobs incluye Santiago (`southamerica-west1`) y publica `$0.000018/vCPU-s` y `$0.000002/GiB-s`; 2 vCPU/4 GiB durante el límite de cuatro horas equivale a `(2 × $0.000018 + 4 × $0.000002) × 14.400 s = $0.6336`, antes de aplicar cuota gratuita agregada de la cuenta. La lista de tarifas de Jobs es distinta de la tabla de servicios Tier 2; no se aplicó el recargo de servicios.

Como referencia separada, un scratch Cloud SQL con la misma configuración Santiago N4 2 vCPU/4 GiB y Hyperdisk Balanced 20 GiB del modelo actual cuesta aproximadamente `$4.86/24 h` o `$145.59/720 h`, antes de backups usados, WAL, tráfico y almacenamiento variable. Un día de scratch más el límite de cuatro horas del Job sería aproximadamente `$5.49`, excluyendo GCS, operaciones, logs y transferencia. No es una cotización ni aprobación de gasto. Direct VPC egress no requiere una instancia de Serverless VPC Access; la transferencia de red se cobra según tráfico aplicable.

## Referencias oficiales

- [Cloud SQL Auth Connectors para PostgreSQL](https://docs.cloud.google.com/sql/docs/postgres/connect-connectors)
- [Cloud Run Direct VPC egress](https://docs.cloud.google.com/run/docs/configuring/vpc-direct-vpc)
- [Cloud SQL: usuarios y roles](https://docs.cloud.google.com/sql/docs/postgres/users)
- [Extensiones PostgreSQL en Cloud SQL](https://docs.cloud.google.com/sql/docs/postgres/extensions)
- [Precios de Cloud Run](https://cloud.google.com/run/pricing)
- [Precios de Cloud SQL](https://cloud.google.com/sql/pricing)
- [Precios de red de VPC](https://cloud.google.com/vpc/network-pricing)
