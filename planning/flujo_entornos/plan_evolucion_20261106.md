# Plan de evolución de desarrollo, migraciones y despliegue

Fecha: 2026-11-06; revisión 2 tras la conversación aportada por el usuario. Estado: **propuesta para decisión; implementación y gasto no autorizados por este documento**. Base revisada: `771134d165046b290b1f5ad3a1074a6357c2327f`. Alcance de este corte: auditoría estática y plan. No se modificaron SQL, IAM, Terraform, configuración de despliegue ni datos. No se crearon develop/staging, PRs ni recursos cloud. No se integra este plan a main, cuyo push puede disparar un rollout.

## Objetivo y recomendación

Publicar backend y evolución de esquema como una única operación coordinada y auditable, conservando los datos de Cloud SQL y eliminando la dependencia habitual de bastión/portátil. Mantener SQL versionado y migrador Go. Incorporar contrato de release, pruebas SQL obligatorias, staging aislado bajo demanda con cierre automático, recuperación ensayada y Cloud Run Jobs dentro de VPC.

Recomendación inicial: ramas de tarea → main de integración → staging → promoción aprobada a producción. Staging y producción son destinos de despliegue; no necesitan ramas permanentes del mismo nombre. La alternativa con develop se evalúa abajo. Main dejaría de significar «desplegar inmediatamente a usuarios»; exige cambiar primero el trigger actual, en un corte operativo separado.

## Preferencia de arquitectura registrada en esta revisión

El usuario solicita mejorar el plan y expresa preferencia por staging bajo demanda con apagado automático después de las pruebas. Se adopta **B + C como diseño propuesto preferido**:

- **C: PostgreSQL efímero en CI** para cambios de backend/SQL y comprobaciones de integración afectadas. Se crea durante el job de pruebas y se elimina al terminar; no espera a que se active staging cloud. Los pushes puramente documentales no necesitan DB ni staging; la selección por paths debe incluir dependencias compartidas y fallar de forma conservadora si el impacto es ambiguo.
- **B: una instancia Cloud SQL staging independiente, retenida y normalmente detenida**, que se inicia solo para una sesión cloud solicitada sobre un candidato fijo. Al cerrar pruebas se detiene automáticamente; conserva almacenamiento y datos sintéticos, no sesiones de cómputo permanentes.
- **Job Go por release** para migrar sin túnel SSH, separado del controlador que enciende/apaga staging. El mismo release coordina SQL y aplicación en pasos verificables, sin prometer atomicidad entre servicios.

Esta preferencia orienta el plan; no aprueba tier, precio, creación/encendido de recursos, Scheduler, IAM ni automatización de recorridos cloud. El costo real y la viabilidad se validan en FLUJO-01 antes de aprovisionar. No se repite la auditoría estática completa ni se sustituyen sus límites por las afirmaciones de la conversación externa.

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
| Staging bajo demanda | Cloud Run API/web necesarios, mínimo 0 cuando sea compatible; Cloud SQL independiente detenido fuera de sesión; Job migrador y controlador de ciclo de vida separados | Datos sintéticos versionados, secretos/buckets sandbox exclusivos y acceso cerrado fuera de sesión |
| Producción | Servicios y Cloud SQL propios, backup/PITR y observabilidad conforme a objetivos aprobados | Datos y proveedores reales solo bajo alcance autorizado |

Preferir proyectos GCP separados para staging/producción por IAM, cuotas y radio de impacto; si el costo administrativo lo impide, recursos/identidades/secretos separados dentro de un proyecto como transición documentada. No conectar staging a la base de producción ni copiar sus datos por defecto.

Una segunda base en la misma instancia puede ahorrar cómputo, pero comparte roles de clúster, capacidad, mantenimiento y fallos: no equivale a aislamiento completo. El historial de roles/owners del repo hace especialmente importante este punto. Una réplica de lectura no sirve como destino de pruebas DDL.

