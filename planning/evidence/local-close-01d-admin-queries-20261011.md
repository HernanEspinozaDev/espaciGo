# LOCAL-CLOSE-01D — consultas administrativas locales

Estado: implementación publicada en rama para revisión; #250 permanece abierta y LOCAL-CLOSE-01/#246 sigue En curso. El merge de #253 integró LOCAL-CLOSE-01C/#249, cuyo alcance fue aceptado por el usuario; no se cierra LOCAL-1 con esta evidencia.

## Alcance implementado

- Búsqueda administrativa por UUID técnico exacto de cuenta sintética; respuesta mínima con estado, fecha de creación, bloqueo y roles. No consulta ni revela correo, RUT, contacto, credenciales o texto libre. También audita una búsqueda sin resultado.
- Lista y detalle de publicaciones activas, orden estable descendente por actualización e ID. La proyección omite dirección, descripción, atributos privados y contacto. Borradores y publicaciones ocultas no aparecen.
- Tercer reporte local ratificado para CU-45: reclamos sintéticos cuyo `abierto_en` cae en el período obligatorio (desde inclusivo, hasta exclusivo, hasta 31 días calendario en zona IANA), agrupados por estado vigente al generar el reporte. No reconstruye estados históricos ni muestra texto, evidencia o identidades.
- Solo lectura, administrador activo, una auditoría mínima por petición y ninguna mutación financiera o de negocio. No hay paginación/cursor para la lista de publicaciones porque el criterio ratificado de este slice no fijó esos parámetros.
- Mock integrado con controles para búsqueda de cuenta, publicaciones y reporte de disputas; cambio de sesión o criterio invalida la respuesta en curso y limpia datos visibles. No se añadió proveedor ni migración.

## Evidencia ejecutada

- `GO_TEST_RUN='^TestLocalAdminAccountPublicationAndDisputeQueriesAreMinimalAndAudited$' bash scripts/test-local-comm-mailpit.sh ./internal/adapters/postgres/identity` — PASS en PostgreSQL desechable. Usa router/handler ensamblados y autenticación reales; comprueba administrador, rechazo de no administrador, respuesta HTTP/JSON de cuenta/publicación/reporte, omisión de datos privados, auditoría por consulta, y que la lectura no cambia el estado de la publicación.
- `GO_TEST_RUN='^TestLocalBookingTrialPostgresLifecycleAndConcurrentRetry$' bash scripts/test-local-comm-mailpit.sh ./internal/adapters/postgres/booking` — PASS en PostgreSQL/PostGIS desechable. Verifica el agregado de disputas al abrir y cerrar, conserva las demás validaciones del recorrido que esta prueba cubre.
- `go test ./internal/adminlocal/... ./cmd/api` — PASS.
- `npm --prefix mock run build` — PASS.
- `node --test mock/test/admin-local-state.test.mjs` — PASS; descarta resultados tras cambio de criterios/sesión/rol.
- `python3` + PyYAML parseó `planning/openapi.yaml` — PASS. La validación HTTP del handler comprueba los campos de respuesta representativos contra el contrato OpenAPI.
- `git diff --check` — PASS.

No se ejecutó todavía la regresión integral LOCAL-1. El usuario indicó reservarla para después de integrar #250 y conciliar la matriz. El entorno persistente está en main/V48 y saludable; ninguna prueba de este corte usó ese volumen para mutar datos. No se inició GCP.

## Pendientes conservados

#250 requiere revisión y merge; #246 permanece abierta. La regresión integral LOCAL-1 debe ejecutarse después de integrar #250. Los criterios generales de M11, #185/#40, M08/M10 y las demás Issues padre permanecen abiertos según sus propios criterios. Estos reportes son exclusivamente locales y no sustituyen otros criterios CU-45 no especificados.

## Corrección de autorización del mock (#254)

- Un 403 de una consulta administrativa vigente limpia resultados y criterios e invalida respuestas pendientes, aunque la revisión de filtros haya cambiado. La vigencia de autorización se verifica por cuenta, token y generación; los datos de resultado conservan además su revisión/filtros.
- El mock conserva la sesión al recibir el 403, oculta temporalmente el acceso administrativo y vuelve a consultar `/session`. Aplica roles/modo restringido solo si la cuenta, token y generación siguen siendo los mismos. Si el backend confirma que falta el rol administrador o que la sesión está restringida, vuelve a limpiar los resultados administrativos; la sesión sigue disponible para operaciones permitidas.
- Las pruebas simulan 403 después de cambiar filtros, pérdida de rol, modo restringido, un 403 tardío de sesión anterior y una consulta de roles tardía tras logout/relogin. Una respuesta anterior no limpia ni modifica la nueva sesión.
- La acción manual «Consultar sesión» captura cuenta, token y generación antes de la petición y comprueba que continúan vigentes antes de aplicar roles/modo restringido o mostrar la respuesta. Los cambios de filtros no invalidan esa comprobación de sesión.
- Si la API confirma pérdida del rol administrador o modo restringido, se conserva el token/sesión para operaciones permitidas, pero se limpia la vista administrativa y aumenta su revisión para descartar respuestas pendientes.
- Las pruebas enfocadas cubren consulta manual con pérdida de rol, consulta manual con modo restringido y respuesta manual tardía tras logout/login de la misma cuenta con token y generación nuevos. El resultado tardío no aplica roles ni limpia la sesión nueva.
- `npm --prefix mock run build` — PASS.
- `node --test mock/test/admin-local-state.test.mjs mock/test/admin-local-access.test.mjs` — PASS (11 pruebas enfocadas, incluidas las tres de consulta manual).
- `git diff --check` — PASS.

No se ejecutaron suites Backend ni la regresión integral LOCAL-1. #250 permanece En revisión y #246 abierta.
