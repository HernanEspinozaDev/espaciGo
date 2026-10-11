# Contexto operativo para agentes de desarrollo

## Actualización vigente posterior a PR #264 — 2026-10-11

PR #260 se fusionó en `main` (`38c3e1300b6d3f843238f4ee6b8758bfe3b71913`) y #256 fue aceptada/Cerrada/Hecho por portada y modalidad. PR #261 integró búsqueda textual, mapa sintético, superficie y limpiar filtros; #257 fue aceptada/Cerrada/Hecho. PR #262 integró #258 (RQF-091/092, eliminación con conservación histórica) y PR #263 integró #259 (RQF-177, cierre tras liquidación fake); ambas Issues fueron aceptadas/Cerradas/Hecho solo por sus slices. V50/V51 y después V52 se aplicaron incrementalmente; la regresión integral LOCAL-1 pasó con el mismo código que quedó integrado en PR #264. Se comprobaron los permisos efectivos del runtime y la limpieza acotada en PostgreSQL desechable. #246 sigue abierta hasta la aceptación explícita de LOCAL-1. Conservar volumen/secretos y usar DB desechable para pruebas. Sin GCP. Véanse matriz/evidencias e Issues.

## Estrategia histórica — cierre backend local, conciliada con main #243 (2026-10-10)

El baseline integrado es PR #243, merge `65e5a0850c3e04710db5c022b21102a2a8726c5a`. LOCAL-QA-01/#238, LOCAL-CORE-02/#240 y LOCAL-QA-02/#242 están aceptadas/Cerradas/Hecho únicamente por sus alcances; QA-01/CORE-02/QA-02 aportan runner, continuidad durable y actualización/restauración aisladas ya evidenciadas. LOCAL-QA-03/#244 fue aceptada por su alcance mediante PR #245. La conciliación vigente posterior a #264 está arriba; no presenta LOCAL-1 como terminado. La aceptación QA-02 no declara M01–M11 ni LOCAL-1 terminados. #109/#110, #111–#118 y #185/#40 siguen abiertos por criterios generales/productivos. Preservar checkout, volumen persistente, secretos y cambios locales ajenos; no realizar trabajo de GCP.

Los cortes locales aceptados se concilian en [la matriz de cierre local](matriz_cierre_backend_local.md): LOCAL-BOOK-02/#214, LOCAL-CONT-01/#216, LOCAL-OPS-01/#218, LOCAL-COMM-01/#220, LOCAL-DIS-01/#222 y LOCAL-FIN-01/#224 cumplen solo sus alcances; PR #225 aplicó V40 incrementalmente. V44/V45 de #236 se aplicaron incrementalmente después de la aceptación; no modificar V40 ni recrear el volumen. LOCAL-KYC-01/#200 y la publicación/edición/galería M04 (#204–#212) siguen parciales respecto de sus padres. Exportación y baja local (#192/#194/#196) no completan #185/#40; V26 conserva la reaplicación de bajas en restauraciones aisladas. #142, proveedor real, M06/M10 generales y #123 siguen separados. El cierre se ejecuta mediante L1–L5 en [el plan](plan_cierre_backend_local.md), sin trabajo de GCP.

LOCAL-ADMIN-01A reutilizó #180, #222, #224 y #214; su alcance aceptado está en `evidence/local-admin-01a-20261009.md`. LOCAL-ADMIN-01B/#228 reutiliza `evento_auditoria_local`, consulta/exporta campos minimizados y audita peticiones administrativas autenticadas; aceptación y smoke HTTP posterior al merge registrados en su evidencia. No amplía productores ni altera el ledger. #111–#114 se revisaron: el registro local existe, pero cobertura transversal/outbox/durabilidad y contratos generales siguen abiertos. LOCAL-DIS-TEST-MOCK-01/#230 quedó aceptada, cerrada y Hecho tras PR #231, merge `ca259396`; su evidencia concilia #109/#110 con #222/#225 y muestra la prueba de API y recorrido con anfitrión/arrendatario/admin distintos. #109/#110 no se cerrarán por un fake parcial: #236 ensaya liquidación local, pero proveedor real, comisión productiva, documento fiscal y criterios generales siguen pendientes. LOCAL-ADMIN-ARCH-01/#232 documenta el diseño de permisos/ownership, productores auditables, contrato outbox y uso de #112–#118; las decisiones CU-43/CU-45 quedaron ratificadas para un alcance local parcial. LOCAL-ADMIN-01C/#233 implementa ese corte y fue aceptada por su alcance; no completa M11 ni CU-45. LOCAL-QA-01/#238 ahora consolida las comprobaciones locales, sin declarar completos LOCAL-QA-03 ni LOCAL-1. #111 conserva su dependencia nativa abierta de #110. El Project operativo es la fuente de estado.