Determinar primero si el testprod actual seguirá siendo preproducción o tiene obligaciones de producción. Reutilizarlo como staging solo tras confirmar sus dependencias/datos; no renombrar, borrar ni migrar contenido por inferencia. Si todavía no existe producción aceptada, no duplicar recursos solo por el nombre del flujo.

## Cuándo se ejecuta cada comprobación

| Evento | PostgreSQL efímero | Staging cloud | Producción |
| --- | --- | --- | --- |
| Push documental | No, si el clasificador acredita que no afecta código/política | Sigue detenido | Sin despliegue |
| Push backend, SQL o contrato compartido | Obligatorio: migraciones y pruebas afectadas con extensiones fijadas | Sigue detenido | Sin despliegue por este evento |
| Push solo frontend | Calidad frontend; DB si afecta BFF/contrato o lo exige la regresión | Sigue detenido | Conserva el flujo vigente hasta cambiar el trigger autorizado |
| Solicitud de integración cloud / candidato de release | Debe estar verde para el SHA exacto | Inicia sesión temporal y ejecuta validación autorizada | No se promociona todavía |
| Solicitud de recuperación | Base sintética si hace falta preparar fixtures | Sesión acotada; restore a destino aislado adicional si corresponde | No altera el destino productivo |
| Fin de sesión, éxito/fallo/cancelación/caducidad | Recursos del job se retiran | Cierre y verificación de SQL detenido; app cerrada al acceso/consumo | Permanece intacta |
| Promoción aprobada del candidato validado | Reutiliza evidencia vigente | Puede continuar detenido | Preflight, backup, SQL, postcheck y aplicación del manifest aprobado |

Para promociones a producción, validar el candidato en cloud conforme a una política de vigencia definida en FLUJO-01. Un nuevo digest, SQL, configuración relevante, IAM, proveedor o cambio incompatible de la base invalida la evidencia afectada; reabrir una sesión de staging. No omitir staging de un release por llamarlo «pequeño». Tampoco encenderlo por cada push. Mantener el mismo candidato entre pruebas y promoción, aunque staging ya esté apagado.

## Ciclo de vida de staging y apagado automático

### Estado persistente y exclusión de sesiones

```text
DETENIDO → RESERVADO → ARRANCANDO → PREPARANDO → PRUEBAS / QA HUMANO
    ↑                                                   │
    └── CERRADO Y VERIFICADO ← CERRANDO ← éxito/fallo/cancelación/TTL
                                      ↘ si no cierra: ALERTA Y REINTENTO
```

El estado de sesión vive fuera del runner y de Cloud SQL, en metadata durable privada con actualización condicional. Registrar ambiente y recursos exactos, `session_id`, SHA/digests, versión de dataset/baseline, propietario, build/job IDs, generación de reserva, `started_at`, `expires_at`, último estado y resultado. No guardar credenciales ni datos de negocio. Un único propietario por staging; el guard de costo comparte esta coordinación. No usar solo una etiqueta de instancia o un PID como lock.

Valores iniciales **propuestos para cotizar/probar**: TTL total de sesión de 2 horas desde la reserva, vigilante cada 5 minutos, cierre con plazo de 10 minutos y reintentos limitados con alerta. Los timeouts internos de arranque, migración, pruebas y restore deben dejar margen dentro del TTL. Una ejecución larga se divide o solicita una ventana especial antes de empezar. Un heartbeat acredita actividad, pero no prorroga indefinidamente el TTL. La extensión necesita operador autorizado, nuevo vencimiento acotado y motivo auditado; nunca la inferencia «parece que todavía trabaja».

### Apertura y pruebas

