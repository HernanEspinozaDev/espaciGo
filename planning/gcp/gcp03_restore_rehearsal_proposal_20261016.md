# GCP-03: propuesta de ensayo sintético de restore

**Estado: ensayo sintético local ejecutado; cualquier operación equivalente en GCP sigue siendo una propuesta sin autorización de provisión.** No se consultó, leyó, exportó ni usó la base persistente, su volumen, sus respaldos ni el dataset. El test usó solo un contenedor PostgreSQL 18/PostGIS desechable con red deshabilitada, almacenamiento temporal y fixtures deterministas nuevos. No se creó recurso Cloud, secreto ni usuario SQL cloud, ni se ejecutó un Job Cloud Run. Este ensayo prueba mecánica de dump/restore y objetos en PostgreSQL local; no restaura un snapshot real del origen ni completa GCP-03.

## Criterios GCP-03 que siguen abiertos

1. **Restore de snapshot de origen:** pendiente y requiere un snapshot aprobado y un plan separado. El resultado local no sustituye este gate.
2. **Cloud SQL real:** quedan por registrar versión `postgis_full_version()`, extensiones/roles/ACL en el target Cloud SQL y sus diferencias frente al PostgreSQL local. El test local no modela `cloudsqlsuperuser` ni política administrada.
3. **Integración y compatibilidad restante:** ruta privada/TLS de cliente, owners/ACL de todo el esquema, todas las funciones `SECURITY DEFINER` y triggers, secuencias/índices/consultas espaciales de la app, rollback y evidencias saneadas.
4. **Calendario con reglas de negocio:** la fixture cubre día DST de Santiago de 23 h, adyacencia, solape excluido y carrera concurrente en el constraint; queda verificar estados que bloquean disponibilidad mediante la integración Cloud SQL/booking y repetir el comportamiento en el target.
5. **Aceptación GCP-03:** integrar los resultados de snapshot y Cloud SQL en la matriz, revisar limitaciones y obtener aceptación humana. No se infiere por pasar el test local.

La revocación de credenciales, replay y cleanup Terraform stage 1/2 constan en [la evidencia del corte](evidencia_ejecucion_revoke_retired_membership_gcp03_20261016.md). Stage 2 ya se aplicó exactamente: Jobs/IAM temporales retirados, SQL/base/secret metadata/service accounts preservados. Las versiones secretas retiradas no se reutilizarán. Un ensayo Cloud futuro necesita credenciales efímeras nuevas y un procedimiento/plan independiente revisado.

## Fixture propuesto

El generador [scripts/gcp03-restore-fixture.py](../../scripts/gcp03-restore-fixture.py) produce SQL determinista con dos cuentas `.test`, un espacio y dos ocupaciones adyacentes (1.852 bytes, sin credenciales válidas). Genera manifiesto con hash del fixture, SHA-256 individual de V1–V61 y hash agregado del inventario. El test [scripts/test-gcp03-restore-rehearsal-postgres.sh](../../scripts/test-gcp03-restore-rehearsal-postgres.sh) lo pasa a `internal/gcp03scratch/TestSyntheticRestoreRehearsalPostgres18`.

El test exige URL Unix socket bajo el directorio efímero exacto del harness, `postgres` passwordless, `TEST_DATABASE_URL`, rutas `.local/testing/` y PostgreSQL 18; rehúsa TCP o rutas arbitrarias. Ejecuta V1–V61, siembra el fixture, crea un owner `NOLOGIN` único dentro del contenedor, vuelca con `pg_dump -Fc --no-owner --no-acl`, revisa TOC, restaura a otra base del contenedor y reaplica ownership explícito. Verifica checksums, conteos, PostGIS, constraint de exclusión, adyacencia, solape rechazado, dos inserciones concurrentes con un único ganador, día DST de 23 horas en `America/Santiago`, trigger V60 y owner/search_path de una función `SECURITY DEFINER`. No crea cuentas reales, pagos, sesiones ni secretos. El harness fija imagen con digest, `--network none`, tmpfs, socket en `/tmp`, y retira el contenedor al terminar.

**Resultado ejecutado el 2026-10-16:** `scripts/test-gcp03-restore-rehearsal-postgres.sh` pasó en PostgreSQL 18.6/PostGIS 3.6.4 (`POSTGIS="3.6.4 94d984b"`). Fixture SHA-256 `e8c47dba25bf7261cd3e6c8cfc75337549b5e7a1fe120fc5f630dc1ec4883876`; inventario V1–V61 `e4086e95c3ecf05f9262aabfaa4e536a72503b992cc806b4fcf610748c8fa8c4`; dump custom de 454.328 bytes SHA-256 `5f0f89a8dee552c4153e4f7ca24872854cf7c1f063338a43f02a050f23fe23e9`; hash TOC `8f835ec53ff9b0062d8b48ae774d457bc044d34b0f6f77f9ca2b493da8548085`. Las diez comprobaciones aparecen `true` en `restore-evidence.json`. También pasaron `TestLocalBookingTrialPostgresLifecycleAndConcurrentRetry`, `TestGADM04RestrictedBootstrapRoleMigrationFromV59` y `TestMPV212HourlyScheduleMigrationFromFreshAndV60` en harnesses PostgreSQL 18/PostGIS desechables. Manifiesto, SQL, dump, reporte y logs están bajo `.local/testing/gcp03-restore-rehearsal-20261016/`, directorio 0700, archivos 0600; los contenedores temporales se retiraron. Esta es evidencia local, no Cloud SQL.

