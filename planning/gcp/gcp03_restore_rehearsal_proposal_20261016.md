# GCP-03: propuesta de ensayo sintético de restore

**Estado: propuesta local, no aprobada para provisión ni ejecución.** No se consultó, leyó, exportó ni usó la base persistente, su volumen, sus respaldos ni el dataset. No se creó infraestructura, dump, secreto, usuario SQL ni ejecución Cloud Run. Este ensayo permitiría probar la mecánica de dump/restore con datos nuevos y sintéticos; no sustituye un restore verificable de snapshot del origen ni completa GCP-03.

## Criterios GCP-03 que siguen abiertos

1. Restore verificable: repetir dump y restore en destino aislado, registrar checksums y comparar resultados. El ensayo sintético de este documento prueba el procedimiento y la compatibilidad de objetos, pero no prueba el snapshot real del origen.
2. PostGIS y compatibilidad: obtener `postgis_full_version()` real en el destino; probar tipos, operadores, geometrías, consultas espaciales e índice GIST. Confirmar también `btree_gist`, `unaccent` y toda extensión usada por V1–V61.
3. Funciones, triggers y ownership: inventariar funciones, `SECURITY DEFINER`, `search_path`, ACL `EXECUTE`, triggers, owners, secuencias e índices y compararlos con la expectativa saneada.
4. Roles y runtime: demostrar permisos mínimos, ausencia de superusuario, membresías y ACL requeridas después del restore; confirmar que API/runtime no obtiene permisos de migrador/owner.
5. Calendario y solapes: casos de límites inclusivos/exclusivos, huso horario, DST donde corresponda, estados que bloquean disponibilidad, intervalos adyacentes y conflictos concurrentes; verificar tanto consultas como constraints/índices.
6. Integraciones y compatibilidad restantes: conexión TLS por ruta privada, reintento/rollback controlado, artefactos de evidencia, límites documentados por matriz y limpieza aislada. No se prueba aquí la aplicación desplegada ni el dataset completo.

La revocación de credenciales y el replay de roles constan en la evidencia enlazada desde la matriz. El cleanup Terraform stage 1 ya se aplicó; el plan stage 2 de 25 retiros sigue pendiente de revisión y no se aplicó. No conservar ni renovar credenciales retiradas para ejecutar esta propuesta. Un futuro ensayo debe recibir credenciales efímeras nuevas mediante un procedimiento independiente revisado.

## Fixture propuesto

Generar desde cero y localmente un fixture pequeño (objetivo inicial: menos de 10 MiB comprimidos) con UUIDs sintéticos deterministas y datos claramente ficticios. Aplicar V1–V61 a una base desechable PostgreSQL 18 local; insertar únicamente filas mínimas que satisfagan relaciones y constraints para:

- una cuenta propietaria, una cuenta de prueba y dos espacios sintéticos en categorías/unidades distintas;
- una disponibilidad normal y otra que cruza medianoche/huso configurado;
- intervalos adyacentes sin solape, un solape rechazado y estados de reserva que deben o no bloquear calendario;
- filas relacionadas que ejerciten FK, secuencias, índices, funciones y triggers existentes;
- una geometría fija y consultas espaciales válidas e inválidas controladas.

No crear pagos, correos, contraseñas reales, sesiones recuperables ni material que represente a una persona. El generador debe fallar si intenta abrir una conexión fuera del PostgreSQL local desechable. No buscar ni reutilizar el generador/dump del dataset persistente. Guardar el fixture y sus credenciales únicamente bajo `.local/testing/` con directorio `0700` y archivos `0600`; revisar `pg_restore -l`, lista allowlisted y SHA-256 antes de transferirlo.

## Topología aislada a considerar en un plan posterior

No se solicita usar la instancia scratch actual para este restore. Un plan futuro deberá escoger un `run_id` irrepetible y raíz/state aislados, y crear únicamente:

- una instancia Cloud SQL PostgreSQL 18 temporal, privada, zonal, sin IP pública, con base vacía dedicada y límites de disco/auto crecimiento explícitos;
- un bucket regional privado de vida corta para el dump sintético comprimido, con uniform bucket-level access, cifrado gestionado, retención/expiración revisada y permisos de objeto limitados al job/importador;
- una identidad temporal con solo lectura del objeto y conectividad `cloudsql.client` condicionada a esa instancia; ninguna identidad de API/runtime, ningún acceso a secretos persistentes y ningún permiso sobre la base de origen;
- un ejecutor one-shot con imagen por digest, sin reintentos automáticos, timeout finito, red privada, logs sin payloads y operaciones separadas de crear destino, importar y verificar;
- una identidad de operador de alcance exacto y teardown de dos pasos con `run_id`, preservando el state y recibos hasta revisión.

El método de importación se decidirá en el plan operativo. Google Cloud documenta import desde Cloud Storage y recomienda verificar el resultado; `pg_dump` es necesario para conservar triggers/procedimientos de forma adecuada al caso, pues el comando de export SQL administrado no los incluye ([import/export Cloud SQL PostgreSQL](https://cloud.google.com/sql/docs/postgres/import-export/import-export-sql), [prácticas recomendadas](https://docs.cloud.google.com/sql/docs/postgres/import-export)). La lista TOC y objetos de extensiones administradas requieren revisión explícita. No dar permisos de `storage.objectAdmin` amplios al job si un set mínimo de `storage.objects.get` basta.

## Secuencia y salidas de aceptación propuestas

1. Aprobar fixture, direcciones Terraform, identidades, región, expiración, límites y presupuesto antes de cualquier aprovisionamiento.
2. Generar schema/fixture local sintético, dump consistente `pg_dump -Fc --no-owner --no-acl`, SHA, tamaño y manifiesto TOC allowlisted. Registrar checksums de V1–V61 sin editarlas.
3. Aplicar un plan de infraestructura exacto y revisado solo a la raíz/run ID del ensayo. Comprobar proyecto/región/state, IP privada, protecciones, IAM efectiva, ausencia de acceso al origen y límites de costo.
4. Importar una vez a la base scratch vacía; capturar ID/estado de operación. Un reintento requiere destino limpio creado expresamente, nunca repetir ciegamente sobre estado incierto.
5. Ejecutar verificación read-only: historial/checksums V1–V61, conteos por tabla/fixture, secuencias, FK, índices, extensiones y `postgis_full_version()`, owners/ACL, funciones/triggers, geometrías, consultas calendario y casos de solape. Capturar outputs saneados y hashes, nunca filas personales o credenciales.
6. Demostrar rollback por descarte del destino sintético, sin tocar el origen. Destruir recursos efímeros solo mediante plan exacto de teardown revisado; conservar state/logs saneados hasta cierre, y luego borrar credenciales/dump según retención aprobada.

Aceptación del ensayo requiere coincidencia de hashes del artefacto de entrada, historial V1–V61, conteos del fixture, constraints/índices esperados, funciones/triggers/ACL y resultados espaciales/calendario reproducibles; operación limpia e idempotencia del proceso; no aparición de datos ajenos; ninguna conexión al origen; y un teardown que deje intactos recursos protegidos. Cualquier fallo mantiene GCP-03 parcial y bloquea importación de datos no sintéticos.

## Modelo de recursos y costo

Estimación de ingeniería, no cotización: el documento de compatibilidad GCP-03 calcula para Cloud SQL Enterprise `db-custom-N4-2-4096` con Hyperdisk Balanced de 20 GiB en Santiago un orden de **USD 4.86 por 24 h** (aprox. USD 0.20/h) antes de Cloud Run, logs, transferencia, backups y variación de consumo. Se usa solo como referencia preliminar; recalcular SKUs/región/edición y cuotas con la calculadora oficial antes del plan. Mantener la instancia viva solo durante una ventana reservada de hasta 8 h daría un orden aritmético de USD 1.62 en ese componente, no un total garantizado ni incluyendo tiempo de aprovisionamiento/teardown.

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

Este documento no concede autorización de provisionar, ejecutar SQL, crear credenciales, ejecutar Job, restaurar o aplicar stage 2. La siguiente revisión humana deberá aprobar plan/costo y procedimiento concretos antes del ensayo. La base persistente, dataset, dump, archivos privados, credenciales y volumen local quedan fuera de esta propuesta. GCP-04 continúa con su alcance original: backend y ejecución durable en destino; no se reduce a readiness local ni se activa despliegue/importación.
