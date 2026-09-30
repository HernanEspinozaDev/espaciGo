# Propuesta final de backend e infraestructura para EspaciGo

**Versión:** 1.2 — 29-09-2026
**Estado:** propuesta oficial y documento base único de arquitectura backend ES2; decisiones consolidadas para iniciar la implementación. No indica que exista código, despliegue, acceso a proveedores o cumplimiento probado.  
**Alcance:** backend, infraestructura GCP y almacenamiento/analítica necesarios para él. El frontend se diseña después y queda fuera de este documento.

Esta versión sincroniza el backend con el [diccionario oficial de diseño del Anexo B](../anexos/B_diccionario_datos.md). Por decisión posterior del usuario, la planificación cubre el producto completo y todas las categorías desde el esquema inicial; la construcción sigue siendo incremental. La Ley 21.719 se incorpora desde el primer incremento como criterio de diseño, independientemente de su fecha de vigencia. Mercado Pago conserva una investigación de integración separada.

## 1. Decisión ejecutiva

Construir **un monolito modular en Go**, con módulos internos por dominio y una API HTTP/JSON versionada mediante OpenAPI. En la primera versión se despliega una aplicación API en un contenedor Cloud Run. No se crean contenedores separados para pagos, usuarios, contratos e imágenes. Las tareas asíncronas corren como goroutines de trabajo dentro del mismo servicio, leyendo trabajos durables de PostgreSQL; no se despliegan Cloud Run Jobs ni un servicio worker separado en esta fase. Extraer un dominio a otro servicio requeriría una decisión posterior respaldada por evidencia de escala, aislamiento o despliegue independiente.

Usar **PostgreSQL/PostGIS en Cloud SQL como fuente de verdad transaccional** para el estado de la plataforma. Usar **Cloud Storage privado** para imágenes, contratos y evidencias. Administrar recursos con **Terraform** y estado remoto en GCS con versionado/locking. Usar **Cloud Logging** para logs técnicos y correlación, un registro durable de auditoría de negocio para acciones críticas, y **BigQuery** para análisis de eventos/agregados autorizados. Implementar outbox en PostgreSQL y publicarlo a Pub/Sub para ingesta a BigQuery; **Datastream queda descartado para esta fase**. Configurar presupuesto y alertas de facturación desde el primer despliegue GCP, junto con límites operativos por servicio.

El alcance de diseño es **el producto completo**: todas las categorías de espacios de la entrega anterior y los flujos de identidad, publicación, búsqueda, cotización, reserva, pago, firma, uso, disputa, liquidación, comunicación, promoción, analítica y derechos de titulares. `categoria_espacio` es catálogo referenciado por `espacio`; cada publicación representa una unidad reservable exclusiva. La entrega de funciones puede organizarse por incrementos, sin reducir el modelo a una categoría ni declarar implementadas integraciones externas. Desarrollo, integración y staging invocan Mercado Pago exclusivamente en sandbox; un simulador local sirve para pruebas aisladas y no acredita cobro real.

Para acceso a datos, usar **pgx/pgxpool + sqlc** en operaciones SQL definidas y transaccionales. Permitir `pgx` con consultas parametrizadas y filtros explícitamente permitidos para la búsqueda dinámica. Las transacciones ACID cubren escrituras de PostgreSQL; no revierten una operación que un proveedor externo ya realizó.

## 2. Arquitectura objetivo

```text
Futuro consumidor de la API (fuera de esta etapa)
                  |
           HTTPS / OpenAPI
                  |
     Cloud Run: API Go modular
       |          |           |
       |          |           +---- adaptadores HTTPS a pago/firma/identidad/correo
       |          |
       |          +---------------- Cloud Storage privado (objetos)
       |
       +---- Cloud SQL PostgreSQL/PostGIS
                    |
                    +-- estado operacional, auditoría/outbox y migraciones
                                |
                     publicador asíncrono idempotente
                                |
                           Pub/Sub (fase analítica)
                         /                     \
        BigQuery subscription             consumidor/transformación
                         |
                      BigQuery
         (métricas, NPS agregado, analítica, recomendaciones futuras)

Cloud Run/Cloud SQL/Storage/Pub/Sub/BigQuery/IAM/logging/state
                         ↑
                     Terraform
```