## Topología aislada a considerar en un plan posterior

Comparación antes de cualquier plan cloud:

| Alternativa | Costo/aislamiento | Evaluación |
| --- | --- | --- |
| Base nueva con `run_id` en `espacigo-gcp03-pilot01-pg` | Menor costo incremental de SQL: evita otra instancia; añade almacenamiento/IO y carga a la instancia actual. La base aísla objetos de DB, pero `pg_roles`, roles/atributos/membresías y ownership de roles son del cluster; aplicar V60 puede reutilizar/comentar roles globales compartidos. También comparte CPU/IO. | No elegida para el ensayo Cloud: los checks de roles y ownership requieren un catálogo global limpio; reducir el costo acoplaría el ensayo al scratch actual.
| Instancia Cloud SQL temporal nueva y privada | Costo adicional: referencia del documento de infraestructura scratch de aproximadamente USD 4.86/24 h para N4 2 vCPU/4 GiB + Hyperdisk Balanced 20 GiB, antes de Job/red/logs y sin ser cotización. Ocho horas equivalen aritméticamente a ~USD 1.62 en ese componente. Catálogo de roles, DB y límites separados. | Recomendada como propuesta futura por aislamiento global; volver a cotizar SKUs/región y revisar policy/cuotas antes de plan. No está aprobada ni aprovisionada.

El resultado del contenedor local confirma que V60 y la reasignación de owner alteran el catálogo de roles global, aunque los objetos y grants de tabla vivan dentro de la DB. Por eso la preferencia de menor costo solo sería adecuada si la validación no cambia ni depende de esos roles compartidos; aquí sí los verifica. La instancia existente sigue preservada y sin cambios por este ensayo.

No se autoriza usar la instancia scratch actual para el restore Cloud. Un plan futuro deberá escoger un `run_id` irrepetible y raíz/state aislados, y crear únicamente:

- una instancia Cloud SQL PostgreSQL 18 temporal, privada, zonal, sin IP pública, con base vacía dedicada y límites de disco/auto crecimiento explícitos;
- un bucket regional privado de vida corta para el dump sintético comprimido, con uniform bucket-level access, cifrado gestionado, retención/expiración revisada y permisos de objeto limitados al job/importador;
- una identidad temporal con solo lectura del objeto y conectividad `cloudsql.client` condicionada a esa instancia; ninguna identidad de API/runtime, ningún acceso a secretos persistentes y ningún permiso sobre la base de origen;
- un ejecutor one-shot con imagen por digest, sin reintentos automáticos, timeout finito, red privada, logs sin payloads y operaciones separadas de crear destino, importar y verificar;
- una identidad de operador de alcance exacto y teardown de dos pasos con `run_id`, preservando el state y recibos hasta revisión.