La base `pgdata` y secretos locales son persistentes: aplicar migraciones incrementales, nunca ejecutar `clean` ni pruebas destructivas contra ella. Las pruebas automáticas PostgreSQL usan una instancia/base de prueba separada. Reutilizar imágenes/cachés. DB02-09 RQF-213–218 tiene función local implementada en #183/#199/#251; los párrafos históricos que describen sus brechas no prevalecen. Las decisiones de retención aún pendientes están listadas en la matriz vigente. Para el corte M03 #47 existe autorización local temporal únicamente para fixtures PNG sintéticos; archivos reales, proveedor, consentimiento y retención siguen pendientes en #142. Pasos y limpieza puntual: [evidencia sintética M03](evidence/m03-evidencia-sintetica-local.md). Comando y límites del recorrido: [prototipo_local_m02.md](prototipo_local_m02.md); decisión de datos: [decisiones_m02_perfil_privacidad.md](decisiones_m02_perfil_privacidad.md). GitHub Issues y Projects son el registro operativo; no reimportar tarjetas ni reactivar Hermes.

Este archivo permite trabajar dentro del repositorio `espaciGo` sin leer el repositorio académico vecino. Los requisitos y la arquitectura que justifican el backlog están versionados en [referencias](referencias/README.md); el [backlog](backlog.md) y esta política son la guía operativa de ejecución.

## Qué se construye y por qué

EspaciGo es un marketplace transaccional para publicar espacios físicos subutilizados y reservarlos de forma flexible. Quiere reducir la fricción entre arrendadores con espacios ociosos y pymes/emprendedores/profesionales que los necesitan por períodos breves. No es solo un catálogo: debe coordinar identidad, publicación, precio/disponibilidad, reserva, pago externo, contrato, evidencia de uso, disputa, liquidación y auditoría.

La formulación nació en ES1. La demanda por categoría, la economía, el acceso a proveedores y la capacidad de operar custodia siguen sin validarse por completo. Diseñar una función o tener un fake/test no demuestra que la empresa, la integración o el producto sean viables.

## Orden y límites obligatorios

1. Base de Datos.
2. Backend.
3. API HTTP/JSON.
4. Pruebas.
5. Mock visual temporal al final de cada módulo.

El frontend definitivo se decidirá después. No anticipar framework, arquitectura, UX/UI, sistema de diseño, navegación, estado global ni librerías de producción. El mock usa solo HTML, CSS mínimo, TypeScript compilado, módulos ES nativos, DOM y `fetch`; va en contenedor propio y consume exclusivamente API pública. No HTMX por defecto, no HTML generado por backend y nunca DB desde browser/mock.

## Arquitectura de referencia

- Backend: Go como **monolito modular** en una unidad desplegable; interfaces/límites por dominio, no microservicio por módulo.
- Operacional: PostgreSQL 18 + PostGIS + `btree_gist` es el objetivo del modelo; comprobar versión/extensiones antes de fijarla en el entorno.
- SQL: `pgx/pgxpool` y `sqlc` para consultas revisables, transacciones explícitas y errores comprobados; filtros dinámicos usan parámetros y allowlist.
- Infraestructura objetivo ES2: GCP/Terraform, Cloud Run/Cloud SQL, Cloud Storage privado, Pub/Sub y BigQuery para analítica. La construcción local DB/backend/API va primero.
- Eventos: Outbox en la transacción local → publicador idempotente → Pub/Sub/analítica. El sistema operativo no deriva estados de BigQuery.
- Privacidad: Ley 21.719 como criterio de diseño desde primer incremento; no equivale a vigencia anticipada ni cumplimiento probado. Finalidad, mínimo dato, autorización por recurso, retención y borrado/desidentificación se revisan por tipo.
- Workers: trabajo durable en PostgreSQL, claims/leases/reintentos idempotentes; una goroutine/memoria/timer local no es garantía de entrega.

## Invariantes que no se deben romper

- Una publicación es una unidad física reservable exclusiva; todas las categorías viven en catálogo de datos.
- Reserva y bloqueo manual usan un solo calendario `ocupacion`; intervalos finitos semiabiertos `[inicio, fin)`, con restricción de exclusión para impedir solapes concurrentes. Adyacencia sí se permite.
- Cotizar no reserva. Al confirmar, se revalida disponibilidad y se snapshottean tarifa/condiciones; reglas futuras no reescriben acuerdos existentes.
- `reserva.estado`, `pago.estado`, garantía, liquidación y movimientos financieros son conceptos distintos. Montos exactos y moneda explícita; nunca `float`.
- Una transacción DB no puede deshacer una operación que ya aceptó un proveedor externo. Timeout va a conciliación; no reintentar un cobro con otra clave sin resolver el anterior.
- Proveedores de pago/firma/KYC/correo/storage se aíslan detrás de puertos. Desarrollo/CI/staging usan sandbox o fake; simulación local no es integración real. **No llamar Split “Escrow”** ni prometer retener/liberar fondos hasta evidencias de capacidad/contrato.
- Webhooks se autentican, deduplican y correlacionan; se persisten antes de producir efectos de dominio.
- Hechos históricos de pago, contrato, evidencia, solicitud de titular y auditoría no se borran en cascada. El UUID puede seguir siendo dato personal vinculable.
- Bucket de objetos privado; no persistir URLs firmadas ni tokens/secretos de proveedor; verificar autorización, tamaño, MIME real y hash.
- BigQuery es analítico, no OLTP y no garantiza inmutabilidad. La propuesta ES2 para RNF-017 es exportación minimizada con retención bloqueada/hash por lote, pero plazo, alcance y ensayo siguen pendientes.