El diagrama es una dirección propuesta, no un despliegue existente. Los servicios de pago, firma, identidad y correo dependen de contratos, acceso y pruebas. BigQuery no está en el camino síncrono de reservar o pagar.

## 3. Estructura del backend Go

```text
cmd/api/                     servidor HTTP principal
internal/
  identity/                  cuenta, roles, perfil y verificación
  catalog/                   espacios, categorías, tarifas y publicaciones
  search/                    búsqueda geográfica y filtros permitidos
  booking/                   cotización, reserva y calendario de ocupación
  payments/                  intentos, reembolsos, estado y conciliación
  contracts/                 versiones de contrato y firmas
  operations/                check-in/out y evidencias
  disputes/                  reclamos, descargos y resolución
  communications/            notificaciones, mensajes de reserva y reseñas
  promotions/                 campañas, órdenes y permisos de métricas
  audit/                     trazas de acciones críticas y consultas autorizadas
  analytics/                 esquemas/eventos, agregados y acceso a reportes
  worker/                    ejecución de tareas durables dentro de cmd/api
  platform/
    authz/ database/ config/ observability/
  adapters/
    postgres/ storage/ payment/ signature/ identity/ messaging/
api/openapi.yaml              contrato API
db/query/                     SQL que consume sqlc
db/migrations/                cambios SQL versionados
deploy/terraform/              IaC separada por módulos/ambientes
```

### Reglas de dependencia

- Los módulos representan límites de negocio, no servidores independientes.
- Dominio y casos de uso no importan SDKs GCP ni implementaciones HTTP.
- Los controladores traducen HTTP a comandos/consultas y no contienen reglas financieras.
- Las dependencias externas implementan interfaces definidas junto al caso de uso.
- Los paquetes no realizan consultas cruzadas arbitrarias a tablas ajenas; las colaboraciones se declaran en casos de uso.
- No mantener transacción de BD abierta mientras se espera red de un proveedor.
- El `pgxpool.Pool` es compartido por configuración, pero las transacciones se pasan explícitamente a consultas/repositorios que deben participar en ellas.
- Usar `sqlc` para SQL revisable y tipos generados; `pgx` dinámico solo con parámetros y allowlist de campos de orden/filtro.

## 4. Infraestructura GCP y Terraform

### Recursos por ambiente

Provisionar ambientes separados, idealmente en proyectos GCP distintos para dev, staging y producción. Cada ambiente tiene cuentas de servicio, secretos, base de datos, bucket, logs y datasets segregados. Región de operación propuesta: `southamerica-west1` (Santiago), sujeta a disponibilidad y costo de cada producto.

Terraform crea y mantiene:

- APIs necesarias, IAM mínimo, Artifact Registry, Cloud Run y Cloud SQL.
- Secret Manager y permisos de lectura; ningún secreto se deja en Git, imagen ni estado/variables de Terraform.
- Buckets privados, políticas de retención y logs sinks cuando la fase y el análisis de datos lo requieran.
- Pub/Sub y BigQuery para el flujo analítico aprobado. Datastream no se provisiona en ES2.
- Logging/Monitoring, alertas, backups, red y conectividad.

El bucket GCS para el state de Terraform se prepara mediante bootstrap controlado antes de utilizar el backend remoto. Activar versionado y locking; restringir IAM porque state puede contener valores sensibles. No compartir state entre ambientes. Revisión de `plan` antes de aplicar cambios en ambientes persistentes.

El proceso de despliegue construye imagen Docker reproducible, publica un digest/versionado en Artifact Registry y actualiza una revisión identificable de Cloud Run. Terraform gestiona la infraestructura; un pipeline puede gestionar el rollout de revisiones, con autenticación de CI de corta duración e IAM mínimo.

### Cloud Run y conexiones a Cloud SQL

API en Cloud Run con cuenta de servicio propia, acceso a Cloud SQL configurado de forma segura, límites máximos de instancias y pool acotado. Calcular el máximo agregado como `instancias máximas × conexiones por instancia` y mantenerlo dentro del límite de Cloud SQL. Tener health/readiness, cierre graceful, timeouts, concurrencia y memoria configurados. No declarar capacidad de usuarios sin prueba bajo el perfil definido.

