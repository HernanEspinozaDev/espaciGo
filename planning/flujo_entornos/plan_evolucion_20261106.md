# Plan de evolución de desarrollo, migraciones y despliegue

Fecha: 2026-11-06. Estado: **propuesta para decisión; implementación y gasto no autorizados por este documento**. Base revisada: `771134d165046b290b1f5ad3a1074a6357c2327f`. Alcance de este corte: auditoría estática y plan. No se modificaron SQL, IAM, Terraform, configuración de despliegue ni datos. No se crearon develop/staging, PRs ni recursos cloud. No se integra este plan a main, cuyo push puede disparar un rollout.

## Objetivo y recomendación

Publicar backend y evolución de esquema como una única operación coordinada y auditable, conservando los datos de Cloud SQL y eliminando la dependencia habitual de bastión/portátil. Mantener SQL versionado y migrador Go. Incorporar contrato de release, pruebas SQL obligatorias, staging aislado, recuperación ensayada y Cloud Run Jobs dentro de VPC.

Recomendación inicial: ramas de tarea → main de integración → staging → promoción aprobada a producción. Staging y producción son destinos de despliegue; no necesitan ramas permanentes del mismo nombre. La alternativa con develop se evalúa abajo. Main dejaría de significar «desplegar inmediatamente a usuarios»; exige cambiar primero el trigger actual, en un corte operativo separado.

## Flujo actual confirmado por repositorio

```text
Rama de tarea → revisión/pruebas locales → integración autorizada a main
                                                │
               trigger main → Cloud Build → imágenes → Cloud Run
               modo por defecto: solo web; full con evidencia SQL previa

SQL versionado → cmd/dbmigrate local → túnel SSH/bastión → Cloud SQL
                       └─ plan, checksums, lock y transacción por archivo
```

Instancia/base histórica vigente: `espacigo-testprod-db` / `espacigo_dms_stage_20261020`, proyecto `espacigo-testprod`, región `southamerica-west1`. El sufijo stage de la base y la demo privada no acreditan staging separado. Evidencia de V83, backups y transporte cerrado: runbook canónico; no revalidada online en este corte. Detalle con archivos/líneas y respuestas a la conversación: [auditoría](auditoria_actual_20261106.md).

## Lo que significa versionar y desplegar PostgreSQL

El cambio se escribe en un nuevo `db/migrations/VNNNNNN__descripcion.sql`, se prueba con la aplicación y viaja en Git. El pipeline ejecuta las migraciones pendientes del artefacto aprobado contra la base destino. No «sube la base local» ni copia sus usuarios, reservas o documentos.

Modificar PostgreSQL manualmente en local no genera una migración automáticamente. Un diff de esquema puede ayudar a preparar SQL, pero necesita revisión. Los seeds sintéticos se mantienen aparte de las migraciones de estructura/datos imprescindibles de producto; nunca se ejecutan seeds de ensayo en producción. V1–V83 y cualquier archivo ya aplicado se conservan byte a byte. No se agrega V84 hasta que un cambio concreto lo justifique y se verifique la numeración vigente.

## Decisión sobre ramas y entornos

| Modelo | Cuándo aporta valor | Coste operativo / encaje |
| --- | --- | --- |
| Tarea → main; promoción de artefacto staging→prod | Equipo pequeño/agentes, integración frecuente y despliegue desacoplado | Recomendado. Mantiene main como única permanente y el flujo directo vigente; requiere cambiar el disparo de producción |
| Tarea → develop → main; staging sobre candidato congelado | Necesidad real de integrar trabajo que aún no entra a main | Válido si el usuario lo elige. Cambia AGENTS; duplica reconciliaciones/hotfixes y exige definir qué SHA se prueba y promociona |
| Tarea → develop → release temporal → main | Versiones agrupadas y QA prolongado | Mayor coordinación; release es temporal, no otra permanente. Sin necesidad demostrada para este proyecto |

Crear ramas no crea servidores, bases, aislamiento ni pruebas. Las ramas Git no tienen un precio GCP por existir; los entornos y ejecuciones sí. Un fallo puede ocurrir en cualquiera de estos modelos. No se garantiza cero caída por el número de ramas.

