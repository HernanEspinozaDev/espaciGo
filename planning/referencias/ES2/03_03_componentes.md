# Componentes y diseño del backend

El backend de EspaciGo se define como un **monolito modular en Go**: una aplicación y unidad de despliegue con módulos internos delimitados por responsabilidades de negocio. En esta fase se construirá un servicio API en Cloud Run; pagos, identidad, catálogo, reservas, contratos y operación no se desplegarán como microservicios separados. La modularidad se expresa en límites de paquetes, interfaces y dependencias internas, no en contenedores por carpeta. Esta decisión reduce los puntos de falla distribuida y conserva una trayectoria de extracción futura si las mediciones demuestran una necesidad independiente de escala, aislamiento o despliegue.

El alcance arquitectónico de ES2 cubre **todas las categorías y flujos del producto** definidos en la entrega anterior, con implementación incremental. El Anexo B fija el diccionario completo de diseño y `categoria_espacio` referencia cada publicación. Las APIs externas de pago se invocan solo desde sandbox en desarrollo, integración continua y staging; el simulador local sirve únicamente para pruebas aisladas. La solución concreta del proveedor se documentará en una investigación de integración separada, sin incorporarla como dependencia al núcleo del backend.

## 3.3.1 Vista lógica y responsabilidades

La API es responsable de autenticación y autorización, validación del contrato HTTP, reglas de negocio, transacciones operacionales y coordinación con adaptadores. El cliente futuro será consumidor de HTTPS/JSON; no accederá directamente a PostgreSQL, credenciales de proveedor ni tablas de BigQuery. La especificación OpenAPI documentará rutas, esquemas, permisos, paginación, errores y versiones antes de entregar la API a otro cliente. Su función de contrato formal e interoperable respalda la separación entre consumidor y servicio [@es2openapispec].

![Componentes lógicos del backend modular](imagenes/figura-componentes_flujos.png){width=6.3in} <!--#fig:es2-componentes-flujos--> <!--#fuente:elaboración propia a partir de la arquitectura definida para ES2.-->

![Interfaces de persistencia y proveedores previstas](imagenes/figura-componentes_integraciones.png){width=6.3in} <!--#fig:es2-componentes-integraciones--> <!--#fuente:elaboración propia a partir de la arquitectura definida para ES2.-->

*Tabla. Módulos lógicos y responsabilidades del backend.* <!--#tab:es2-componentes-interfaces-->

| Módulo o capacidad | Responsabilidad | Dependencias permitidas |
| --- | --- | --- |
| API y autorización | Traducir solicitudes al caso de uso, validar entradas y aplicar permisos por usuario, rol y recurso | Servicios de aplicación; no contiene SQL de dominio ni lógica del proveedor |
| Identidad y perfiles (M01–M03) | Cuenta, roles coexistentes, perfil y solicitud/resultado de verificación | Repositorios internos y adaptador de identidad cuando esté habilitado |
| Oferta y búsqueda (M04–M05) | Publicaciones para todas las categorías, calendario de consulta, tarifas y búsqueda geográfica | PostgreSQL/PostGIS, módulo de archivos y consulta de ocupación |
| Reserva y contrato (M06–M07) | Cotización, reserva, instantánea de condiciones, coordinación de pago y versiones de contrato | Repositorios, eventos de dominio, adaptadores de pago y firma detrás de interfaces |
| Operación y reputación (M08–M09) | Check-in/check-out, recepción, evidencia, comunicación y reseñas según el alcance habilitado | Reserva, archivos y notificaciones |
| Disputas y liquidación (M10) | Reclamo, descargos, resolución autorizada, bloqueo de liquidación y conciliación | Reserva, adaptador de pago y auditoría |
| Administración y auditoría (M11) | Moderación, reportes y consulta restringida de acciones | Autorización administrativa y registros auditables |
| Trabajadores asíncronos internos | Expirar retenciones, publicar outbox, reprocesar eventos y conciliar operaciones | PostgreSQL durable, interfaces de proveedor y telemetría |
| Telemetría y analítica | Validar eventos de interacción, publicar a Pub/Sub y servir reportes agregados autorizados | Esquema de eventos, Pub/Sub y BigQuery; no modifica estado operacional desde consultas analíticas |

La promoción pagada es una capacidad de negocio planificada para medir publicaciones y habilitar reportes, no una autorización para modificar la reserva o el pago por vía analítica. Su cobro y políticas comerciales se especificarán en su módulo/caso correspondiente antes de activar ventas.