Las goroutines de fondo viven dentro de cada instancia del servicio API. Se configura facturación por instancia y al menos una instancia mínima para mantener CPU disponible fuera de una petición. Cloud Run puede reiniciar o escalar instancias, por lo que **no se confía en memoria, timers locales ni ejecución exactamente una vez**: cada worker reclama trabajo durable en PostgreSQL usando lease/bloqueo transaccional y `SKIP LOCKED`, guarda checkpoints, reintenta idempotentemente y cierra en SIGTERM. Varias instancias pueden ejecutar el mismo tipo de worker con seguridad por el mecanismo de reclamo. Los costos de la instancia mínima se incorporan al presupuesto. El estado vencido también se valida sincrónicamente al operar sobre una reserva; el proceso de fondo limpia retenciones y envía notificaciones, no es la única protección contra una retención expirada.

## 5. Datos operacionales y conexión a PostgreSQL

PostgreSQL registra y decide:

- Usuarios, roles, estado de cuenta, perfiles y verificaciones.
- Espacios de todas las categorías mediante FK a catálogo, capacidad, tarifas y condiciones versionadas; una publicación es una unidad exclusiva.
- Ocupación, bloqueos, reservas y snapshots de precio/condiciones.
- Intentos de cobro/reembolso, referencias de proveedor, liquidaciones y disputas.
- Contratos versionados, estado por firmante, documentos y evidencias.
- Campañas/promociones pagadas, órdenes y derecho de consulta de métricas, con activación comercial posterior a precio/política aprobados.
- Mensajes, reseñas, notificaciones, solicitudes de titulares, auditoría operacional y respuestas NPS voluntarias.

El [Anexo B](../anexos/B_diccionario_datos.md) fija el contrato lógico completo con 43 entidades/tablas de diseño, campos, nulabilidad, claves, finalidades, cardinalidades y estados. Incluye perfiles/sesiones, tarifas y comisiones versionadas, historia de estados, movimientos financieros, comunicación, promoción y derechos. Su implementación en migraciones PG18/PostGIS y los ensayos MD/PT siguen pendientes; el conteo anterior de 17+1 describía solo el núcleo preliminar y ya no define el alcance.

### Transacción local y pagos externos

Una transacción puede crear reserva+ocupación+outbox y confirmar esos cambios juntos. Una transacción no puede hacer rollback de una autorización/captura en Mercado Pago u otro proveedor. Flujo recomendado:

1. Validar autorización, disponibilidad, precio y política.
2. Crear reserva pendiente y ocupación temporal bajo restricción de no solapamiento; emitir outbox local en la misma transacción.
3. Crear o confirmar operación de pago mediante adaptador con clave idempotente, fuera de una transacción DB larga.
4. Procesar respuesta/webhook verificado, guardar evento externo deduplicado y avanzar estados locales.
5. Para integración real, ejecutar llamadas exclusivamente contra sandbox y con usuarios/medios de pago de prueba. Ante timeout ambiguo, marcar `por_conciliar`; consultar al sandbox antes de intentar de nuevo.
6. Si el pago se confirmó y después falla un paso de negocio, ejecutar compensación/reembolso conforme a política y proveedor. Registrar la compensación como nueva operación, no borrar el cobro original.
7. Generar contrato/versionado y habilitar firmas/check-in según estados y reglas aprobados.

Conciliar cuatro valores distintos: importe comprador, cargo proveedor, comisión EspaciGo y neto arrendador. El escenario económico vigente usa `regla_comision` inicial de **3 % neto** del arriendo publicado; IVA de la comisión separado si corresponde. La tarifa del proveedor se descuenta primero al vendedor según la documentación pública de Split 1:1 y se registra separada del ingreso de la plataforma. El comprador paga el precio final publicado en el caso base; cualquier cargo adicional requiere cotización aceptada. El Anexo A recalcula caja y margen con este contrato y con una instancia API mínima para las goroutines. Split 1:1 no prueba custodia, escrow ni liberación condicionada. Antes de dinero real hacen falta contrato, cuenta/KYC y ensayos de cobro, reparto, reverso, saldo insuficiente y contracargo en sandbox.