Para el modelo recomendado, integrar serialmente y volver a probar el candidato combinado: dos ramas verdes por separado pueden fallar juntas. Mientras se mantenga la política directa vigente, la calidad se ejecuta en pushes de ramas de tarea y antes de integrar, sin introducir PRs como gate. Cualquier futura política de PR requiere decisión separada.

Para develop, si se decide adoptarlo: CI en cada tarea y develop; fijar un candidato por SHA desde develop; QA en staging; integrar a main sin mezclar cambios nuevos; comprobar identidad del árbol probado. Si el merge cambia el contenido, construir/probar otro candidato. Las correcciones de release/main vuelven a develop por merge normal. El despliegue consume un manifest/digests aprobados, nunca «lo último de develop» al momento de ejecutar.

## Entornos propuestos

| Entorno | Aplicación y DB | Datos/servicios externos |
| --- | --- | --- |
| Local/CI | PostgreSQL 18 y extensiones requeridas en recursos efímeros exclusivos; API/frontend del candidato | Sintéticos recreables, correo capturado, proveedores sandbox o simulados identificados |
| Staging | Cloud Run API/web necesarios y Cloud SQL independiente; Job de migración e identidades propios | Datos sintéticos; buckets, secretos y credenciales sandbox separados; acceso restringido |
| Producción | Servicios y Cloud SQL propios, backup/PITR y observabilidad conforme a objetivos aprobados | Datos y proveedores reales solo bajo alcance autorizado |

Preferir proyectos GCP separados para staging/producción por IAM, cuotas y radio de impacto; si el costo administrativo lo impide, recursos/identidades/secretos separados dentro de un proyecto como transición documentada. No conectar staging a la base de producción ni copiar sus datos por defecto.

Una segunda base en la misma instancia puede ahorrar cómputo, pero comparte roles de clúster, capacidad, mantenimiento y fallos: no equivale a aislamiento completo. El historial de roles/owners del repo hace especialmente importante este punto. Una réplica de lectura no sirve como destino de pruebas DDL.

Determinar primero si el testprod actual seguirá siendo preproducción o tiene obligaciones de producción. Reutilizarlo como staging solo tras confirmar sus dependencias/datos; no renombrar, borrar ni migrar contenido por inferencia. Si todavía no existe producción aceptada, no duplicar recursos solo por el nombre del flujo.

## Secuencia del release objetivo

```text
Commit fijo + SQL → calidad + ensayo SQL → imágenes API/web/migrador por digest
                                           │
                            manifest inmutable del candidato
                                           ↓
Staging: lock → preflight/backup → Job SQL → postcheck/compatibilidad
         → revisiones sin tráfico → pruebas autorizadas → promoción/observación
                                           ↓
                         aprobación del mismo manifest
                                           ↓
Prod: lock → preflight/backup → Job SQL → postcheck/compatibilidad
      → revisiones sin tráfico → pruebas autorizadas → tráfico → observación
```

1. Calidad backend/frontend y tests PostgreSQL realmente ejecutados para el SHA candidato; el orquestador espera sus resultados. Conservar validaciones de política, provenance y separación del modo web-only. Un cambio solo documental no debe desplegar aplicación; un cambio solo web no necesita aplicar DDL.
2. Construir una vez API, web y migrador actuales. Manifest con SHA completo, digests, inventario SQL con nombres/checksums, identidad del builder/provenance y contrato de compatibilidad. No usar `latest` ni permitir que un tag mutable cambie la selección. Configuración por ambiente fuera de la imagen cuando sea viable; comprobar variables frontend que pudieran estar fijadas durante build antes de prometer promoción idéntica.
3. Adquirir lock de release por ambiente antes de modificar Job/esquema/tráfico y conservarlo hasta cierre. El advisory lock DB sigue protegiendo al migrador; no sustituye el lock de release. Rechazar candidato obsoleto y verificar que el Job no pueda ser reemplazado por otra ejecución concurrente.
4. Preflight READ ONLY: destino completo, login/SET ROLE, ledger/checksums, objetos/ACL requeridos, versión/extensiones, disponibilidad de backup y recuperación. Volver a validar al aplicar; el plan sin lock no congela la base. Aprobar exactamente candidato, ambiente y delta.
5. Ejecutar Job de una tarea/paralelismo 1, imagen por digest y destino fijado. Cloud Build solicita ejecución, espera finalización y comprueba resultado/identidad de la ejecución; un envío aceptado no es éxito. Inicialmente reintentos automáticos 0; reintento supervisado tras readback del commit/ledger. No ejecutar modos F5/Cloud Owner en el Job ordinario.
6. Postcheck cero pendientes para el inventario exacto, checksums, objetos/ACL y pruebas del contrato de la API anterior/nueva. Si falla, no publicar revisión nueva. La base podría haber avanzado parcialmente: no asumir rollback de todo el lote.
7. Desplegar revisiones sin tráfico y verificar configuración/digest/contrato, con consumidores de fondo deshabilitados durante preflight cuando corresponda. Validación funcional cloud reservada al usuario bajo las reglas actuales; la automatización de esos recorridos necesita un alcance posterior explícito. No afirmar un E2E completo por readiness.
8. Promover los digests ya probados, comprobar tráfico y observar errores/latencia/conexiones. No hay transacción atómica entre SQL y tres servicios: asegurar compatibilidad de versiones durante la transición y orden API/web definido por contrato.
9. Guardar manifest, IDs de ejecución/revisiones, evidencia SQL, backup, resultado y límites. Liberar lock solo después de readback; un fallo conserva estado verificable y procedimiento de desbloqueo, nunca borrar un lock a ciegas.