## 3.3.2 Organización del código y límites de dependencia

La organización propuesta concentra el arranque HTTP en `cmd/api` y mantiene la lógica no pública bajo `internal/`. Los paquetes se separan por dominio; las implementaciones de PostgreSQL, Cloud Storage, Pub/Sub y proveedores se mantienen en adaptadores. Esta estructura permite probar reglas sin iniciar servicios externos y limita el acoplamiento entre dominio e infraestructura.

```text
cmd/api/                         inicio y ciclo de vida del servidor
internal/
  identity/                      cuenta, roles y verificación
  catalog/                       espacios, categorías, tarifas y publicación
  search/                        filtros y consulta geográfica
  booking/                       cotización, ocupación y estados de reserva
  payments/                      intents, reembolsos y conciliación
  contracts/                     documento versionado y firma
  operations/                    check-in, check-out y evidencias
  disputes/                      reclamos y resolución
  communications/                chat de reserva, reseñas y notificaciones
  promotions/                    campañas, órdenes y derecho de métricas
  audit/                          acciones críticas y consultas autorizadas
  analytics/                      eventos, métricas agregadas y reportes
  worker/                         ejecución asíncrona dentro del servicio
  platform/                       configuración, autorización y observabilidad
  adapters/                       postgres, storage, pubsub y proveedores
api/openapi.yaml                  contrato HTTP versionado
db/query/                         consultas SQL fuente para generación
db/migrations/                    migraciones PostgreSQL versionadas
deploy/terraform/                 recursos e IAM por ambiente
```

Las dependencias apuntan desde controladores a casos de uso y desde estos a interfaces de repositorio/adaptador. Los SDK de GCP y los clientes HTTP de proveedores no forman parte de las reglas de dominio. Cada módulo expone operaciones explícitas; no se habilitan imports circulares ni consultas arbitrarias a tablas de otros módulos. La separación futura de un componente solo se evaluará si carga, requisitos de seguridad, disponibilidad o ciclo de entrega medidos justifican el costo de operar una frontera de red.

## 3.3.3 API, persistencia y transacciones locales

La API usa HTTPS y JSON, con versión explícita, esquema de error estable y validación de tamaño y formato. Cada operación registra `request_id`/`correlation_id`; la identidad autorizada se resuelve en el servidor. Los permisos se verifican sobre el recurso concreto en cada comando y consulta, no solo en la vista de interfaz. La base operacional será PostgreSQL/PostGIS en Cloud SQL según RNF-038; la selección del servicio gestionado y su perfil se detallan en 3.4 y 3.6.

El acceso a PostgreSQL se implementará con `pgxpool` y consultas SQL revisables generadas mediante `sqlc` para operaciones estables. Las consultas dinámicas del buscador podrán usar `pgx` con parámetros enlazados y una lista cerrada de filtros y campos de orden. Los nombres de columnas y fragmentos SQL nunca se concatenarán desde texto libre recibido del cliente. `sqlc` permite asociar una instancia de consultas a una transacción, mientras PostgreSQL define el commit o rollback de las operaciones locales [@es2sqlctransactions; @es2pgtransactions]. Los montos se almacenarán en decimales exactos y con moneda; no se empleará coma flotante binaria para importes.

La reserva y su ocupación se crearán en una transacción local única. El calendario de `ocupacion` utiliza intervalos semiabiertos `[inicio, fin)` y la restricción de exclusión definida en el Anexo B, de modo que solicitudes concurrentes sobre un mismo espacio no confirmen rangos incompatibles [@es2pgrangos]. También se guarda en la misma transacción un evento en outbox cuando el cambio de dominio requiera publicación posterior. Los procesos de red no se ejecutan dentro de una transacción de base de datos abierta.

La transacción PostgreSQL **no incluye operaciones de proveedores externos**. Una operación que un proveedor externo haya aceptado no se deshace mediante `ROLLBACK` local. El backend registra el intento y su referencia, procesa una respuesta o webhook autenticado y realiza una operación compensatoria (por ejemplo, solicitar un reembolso) como un hecho nuevo e idempotente cuando proceda. Un timeout de pago deja la operación en estado por conciliar; no se inicia otro cobro con una clave distinta. El adaptador general incluye inicio de operación, consulta, devolución y correlación; reglas, credenciales, estados exactos y pruebas por proveedor se documentarán en el estudio/anexo de integración correspondiente.