La exclusividad de sandbox se aplica a toda prueba que invoque Mercado Pago: integración continua, pruebas manuales y staging usan credenciales y medios de pago de prueba. Las pruebas unitarias pueden usar un simulador local sin conectar al proveedor. Credenciales productivas no estarán disponibles en desarrollo/staging; habilitarlas después requiere decisión separada y controles de acceso.

## 6. Archivos: imágenes, contratos y evidencia

Guardar contenido binario en Cloud Storage, no en PostgreSQL. PostgreSQL guarda propietario, objeto, categoría, hash, tamaño, tipo declarado/detectado, fecha, estado de validación, política y referencias a reserva/espacio/contrato/disputa.

Proceso de imagen:

1. Backend autentica y comprueba que el usuario puede subir al espacio.
2. Backend genera clave de objeto impredecible y permiso/URL firmado de duración y alcance breves.
3. El cliente sube directo al bucket privado; el backend no proxifica el archivo grande.
4. Backend verifica metadatos/objeto, tamaño, MIME real y hash; marca disponible solo después de validación. Rechaza contenido no esperado; transformación/antivirus pueden ser job futuro.
5. Lectura se habilita con CDN/origen controlado para imágenes públicas aprobadas o URL firmada para contratos/evidencia privada.

Las URLs firmadas son bearer credentials: quien las tiene puede ejecutar esa acción hasta vencimiento. No escribirlas en logs ni bases analíticas. Aplicar cuotas, límites de tamaño, protección de malware y regla de eliminación/retención.

## 7. Estados y módulos funcionales

### Máquina de estados del producto completo

```text
pendiente_de_pago → pagada → aprobada_host → firma_parcial → lista_para_checkin
                                                          → en_curso → finalizada → cerrada
                                                                            ↕
                                                                        en_disputa
```

El flujo agrega cancelaciones por pago, rechazo/vencimiento del anfitrión, falta de firma y cancelación del arrendatario conforme a política. Si ambas firmas llegan juntas, `aprobada_host` puede pasar a `lista_para_checkin` mediante transición explícita. `disputa` y `pago` conservan estados propios; una disputa abierta bloquea liquidación. Cada salto registra actor o proceso, motivo, tiempo, versión y correlación en `reserva_transicion` durante el mismo commit. El cliente y un webhook no asignan estados arbitrariamente. Los arcos y literales completos están en Anexo B.

| Dominio | Primera responsabilidad backend |
| --- | --- |
| Identidad/acceso | Registro, inicio/cierre, recuperación, roles coexistentes, permisos por objeto; verificación manual o adaptada solo con proveedor autorizado |
| Catálogo | CRUD de publicaciones, estado de moderación, tipos de espacio, capacidad, precios/unidades, medios |
| Búsqueda | Filtros parametrizados, PostGIS y disponibilidad; no debe reservar |
| Reserva | Cotización con snapshot, ocupación semiabierta `[inicio, fin)`, expiración y prevención concurrente de doble reserva |
| Pago | Estados técnicos separados de reserva, idempotencia, eventos autenticados, conciliación y compensaciones |
| Contrato | Render/versionado, hash, firmas por parte; no habilitar ingreso con firmas incompletas |
| Operación | Check-in/out/recepción, evidencias y timestamps; acceso por participantes de la reserva |
| Disputa/liquidación | Resolución autorizada, motivo, bloqueo mientras disputa abierta y liquidación luego de confirmación externa |
| Auditoría | Registrar quién hizo qué sobre qué recurso, cuándo, resultado, motivo/correlación y origen técnico permitido |
| Campaña/métricas | Futuro opcional: configuración y entitlement operacional, informes agregados, medición válida de cada campaña |

Todos los tipos de espacio se mantienen en el dominio desde el diseño. Cada incremento construye funciones verificables sin convertir una categoría de ejemplo o un pago simulado en el alcance oficial del producto.

## 8. Observabilidad, auditoría y privacidad

### Tres datos distintos

