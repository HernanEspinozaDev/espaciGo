# Alcance del backend local y transición a Google Cloud

Fecha de revisión: 2026-10-07. Código observado: `main`, commit `1d79b9af38c4c0d2a85b15e7fbc4c89d56cd2345`, merge del PR #176.

> **Línea base histórica (2026-10-07), no estado actual.** Las afirmaciones de esta revisión sobre módulos aún no implementados describen ese checkout. Desde entonces se aceptaron LOCAL-CONT-01 (#216, PR #217) y otras entregas; para el estado conciliado vigente, consulta [matriz](matriz_cierre_backend_local.md), [plan](plan_cierre_backend_local.md) y [backlog](backlog_cierre_backend_local.md). LOCAL-OPS-01 (#218) implementa en su PR actual el corte sintético M08/M10 y sigue pendiente de aceptación.

**Actualización de prioridad por instrucción del usuario:** completar M01–M11 en local antes de proponer GCP. El [plan de cierre local](plan_cierre_backend_local.md) y su backlog sustituyen la secuencia recomendada aquí. La comparación cloud de este documento se conserva como referencia; no es un backlog autorizado ni habilita pruebas o despliegues.

Este documento es una evaluación y propuesta de cierre. No cambia requisitos, estados de Issues, dependencias, infraestructura ni criterios de aceptación. No se ejecutaron pruebas, contenedores, migraciones ni consultas a la base de desarrollo durante esta revisión. Los resultados previos se atribuyen a sus evidencias publicadas; leer una prueba no equivale a volver a ejecutarla.

## 1. Qué significa terminar el desarrollo local

Conviene separar cuatro hitos. No son cuatro implementaciones distintas del sistema: comparten dominio, casos de uso, persistencia y contratos API.

| Hito | Resultado | Situación observada |
| --- | --- | --- |
| A. Núcleo local consolidado | Cuenta → espacio de ensayo → búsqueda/cotización → reserva → pago fake → decisión/cancelación → comunicación, con pruebas reproducibles. | El recorrido existe. Faltan consolidación de criterios, algunas pruebas específicas y una definición de cierre transversal. |
| B. Backend funcional completo en local | M01–M11 cumplen el alcance local acordado, usando datos sintéticos y adaptadores fake donde hay terceros. | No está completo: siete módulos tienen recorridos parciales; M07, M08, M10 y M11 no tienen implementación funcional específica. |
| C. Ensayo restringido en Google Cloud (fuera de la fase vigente) | Futura evaluación de compatibilidad/operación en el entorno objetivo, con alcance que se definirá después. | No encontré código Terraform, workflows CI ni evidencia de despliegue en el repositorio revisado. Por decisión del usuario, no se propone ni ejecuta antes de completar y aceptar B. |
| D. Backend del producto global | Reglas comerciales y privacidad ratificadas, integraciones reales verificadas, operación y pruebas de capacidad/continuidad en el entorno objetivo. | Pendiente. No se demuestra con un despliegue del fake ni con la existencia de documentos de diseño. |

**Definición recomendada para B:** el flujo completo puede ejecutarse desde una base vacía, se conservan datos al actualizar una base existente, cada operación tiene autorización y estados definidos, las integraciones se sustituyen por fakes explícitos, las pruebas críticas se ejecutan sin omisiones y el mock permite comprobar cada módulo. Un fake prueba el contrato y las reglas locales; no acredita firma legal, identidad real, cobro, devolución, liquidación ni documento tributario real.

La elección del frontend definitivo permanece fuera del alcance. El mock HTML/CSS/TypeScript es una herramienta de validación.

## 2. Inventario verificado

- Backend Go modular: `identity`, `privacy`, `verification`, `spaces`, `occupancy`, `pricing`, `booking` y `conversation`, con adaptadores PostgreSQL y handlers HTTP.
- 21 migraciones SQL versionadas en `db/migrations`, V1–V21. La secuencia incluye el migrador con checksum y serialización; sus resultados previos están en las evidencias correspondientes.
- 34 declaraciones `CREATE TABLE` en esas migraciones, contadas estáticamente. Hay tablas específicas de ensayo y extensiones del catálogo. **No equivalen a 34 de las 43 entidades del diccionario global implementadas.** Falta una conciliación entidad por entidad antes de convertir estructuras de ensayo en modelo general.
- Compose con database, migrate, backend, Mailpit y mock; red de datos separada, volumen persistente y secretos locales. La base de pruebas de integración se prepara aparte y es desechable.
- OpenAPI y pruebas de dominio, repositorio, handler, mock y recorridos publicados. No encontré `.github/workflows` en este checkout.
- GitHub devuelve 133 Issues: 54 cerradas y 79 abiertas. De las 104 Issues importadas —incluida la recuperación de AUTH-BE-01—, 28 están cerradas y 76 abiertas. Las otras 29 incluyen subentregas y seguimientos: 26 cerradas y 3 abiertas. Son conteos operativos de Issues, **no un porcentaje funcional ni de esfuerzo**.
- GitHub Issues/Projects son el registro operativo. Parte de `backlog.md`, la matriz M06 y documentos fundacionales todavía contienen frases históricas, como ausencia de código o #74 en revisión. No deben usarse como estado actual sin contrastar merges, Issues y evidencias.

## 3. Estado por módulo

| Módulo | Recorrido construido localmente | Trabajo que falta para el alcance funcional completo local | Diferencia adicional del producto global |
| --- | --- | --- | --- |
| M01 Identidad | Registro, términos de ensayo, verificación/reemisión con Mailpit, login/logout, sesiones, cambio/recuperación, expiración y límites definidos. | Resolver DB02-09: preferencia de uso, historial de claves y avisos durables; conciliar criterios de #31/#34. | Correo productivo, políticas ratificadas y límites compatibles con varias instancias. El limitador local por IP está en memoria. |
| M02 Perfil y privacidad | Consulta/edición del perfil propio y registro/listado de solicitudes de acceso/supresión. | Cuenta de cobro bajo contrato local, foto/almacenamiento si aplica, tramitación administrativa con estados/motivos y retención/desidentificación definida. #37–43. | Vinculación real de vendedor/proveedor y procedimientos de privacidad respaldados por evidencia. La solicitud `en_revision` no demuestra supresión. |
| M03 Verificación | Casos sintéticos KYC/KYB, aprobación/rechazo/reintento manual y PNG sintético privado generado por la API. | Contrato de evidencia/retención y gating de publicación/reserva que consume el estado aprobado, en un corte local expresamente acordado. #45, #48–50. | Proveedor, consentimiento, documentos reales y reglas productivas siguen en #142; no están autorizados por el fixture. |
| M04 Espacios | Borradores propios, ocho categorías, perfiles de atributos versionados, tarifas, zona IANA, bloqueos y horario semanal local por hora. | Ciclo general de publicar/despublicar, elegibilidad, galería privada, reglas comerciales de tarifa/comisión/cancelación y pruebas originales. #52–61. | Oferta comercial real, archivos reales, moderación y condiciones acordadas. Hoy un borrador no se convierte automáticamente en publicación comercial. |
| M05 Búsqueda/cotización | Catálogo de fixtures autorizados, filtros, estimación, proximidad PostGIS, paginación y cotizaciones con snapshot. | Generalizar consultas a oferta publicada, consolidar #62–68 y definir límites de consulta. La paginación actual calcula y ordena resultados antes de recortar la página. | Medir índices, planes de consulta y carga real; direcciones/ubicación deben seguir la política de divulgación acordada. |
| M06 Reserva/pagos | Retención en `ocupacion`, solicitud, aprobación/rechazo, historial, vencimientos, cancelación/devolución fake, pago durable, HMAC/inbox y recuperación tras caída. Mock de pago aceptado en #175. | Consolidar #69/#73/#75/#77 y cierre local de #79. Completar evidencia directa de exclusión y reservas adyacentes; extender lifecycle con contratos/operación/disputas cuando existan esos módulos. | Políticas comerciales, garantías, contrato del proveedor, sandbox y conciliación externa de #76/#78. El historial de transición local no sustituye auditoría global. |
| M07 Contratos/firma | Sin paquete, migraciones ni recorrido específico observado. | Snapshot contractual versionado, estados por firmante, generación/consulta privada, adaptador fake y callbacks idempotentes. #80–87. | Proveedor y validez jurídica de la firma/documento, accesos y retención acordados. |
| M08 Operación | Sin implementación específica de check-in/out observada. | Entrega, recepción/objeción y devolución; ventanas, evidencia privada y transiciones de reserva con pruebas. #88–94. | Evidencia real, excepciones de operación y reglas que afecten garantías/disputas. |
| M09 Comunicación/reputación | Chat por reserva, paginación, idempotencia, autorización y cursor/contador de lectura independiente. | Reseñas elegibles, reportes/moderación y notificaciones persistentes con reintentos; consolidar #95–102. | Canales reales, políticas de contenido y retención. Los avisos directos a Mailpit no son un outbox general durable. |
| M10 Disputas/cierre económico | Sin dominio ni recorrido específico observado; la devolución fake de M06 es una capacidad parcial, no M10. | Reclamos, descargos, resolución motivada, garantía/ledger/liq. simuladas y estado documental explícito. #103–110. | Hechos financieros observados del proveedor y decisiones tributarias/comerciales; no generar un documento legal ficticio como si fuera real. |
| M11 Administración/auditoría | Existe revisión administrativa acotada de M03; no el módulo global. | Gobierno de cuentas, moderación, consulta de auditoría, eventos atómicos y outbox durable con lease/retry/deduplicación. #111–118. | Operación/IAM, retención e inmutabilidad verificadas del mecanismo elegido. BigQuery no prueba por sí solo inmutabilidad. |

**Lectura del avance:** existe un prototipo transaccional que llega hasta reservar, pagar fake y comunicar. El backend completo todavía requiere cuatro módulos funcionales nuevos, completar los siete parciales y cerrar capacidades transversales. No faltan únicamente pruebas o desplegar Docker en la nube.

## 4. Alcance de las pruebas

| Área | Evidencia existente | Cierre pendiente |
| --- | --- | --- |
| Migrador/DB | Reconstrucción vacía, repetición, checksums, deriva, rollback y concurrencia, con scripts PostgreSQL aislados. | Integrar la comprobación de toda la secuencia en el gate de release; respaldo/restauración no demostrado por mantener el volumen. |
| Identidad/permisos | Pruebas de sesiones, credenciales, tokens y ownership en varios módulos. | Matriz transversal por endpoint/rol/estado y resolución de brechas de privacidad/historial. |
| Reserva/calendario | Solicitudes solapadas concurrentes: un éxito y un conflicto; expiración y liberación; pruebas con reloj y bloqueo. | SQLSTATE `23P01` observado directamente en una prueba de la restricción y dos reservas adyacentes realmente creadas. No basta probar opciones ofrecidas. |
| Pago fake | Inbox HMAC, replay, timeout durable, idempotencia y recuperación tras reinicio incluso después de vencer/cancelar. #172/#174. | Contrastar los criterios originales locales; sandbox externo y firma del proveedor se prueban aparte. |
| API/mock | Handlers, OpenAPI, errores, carreras, limpieza de sesión y recorridos publicados; #176 añade prevalencia del estado confirmado. | Gate reproducible del núcleo con cuentas/fixtures conocidos. Parsear YAML no sustituye validar requests/responses contra OpenAPI. |
| Continuidad/operación | Health/readiness y limpieza de bases temporales tienen evidencia. | Backup/restore local, actualización con datos, reinicio de workers, CI que no pase integraciones omitidas y observabilidad mínima. |
| No funcional | Hay invariantes y concurrencia enfocada. | Rendimiento con escenarios de carga, capacidad, RPO/RTO y seguridad/privacidad de release; las metas de nube requieren medición en entorno comparable. |

Algunos tests PostgreSQL llaman `t.Skip` cuando falta `TEST_DATABASE_URL`. Por eso `go test ./...` sin esa variable no acredita por sí solo toda la integración. Para cerrar el hito se debe registrar qué suite/script ejecutó realmente esos casos.

No recomiendo repetir todas las suites después de cada cambio documental. Ejecutar pruebas enfocadas por cambio y un gate consolidado por hito/release; registrar el commit y conservar evidencias previas cuando sean aplicables.

## 5. Comparación local frente a Google Cloud

La arquitectura documentada propone un monolito modular Go en Cloud Run, Cloud SQL PostgreSQL, Cloud Storage privado, Secret Manager y Artifact Registry, con Terraform. Pub/Sub/BigQuery se reservan para eventos/analítica seleccionados. La región acordada es Santiago; el dimensionamiento sigue siendo una propuesta hasta medirlo.

| Capacidad | Local actual | Trabajo para ensayo GCP | Trabajo adicional para operación global |
| --- | --- | --- | --- |
| API | Contenedor Go con `HTTP_ADDR`; las rutas funcionales se registran dentro de `LOCAL_AUTH_PROTOTYPE=1`, y reservas bajo `LOCAL_BOOKING_TRIAL=1`. | Separar composición por ambiente: rutas de negocio normales y selección explícita de adaptadores fake. Configurar puerto, HTTPS externo, orígenes y despliegue versionado. | Excluir herramientas/fixtures de desarrollo del perfil productivo y estabilizar contrato API general. |
| PostgreSQL | Imagen PostGIS fijada, pgx, migrador, roles y volumen Docker. | Cloud SQL, conexión/credenciales, extensiones/versiones, permisos y ejecución controlada de migraciones. `config.go` fija actualmente `sslmode=disable`; elegir transporte seguro apropiado a TLS/conector/proxy en vez de transportar la configuración local sin revisión. | HA según objetivo, PITR/backups, restauración, mantenimiento y límite de conexiones por réplica. |
| Archivos | Evidencia sintética en bind mount privado del host. | Adaptador Cloud Storage privado; metadatos en SQL y autorización en API. No confiar en disco del contenedor para persistir blobs. | Cargas reales, retención, finalidades, validación y acceso revocable según políticas. |
| Secretos | Archivos ignorados de `.local/secrets`. | Secret Manager y cuentas de servicio con IAM. No copiar secretos locales a Git, imágenes o Terraform literal. | Rotación y procedimientos operacionales. |
| Correo/terceros | Mailpit y adaptadores fake. | Mantener fakes identificados para ensayo sintético; Mailpit no acredita entrega real. | Adaptadores reales/sandbox, OAuth/firmas/callbacks, credenciales y conciliación del tercero. |
| Workers | Reconciliador dentro de la API; estado de pago durable. | Probar reinicio y varias instancias, CPU disponible para tareas de fondo y configuración de escalado. Completar lease/retry donde corresponda. | Outbox general, entregas idempotentes y operación ante fallos prolongados. |
| CI/despliegue | Tests/scripts locales; sin workflow alojado encontrado. | Pipeline: build, pruebas realmente ejecutadas, imagen en Artifact Registry, migración, despliegue y smoke/rollback. | Promoción de ambientes, alertas y respuesta a incidentes. |
| Observabilidad/costo | Readiness y logs locales. | Correlación, logs mínimos, métricas/alertas y presupuestos de ensayo. | Medir SLA/carga/RPO/RTO y facturas reales; no confundir presupuesto documental con gasto/capacidad observados. |

Google documenta PostgreSQL 18 en Cloud SQL y soporte de PostGIS/`btree_gist`; esto permite plantear compatibilidad, pero no prueba que la imagen local y la instancia objetivo tengan idénticas versiones o privilegios. [Versiones Cloud SQL](https://docs.cloud.google.com/sql/docs/postgres/db-versions), [extensiones](https://docs.cloud.google.com/sql/docs/postgres/extensions).

Cloud Run usa el puerto configurado para la entrada y su filesystem normal no conserva datos al terminar la instancia. Esa es la razón concreta para adaptar configuración y almacenamiento. [Contrato del contenedor](https://docs.cloud.google.com/run/docs/container-contract).

Los workers de fondo requieren una configuración que les asigne CPU fuera de solicitudes; la propuesta existente de facturación por instancia y mínimo de instancias debe validarse con costo y pruebas de reinicio. [Facturación/CPU](https://docs.cloud.google.com/run/docs/configuring/billing-settings). La conexión y las referencias de secretos tienen guías específicas: [Cloud SQL desde Cloud Run](https://docs.cloud.google.com/sql/docs/postgres/connect-run), [Secret Manager](https://docs.cloud.google.com/run/docs/configuring/services/secrets).

No se inspeccionó una cuenta GCP durante esta revisión. La ausencia de archivos/evidencias aquí no prueba que nadie haya creado recursos fuera del repositorio.

## 6. Plan recomendado de cierre

### A. Cerrar el núcleo ya implementado

1. Una matriz de aceptación local por criterio para M01–M06 y chat M09: hecho con evidencia, falta código, falta prueba o depende de decisión/proveedor. Reutilizar PRs fusionados; no reimplementar funcionalidades.
2. Completar #77 local: restricción PostgreSQL con `23P01`, API/servicio con conflicto y dos reservas adyacentes reales. Consolidar #73/#75 local en la misma entrega.
3. Resolver DB02-09 y procedimiento mínimo M02/gating M03 que afecten el cierre elegido. Separar explícitamente las partes todavía excluidas.
4. Añadir la base transversal de auditoría/outbox necesaria, con trazabilidad a #112–114, sin esperar a que un fake de correo se considere entrega durable.
5. Un gate final reproducible: migrar desde vacío y con datos previos, flujo API del núcleo, pruebas PostgreSQL sin omisiones, mock por módulo, reinicio y backup/restore en una base aislada. Incorporarlo a CI en una tarea concreta.

### B. Completar el backend local integral

Después del núcleo: ciclo de publicación/storage general y privacidad/KYC local → M07 contratos/firma fake → M08 check-in/out → M09 reseñas/avisos → M10 disputa/cierre simulado → M11 gobierno y auditoría. Integrar transiciones de reserva y pruebas a medida que se incorporen esos módulos. Cada módulo termina con API y un mock mínimo; no ampliar la UI definitiva.

### C. Preparar ensayo GCP

Fase posterior deshabilitada en el plan actual. Primero se completa y acepta B mediante LOCAL-1. Después, con una instrucción nueva, podrá elaborarse una planificación cloud; no preparar Terraform, crear recursos ni ejecutar pruebas de GCP durante el cierre local. La alternativa inicial de avanzar a C tras A queda descartada por la instrucción posterior del usuario.

**No hay una estimación fiable en días ni un porcentaje global.** Faltan esfuerzo por criterio, políticas y capacidad semanal. Se puede estimar después de fijar A/B/C como hitos y descomponer los trabajos pendientes. Conteos de tarjetas, módulos parciales o tablas no son unidades equivalentes de esfuerzo.

## 7. Evaluación de la propuesta #73 → #75 → #77

La brecha descrita es real: `repository_integration_test.go` comprueba una solicitud confirmada y otra en conflicto, pero no observa directamente el SQLSTATE; el mapper convierte `23P01` y `23505` en `ErrConflict`. Las comprobaciones de adyacencia visibles corresponden al selector, no a dos reservas creadas. V9 sí declara la exclusión GiST sobre intervalos activos `[inicio,fin)`.

GitHub confirma las dependencias nativas: #77 espera #75 y #24; #75 espera #73 y #23; #73 espera #65 y #72. #24/#23/#72 están cerradas y #65/#73/#75 abiertas.

**Recomendación:** autorizar una sola consolidación local bajo #77 con criterios acotados y dependencias de entregas locales aceptadas. Documentar en esa misma entrega qué satisfacen los recorridos fusionados de #73/#75. Mantener las Issues generales abiertas si conservan criterios incumplidos. Registrar la separación en Projects antes de comenzar; no eliminar bloqueos generales ni esperar al proveedor real para probar invariantes ya implementadas.

Pruebas requeridas de esa entrega:

- Dos transacciones/conexiones incompatibles: una confirma; la otra obtiene `23P01` de PostgreSQL en la prueba de restricción. No exponer el SQLSTATE como respuesta pública.
- El servicio/API traduce el conflicto a su error/HTTP 409 y no deja una reserva/ocupación adicional ni historial parcial.
- Dos reservas reales del mismo espacio en `[10:00,11:00)` y `[11:00,12:00)` confirman y conservan ocupaciones activas distintas.
- Un solape estricto se rechaza; reloj y fixtures controlados, base PostgreSQL desechable y rol runtime.

Un único PR enfocado con evidencia, sin migraciones si la restricción existente basta, sin proveedor externo, sin nuevas funcionalidades ni borrado del volumen. Esta propuesta no declara cerrado el backend local integral ni M06 general.

## 8. Fuentes internas de esta revisión

- `vision_y_modulos.md`, `backend_y_api.md`, `base_de_datos.md`, `pruebas.md` y snapshots ES1/ES2: alcance y diseño, no estados actuales.
- `cmd/api/main.go`, `cmd/api/config.go`, `compose.yaml`, `Dockerfile`, migraciones V1–V21 y paquetes `internal/`: implementación observada.
- `evidence/m06-booking-payment-inbox-20261007.md`, `evidence/m06-local-payment-api-20261007.md`, `evidence/m06-local-mock-payment-20261007.md` y evidencias anteriores: resultados publicados por entrega.
- Issues vigentes y dependencias nativas de `HernanEspinozaDev/espaciGo`; merge del PR #176 confirmado por GitHub. [Project](https://github.com/users/HernanEspinozaDev/projects/1).
- La línea académica permanece congelada/histórica donde corresponde. Esta revisión no modifica ES1/ES2 ni declara cumplimiento legal, aprobación de políticas o resultados no ejecutados.