1. Obtener reserva exclusiva y comprobar allowlist exacta de proyecto/instancia/servicios staging, sin depender solo de que el nombre incluya stage. Rechazar producción y sesión ajena. Verificar antes de arrancar que el controlador de cierre y el vigilante estén habilitados y saludables; si no, no encender.
2. El controlador solicita `activationPolicy=ALWAYS` solo al SQL de staging, espera la operación y confirma disponibilidad SQL con identidad acotada. Tener estado RUNNABLE no sustituye la conexión/preflight. El API no arranca consumidores antes de completar esquema y configuración.
3. Comprobar baseline, ledger/ACL, semilla sintética y configuración del candidato. Si hay contaminación o esquema adelantado incompatible, no aplicar migraciones antiguas ni manipular ledger: reconstrucción controlada de la base sintética exclusiva mediante receta aprobada, o nueva sesión desde baseline compatible.
4. Ejecutar migración, postcheck, revisión y pruebas permitidas del mismo manifest. Abrir acceso de QA solo durante la reserva vigente, con autenticación y destinos sandbox. Guardar resultados fuera de la instancia antes del cierre.
5. Distinguir tests técnicos terminados de validación humana pendiente. Bajo las reglas actuales el recorrido funcional cloud lo hace el usuario. Una sesión que espera QA conserva vencimiento visible; al vencer se cierra y queda «QA incompleto», nunca aprobado automáticamente. El cierre de QA humano se comunica mediante acción explícita autenticada de «finalizar sesión», ligada a session_id, candidato y resultado; terminar un job técnico no sustituye esa señal. Reabrirla requiere otra sesión y comprobar vigencia del candidato.

### Dos capas de cierre, sin depender del build exitoso