| Tipo | Ejemplo | Destino primario | No hacer |
| --- | --- | --- | --- |
| Log técnico | error HTTP, latencia, nombre módulo, revisión Cloud Run, trace ID | Cloud Logging | Contraseñas, token, documento, PAN/CVV, cuerpo entero de requests |
| Auditoría de negocio | cambio de email, login rechazado, publicación editada, intento/reembolso, firma, disputa resuelta, acceso administrativo | Registro durable PostgreSQL y archivo/exportación bajo política de retención | Tratar stdout/BigQuery como única evidencia o dejar que datos analíticos decidan estados |
| Evento analítico | búsqueda resumida, impresión/clic, campaña, reserva completada, NPS | Outbox/Pub/Sub/BigQuery con schema/versionado | IP completa, correo/nombre, payload libre y eventos de alta cardinalidad sin finalidad |

El log estructurado puede llevar `timestamp`, `severity`, `service`, `module`, `action`, `actor_id` seudónimo cuando aplique, `resource_type`, `resource_id`, `result`, `reason_code`, `request_id`, `trace_id` y revisión. Para login fallido no existe actor autenticado: usar ID de intento/cuenta seudónima, proteger la IP/UA bajo una política explícita y limitar retención.

Cloud Run envía request/container/system logs a Cloud Logging; un sink puede exportar una selección a BigQuery. Esto habilita consulta, no certifica integridad por sí solo. RNF-017 en ES2 propone Cloud Storage con retención bloqueada y hash por lote para la garantía de inmutabilidad; plazo, cobertura y tratamiento frente a derechos deben acordarse antes de bloquear una política irreversible y comprobarse con una prueba de alteración.

Privacidad desde primer incremento: inventario de datos por finalidad, minimización, control de acceso, conservación por categoría, exportación/bloqueo/supresión, anonimización selectiva y registro de solicitudes de titulares. No copiar todas las filas a BigQuery/Datastream. Ley 21.719 como criterio de diseño ES2; no afirmar cumplimiento sin evaluación y evidencia.

## 9. Analítica, Customer Success, promociones y recomendaciones

### Ingesta analítica

Los eventos son un contrato de datos aparte de la API de negocio. Un esquema inicial puede tener `event_id`, `event_name`, `schema_version`, `occurred_at`, `received_at`, `listing_id`, `category`, `coarse_area`, `campaign_id`, `session_key` seudónimo, `source` y campos mínimos. Definir retención, deduplicación, evento tardío, bot/fraude, consentimiento/finalidad y acceso por dataset. La interfaz exacta que emitirá eventos se definirá al iniciar frontend; no convertir una petición de telemetría sin autenticación en fuente confiable.

Los eventos de dominio que deben corresponder a un cambio transaccional se escriben en outbox junto a sus cambios PostgreSQL y luego se publican a Pub/Sub con reintento/idempotencia. El publicador es una goroutine del monolito que reclama filas de outbox con lease; los errores dejan la fila reintentable. Las impresiones y clics de alto volumen se reciben por una interfaz de telemetría validada y se publican directamente a Pub/Sub, sin llenar la BD operacional con cada vista. Pub/Sub escribe a BigQuery mediante BigQuery subscription porque el primer pipeline no requiere transformación. Configurar schema versionado/compatible, dead-letter topic, monitoreo y roles IAM mínimos. No hacer una llamada bloqueante a BigQuery por cada clic.

### Métricas de publicaciones y promociones

Medir impresiones y clics para todas las publicaciones desde la primera etapa analítica, sin exigir compra de promoción para empezar el conteo. El acceso a reportes de alcance, vistas, clics y CTR se habilita cuando el arrendador autorizado tenga activo el ticket premium para esa publicación y período. Definir ventana/retención y datos agregados antes de instrumentar. Para reportes premium:

1. API autentica solicitante.
2. PostgreSQL verifica que pertenece al arrendador autorizado y que posee entitlement activo para la publicación y fechas solicitadas.
3. API limita intervalo, dimensiones, agregación y costo de consulta.
4. BigQuery devuelve métricas agregadas, nunca listas de visitantes.

Definir “impresión vista” (p.ej. tarjeta realmente renderizada y umbral visible), alcance deduplicado (dispositivo/sesión seudónima dentro de ventana), clic, CTR, periodo, zona aproximada y atribución antes de vender un ticket promocional. Los datos de IP no son una solución por defecto para geografía.

