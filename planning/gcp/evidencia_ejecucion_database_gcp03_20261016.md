# Evidencia de ejecución GCP-03 Database

Fecha: 2026-10-16 (UTC). Proyecto `espacigo-testprod` (`447373903153`), región `southamerica-west1`. PR de preparación #336, HEAD revisado `908801dcafbc022adcdea2c08217a5126eb2487f`, merge `c087c13f6bde97ccd571c58cd702e906fd0212a5`. Desarrollo ejecutó el plan después de la revisión técnica y liberación de este chat, bajo la autorización previa del usuario; no se atribuye una aprobación puntual del binario al usuario. Esta evidencia registra la ejecución exacta; no acredita compatibilidad de datos, restauración, aceptación de producto ni autorización para servicios/edge.

## Plan exacto y verificaciones antes de aplicar

- Archivo de plan privado: `.local/planning/gcp/gcp-03-cloudsql-20261016/gcp03-database.tfplan`, SHA-256 `81c7fc5cc1654123288234cf121666f7c520c08382b9130343af73d3953e230c`. Permisos: directorio 0700, archivos 0600. No se versionan planes, state, tfvars, identidad, respuestas completas ni logs.
- El plan se revisó técnicamente y su aplicación se liberó en este chat bajo la autorización previa del usuario; desarrollo ejecutó exclusivamente ese binario.
- Terraform 1.16.6 y Google provider 8.7.0. Identidad ADC verificada contra la cuenta activa de `gcloud`, con permiso `roles/owner` en el proyecto consultado explícitamente. El `core/project` predeterminado de gcloud era otro; todos los comandos operativos y variables Terraform especificaron `espacigo-testprod`/`447373903153` de forma explícita.
- State remoto GCP antes: serial 7, lineage `95bafa42-5d1a-bef2-909b-7ce4b6a92096`, 41 instancias. El plan Foundation=true/Database=false/Services=false/Edge=false, refrescado justo antes del apply, terminó código 0 (`No changes`).
- Preflight de nombres: cero Cloud SQL, cero rangos globales y cero peerings antes del corte; una subred existente `10.80.0.0/24`; el secreto nuevo no existía; el grant del SA migrador apuntaba al secreto runtime.
- Políticas efectivas leídas desde el proyecto: `gcp.resourceLocations` y `serviceuser.services` con `allowAll`; `sql.restrictPublicIp` y `sql.restrictAuthorizedNetworks` no estaban enforceras. No había policies explícitas en el nivel proyecto.
- Cuotas/capacidad: Cloud SQL mostraba cero instancias antes. La instancia creada quedó en `NEW_NETWORK_ARCHITECTURE`; los límites publicados son hasta 1.000 instancias por proyecto en arquitectura nueva (100 en antigua), así que el estado posterior de una instancia queda bajo ambos topes ([cuotas y límites](https://docs.cloud.google.com/sql/docs/postgres/quotas)). Antes había 0/8 direcciones globales estáticas y 0/25 peerings de VPC; el corte consume una dirección/rango y un peering. La cuota consultada del SQL Admin API informa límites de solicitudes; no ofrece una reserva de capacidad de hardware. La creación exitosa confirma capacidad disponible en ese momento, no reserva futura.
- La facturación ya estaba habilitada antes del corte. El usuario confirmó Free Trial y que no se habilitara Paid mode; no se modificó la cuenta de facturación ni se cambió de modo. Saldo y vencimiento de créditos no se verificaron.

El plan autorizado declaraba **6 crear, 0 cambiar, 1 destruir**. El único destroy reemplazaba el grant IAM del SA migrador, quitándole el acceso a `database-password` de runtime y otorgándoselo al nuevo contenedor vacío `database-migrator-password`. Las otras cinco altas eran: rango de peering, conexión Private Service Access, instancia Cloud SQL, base `espacigo` y metadata del secreto migrador. No había usuarios SQL Terraform, valores/versiones secretas, servicios, edge, DNS o datos en el plan.

## Resultado del apply exacto

El apply del binario guardado terminó código 0: **6 añadidos, 0 cambiados, 1 destruido**. No se replanificó ni se alteró el plan. El state remoto conservó lineage y avanzó serial `7 → 10`, con 46 instancias Terraform. El refresh plan posterior con Database=true terminó código 0 y `No changes`.

| Recurso verificado | Resultado observado |
| --- | --- |
| Instancia `espacigo-testprod-pg` | RUNNABLE en `southamerica-west1-b`; PostgreSQL 18, Enterprise, `db-custom-N4-2-4096`; SQL network architecture `NEW_NETWORK_ARCHITECTURE` |
| Disco y disponibilidad | Hyperdisk Balanced, 20 GiB, auto crecimiento activo con cap 100 GiB; 4.000 IOPS/170 MiB/s predeterminados; Zonal |
| Red | IP privada `10.189.0.3`, `ipv4Enabled=false`, ninguna IP pública; rango PSA asignado `10.189.0.0/16`; peering `servicenetworking-googleapis-com` |
| Backups | Automáticos activos, PITR activo, 7 días de logs transaccionales, 7 backups retenidos |
| Bases | Existe `espacigo` (`UTF8`, `en_US.UTF8`) además de la base de sistema `postgres`; no se ejecutó SQL ni se importaron tablas/filas de aplicación |
| Usuarios SQL | Cloud SQL creó automáticamente el usuario admin predeterminado `postgres`, sin contraseña configurada. No se crearon usuarios de aplicación ni se asignó password. El servicio crea esta cuenta por defecto ([usuarios y roles PostgreSQL](https://docs.cloud.google.com/sql/docs/postgres/users)). |
| Secretos | Metadata de `database-migrator-password` creada; versiones en los cuatro secretos runtime: cero. El migrador solo accede al secreto separado; no conserva acceso a `database-password`; el API conserva su grant runtime. No se crearon valores secretos. |
| Servicios | Cloud Run: cero servicios. No se cambió DNS, edge ni V1–V61; no se conectó ningún cliente. |

Los snapshots completos del recurso y los controles IAM se conservan bajo `.local/planning/gcp/gcp-03-cloudsql-20261016/` con permisos privados. Los archivos `apply-exact-plan.log`, `state-preapply.json`, `state-postapply.json` y `postapply-no-drift.log` permanecen allí; no publicar su contenido.

## Aclaración de protección de borrado

El API devuelve `settings.deletionProtectionEnabled=false`, aunque el plan aplicó `deletion_protection=true` y `deletion_policy=PREVENT`. En el provider, `deletion_protection` solo bloquea borrados ejecutados por Terraform; para bloquear API, gcloud y consola se necesita `settings.deletion_protection_enabled=true` ([referencia `google_sql_database_instance`](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/sql_database_instance)). No se cambió ese valor fuera del plan autorizado. El estado actual tiene guarda de borrado Terraform, pero no la protección nativa Cloud SQL; cualquier ajuste requiere un plan separado revisado y autorización explícita.

## Límites y siguiente paso

La base Cloud SQL permanece vacía salvo la cuenta y base predeterminadas del servicio, y la base creada `espacigo` sin datos de aplicación. No se ejecutó ninguna sentencia SQL, migración V1–V61, dump, restore, import, conexión, ni bootstrap de roles. No se tocó `espacigo_pgdata`, sus respaldos o ficheros locales. La compatibilidad de PostGIS (origen 3.6.4, Cloud SQL 3.6.0), roles, extensiones y restauración queda para el procedimiento controlado ya descrito en `gcp03_cloudsql_compatibility.md`; no es evidencia de éxito de restauración.

La instancia quedó funcionando y puede acumular costo mientras siga activa. Según la estimación de lista de Santiago, SQL+disco/performance predeterminada es aproximadamente USD 4.85 por 24 horas o USD 145.59 por 720 horas, antes de almacenamiento usado de backups y demás SKU ([cost model](../../infra/terraform/cost-model.md)). No se habilitó Paid mode; el monto/tiempo Free Trial disponible no se midió.