## Mapa de módulos

El detalle exacto de IDs está en [visión y módulos](vision_y_modulos.md); los textos completos están en `referencias/ES1/`.

| Módulo | Trabajo |
| --- | --- |
| M01 | Cuenta, términos, autenticación, sesión y credenciales. |
| M02 | Perfil, datos de cobro y derechos de titulares. |
| M03 | KYC/KYB y revisión manual/externa. |
| M04 | Catálogo, publicaciones, tarifas/políticas, archivos y calendario. |
| M05 | Búsqueda geográfica, filtros, detalle y cotización. |
| M06 | Reserva, ocupación atómica, pago idempotente y conciliación. |
| M07 | Contratos, firmas y documentos. |
| M08 | Check-in/out, recepción y evidencia. |
| M09 | Mensajes por reserva, reseñas, moderación/reportes y avisos. |
| M10 | Disputas, liquidación observada y documento tributario. |
| M11 | Gobierno de cuentas, administración, reportes, auditoría/outbox. |

## Prioridad de fuentes y decisiones

En caso de contradicción, aplica este orden:

1. Instrucción directa más reciente del usuario y restricciones del entorno/repositorio.
2. Tarjeta activa de [backlog](backlog.md), incluyendo sus dependencias y criterios de aceptación; nunca exceder su alcance.
3. Esta política y decisiones vigentes resumidas en este directorio.
4. Snapshot técnico de ES2, especialmente propuesta de backend y Anexo B de `referencias/ES2/`.
5. Requisitos congelados ES1 de `referencias/ES1/`.
La descripción del entorno de desarrollo está en [entorno_de_desarrollo.md](entorno_de_desarrollo.md). Sirve para orientar la verificación de herramientas locales; no es fuente de requisitos, stack del producto ni arquitectura y no modifica las fuentes anteriores.

## Continuidad operativa en GitHub Projects

El registro operativo es el Project [EspaciGo — Desarrollo](https://github.com/users/HernanEspinozaDev/projects/1) y sus Issues; la correspondencia Hermes–GitHub está en [github_issue_map.csv](github_issue_map.csv) como referencia de migración, no como estado vigente. Mantén intactas las fuentes académicas y sus requisitos: `backlog.md` conserva las tarjetas originales y `t_112e6827` preserva la recuperación como Issue 120. No despaches nuevos workers en Hermes Kanban; su board se archivó y permanece como histórico.

Antes de iniciar una tarjeta, lee su Issue completa, dependencias `blocked_by`, criterios, referencias y evidencia actual de PR/CI; no derives aceptación de un estado histórico `done`. Solo la aceptación verificada permite `Hecho`. Los ejemplos #148/#151/#153/#156 son snapshots de entregas tempranas, no estado actual. Los Issues padre permanecen abiertos por criterios generales; DB02-09 RQF-213–218 tiene función local implementada y el estado de sus hijos se detalla en la matriz. #123 continúa independiente. LOCAL-1 no está listo por la brecha del ZIP contractual y decisiones de retención pendientes. Consulta [la guía para continuar desde el PC](ejecucion_con_github_projects.md). No habilites auto-merge, no introduzcas cambios de producto fuera de Issues autorizadas y no alteres/reinicies trabajo local.

## Cómo resolver vacíos

Si una tarjeta no contiene criterios suficientes, el agente lee la fila RQF, ficha CU y HU concreta dentro de `referencias/ES1/`, y el diseño de tabla/privacidad en `referencias/ES2/`. Si continúa la ambigüedad, documenta el hallazgo, impacto y pregunta/decisión necesaria; crea un ticket de refinamiento con trazabilidad. No inventa respuesta, requisito, regla legal, proveedor ni permiso.

Las capacidades que dependen de contador, municipio, proveedor, docentes, investigación de mercado o medición real permanecen pendientes. Los agentes pueden diseñar puertos y fakes dentro de una tarjeta; no declarar resuelta la dependencia externa.