Para mostrar métricas antes de terminar una interfaz propia, se implementará un panel de demostración en **Looker Studio (antes Data Studio)** conectado a vistas autorizadas de BigQuery. En esta fase se habilita solo a vendedores de prueba nominados. Antes de compartirlo, se prueba con dos identidades distintas que cada una vea exclusivamente sus filas: usar credenciales del viewer con acceso BQ mínimo y políticas de acceso por fila basadas en identidad, o mantener vistas/datasets segregados para el piloto. No compartir un reporte multi-vendedor con credenciales del propietario ni confiar en filtros visuales como control de acceso. Los usuarios del marketplace que no tengan identidad Google apta para BQ accederán posteriormente a sus métricas mediante la API autenticada, con entitlement comprobado en PostgreSQL.

### NPS y customer success

NPS es una respuesta voluntaria a una encuesta con momento, puntuación y consentimiento/finalidad; no inferir NPS de clicks. Conservar en PostgreSQL los registros necesarios para dar seguimiento y derechos del titular; exportar a BigQuery datos minimizados/seudonimizados y agregados. Separar texto libre, restringirlo y definir retención. Diseñar indicadores (respuesta, promotores/pasivos/detractores y tendencia) con umbral mínimo de participantes por segmento para no identificar personas.

### Recomendaciones

Orden de madurez:

1. Reglas basadas en disponibilidad, categoría, ubicación general y popularidad reciente, siempre explicables y con opción de búsqueda completa.
2. Evaluar con resultados: apertura, reserva confirmada, cancelación, variedad y calidad; comparar contra baseline.
3. Entrenar offline cuando haya suficientes eventos representativos y finalidad permitida. BigQuery ML matrix factorization es candidato posterior, no decisión de inicio; revisar reservas/costo y `ML.EVALUATE`/sesgo/cold-start.
4. Servir un catálogo/versionado de recomendaciones y poder volver a reglas cuando el modelo falle o se deteriore.

No se incorpora Vertex AI en esta propuesta: no se necesita para fijar la primera arquitectura y no fue una decisión solicitada. Se evalúa solo si el caso posterior lo requiere.

### CDC / Datastream

Datastream se **descarta para ES2**. La arquitectura analítica usa exclusivamente eventos curados mediante outbox→Pub/Sub→BigQuery. No replicar tablas PostgreSQL con CDC en esta fase.

## 10. Despliegue, calidad y operación

- Configuración validada al inicio y separada por entorno; secretos solo por Secret Manager.
- Migraciones versionadas, revisables y compatibles hacia atrás cuando sea posible; evitar editar esquema manualmente en consola.
- Health/readiness, métricas técnicas, logs estructurados, trace/request ID, graceful shutdown.
- Pruebas de unidad para reglas; integración con PostgreSQL real para restricciones; simuladores externos separados de sandboxes; humo/carga/restauración según Anexo C.
- CI construye, ejecuta checks y produce imagen por digest; deploy de staging; evidencia de promoción a prod.
- Backups, restauración ensayada, RPO/RTO medidos, rollback de código y plan aparte para migraciones/eventos ya aceptados.
- Capar instancias y conexiones de DB; budgets/alertas para Cloud SQL, Cloud Run, Pub/Sub y BigQuery.

Objetivos heredados de ES1 (disponibilidad, latencia, capacidad, RPO/RTO) siguen siendo metas para medir, no resultados. Deben conservarse reportes con ambiente, carga, versión y hora.

## 11. Roadmap por etapas

