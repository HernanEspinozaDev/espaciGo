# Contexto operativo para agentes de desarrollo

## Estrategia vigente autorizada — prototipo local M01, 2026-10-05

La instrucción del usuario de priorizar el recorrido funcional en PR [#124](https://github.com/HernanEspinozaDev/espaciGo/pull/124) prevalece sobre las puertas históricas de merge por capa descritas abajo. Se conservan sus commits y pruebas. #30, #32, #34 y #35 se agrupan en el hito [Prototipo local M01](https://github.com/HernanEspinozaDev/espaciGo/milestone/1). DB → backend → API → mock se integra en la misma rama: una dependencia implementada e integrada permite avanzar, aunque aún no esté fusionada en main. Las dependencias nativas conservan la aceptación completa de las Issues; recuperación/CU-05/PT-50 de #34/#35 queda para el siguiente corte con #31/#33. No cerrar estas Issues ni afirmar M01 completo antes de revisar lo faltante.

GitHub Issues y Projects siguen siendo el registro operativo; este ajuste documenta el motivo y no replica estados del tablero. En la secuencia nativa se quitó #32 de los bloqueadores de #34 porque su API ya está integrada en esta rama, y se pasó #33 a seguimiento del próximo corte porque la recuperación queda fuera; #24 permanece como base de pruebas satisfecha. Se quitó #34 del bloqueo de #35 porque el flujo de pruebas está integrado y el mock se ejecutó en este mismo corte; #25, entorno local ya entregado, permanece como base satisfecha. Los textos originales de aceptación/dependencias y referencias permanecen en las Issues; los comentarios registran el cambio de etapa sin cerrar tarjetas. No reimportar tarjetas ni reactivar Hermes. Controles productivos por IP/correo pueden usar adaptadores locales explícitos para este prototipo, con evolución pendiente documentada. DB02-09 y la limpieza del servidor #123 quedan fuera. Comando, pasos y límites: [prototipo_local_m01.md](prototipo_local_m01.md).

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

Al 2026-10-05, el registro de ejecución vigente es el Project [EspaciGo — Desarrollo](https://github.com/users/HernanEspinozaDev/projects/1) y sus 104 Issues; la correspondencia Hermes–GitHub está en [github_issue_map.csv](github_issue_map.csv). Mantén intactas las fuentes académicas y sus requisitos: `backlog.md` conserva el detalle de 103 tarjetas originales, y `t_112e6827` preserva la recuperación como Issue 120. No despaches nuevos workers en Hermes Kanban; su board se archivó y permanece como histórico.

Antes de iniciar una tarjeta, lee su Issue completa, dependencias `blocked_by`, criterios, referencias y evidencia actual de PR/CI; no derives aceptación de un estado histórico `done`. Solo la aceptación verificada permite `Hecho`. En este corte hay 13 `Hecho`, 2 `En revisión` y 89 `Bloqueado`, sin tarjetas `Listo`; AUTH-BE-01 (#29) y su recuperación (#120) siguen en revisión por PR #16 abierto, y AUTH-BE-02 (#30)/AUTH-BE-03 (#31) siguen bloqueadas por ambas. Consulta [la guía para continuar desde el PC](ejecucion_con_github_projects.md). No habilites auto-merge, no introduzcas cambios de producto fuera de una Issue autorizada y no alteres/reinicies trabajo local.

## Cómo resolver vacíos

Si una tarjeta no contiene criterios suficientes, el agente lee la fila RQF, ficha CU y HU concreta dentro de `referencias/ES1/`, y el diseño de tabla/privacidad en `referencias/ES2/`. Si continúa la ambigüedad, documenta el hallazgo, impacto y pregunta/decisión necesaria; crea un ticket de refinamiento con trazabilidad. No inventa respuesta, requisito, regla legal, proveedor ni permiso.

Las capacidades que dependen de contador, municipio, proveedor, docentes, investigación de mercado o medición real permanecen pendientes. Los agentes pueden diseñar puertos y fakes dentro de una tarjeta; no declarar resuelta la dependencia externa.