El diagrama de secuencia resume el intercambio de reserva y pago. La escritura de la reserva, ocupación y outbox se confirma primero en PostgreSQL; la llamada de pago ocurre después del commit para no retener bloqueos mientras responde la red. Las pruebas aisladas usan simulador y las que invocan un proveedor permanecen en sandbox. La persistencia de eventos externos y la conciliación permiten resolver respuestas tardías sin repetir efectos. El diseño conserva la separación entre commit local y efecto remoto, con una interfaz HTTP definida mediante OpenAPI [@es2pgtransactions; @es2openapispec].

![Secuencia propuesta para reservar y procesar el pago](imagenes/figura-reserva_pago_secuencia.png){width=6.3in} <!--#fig:es2-reserva-pago-secuencia--> <!--#fuente:elaboración propia a partir de PostgreSQL y OpenAPI.-->

## 3.3.4 Máquina de estados del producto completo

La reserva conserva una máquina de estados explícita para pago, aprobación del arrendador, firmas, uso, disputa, liquidación y cancelaciones. La figura muestra el recorrido central; el Anexo B enumera todos los literales y arcos, implementados en la capa de aplicación y registrados en `reserva_transicion`:


![Máquina de estados propuesta para la reserva ES2](imagenes/figura-reserva_estados.png){width=6.3in} <!--#fig:es2-reserva-estados--> <!--#fuente:elaboración propia a partir de las transiciones del Anexo B.-->

`en_disputa` suspende la liquidación mientras se resuelve el reclamo. Al resolverla, la reserva vuelve a `finalizada` y solo pasa a `cerrada` tras conciliación del cierre financiero. Una expiración de retención es también una transición persistida; la búsqueda no la interpreta por una comparación transitoria de hora. El pago y la disputa poseen estados separados de la reserva. La figura es una vista simplificada; los nombres y transiciones exigibles son los del diccionario del Anexo B.

## 3.3.5 Procesos asíncronos y tolerancia a reinicios

La expiración de retenciones, publicación de outbox y conciliación se ejecutan mediante goroutines dentro de la misma aplicación Cloud Run; no se desplegará un Cloud Run Job separado en esta fase. Como las tareas requieren CPU fuera de las solicitudes, el servicio se configura con facturación por instancia y un mínimo de una instancia. Esta configuración tiene costo inactivo y Cloud Run puede reiniciar instancias, por lo que un timer o estado en memoria no ofrece ejecución garantizada [@es2cloudrunbilling].

El trabajo se conserva en PostgreSQL con estado pendiente, número de intentos, disponibilidad/reintento y lease de ejecución. Una goroutine reclama filas de manera transaccional (por ejemplo, `FOR UPDATE SKIP LOCKED` y lease con vencimiento), ejecuta una acción idempotente y registra el resultado. Otra instancia podrá continuar una tarea cuyo lease haya vencido tras un reinicio. El cierre por señal termina solicitudes activas, persiste el checkpoint y libera el trabajo cuando sea seguro. Reintentos, lease y clave idempotente impiden asumir “exactamente una vez”: la ejecución es recuperable y los efectos deben ser repetibles sin duplicación.

El endpoint de reserva valida `expira_en` en línea; la tarea de fondo cancela/libera la retención vencida y genera notificaciones. De ese modo una instancia temporalmente ausente puede retrasar limpieza, pero no hace reservable un rango vencido de forma incorrecta. El máximo de instancias y conexiones por pool se dimensiona conjuntamente con el límite de Cloud SQL; la documentación de Cloud Run advierte que el total posible de conexiones crece con cada instancia [@es2cloudsqlrunconnections].

El diagrama siguiente muestra el ciclo operativo del worker desde el reclamo transaccional hasta el cierre o reintento. PostgreSQL permite `SKIP LOCKED` para evitar contención cuando varios consumidores procesan una tabla tipo cola, aunque advierte que esta lectura no es apropiada para consultas generales [@es2postgreslocking]. Cloud Run permite CPU fuera de solicitudes con facturación por instancia y puede terminar instancias; por ello el lease, el checkpoint y el efecto idempotente son parte del protocolo de recuperación y no una optimización opcional [@es2cloudrunbilling].

![Ciclo de trabajo y recuperación de los workers internos](imagenes/figura-worker_recuperacion.png){width=6.3in} <!--#fig:es2-worker-recuperacion--> <!--#fuente:elaboración propia a partir de PostgreSQL y Cloud Run.-->

## 3.3.6 Eventos de dominio, logs y auditoría

Se diferencian tres registros porque responden preguntas y requisitos distintos:

| Registro | Propósito y contenido | Destino definido |
| --- | --- | --- |
| Log técnico | Errores, duración, módulo, severidad, revisión, request/trace ID y resultado técnico | JSON estructurado a stdout/stderr; Cloud Run recoge los logs en Cloud Logging y permite correlacionarlos con peticiones [@es2cloudrunlogs] |
| Auditoría de negocio | Actor o cuenta de servicio, acción, recurso, resultado, motivo aplicable, fecha y correlación de cambios sensibles | Registro de auditoría propio, separado de Outbox; exportación controlada a almacenamiento con retención acordada para RNF-017. Outbox solo publica el cambio cuando se requiere. |
| Evento analítico | Interacción resumida, exposición/clic de publicación, campaña o respuesta NPS bajo esquema, finalidad y retención definidos | Pub/Sub y BigQuery; no gobierna estados de reserva, pago o disputa |

Los logs omiten contraseñas, tokens, datos de tarjeta, documentos y cuerpos íntegros de solicitudes. La dirección IP no se usa como dimensión geográfica analítica por defecto. Para RNF-017, BigQuery sirve consulta, pero no la garantía de inmutabilidad: se conserva la propuesta de Cloud Storage con retención bloqueada y hash por lote, sujeta a plazo compatible con la matriz del Anexo B y a la prueba de alteración prevista. Una salida JSON y su copia analítica no bastan por sí solas para demostrar inmutabilidad.

## 3.3.7 Outbox, Pub/Sub y BigQuery

Los cambios de negocio que necesitan reflejo analítico (reserva confirmada, pago conciliado, disputa resuelta) escriben un evento de outbox en la misma transacción PostgreSQL. La goroutine publicadora reclama el evento, publica a Pub/Sub y persiste su acuse/resultado de publicación. Una clave de evento estable permite deduplicar; errores dejan el trabajo para reintento y los fallos reiterados llegan a un estado operativo de revisión. Pub/Sub admite una suscripción BigQuery que escribe mensajes por lotes en una tabla existente, sin que EspaciGo mantenga un consumidor propio si no necesita transformar el evento [@es2pubsubbigquery].

Las impresiones y clics de alto volumen de **todas las publicaciones** se reciben por la interfaz de telemetría cuando la aplicación cliente se instrumente y se publican a Pub/Sub con validación de esquema y controles de tasa; no se insertará cada vista en la transacción operativa de PostgreSQL. Los eventos son seudónimos y mínimos, con hora del evento/recepción, categoría, publicación, zona gruesa y campaña cuando aplique. El concepto de impresión, deduplicación, tráfico automatizado y retención deben quedar definidos antes de ofrecer una cifra comercial. BigQuery es el plano de análisis, no la fuente de verdad de pagos/contratos/disputas. **Datastream queda descartado para ES2**: el flujo será de eventos seleccionados, no copia CDC de tablas operacionales.

El diagrama Outbox distingue el commit operacional, la entrega asíncrona y la consulta analítica. Pub/Sub entrega por defecto al menos una vez, por lo que un consumidor puede recibir eventos duplicados y no debe asumir orden global; la vista de consumo debe deduplicar por `event_id` y tolerar reintentos [@es2pubsubdelivery]. La suscripción BigQuery permite exportar mensajes a una tabla, mientras el acceso a reportes se mantiene sujeto a autorización y agregación [@es2pubsubbigquery].

![Flujo Outbox desde la transacción operacional hasta la analítica](imagenes/figura-outbox_analitica.png){width=6.3in} <!--#fig:es2-outbox-analitica--> <!--#fuente:elaboración propia a partir de PostgreSQL, Pub/Sub y BigQuery.-->

## 3.3.8 Métricas premium, NPS y recomendaciones

El backend registra métricas agregables de exposición y clic para publicaciones con independencia de que el arrendador haya comprado un ticket. El acceso al reporte se controla separadamente: la API o el panel verifica el propietario, la publicación y el entitlement premium activo y su período. El servicio entrega cantidades agregadas de alcance, impresiones, clics y CTR, nunca una lista de visitantes. Los eventos del navegador miden interacción, no prueban reserva ni pago; los hechos de conversión provienen del estado operativo PostgreSQL.