| Etapa | Construir | Evidencia para pasar a la siguiente |
| --- | --- | --- |
| 0. Base de ejecución | Fijar catálogo de todas las categorías, estados, permisos, API inicial, clases de datos/retención y provider interfaces | Contrato de API y diccionario del producto completo documentados |
| 1. Base de repositorio/backend | Go skeleton, monolito modular, `slog`, OpenAPI inicial, error envelope, health/readiness, Docker local, PostgreSQL/PostGIS, migraciones y sqlc | CI y API local reproducibles; DB migrable desde cero |
| 2. Terraform y GCP dev | Bootstrap state GCS versionado/locking, proyecto/env dev, IAM, Artifact Registry, Secret Manager, Cloud SQL, Cloud Run, logging técnico, presupuesto/alertas y límites operativos | Terraform plan/apply repetible, endpoint dev y alertas comprobadas |
| 3. Identidad y catálogo | Usuarios/roles, autorización por propietario, catálogo de todas las categorías y reglas de tarifa por unidad; Cloud Storage y búsqueda geográfica | Pruebas de permisos, validación de objetos, migraciones y búsqueda por categoría |
| 4. Reserva | Cotización snapshot, ocupación, exclusión, expiración, outbox; workers goroutine con reclamo durable y concurrencia | PT-01/PT-02 pasan; worker retoma tras reinicio sin duplicar efectos |
| 5. Pago y contrato | Adaptador simulado para pruebas aisladas; Mercado Pago solo sandbox durante integración; idempotencia, webhooks, conciliación, contrato versionado, firmas y disputa | Evidencia distingue simulador y sandbox; sin credenciales productivas ni efectos duplicados |
| 6. Operación segura | Check-in, evidencia, administración, auditoría, derechos titulares, backups/restauración y alertas | PT aplicables, ensayo de recuperación y privacidad con fecha |
| 7. Analítica inicial | Outbox→Pub/Sub→BQ para eventos de dominio, Pub/Sub→BQ para impresiones/clics de todas las publicaciones; métricas premium, Looker Studio para vendedores de prueba y NPS | Esquema/deduplicación comprobados, aislamiento entre vendedores y costo revisado |
| 8. Recomendaciones | Reglas baseline y evaluación offline; BigQuery ML solo con datos suficientes y reserva presupuestada | Mejora medida frente al baseline y privacidad/costo aprobados |

El orden de trabajo comienza backend e infraestructura; la terminación de la base de datos como modelo ejecutable sigue con migraciones durante las primeras etapas. Frontend se planificará cuando los contratos de API y estados centrales estén estables.

## 12. Criterios de decisión para microservicios

Extraer un módulo solo cuando al menos una medición/prueba muestre que el despliegue único impide cumplir una necesidad: carga de CPU/memoria distinta, escala muy diferente, aislamiento de seguridad, disponibilidad, despliegue independiente o equipo dueño autónomo. Antes de extraerlo, exigir:

- API/eventos versionados y dueño explícito de datos.
- Fallas/reintentos/timeouts/idempotencia y compensación documentados.
- Observabilidad distribuida y guardia/operación propia.
- Costo total comparado con el monolito.
- Prueba de que la separación produce beneficio que supera complejidad.

Una falla se puede localizar en el monolito con `service`, `module`, `operation`, `trace_id`, logs y métricas. Crear un contenedor por carpeta no mejora por sí mismo la auditoría de fallas.

## 13. Decisiones ya fijadas y límites conocidos

| Decisión | Definición de implementación |
| --- | --- |
| Alcance de producto | Todas las categorías y flujos de ES1 modelados; construcción incremental sin demo de categoría única como definición de arquitectura |
| Estados de reserva | Máquina completa del Anexo B: pago, aprobación, firmas, uso, disputa, cierre y cancelaciones; pago y disputa tienen estados independientes |
| Datos y privacidad | Diccionario completo del Anexo B como contrato lógico; Ley 21.719 aplicada como criterio de diseño desde el primer incremento y PT-16 previsto |
| Pagos | Mercado Pago exclusivamente sandbox en desarrollo, CI, pruebas manuales y staging. El simulador es solo una herramienta de prueba aislada; no hay credenciales productivas |
| Analítica/CDC | Outbox PostgreSQL → Pub/Sub → BigQuery. Datastream descartado en esta fase |
| Métricas de publicaciones | Registrar impresiones y clics de todas las publicaciones desde la etapa analítica; mostrar alcance, vistas y CTR solo con ticket premium vigente y autorización del arrendador |
| Panel temprano | Looker Studio (Data Studio) conectado a vistas autorizadas, limitado a vendedores de prueba y con aislamiento validado; no se usa como sustituto de una futura API de reportes |
| Procesos de fondo | Goroutines del mismo servicio Cloud Run; PostgreSQL durable con leases/reclamo transaccional; facturación por instancia y mínimo una instancia; sin Cloud Run Job independiente |
| Costos | Budget por ambiente y alertas al 50 %, 80 % y 100 % del presupuesto desde el primer despliegue, además de cotas operativas por servicio |

