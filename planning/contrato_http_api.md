# Contrato HTTP/JSON común de EspaciGo

Estado: línea base común para tickets de API. `openapi.yaml` define componentes y respuestas reutilizables; cada módulo agrega operaciones/esquemas con trazabilidad a CU/RQF. Este documento no inventa rutas ni reglas de negocio.

## Versionado y representación

- API exclusivamente HTTP/JSON. La raíz versionada es `/api/v1`; cambios incompatibles requieren nueva versión mayor en la ruta. Cambios compatibles se documentan dentro de la versión.
- Las solicitudes y respuestas usan `application/json`, excepto contenido binario expresamente contratado. Los instantes usan RFC 3339 en UTC con `Z` y las fechas civiles usan `YYYY-MM-DD`; los esquemas reutilizables `UtcTimestamp` y `CivilDate` están en `openapi.yaml`.
- El dinero sigue el esquema `Money` de `openapi.yaml`: decimal exacto como string, nunca punto flotante binario; el código de moneda usa la forma ISO 4217. La escala, el redondeo y los límites los fija la regla de negocio del módulo.
- No devolver campos internos, credenciales, secretos ni datos personales que el actor no requiera.

## Autenticación y autorización

- Las operaciones protegidas usan el esquema HTTP Bearer en `Authorization`, según OpenAPI. Esta base no fija formato del token, emisor, claims ni ciclo de vida. Login, renovación y excepciones públicas declaran explícitamente su seguridad en sus tickets; seguridad Bearer es el valor predeterminado.
- Autenticar no implica autorizar. En cada lectura/escritura, el servidor resuelve identidad y comprueba acción, tenant/alcance, relación con el recurso y estado actual desde datos confiables. No confiar en owner, rol, permiso o asociación enviados por el cliente.
- Aplicar autorización a nivel de recurso en cada operación para prevenir acceso directo inseguro por ID. Responder `404` cuando revelar existencia exponga información protegida; documentar la decisión de cada módulo. `403` es para recurso/acción cuya existencia puede revelarse.
- Roles administrativos requieren permiso explícito y auditoría. Minimizar datos en mensajes y logs.
- Toda respuesta `401` incluye `WWW-Authenticate` con un desafío aplicable al recurso, conforme a RFC 9110 §§11.6.1 y 15.5.2.

## Errores y códigos HTTP

Todos los errores JSON siguen `Error` de `openapi.yaml`: `{ "error": { "code": "...", "message": "...", "request_id": "UUID", "details": [...] } }`. `code` es estable para lógica cliente; `message` es seguro y no es una interfaz de control. `details` es opcional y no contiene secretos ni datos de terceros.

Mapeo común (cada operación declara los que aplica): `400` request inválido; `401` credencial ausente/inválida; `403` prohibido; `404` no encontrado/no visible; `409` conflicto de estado, versión o idempotencia; `422` regla semántica no satisfecha si el módulo distingue este caso de `400`; `429` límite de uso; `5xx` fallo servidor. No presentar stack, SQL, tokens, proveedores ni causas internas. Las respuestas incluyen `X-Request-ID` UUID; el servidor lo genera cuando no existe un ID entrante válido y limita cualquier propagación.

## Paginación

Colecciones grandes usan cursor opaco, estable y ligado a filtros/orden, con límite máximo definido por operación. Response: `{ "items": [...], "next_cursor": "..." }`; `next_cursor: null` termina. No prometer total si es costoso o filtrable por autorización. Cada operación documenta filtros, orden determinista y comportamiento ante cursor inválido (`400`). No aceptar cursores que el cliente pueda manipular para saltar controles de acceso.

## Correlación, idempotencia y conflictos

- `X-Request-ID` identifica una petición para correlación; no es autorización ni secreto. El servidor responde con el ID efectivo.
- Operaciones elegibles de creación/efectos no repetibles aceptan `Idempotency-Key` opaca. El contrato por endpoint fija alcance (actor + operación), retención, huella de request, comportamiento ante misma clave/mismo payload, payload distinto (`409`) y respuesta replay. Persistir clave/resultado de forma durable junto con el efecto local cuando aplique; no prometer exactamente-una-vez ante proveedores externos.
- `409 Conflict` indica estado actual incompatible o repetición incompatible; proveer un código estable. Para concurrencia optimista, el ticket define mecanismo (p. ej. ETag/If-Match o versión) antes de implementarlo. No convertir fallos de negocio en `500`.
- Reintentar una operación de proveedor requiere reconciliar intentos ambiguos; timeout no prueba fallo ni autoriza una nueva clave/cobro.

## Guía obligatoria para tickets de endpoints

Cada ticket de módulo debe especificar: (1) CU/RQF y actor; (2) método, ruta y versión; (3) propósito/visibilidad pública; (4) esquema y validaciones de request/response, incluyendo dinero/tiempo; (5) autenticación y matriz acción–recurso–relación/rol; (6) códigos de éxito y errores con ejemplos; (7) paginación/filtros/orden si corresponde; (8) `X-Request-ID` y política de idempotencia si hay efecto repetible; (9) consistencia/concurrencia y auditoría; (10) datos sensibles a excluir; (11) pruebas HTTP/JSON de éxito, 4xx, 5xx, permiso sobre recurso ajeno, límites y OpenAPI lint/parse.

No ampliar esquemas/rutas globales sin revisar impacto a clientes existentes. Los ejemplos del contrato son sintéticos y no contienen secretos.