Cloud Run Jobs admite este patrón y Direct VPC egress evita mantener una VM o un conector VPC dedicado para el ejecutor. El conector Cloud SQL ofrece autenticación/TLS, pero necesita además la ruta privada. [Patrón de migraciones](https://cloud.google.com/blog/topics/developers-practitioners/running-database-migrations-cloud-run-jobs), [Direct VPC para Jobs](https://docs.cloud.google.com/run/docs/configuring/vpc-direct-vpc), [esperar ejecución](https://docs.cloud.google.com/run/docs/execute/jobs).

## Contrato API–esquema y límites del migrador

Cada release declara migraciones/objetos requeridos y checksums, cambios incompatibles conocidos y revisiones anteriores habilitadas para rollback. No basta comparar `max(version)` ni exigir igualdad universal entre versión de aplicación y de DB. Una API anterior puede seguir funcionando sobre un esquema ampliado; una posterior no debe entrar si faltan requisitos. El verificador de despliegue consulta con identidad de inspección acotada; no ampliar permisos del runtime al catálogo/ledger por comodidad.

Usar expandir → migrar datos por lotes → retirar dependencias → contraer en otra versión. Ejemplo: agregar columna nueva, soportar convivencia y backfill verificable, cambiar lecturas/escrituras, retirar columna vieja tras confirmar ausencia de consumidores. Cada contracción reduce explícitamente el conjunto de revisiones que pueden volver a producción.

El runner actual usa una transacción por archivo. `CREATE INDEX CONCURRENTLY` no cabe en ese contrato; si se necesita, diseñar y probar una fase no transaccional explícita con recuperación/readback, sin saltar guards ni simular atomicidad. [Restricción PostgreSQL](https://www.postgresql.org/docs/current/sql-createindex.html). Backfills grandes se ejecutan con checkpoints/límites, fuera del DDL transaccional largo cuando corresponda. Definir `lock_timeout`, `statement_timeout`, cancelación y alertas según operación.

Separar tres versiones: motor PostgreSQL (p. ej. 18.6), ledger de esquema (V83) y release de aplicación (SHA/digest). Cambiar una no cambia automáticamente las otras.

## Identidades y propiedad operativa

- Cloud Build orquesta y verifica artefactos/resultados; no recibe contraseña SQL ni DDL de runtime.
- Provisionador autorizado crea/configura Job/identidades/red con plan revisado; ejecutor habitual solo lanza el Job fijado y lee resultado. Separar permiso de ejecutar del de actualizar imagen/args/secretos o ejecutar con overrides. Quien puede reemplazar el Job tiene autoridad efectiva sobre DDL y se trata como tal.
- Job usa cuenta de servicio exclusiva con Cloud SQL Client, acceso solo a la versión de su secreto y logs; en PostgreSQL login migrador → schema owner mínimo del delta. IAM GCP no concede ALTER TABLE en PostgreSQL. Revisar alcance efectivo donde el proveedor no permita restringir un permiso al recurso exacto.
- Inspector read-only separado cuando el preflight lo requiera; API permanece en runtime. Credenciales montadas en archivo, no en URL/argumentos/logs. IAM DB Auth se evalúa después: no está cableada en la CLI ordinaria aunque exista soporte de conector en otras rutas.
- Conservar Cloud Build como dueño de revisiones/tráfico y Terraform como dueño de foundation según contrato actual. El aprovisionamiento nuevo de staging/Jobs necesita un dueño único de configuración/state; no importar o recrear el SQL actual ni habilitar inputs Terraform obsoletos.

La cuenta deployer actual carece de permisos Jobs por diseño. Cualquier cambio exige revisar también `validate_cloud_build_transition.py`, pruebas de política y runbook canónico. El plan no concede permisos ni sustituye hoy la ruta bastión.

## Backup, rollback y recuperación

Antes de automatizar escrituras: verificar backup automático, retención/PITR, cifrado y autoridad de restauración; fijar objetivos humanos de pérdida máxima de datos (RPO) y tiempo de recuperación (RTO). Como propuesta inicial a decidir: backup diario con retención de 7 días y PITR de 7 días si la edición/configuración lo permite; no es el estado observado ni un SLA comprometido.

Cada release exige evidencia de backup recuperable con antigüedad admitida; cambios de riesgo generan backup previo y esperan éxito. Ensayar restore en instancia aislada, medir tiempos y validar contenido funcional sintético y ledger. Comprobar soporte/posible reinicio antes de habilitar PITR. [PITR PostgreSQL](https://docs.cloud.google.com/sql/docs/postgres/backup-recovery/pitr).

| Incidente | Respuesta prevista |
| --- | --- |
| SQL falla antes de terminar | Detener release, readback de ledger/objetos; corregir con nueva migración o reanudar desde último commit confirmado |
| API nueva falla con esquema aditivo | Volver a digest/revisión anterior explícitamente compatible; no deshacer SQL automáticamente |
| Cambio destructivo/datos dañados | Contener escrituras, evaluar corrección hacia adelante o restauración/PITR aislada y conmutación autorizada; registrar escrituras posteriores que podrían perderse |
| Job/red/identidad falla | Distinguir causa; conservar estado y evidencia, sin cambiar destino o elevar runtime |

Eliminar el bastión requiere además una vía ensayada de diagnóstico y recuperación bajo demanda dentro de VPC, con acceso temporal/auditado y sin VM fija. No convertir el Job ordinario en shell administrativo genérico.

## Costos y alternativas

Sí: staging aislado suele aumentar el gasto. Cloud SQL encendido cobra capacidad aunque no reciba consultas; Cloud Run puede acercarse a costo de cómputo cero en reposo con mínimo 0 y facturación por solicitudes. Con mínimo activo o facturación por instancia cambian los cargos. Jobs cobra durante ejecución, con mínimo de un minuto por instancia. [Cloud SQL](https://cloud.google.com/sql/pricing), [Cloud Run](https://cloud.google.com/run/pricing), [modalidades de facturación](https://docs.cloud.google.com/run/docs/configuring/billing-settings).

La configuración actual declara API mínimo 1 y `--no-cpu-throttling`: copiarla a staging preservaría costo continuo. Antes de poner mínimo 0, inventariar goroutines/consumidores y requisitos de CPU; los workers durables pueden necesitar otro modelo y no desaparecen por bajar el mínimo del API. Detener SQL suspende cómputo pero conserva cargos de almacenamiento/IP aplicables; revisar backup/PITR por separado. [Detener Cloud SQL](https://docs.cloud.google.com/sql/docs/postgres/start-stop-restart-instance).

| Opción | Costo relativo | Qué demuestra / límites |
| --- | --- | --- |
| PostgreSQL efímero local/CI + destino cloud actual | Menor costo adicional permanente | SQL/API local; no valida aislamiento ni red/IAM cloud antes del destino actual |
| Staging cloud bajo demanda, SQL independiente | Intermedio, con operación de encendido/disponibilidad | Valida integración cloud; almacenamiento/backup persisten y hay espera de arranque |
| Staging cloud permanente, SQL independiente reducido | Mayor piso mensual | QA disponible; misma versión/extensiones/políticas críticas, sin exigir idéntica capacidad o HA para toda prueba |
| Segunda DB en SQL actual | Menor capacidad adicional directa | Compromiso temporal de aislamiento; no recomendado si el destino contiene producción real |

Modelo de estimación regional a completar con SKUs de `southamerica-west1` y consumo real:

```text
Costo adicional mensual = SQL(horas encendido × tarifa de cómputo)
                        + disco + backup/WAL/PITR
                        + Cloud Run solicitudes/CPU/memoria + Jobs
                        + builds + imágenes + secretos + logs
                        + red/egreso + edge/DNS que realmente se añada
                        - costos del bastión realmente retirados
```

Para dimensionar horas: 730 h/mes permanente frente a ejemplo de 8×22 = 176 h/mes de cómputo programado; almacenamiento/backup no se reducen en la misma proporción. Son escenarios de uso, no precios. No se ofrece monto USD/CLP sin tier/edición/HA/disco/retención/tráfico comprobados. Los valores Terraform históricos no describen necesariamente la instancia SQL externa actual. Crear presupuesto/alertas no equivale a un tope automático de gasto. Cotizar antes de aprovisionar; medir ahorro del bastión con discos/IP/snapshots reales, no con una VM supuesta siempre encendida.

## Fases y criterios de cierre

| Fase | Entregable | Gate para cerrar | Estado |
| --- | --- | --- | --- |
| FLUJO-00 | Auditoría estática + este plan, preguntas resueltas desde código/texto | Fuentes/limitaciones y decisiones pendientes registradas | Documentado en este corte; no aceptación humana |
| FLUJO-01 | Inventario cloud read-only y ADR de ramas/entornos/costos/RPO-RTO | Clasificar testprod, verificar SQL/backup/PITR/red/IAM/trigger/Billing y elegir arquitectura | Pendiente |
| FLUJO-02 | Calidad SQL obligatoria y contrato/manifest de release | Base vacía + actualización de versión destino; checksums, fallos/reanudación, concurrencia, permisos y API anterior/nueva; prueba falla si DB se omitió | Pendiente |
| FLUJO-03 | Imagen migrador completa + Job/configuración probados localmente | Inventario de imagen idéntico a Git; política niega destinos/overrides/identidades inválidos; sin activar cloud | Pendiente |
| FLUJO-04 | Staging y recuperación aislados, con presupuesto aprobado | Job real, red/IAM/secretos, restore medido, QA humano y evidencia del SHA; datos sintéticos | Pendiente, operación cloud requiere autorización |
| FLUJO-05 | Orquestación Cloud Build con separación staging/prod y promoción | Pruebas locales del pipeline; candidato validado; cambio controlado del trigger main antes de cualquier push que altere alcance | Pendiente |
| FLUJO-06 | Primer release integrado por Job al destino aprobado | Backup válido, SQL/postcheck/compatibilidad, revisión sin tráfico, validación humana, promoción y observación | Pendiente, operación autorizada concreta |
| FLUJO-07 | Retirada definitiva del bastión y cierre documental | Inventario de dependencias DMS/otros usuarios vacío, alternativa de recuperación ensayada, eliminación exacta aprobada y ahorro medido | Pendiente |

Cada fase usa una rama de tarea temporal desde main y publica allí el trabajo revisado, sin PR ni force-push. Preservar worktrees/demos/volúmenes activos. Actualizar plan/matriz con SHA, comandos realmente ejecutados, resultados y límites; integración/despliegue solo conforme a autorización de ese corte. La aceptación del producto corresponde al usuario.

## Decisiones pendientes del usuario

1. Elegir modelo de ramas: recomendación main + promoción de artefacto, alternativa develop si necesita integración separada. Este plan no cambia AGENTS.
2. Clasificar testprod y seleccionar staging nuevo o reutilización segura, con SQL independiente como opción preferida.
3. Fijar presupuesto y disponibilidad de staging: bajo demanda o permanente; aprobar recursos concretos tras cotización.
4. Fijar RPO/RTO/retención y quién autoriza producción y recorridos cloud. Mantener los límites humanos actuales hasta ampliación expresa.

No es necesario resolver esas preferencias para completar la auditoría estática ni las propuestas de este documento. No se presupone autorización para gasto, cambios cloud, retirada de VM ni despliegue automático.