**Capa 1 — finalizador del controlador de sesión.** Intentar cierre en éxito, error, cancelación y timeout. Un último step ordinario de Cloud Build no ofrece por sí solo esa garantía: los pasos dependen del éxito de sus prerrequisitos y una cancelación puede impedir que se ejecute cualquier limpieza. Diseñar el controlador durable y su manejo de error/finalización antes de elegir YAML; un `trap` del proceso es una ayuda, no la única protección. [Orden de pasos Cloud Build](https://docs.cloud.google.com/build/docs/configuring-builds/configure-build-step-order).

**Capa 2 — vigilante independiente del runner/build.** Propuesta de Cloud Scheduler autenticado → controlador serverless de ciclo de vida que inspecciona reservas vencidas y reconcilia únicamente recursos staging allowlisted. Su identidad no recibe credenciales SQL productivas ni autoridad de detener producción. Scheduler puede entregar más de una vez: cierre idempotente, con control de generación y compare-and-set, sin efectos de una invocación duplicada. Comprobar el alcance real de `cloudsql.instances.update`: no asumir que IAM permite limitarlo únicamente al campo de encendido o a una instancia. Si no se puede aislar de producción, usar proyecto staging separado y autoridad allí; una allowlist de código no reduce por sí sola un grant amplio. Recursos, frecuencia, permisos y costo requieren aprobación operativa; no se crean por actualizar este plan. [Semántica de Scheduler](https://docs.cloud.google.com/scheduler/docs/overview).

Secuencia común de cierre:

1. Marcar CERRANDO bajo la misma reserva/generación y bloquear nuevas aperturas. Un finalizador antiguo nunca puede detener una sesión nueva; no liberar reserva hasta finalizar o transferir explícitamente al reconciliador.
2. Cerrar ingreso de QA y consumidores/eventos periódicos staging. Cancelar únicamente Jobs/ejecuciones propios aún activos y esperar su término con plazo. Evitar nuevas peticiones/health checks externos que provoquen activaciones repetidas del API.
3. Preservar reporte, resultado parcial y evidencia de operaciones SQL. Solicitar `activationPolicy=NEVER`, esperar la operación y leer de nuevo política/estado hasta comprobar que no sirve conexiones. Una respuesta HTTP aceptada no acredita instancia detenida.
4. Confirmar que mínimo de servicios/revisiones es 0 cuando proceda, que no hay workers activos ni tareas/programaciones generando carga. **Mínimo 0 permite escalar a cero, no apaga ni deshabilita Cloud Run**: el acceso cerrado y productores suspendidos evitan reactivaciones fuera de sesión. Si un worker no admite pausa o mínimo 0, diseñar su retiro exacto bajo demanda antes de declarar el entorno cerrado. No borrar servicios/datos por un patrón de nombres.
5. Registrar cierre, tiempo facturable observado y recursos persistentes; liberar reserva mediante su generación exacta. Si falla el apagado, marcar CIERRE FALLIDO, alertar y continuar reconciliación; no presentar como éxito de costo ni habilitar producción con cierre sin resolver.

Al vencer el TTL, el vigilante revoca la sesión y solicita cancelación; tras el plazo de cierre procede al stop del SQL sintético propio aunque la tarea no termine, dejando resultado interrumpido para readback al reabrir. No ejecutar este corte forzado en producción ni en recursos ajenos. La elección protege el presupuesto de staging y acepta perder un ensayo incompleto, no sus pruebas registradas. Antes de implementar hay que ensayar cancelación de migración/COMMIT incierto y recuperación.

No hay garantía absoluta ante indisponibilidad del proveedor o IAM roto: se requiere alerta por cierre no confirmado, por retraso del vigilante y por instancia encendida sin reserva vigente. El objetivo medible es detectar y reconciliar dentro de los plazos probados; no prometer «siempre apagado» por un bloque `finally`.

Cloud SQL detenido no se activa al recibir conexiones: necesita inicio explícito. Verificar también cómo las ventanas detenido afectan el calendario de backups y la ventana recuperable PITR de staging; no atribuirle continuidad de recuperación sin comprobarla. El controlador no toca el SQL actual hasta que se haya clasificado y autorizado el destino de staging. [Inicio y detención](https://docs.cloud.google.com/sql/docs/postgres/start-stop-restart-instance), [mínimos Cloud Run](https://docs.cloud.google.com/run/docs/configuring/min-instances).

## Reproducibilidad con instancia retenida

Conservar instancia no significa aceptar cualquier estado que dejaron ensayos previos. Cada sesión fija motor/extensiones, inventario SQL, bootstrap de roles/owners aprobado, receta de baseline y dataset sintético con checksum/versión. Guardar también configuración cloud y servicios sandbox usados.

CI cubre siempre los recorridos obligatorios desde base vacía y actualización de baseline. Staging cloud cubre la infraestructura gestionada y el delta que se pretende promocionar. Una base staging ya adelantada no demuestra por sí sola que el upgrade desde la versión productiva funciona; preparar baseline sintético equivalente antes del ensayo de upgrade. Los roles son de clúster: reconstruir una base no reinicia roles, grants ni configuración de instancia; verificarlos por separado y no crear identidades compartidas entre sesiones.

La receta solo puede limpiar una base sintética exclusiva identificada y autorizada, con reserva y preflight; no copiar producción ni ejecutar `DROP` genérico, reset de instancia o limpieza de otros worktrees. Tras un fallo, preservar evidencia mínima fuera del SQL, poner el ensayo en cuarentena lógica y detener cómputo; investigar en una nueva ventana. Separar ciclos de baseline/fixtures de las migraciones productivas.

## Secuencia del release objetivo

```text
Commit fijo + SQL → calidad + ensayo SQL → imágenes API/web/migrador por digest
                                           │
                            manifest inmutable del candidato
                                           ↓
Staging: reserva/TTL → encender → baseline/preflight → Job SQL → postcheck
         → revisión/pruebas autorizadas → evidencia → cierre automático verificado
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
9. Guardar manifest, IDs de ejecución/revisiones, evidencia SQL, backup, resultado y límites. En staging ejecutar el cierre automático descrito antes de liberar reserva; producción no se apaga. Liberar lock solo después de readback; un fallo conserva estado verificable y procedimiento de desbloqueo, nunca borrar un lock a ciegas.

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

## Costos y comparación de alternativas

Sí: staging aislado suele aumentar el gasto. Cloud SQL encendido cobra capacidad aunque no reciba consultas; Cloud Run puede acercarse a costo de cómputo cero en reposo con mínimo 0 y facturación por solicitudes. Con mínimo activo o facturación por instancia cambian los cargos. Jobs cobra durante ejecución, con mínimo de un minuto por instancia. [Cloud SQL](https://cloud.google.com/sql/pricing), [Cloud Run](https://cloud.google.com/run/pricing), [modalidades de facturación](https://docs.cloud.google.com/run/docs/configuring/billing-settings).

La configuración actual declara API mínimo 1 y `--no-cpu-throttling`: copiarla a staging preservaría costo continuo. Antes de poner mínimo 0, inventariar goroutines/consumidores y requisitos de CPU; los workers durables pueden necesitar otro modelo y no desaparecen por bajar el mínimo del API. Detener SQL suspende cómputo pero conserva cargos de almacenamiento/IP aplicables; revisar backup/PITR por separado. [Detener Cloud SQL](https://docs.cloud.google.com/sql/docs/postgres/start-stop-restart-instance).

| Opción | Costo relativo | Qué demuestra / límites |
| --- | --- | --- |
| PostgreSQL efímero local/CI + destino cloud actual | Menor costo adicional permanente | SQL/API local; no valida aislamiento ni red/IAM cloud antes del destino actual |
| B: SQL staging independiente retenido y bajo demanda | Cómputo por ventana + costos persistentes | Valida integración cloud; incluye controlador, vigilante y verificación de cierre |
| SQL staging creado/destruido por sesión | Sin almacenamiento del recurso tras borrado confirmado; mayor costo/tiempo por aprovisionamiento | Más limpio; automatizar IAM/red/bootstrap/restore y eliminación exacta; backups retenidos y otros recursos pueden seguir cobrando |
| B + C: CI efímero + SQL bajo demanda | Opción preferida para presupuestar; paga CI y ventanas cloud necesarias | Cobertura diaria SQL y validación cloud por candidato; no requiere dos SQL permanentes encendidos |
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

Para dimensionar horas: 730 h/mes permanente frente a ejemplo de 8×22 = 176 h/mes de cómputo programado; almacenamiento/backup no se reducen en la misma proporción. Son escenarios de uso, no precios. No se ofrece monto USD/CLP sin tier/edición/HA/disco/retención/tráfico comprobados. Los precios micro/small y ahorros del simulador de la conversación no se adoptan como cotización para Santiago ni prueban soporte/adecuación de PostgreSQL 18 y sus extensiones. Los valores Terraform históricos no describen necesariamente la instancia SQL externa actual. Crear presupuesto/alertas no equivale a un tope automático de gasto. Cotizar antes de aprovisionar; medir ahorro del bastión con discos/IP/snapshots reales, no con una VM supuesta siempre encendida.

### Entregable económico obligatorio de FLUJO-01

Sin reauditar código ni cambiar cloud, recoger inventario read-only y Billing autorizado por identificador de recurso: SQL actual/staging propuesto (edición/tier/HA/GB/extensiones), bastión (máquina, horas encendido, discos, IP y snapshots), backups/WAL, Cloud Run/Jobs, builds, imágenes, logs, secretos, controlador/Scheduler y edge/red que realmente se añadan. Si Billing no está accesible, declarar el dato pendiente y no presentar una estimación de lista como costo real.

Entregar comparación en USD mensual para `southamerica-west1`, con fecha/SKU/unidades/fuente y supuestos: permanente (730 h), bajo demanda (ejemplo 20 h y 60 h de sesión total, incluyendo arranque/QA/cierre), instancia creada/destruida por ensayo y B + C. Separar costos reales históricos de proyecciones, cómputo de persistencia y descuentos/créditos/impuestos. Si se convierte a CLP, fijar fuente/fecha de cambio. Ahorro neto = costos retirados del bastión − costos nuevos de staging/automatización; puede ser negativo y no se presupone que bastión estuviera siempre encendido.

Para el diseño elegido se retiene una instancia staging pequeña y compatible, no se crea otra por push. La opción crear/destruir queda pospuesta hasta que tiempos, limpieza y costo medidos la justifiquen. No seleccionar un tier de CPU compartida solo por su tarifa. Los tests en CI también consumen minutos/recursos del proveedor aunque no generen una instancia Cloud SQL permanente.

## Fases y criterios de cierre

| Fase | Entregable | Gate para cerrar | Estado |
| --- | --- | --- | --- |
| FLUJO-00 | Auditoría estática + este plan, preguntas resueltas desde código/texto | Fuentes/limitaciones y decisiones pendientes registradas | Documentado en este corte; no aceptación humana |
| FLUJO-01 | Inventario read-only, comparación económica y ADR B + C | Clasificar testprod; verificar backup/PITR/recuperación, costos reales, tier compatible, TTL y límites; presupuestar controlador/vigilante; resolver ramas/RPO-RTO | Pendiente; preferencia bajo demanda registrada |
| FLUJO-02 | Calidad SQL obligatoria y contrato/manifest de release | Base vacía + actualización de versión destino; checksums, fallos/reanudación, concurrencia, permisos y API anterior/nueva; prueba falla si DB se omitió | Pendiente |
| FLUJO-03 | Imagen completa + Job y controlador de sesión/vigilante diseñados y probados localmente | Inventario idéntico a Git; denegar producción; simular expiración, duplicados, reserva antigua, cancelación y fallo de cierre; sin activar cloud | Pendiente |
| FLUJO-04 | Staging bajo demanda y recuperación aislada, con presupuesto aprobado | Arranque→pruebas→stop confirmado; vigilar fallo/cancelación/TTL/runner perdido; baseline sintético, restore medido y QA humano; cero acceso productivo | Pendiente, operación cloud requiere autorización |
| FLUJO-05 | Orquestación Cloud Build con CI efímero, staging explícito y promoción | Push normal no enciende SQL; misma evidencia/digests; finalizador y vigilante independientes; adaptar clasificadores/validadores y trigger main antes de publicar cambios de alcance | Pendiente |
| FLUJO-06 | Primer release integrado por Job al destino aprobado | Backup/recuperación verificados; candidato QA aprobado, staging cerrado; SQL/postcheck/compatibilidad, revisión sin tráfico, validación humana, promoción y observación; producción no se detiene | Pendiente, operación autorizada concreta |
| FLUJO-07 | Retirada definitiva del bastión y cierre documental | Inventario de dependencias DMS/otros usuarios vacío, alternativa de recuperación ensayada, eliminación exacta aprobada y ahorro medido | Pendiente |

Cada fase usa una rama de tarea temporal desde main y publica allí el trabajo revisado, sin PR ni force-push. Preservar worktrees/demos/volúmenes activos. Actualizar plan/matriz con SHA, comandos realmente ejecutados, resultados y límites; integración/despliegue solo conforme a autorización de ese corte. La aceptación del producto corresponde al usuario.

## Decisiones pendientes del usuario

1. Elegir modelo de ramas: recomendación main + promoción de artefacto, alternativa develop si necesita integración separada. Este plan no cambia AGENTS.
2. Clasificar testprod y seleccionar staging nuevo o reutilización segura, con SQL independiente como opción preferida.
3. Fijar presupuesto y ventanas/TTL del staging bajo demanda preferido; aprobar recursos concretos tras cotización. No se vuelve a pedir elegir entre permanente y bajo demanda salvo evidencia que obligue a revisar la viabilidad.
4. Fijar RPO/RTO/retención y quién autoriza producción y recorridos cloud. Mantener los límites humanos actuales hasta ampliación expresa.

No es necesario resolver esas preferencias para completar la auditoría estática ni las propuestas de este documento. No se presupone autorización para gasto, cambios cloud, retirada de VM ni despliegue automático.

## Registro y verificación de esta revisión

- Fuente humana: revisión y conversación entregadas como archivo adjunto el 2026-11-06; se incorpora B + C y apagado automático como preferencia de diseño, manteniendo pendiente el gasto/operación concreta.
- Se conserva la auditoría original y se ajustan FLUJO-01–07 sin repetirla ni afirmar nueva evidencia GCP. FLUJO-00 sigue documental; los componentes propuestos no están implementados.
- Revisión documental: diff completo, enlaces locales, bloques Markdown y `git diff --check`. Sin suites de aplicación ni pruebas cloud ejecutadas por editar este plan; los escenarios de apagado descritos son gates futuros, no resultados obtenidos.
