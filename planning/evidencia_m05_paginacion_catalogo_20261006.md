# Evidencia M05-LOCAL: paginación del catálogo sintético

Issue #164 · PR de esta subentrega.

## Resultado

El contrato pagina por clave después de aplicar filtros, estimaciones y orden completo; no usa `OFFSET`. El cursor v1 cifra y autentica la clave de continuación con AES-GCM y solo acepta la misma cuenta, filtros normalizados, orden y tamaño de página. Un cursor de otra instancia no se acepta.

El servicio local añadió diez fixtures sintéticos de forma aditiva e idempotente al par que ya estaba autorizado. La comprobación final informó **12 fixtures habilitados**, suficientes para tres páginas predeterminadas de cinco. El comando se ejecutó dos veces y la segunda ejecución verificó que los ejemplos existentes se conservan; no se eliminó ni reemplazó ningún fixture. Se conservaron el volumen persistente y los secretos.

El stack local se reconstruyó con cachés/imágenes reutilizadas, mantuvo PostgreSQL saludable y pasó `scripts/dev-env.sh verify-http`. La página del mock sirve los controles de tamaño, Siguiente y Volver al inicio. No se hicieron operaciones de cotización/reserva manuales con cuentas, pues las pruebas desechables ejercitan la API con el rol runtime sin usar credenciales de usuario reales.

## Criterios cubiertos por pruebas

- `go test ./...`: pasó.
- `go vet ./...`: pasó.
- `bash scripts/test-m06-local-booking-postgres.sh`: pasó en PostgreSQL desechable como `espacigo_runtime`; comprobó continuación y página final, cursor rechazado para otra cuenta, búsqueda vacía/filtros combinados y autorización de catálogo.
- `npm --prefix mock run test:profile-races`: 16 pruebas pasaron, incluyendo generación, tamaño, estado final, invalidación por filtros/sesión y respuestas fuera de orden.
- `planning/openapi.yaml`: parseado correctamente con PyYAML; se verificó la ruta del catálogo.
- `git diff --check`: pasó.
- `bash scripts/dev-env.sh verify-http`: verificó readiness, CORS y que el mock sirve HTML, TypeScript compilado y CSS.
- `bash scripts/enable-local-catalog-pagination-fixtures.sh`: salida verificable `Ejemplos sintéticos listos para 12 fixtures autorizados (al menos tres páginas predeterminadas); fixtures anteriores conservados.`

Los filtros, precios y disponibilidad se consultan completos en el servicio antes de recortar página. Este corte garantiza respuesta paginada correcta, no optimiza todavía toda la carga de consulta.

## Corrección de navegación y sesión (PR #165)

- `npm --prefix mock run build`: PASS con TypeScript 5.9.3.
- `npm --prefix mock run test:profile-races`: 18 pruebas PASS. Incluye integración de la restauración de botones de `action()` con el recálculo de Siguiente, estado final sin cursor, invalidación de una petición pendiente ante un nuevo envío del formulario y descarte de respuestas por generación/sesión.
- `git diff --check`: PASS.
- Reconstruí y reinicié únicamente `mock-frontend`; el build reutilizó las capas de Docker. No reinicié PostgreSQL, no apliqué migraciones ni modifiqué volumen/secretos.
- En el navegador local inicié sesión con la cuenta sintética de evidencia M01, envié la búsqueda real desde el formulario y la API respondió sin resultados autorizados para esa cuenta. El logout cerró la sesión y limpió el detalle, la selección, el quote ID y la vista del catálogo. No pude validar las tres páginas ni el cambio a una segunda cuenta porque no tengo credenciales de una cuenta que participe en los 12 fixtures; no inventé ni restablecí credenciales. Esa comprobación requiere acceso a una cuenta sintética ya autorizada para esos fixtures.