El método de importación se decidirá en el plan operativo. Google Cloud documenta import desde Cloud Storage y recomienda verificar el resultado; `pg_dump` es necesario para conservar triggers/procedimientos de forma adecuada al caso, pues el comando de export SQL administrado no los incluye ([import/export Cloud SQL PostgreSQL](https://cloud.google.com/sql/docs/postgres/import-export/import-export-sql), [prácticas recomendadas](https://docs.cloud.google.com/sql/docs/postgres/import-export)). La lista TOC y objetos de extensiones administradas requieren revisión explícita. No dar permisos de `storage.objectAdmin` amplios al job si un set mínimo de `storage.objects.get` basta.

## Secuencia y salidas de aceptación propuestas

1. Revisar y autorizar expresamente instancia nueva, identidad, alcance, región, expiración, costos y plan exacto. No usar el origen persistente.
2. Regenerar fixture local, preservar SHA/manifest, generar dump custom, hash/tamaño y lista TOC allowlisted; confirmar V1–V61 byte a byte.
3. Revisar y aplicar un plan aislado de infraestructura con instancia nueva, acceso privado, protección nativa, identidad temporal y límites. Confirmar que no se permite acceso a la instancia/origen persistente.
4. Crear credenciales efímeras nuevas por canal privado aprobado, importar a la base sintética una vez y capturar ID/estado. No reintentar sobre un destino incierto.
5. Verificar por identidad read-only: checksums V1–V61, conteos, secuencias, FK, índices, extensiones/versiones, owners/ACL, funciones/triggers, geometrías, calendario/solapes y `postgis_full_version()`. Capturar resultados saneados, no filas o credenciales.
6. Ejecutar teardown con plan exacto separado revisado: retener state/evidencia hasta cierre; luego borrar únicamente recursos del `run_id`, secretos efímeros, dump y bucket según retención autorizada.

Aceptación del ensayo requiere coincidencia de hashes del artefacto de entrada, historial V1–V61, conteos del fixture, constraints/índices esperados, funciones/triggers/ACL y resultados espaciales/calendario reproducibles; operación limpia e idempotencia del proceso; no aparición de datos ajenos; ninguna conexión al origen; y un teardown que deje intactos recursos protegidos. Cualquier fallo mantiene GCP-03 parcial y bloquea importación de datos no sintéticos.

## Modelo de recursos y costo

Estimación de ingeniería, no cotización: [el modelo previo de la instancia scratch](gcp03_scratch_runner_infrastructure.md) calcula para Cloud SQL Enterprise `db-custom-N4-2-4096` con Hyperdisk Balanced de 20 GiB en Santiago un orden de **USD 4.86 por 24 h** (aprox. USD 0.20/h) antes de Cloud Run, logs, transferencia, backups y variación de consumo. Se usa solo como referencia preliminar; el próximo plan debe recalcular SKU/región/edición y cuotas con la [página oficial de precios de Cloud SQL](https://cloud.google.com/sql/pricing) y la calculadora oficial antes de revisión. Mantener la instancia viva solo durante una ventana reservada de hasta 8 h daría un orden aritmético de USD 1.62 en ese componente, no un total garantizado ni incluyendo tiempo de aprovisionamiento/teardown.

Partidas que debe cuantificar el plan exacto:

| Recurso/costo | Supuesto de ensayo | Cómo acotar |
| --- | --- | --- |
| Cloud SQL | 1 instancia zonal temporal; 2 vCPU/4 GiB, 20 GiB; hasta 8 h de vida activa | Cronometrar desde creación hasta borrado; estimar disco por tamaño provisionado, no solo bytes dump; verificar backup/PITR y escoger explícitamente costo frente a protección |
| Cloud Storage | 1 objeto comprimido <10 MiB, regional, vida <24 h | Coste de capacidad pequeño, pero contabilizar operaciones Class A/B, retención y borrado; comprobar ubicación y pricing vigentes |
| Cloud Run Job | Una tarea, hasta 2 vCPU/4 GiB, máximo 1 ejecución, timeout acotado a 1 h | Cotizar región y modalidad efectiva; cerrar concurrencia/reintentos; el límite superior usa timeout, no promedio observado |
| Red y logs | Una transferencia del dump dentro de la región; logs estructurados saneados | Evitar egress interregional/Internet; fijar retención y límite de bytes/logs; revisar cargos VPC/NAT si una ruta los requiere |
| Operación/contingencia | Aprovisionamiento, restore fallido, rollback y teardown | Presupuestar una sola repetición y máximo 2 × duración SQL; si se excede, pausar y actualizar estimación antes de continuar |

Fórmula para una primera cota: `horas_SQL × tarifa_hora_SQL + horas_job × tarifa_job + GiB-mes_bucket × tarifa_ubicación + operaciones + GiB_egress × tarifa_origen-destino + logs + backups + posibles cargos de red`. No se presupuestan importaciones paralelas, backups retenidos ni tráfico externo. Google explica que almacenamiento comprimido reduce capacidad/costo y que importaciones largas bloquean otras operaciones de Cloud SQL ([import/export best practices](https://docs.cloud.google.com/sql/docs/postgres/import-export)); el egress y Cloud Storage tienen tarifas por ubicación, clase y operaciones ([Cloud Storage pricing](https://cloud.google.com/storage/pricing), [VPC network pricing](https://cloud.google.com/vpc/network-pricing)). Los valores de los sitios de precios varían por SKU, ubicación y moneda; el PR no reemplaza una calculadora descargada/archivada para la configuración exacta.

## Gate y límites

La etapa Terraform stage 2 ya se aplicó con plan exacto revisado; no se solicita repetirla. Este documento no concede autorización de provisionar nueva Cloud SQL, crear credenciales cloud, ejecutar Jobs cloud, importar/restaurar datos o usar snapshot de origen. El ensayo Cloud requiere plan/costo/procedimiento revisados y liberación posterior. La base persistente, dataset, backup, archivos privados, credenciales y volumen local quedan fuera. GCP-04 conserva su alcance original de backend y ejecución durable en destino; no se marca iniciado/aceptado ni se activa despliegue/importación.