Para la primera validación de reportes se usarán vistas de BigQuery en **Looker Studio** con acceso a un conjunto de vendedores nominados. El acceso se habilita solo tras comprobar con dos identidades que cada vendedor consulta sus propias filas. La aplicación de políticas de fila debe usar la identidad que ejecuta la consulta y permisos mínimos; un filtro visual o un informe compartido con credenciales amplias no constituye control de acceso [@es2bigqueryrowsecurity; @es2lookercredentials]. La API autenticada seguirá siendo la forma prevista de reportar a usuarios del marketplace que no posean identidad Google con acceso de consulta.

NPS se modelará como respuesta voluntaria con fecha, finalidad y tratamiento de texto libre separado; no se inferirá de clics. La respuesta operacional necesaria se conservará en PostgreSQL y se exportarán a BigQuery agregados minimizados, con tamaño mínimo de segmento para reducir identificación. Para recomendaciones, primero se compararán reglas transparentes basadas en categoría, disponibilidad, zona amplia y popularidad contra búsquedas y reservas. Un modelo de filtrado colaborativo es una etapa futura sujeta a volumen, evaluación offline y presupuesto; BigQuery ML documenta factorización de matrices para recomendaciones, pero no se adopta como requisito inicial [@es2bqmlrecommendations].

## 3.3.9 Archivos e interfaces externas

Imágenes, contratos y evidencias binarias se almacenan en Cloud Storage, mientras PostgreSQL conserva propietario, clave de objeto, tipo, tamaño, hash, estado de validación y referencia al recurso. El bucket se mantiene privado. Una carga pasa por autorización en backend y URL firmada de corta duración para el objeto específico; el backend verifica tamaño, tipo y hash antes de marcarlo disponible. Quien posee una URL firmada puede usarla durante su vigencia, por lo que no se escribe en logs ni eventos analíticos [@es2gcssignedurls].

Pago, firma e identidad se encapsulan en adaptadores y contratos tipados. El webhook se autentica según el mecanismo real del proveedor, se conserva con su identificador externo y se deduplica antes de aplicar el efecto. Los reintentos dependen de operación e idempotencia, no se aplican ciegamente a un cobro. La investigación específica de Mercado Pago —incluidos medios admitidos, autenticación, split, liquidación, reembolsos y pruebas— se mantiene fuera de esta propuesta base para añadirse como anexo de integración una vez revisada. La propuesta no declara que garantía, custodia o liberación condicionada estén disponibles.

## 3.3.10 Secuencia de implementación y criterios de aceptación

| Etapa | Entregable backend | Evidencia para cerrar |
| --- | --- | --- |
| 1. Esqueleto local | Módulo Go, API/OpenAPI inicial, salud/readiness, logs estructurados, Docker, PostgreSQL/PostGIS local, migraciones y generación `sqlc` | API reproducible desde cero y pipeline local/CI con versión registrada |
| 2. Infraestructura dev | Terraform: state GCS versionado/locking, IAM, Artifact Registry, Secret Manager, Cloud SQL, un servicio API Cloud Run, logging, budgets y alertas | `plan` revisado, infraestructura reproducible, endpoint disponible, límites de instancias/pool y alertas verificadas |
| 3. Identidad/catálogo | Roles y permisos, categorías como datos configurables, publicación/búsqueda, metadatos de objetos | Pruebas de dueño/rol, filtros permitidos, categorías sin migración de esquema y archivos privados autorizados |
| 4. Reserva | Cotización, intervalo y ocupación atómicos, estados, expiración y outbox | PT-01/PT-02, MD-01–MD-03 y MD-07 aplicables; concurrencia no genera doble reserva |
| 5. Pago y operación | Adaptador simulado para pruebas aisladas, integración de proveedor solo sandbox, contrato, check-in y disputa | No existen credenciales productivas; timeout queda conciliable; repetición no duplica operación |
| 6. Trabajos/auditoría | Worker interno con leases, event log, auditoría, transición de estados, restauración | Reinicio retoma trabajo; MD/PT de integridad, recuperación y privacidad con evidencia fechada |
| 7. Analítica de producto | Outbox→Pub/Sub→BigQuery, eventos de interacción, Looker Studio limitado a vendedores de prueba, NPS agregado | Schema/deduplicación y retención; prueba de aislamiento entre identidades y costo revisado |
| 8. Evolución | Reportes API a vendedores, firma/proveedor bajo anexo, recomendaciones y optimización de escala | Integración en sandbox, evaluación de calidad, privacidad y costo antes de habilitar producción |

Estas etapas organizan la construcción; no son evidencia de avance. Los casos PT y MD siguen en estado planificado hasta que haya código, entorno y resultado fechados. El inventario del producto y sus resultados se actualizarán en la matriz de trazabilidad y en pendientes.
