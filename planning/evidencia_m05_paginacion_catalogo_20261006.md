# Evidencia M05-LOCAL: paginación del catálogo sintético

Issue #164 · PR de esta subentrega.

## Resultado

El contrato pagina por clave después de aplicar filtros, estimaciones y orden completo; no usa `OFFSET`. El cursor v1 cifra y autentica la clave de continuación con AES-GCM y solo acepta la misma cuenta, filtros normalizados, orden y tamaño de página. Un cursor de otra instancia no se acepta.

El servicio local añadió diez fixtures sintéticos de forma aditiva e idempotente. Para esta comprobación se registraron y verificaron dos cuentas sintéticas nuevas por el flujo de la UI y Mailpit, y se eligió esa pareja explícitamente en el comando. También se habilitó un fixture base; el helper informó **11 fixtures habilitados**, suficientes para tres páginas predeterminadas de cinco. Las cuentas y fixtures previos se conservaron; no se reinició el volumen ni se eliminaron secretos.

El stack local se reconstruyó con cachés/imágenes reutilizadas, mantuvo PostgreSQL saludable y pasó `scripts/dev-env.sh verify-http`. La página del mock sirve los controles de tamaño, Siguiente y Volver al inicio. No se hicieron operaciones de cotización/reserva manuales con cuentas, pues las pruebas desechables ejercitan la API con el rol runtime sin usar credenciales de usuario reales.

## Criterios cubiertos por pruebas

- `go test ./...`: pasó.
- `go vet ./...`: pasó.
- `bash scripts/test-m06-local-booking-postgres.sh`: pasó en PostgreSQL desechable como `espacigo_runtime`; comprobó continuación y página final, cursor rechazado para otra cuenta, búsqueda vacía/filtros combinados y autorización de catálogo.
- `npm --prefix mock run test:profile-races`: 16 pruebas pasaron, incluyendo generación, tamaño, estado final, invalidación por filtros/sesión y respuestas fuera de orden.
- `planning/openapi.yaml`: parseado correctamente con PyYAML; se verificó la ruta del catálogo.
- `git diff --check`: pasó.
- `bash scripts/dev-env.sh verify-http`: verificó readiness, CORS y que el mock sirve HTML, TypeScript compilado y CSS.
- `bash scripts/enable-local-catalog-pagination-fixtures.sh --host-email m05-pagination-host-20261006@ejemplo.invalid --renter-email m05-pagination-renter-20261006@ejemplo.invalid`: salida verificable `Ejemplos sintéticos listos para 11 fixtures autorizados (al menos tres páginas predeterminadas); fixtures anteriores conservados.`

Los filtros, precios y disponibilidad se consultan completos en el servicio antes de recortar página. Este corte garantiza respuesta paginada correcta, no optimiza todavía toda la carga de consulta.

## Corrección de navegación y sesión (PR #165)

- `npm --prefix mock run build`: PASS con TypeScript 5.9.3.
- `npm --prefix mock run test:profile-races`: 18 pruebas PASS. Incluye integración de la restauración de botones de `action()` con el recálculo de Siguiente, estado final sin cursor, invalidación de una petición pendiente ante un nuevo envío del formulario y descarte de respuestas por generación/sesión.
- `git diff --check`: PASS.
- Reconstruí y reinicié únicamente `mock-frontend`; el build reutilizó las capas de Docker. No reinicié PostgreSQL, no apliqué migraciones ni modifiqué volumen/secretos.
- En navegador, la cuenta arrendataria buscó desde el formulario real y recorrió las páginas 1, 2 y 3 con tamaño 5: las dos primeras mostraron cinco resultados y Siguiente habilitado; la última mostró el resultado restante y Siguiente deshabilitado. **Volver al inicio** retornó a la primera página y habilitó Siguiente.
- Desde un resultado se abrió el detalle y se creó una cotización snapshot real para un intervalo futuro; la API devolvió su ID, espacio, tarifa y vencimiento. Al cerrar sesión se vaciaron el listado/detalle, el ID de espacio seleccionado y el ID de cotización, y Siguiente quedó deshabilitado.
- Se inició sesión después con la cuenta anfitriona de la pareja. La vista mostró el estado vacío que requiere una nueva búsqueda; no retuvo resultados, selección ni cotización de la sesión arrendataria. El cursor quedó invalidado. Los datos de acceso y tokens de verificación no se guardan en esta evidencia.
- No se modificó el mock en este ajuste adicional: la compilación y la cobertura de `action()`/estado de botones corresponden a la corrección ya publicada; aquí se comprobó el formulario real en navegador con ambas sesiones.
