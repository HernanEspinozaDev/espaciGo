# GCP-03: plan de Cloud SQL y compatibilidad

Fecha: 2026-10-16. Base `main`: `48183d22d1f5d7f11c3b610a625ca392a8503c71` (merge #334). Desarrollo ejecutó el plan después de la revisión técnica y liberación en este chat, bajo la autorización previa del usuario; no se atribuye una aprobación puntual del binario al usuario. PR #336 se fusionó en `c087c13f6bde97ccd571c58cd702e906fd0212a5`. Proyecto `espacigo-testprod` (`447373903153`), región `southamerica-west1` (Santiago). Esta evidencia acredita el aprovisionamiento revisado, pero no la compatibilidad de restauración ni la aceptación de producto.

## Guardas y estado antes del corte

- El root `infra/terraform/environments/testprod/gcp` usa el backend GCS ya establecido. Se reutilizó el state GCP existente; no se creó ni migró backend, y no se reinició ni importó la base local `espacigo_pgdata`.
- El state Foundation observado antes del plan tenía serial 7, lineage conservado y 41 instancias. La planificación fue refresh-only respecto a recursos existentes: con `enable_foundation=true`, `enable_database=false`, `enable_services=false`, `enable_edge=false`, la comprobación posterior al ajuste Terraform devolvió `No changes` (código 0). No se recrea Foundation.
- El plan revisable cambia solo el gate de base de datos a `true`; servicios y edge siguen `false`; DNS no se toca. No se ejecutó `terraform apply`, ninguna sentencia SQL, importación o migración de datos, ni se creó recurso Cloud SQL. La capacidad/quota real y disponibilidad al momento de crear no quedan demostradas por el plan.
- Los binarios, JSON, variables, logs completos y snapshot state se guardan solo bajo `.local/planning/gcp/gcp-03-cloudsql-20261016/`, con directorio 0700 y archivos 0600. No incluir esos archivos en el PR.

## Cambios exactos que declara el plan

Configuración: PostgreSQL 18, edición Enterprise, `db-custom-N4-2-4096`, Santiago, Hyperdisk Balanced, 20 GiB iniciales, auto crecimiento con límite 100 GiB, Zonal, solo IP privada, backups automáticos, PITR y 7 backups retenidos. `deletion_protection=true` y `deletion_policy=PREVENT` son protecciones del proveedor/flujo Terraform; no se debe asumir que activan la protección del API Cloud SQL (verificación posterior abajo). La reserva de servicios privados se define con prefijo `/16`; GCP seleccionará el CIDR concreto al crearla.

| Dirección | Acción | Alcance |
| --- | --- | --- |
| `google_compute_global_address.private_services[0]` | crear | Rango reservado para Private Service Access |
| `google_service_networking_connection.private_services[0]` | crear | Peering de servicios privados sobre la VPC actual |
| `google_sql_database_instance.postgres[0]` | crear | Instancia vacía y privada; el parámetro de bloqueo Terraform no es la protección de borrado del API |
| `google_sql_database.application[0]` | crear | Base vacía `espacigo` |
| `google_secret_manager_secret.runtime["database-migrator-password"]` | crear | Solo metadatos del secreto; no incluye versión ni valor |
| `google_secret_manager_secret_iam_member.migrator_db_access[0]` | reemplazar | Revoca al SA migrador acceso al secreto de runtime `database-password` y se lo concede al secreto separado `database-migrator-password` |

Resumen de Terraform: **6 crear, 0 actualizar, 1 destruir**; la única acción de destrucción es el reemplazo del miembro IAM indicado. Terraform no declara usuarios SQL, contraseñas, versiones de Secret Manager, tablas, extensiones ni datos. Cloud SQL crea automáticamente su usuario de administración predeterminado `postgres` sin contraseña al crear la instancia. La API mantiene acceso al secreto runtime ya existente. El binario privado `gcp03-database.tfplan`, SHA-256 `81c7fc5cc1654123288234cf121666f7c520c08382b9130343af73d3953e230c`, fue revisado técnicamente y su aplicación se liberó en este chat bajo la autorización previa del usuario.

La división de secretos permite preparar cuentas SQL distintas: API/runtime lee `database-password`; el migrador lee `database-migrator-password`. Terraform agrega únicamente el contenedor vacío y modifica el grant del SA migrador. La creación segura de versiones y usuarios SQL queda fuera de este plan y requerirá procedimiento aprobado, fuera del state Terraform. No escribir valores en código, PR, consola, shell history, logs o state.

## Compatibilidad del producto y límites de verificación

- Cloud SQL documenta PostgreSQL 18 como versión disponible ([versiones](https://docs.cloud.google.com/sql/docs/postgres/db-versions)). La serie N4 usa formato `db-custom-N4-{CPU}-{memoria}` y admite 2 vCPU/4 GiB ([series de máquinas](https://docs.cloud.google.com/sql/docs/postgres/machine-series-overview)). La disponibilidad de N4 incluye Santiago ([disponibilidad regional](https://docs.cloud.google.com/sql/docs/postgres/region-availability-overview)). El proveedor Terraform 8.7.0 aceptó la configuración exacta en el plan. Estas fuentes y el plan validan la configuración declarada; no garantizan cuota, capacidad reservada o creación exitosa.
- Cloud SQL PG18 ofrece PostGIS 3.6.0, mientras el origen local observado ejecuta PostGIS 3.6.4 ([extensiones compatibles](https://docs.cloud.google.com/sql/docs/postgres/extensions)). Es una diferencia de versión explícita que exige prueba funcional; no cambiar la instancia local para forzar coincidencia.
- La VPC actual tiene la subred `10.80.0.0/24`. La consulta previa encontró cero rangos globales reservados y cero peerings en esa VPC. `/16` es solo el prefijo configurado, no un CIDR asignado. No se habilita IP pública para SQL.
- Hyperdisk Balanced admite el tamaño planificado y tiene rendimiento predeterminado de 4.000 IOPS y 170 MiB/s para el tamaño actual; la referencia documenta límites, baseline facturable y tarifas ([almacenamiento](https://docs.cloud.google.com/sql/docs/postgres/storage-options-overview)). No se midió I/O de la aplicación.
- Al inspeccionar la política IAM después de habilitar APIs en Foundation se observaron seis roles de agentes de servicio gestionados por Google (Artifact Registry, Compute, Compute Instance Group Manager, Container Registry, Pub/Sub y Cloud Run). No son parte de los 14 grants explícitos del plan Foundation. Se conservan, sin limpieza.

## Verificación posterior al apply exacto

El plan autorizado se aplicó el 2026-10-16; el log y los snapshots completos permanecen en `.local/planning/gcp/gcp-03-cloudsql-20261016/` con permisos privados. Resultado: **6 altas, 0 cambios, 1 destrucción**. El state remoto conservó lineage y avanzó serial 7→10; una verificación `terraform plan -refresh=true` con Database=true terminó código 0, sin drift.

Recursos observados: instancia `espacigo-testprod-pg` RUNNABLE en `southamerica-west1-b`, PostgreSQL 18, Enterprise N4 `db-custom-N4-2-4096`, Hyperdisk Balanced 20 GiB con auto crecimiento y límite 100 GiB, Zonal, sin IP pública. GCP asignó IP privada `10.189.0.3` y rango PSA `10.189.0.0/16`; este rango no se conocía antes del apply. Backup automático/PITR está activo con retención de logs de 7 días y 7 backups; la base `espacigo` fue creada vacía. Servicios Cloud Run: cero.

El secreto `espacigo-test-database-migrator-password` existe como metadata y tiene cero versiones. El SA migrador tiene acceso a ese secreto y ya no accede a `espacigo-test-database-password`; el SA de API conserva el acceso runtime. No se añadieron versiones ni valores de secretos. No se ejecutaron SQL, migraciones, conexiones, importación o restauración de datos.

Cloud SQL creó el usuario predeterminado `postgres` sin contraseña, una conducta documentada del servicio ([usuarios y roles PostgreSQL](https://docs.cloud.google.com/sql/docs/postgres/users)). No se crearon cuentas de aplicación ni se establecieron credenciales para `postgres`.

**Aclaración de borrado:** la lectura de la instancia después del apply reporta `settings.deletionProtectionEnabled=false`. El atributo `deletion_protection=true` del root Terraform solo bloquea borrados realizados por Terraform; el atributo `settings.deletion_protection_enabled` es el que protege también frente a API, CLI y consola ([referencia del proveedor](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/sql_database_instance)). No se activó ese atributo porque no estaba en el plan aprobado. Hasta que se revise/aplique un plan separado, existe protección Terraform `PREVENT`, pero no protección nativa de borrado Cloud SQL. No se intentó cambiarla silenciosamente.

## Estimación revisada de Cloud SQL

Tarifas brutas de lista oficiales de Santiago consultadas el 2026-10-16 ([precios Cloud SQL](https://cloud.google.com/sql/pricing), [tarifas Hyperdisk y rendimiento](https://docs.cloud.google.com/sql/docs/postgres/storage-options-overview)); USD, antes de impuestos y créditos Free Trial, sin descuentos o CUD. N4 Enterprise: $0.05782/vCPU-h y $0.0098/GiB-h de RAM. Hyperdisk a 20 GiB: capacidad $0.000220548/GiB-h; rendimiento incluido 4.000 IOPS/170 MiB/s frente a 3.000/140 de baseline facturable, así que el cálculo incorpora 1.000 IOPS y 30 MiB/s adicionales: $0.000038356/IOPS-h y $0.000153425/(MiB/s)-h.

| Configuración | SQL + disco, 24 h | 720 h / 30 días | 730 h |
| --- | ---: | ---: | ---: |
| 20 GiB, capacidad inicial | $4.85 | $145.59 | $147.61 |
| 100 GiB, límite de auto crecimiento | $5.28 | $158.29 | $160.49 |

Backups usados son variables: $0.000153425/GiB-h; 20 GiB promedio almacenados equivalen a $0.07/24 h o $2.21/720 h. Siete backups retenidos no equivalen a siete copias completas: los backups automáticos son incrementales ([FAQ de backups](https://docs.cloud.google.com/sql/docs/postgres/faq)). El PITR/WAL de siete días, uso real de almacenamiento, conexiones y otros SKU pueden variar. El costo del resto del entorno está separado en [cost-model.md](../../infra/terraform/cost-model.md); allí se actualizó el piso para reflejar IOPS/throughput por defecto.

## Procedimiento de compatibilidad y restauración para un corte futuro

Este procedimiento de compatibilidad/restauración es futuro y requiere autorización separada para operaciones SQL y ensayo de datos. El aprovisionamiento descrito arriba ya ocurrió; ningún paso de migración, conexión, dump o restore de abajo se ejecutó.

1. Preservar dumps y respaldos fuente originales de solo lectura con SHA-256, tamaño, timestamps e inventario. No editar ni reiniciar `espacigo_pgdata`; no borrar archivos fuente.
2. Mantener intactas las migraciones V1–V61 (checksum/byte a byte). Usar la base `espacigo`, creada vacía por GCP-03, como destino solo después de revisar un procedimiento SQL separado; probar la ruta de migración hasta V61 por un usuario migrador sin superusuario. No sustituir la historia de migraciones.
3. Inventariar versiones de servidor/extensiones desde el origen sin escritura. Probar por separado PostGIS 3.6.4 frente a Cloud SQL 3.6.0: funciones, tipos, operadores, índices espaciales, triggers, vistas y consultas de calendario/solapamiento. Comparar también `btree_gist`, `unaccent` y `fuzzystrmatch` con las extensiones/versiones compatibles de Cloud SQL. Si una función no es compatible, detener y documentar un plan de aplicación antes de cargar datos; no cambiar el origen.
4. Crear roles y credenciales distintas para migración y runtime con privilegio mínimo. Cloud SQL ya creó el usuario predeterminado `postgres`, sin password; no establecerle password ni usarlo como aplicación, runtime o migrador. `espacigo_gadm_bootstrap` de V60 es un rol local `NOLOGIN` restringido para bootstrap, no es el runtime ni el migrador. Revisar sus grants explícitamente en Cloud SQL; no tratarlo como superusuario administrado. El plan no creó cuentas de aplicación ni versiones secretas. Entregar valores por canal secreto aprobado fuera de Terraform y verificar que no queden en logs o estado.
5. Hacer dump consistente de solo lectura con `pg_dump -Fc --no-owner --no-acl`; registrar hash/tamaño y revisar `pg_restore -l`. Preparar una allowlist revisada para retirar solo entradas de TOC dependientes de ownership de extensiones que Cloud SQL gestione. Crear extensiones soportadas en versiones destino antes de restaurar sus objetos dependientes; nunca restaurar a ciegas ownership o ACL de extensiones.
6. Restaurar primero en base desechable aislada. Verificar conteos, secuencias, constraints, índices, funciones, extensiones y checksums de migraciones. Ejecutar consultas funcionales con geometrías y datos sintéticos; usar verificaciones de datos reales solo desde almacenamiento controlado, sin publicarlos en logs.
7. Probar Go mediante Cloud SQL Auth Connector por IP privada y TLS, con salida privada/PSA equivalente. No habilitar Cloud Run, balanceador, Armor, edge ni DNS durante este corte. Revisar el plan exacto y todos los gates antes de cualquier apply posterior.
8. Para rollback, preservar origen/dump inmutable. Si el scratch falla, retirar únicamente el scratch mediante un plan separado revisado y autorizado; nunca borrar ni modificar el origen o el volumen local.

## Evidencia de este corte y siguiente gate

Preparación realizada con Terraform 1.16.6 y provider Google 8.7.0. El plan de Foundation previo al cambio Database devolvió cero drift; el plan Database declara el conjunto de acciones de esta evidencia. No se ejecutaron SQL, apply, importaciones o cambios de datos locales. Servicios, edge y DNS permanecieron cerrados. El siguiente gate es revisar y aprobar el contenido del PR y el binario/hash del plan; una aprobación de PR no autoriza por sí sola `terraform apply`.