El Anexo B fija roles y controles de acceso de diseño, matriz de finalidades y procedimiento de derechos desde el primer incremento. La base jurídica concreta, los plazos exactos por clase, el bloqueo irreversible de RNF-017 y la tributación de cada cargo requieren revisión competente antes de automatizar reglas productivas. Las alertas de Billing Budget notifican umbrales, pero por sí solas no cortan consumo. Terraform establece máximo de instancias, pool de DB y límites de consulta/cuota cuando estén disponibles; para un corte duro se necesita control aparte y los recursos persistentes pueden seguir generando costo.

## 14. Trazabilidad de la propuesta oficial

Este documento es la única propuesta base vigente del backend. Sus decisiones se incorporan al capítulo III, especialmente en 3.3–3.6, y se reflejan en el Anexo B para el modelo de datos y el mecanismo Outbox. La investigación de Mercado Pago permanece independiente ([INV-026](INV-026_mercado_pago_split.md)) y podrá incorporarse como anexo técnico cuando se cierre su evidencia; no constituye una segunda propuesta de arquitectura. Las fuentes PlantUML editables de la vista de componentes, secuencia reserva/pago, estados, ciclo de worker y Outbox/analítica se mantienen en `../diagramas/` y se insertan en la sección 3.3 del informe.
- [ES2, arquitectura general](../secciones/03_00_arquitectura.md), [componentes](../secciones/03_03_componentes.md), [datos](../secciones/03_04_datos.md), [comunicaciones](../secciones/03_05_comunicaciones.md), [infraestructura](../secciones/03_06_infraestructura.md)
- [Anexo B: diccionario oficial de diseño](../anexos/B_diccionario_datos.md); [Anexo C: pruebas previstas](../anexos/C_casos_de_prueba.md); [pendientes ES2](../pendientes.md)
- Documento comparado: `/home/nandev/Descargas/arquitectura_y_roadmap_del_marketplace.md`
- Conversación analizada: archivo adjunto `Texto pegado.txt` (la mención a agentes se entiende como reglas de desarrollo tipo `AGENTS.md`; no se está proponiendo un agente de producto).

### Referencias técnicas primarias

- [OpenAPI Specification 3.0.4](https://spec.openapis.org/oas/v3.0.4.html)

- [Cloud Run logging](https://docs.cloud.google.com/run/docs/logging); [Cloud Logging export sinks](https://docs.cloud.google.com/logging/docs/export/configure_export_v2)
- [Pub/Sub BigQuery subscriptions](https://docs.cloud.google.com/pubsub/docs/bigquery); [subscription delivery guarantees](https://docs.cloud.google.com/pubsub/docs/subscription-overview)
- [Datastream PostgreSQL sources](https://docs.cloud.google.com/datastream/docs/sources-postgresql); [Datastream BigQuery destination](https://docs.cloud.google.com/datastream/docs/destination-bigquery)
- [Cloud SQL from Cloud Run and connection limits](https://docs.cloud.google.com/sql/docs/postgres/connect-run); [PostgreSQL SELECT locking and SKIP LOCKED](https://www.postgresql.org/docs/current/sql-select.html)
- [Cloud Storage signed URLs](https://docs.cloud.google.com/storage/docs/access-control/signed-urls)
- [Terraform GCS backend locking/versioning](https://developer.hashicorp.com/terraform/language/backend/gcs)
- [BigQuery ML recommendation overview](https://docs.cloud.google.com/bigquery/docs/recommendation-overview); [matrix factorization pricing/reservation note](https://docs.cloud.google.com/bigquery/docs/reference/standard-sql/bigqueryml-syntax-create-matrix-factorization)
- [Cloud Run billing and background processing](https://docs.cloud.google.com/run/docs/configuring/billing-settings); [minimum instances can restart](https://docs.cloud.google.com/run/docs/configuring/min-instances)
- [Cloud Billing budget alerts](https://docs.cloud.google.com/billing/docs/how-to/budgets)
- [BigQuery row-level access policies](https://docs.cloud.google.com/bigquery/docs/managing-row-level-security); [Looker Studio data credentials](https://docs.cloud.google.com/data-studio/data-credentials)
