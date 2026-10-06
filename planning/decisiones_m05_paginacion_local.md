# M05-LOCAL: paginación del catálogo sintético

Issue #164, continuación de la búsqueda sintética autorizada en #151/#160/#162. El alcance es solo paginar el catálogo local de fixtures explícitamente habilitados; las Issues generales de búsqueda y publicación siguen abiertas.

## Contrato

- `GET /api/v1/local/booking-trial/catalog` admite `page_size` (predeterminado 5; 1–25) y `cursor`. Responde `data.items`, el aviso **ENSAYO LOCAL — SIN COBRO REAL** y `data.next_cursor` solo cuando hay otra página. No informa totales ni números de página.
- Todos los filtros, cálculos de estimación y orden se aplican antes de paginar. No se usa `OFFSET`. En el orden de categoría, el repositorio conserva el orden `categoria_espacio.orden`, título y UUID que entrega PostgreSQL; el cursor busca la combinación anterior y continúa justo después de ella.
- Orden: proximidad por distancia PostGIS exacta en metros, total estimado cuando se filtra por intervalo y UUID; intervalo sin proximidad por total estimado y UUID; sin intervalo ni proximidad por orden de categoría, título y UUID.
- El cursor v1 es cifrado y autenticado con AES-GCM, codificado Base64 URL-safe y vinculado al actor, el hash de filtros normalizados, modo de orden y tamaño. Normaliza los instantes a UTC y aprovecha la serialización determinista de mapas JSON de Go. No contiene token de sesión, dirección ni coordenadas; conserva la clave de continuación, incluida la distancia sin redondear. El Backend vuelve a validar el acceso a fixtures en cada petición.
- Un cursor inválido, versión desconocida, tamaño o búsqueda distintos, o cuenta distinta responde 422. La clave vive en memoria del proceso, así que reiniciar el Backend invalida cursores activos.
- No se conserva estado de búsqueda ni transacciones entre solicitudes. Cada página aplica vencimientos y consulta el catálogo vigente. Con datos estables, la clave compuesta permite recorrer sin duplicados ni omisiones. Cambios de tarifa, disponibilidad o catálogo pueden alterar páginas posteriores; se reinicia la búsqueda para empezar otra vez. Haber visto un resultado no garantiza la cotización o disponibilidad posterior.
- La paginación limita el tamaño de respuesta, pero en este primer corte el servicio calcula, filtra y ordena el catálogo completo antes de seleccionar la página. Esto conserva la corrección del filtro de precio, aunque aún no reduce todo el costo de consulta.

## Mock y ejemplos sintéticos

El mock ofrece tamaño de página, **Siguiente** y **Volver al inicio**. Cambiar filtros/tamaño reinicia la búsqueda; avanzar invalida detalle y cotización. Respuestas antiguas se descartan por generación de solicitud y sesión. Sigue siendo HTML, CSS y TypeScript compilado con DOM y `fetch`.

Para visualizar al menos tres páginas con el tamaño predeterminado, después de tener dos cuentas sintéticas activas/verificadas. Se puede conservar el modo anterior (elige la pareja de una fixture de paginación existente, o la primera pareja habilitada) o fijar explícitamente los participantes:

```sh
bash scripts/enable-local-catalog-pagination-fixtures.sh
# o, para elegir la pareja sin depender de la selección automática:
bash scripts/enable-local-catalog-pagination-fixtures.sh \
  --host-email anfitrion@example.invalid \
  --renter-email arrendatario@example.invalid
```

El comando añade de forma idempotente diez fixtures sintéticos a la pareja ya autorizada o indicada. Los dos correos deben corresponder a cuentas distintas y ambos se normalizan; con flags se resuelven usuarios activos. Respeta las ocho categorías y agrega coordenadas explícitamente sintéticas. No modifica fixtures previos, no publica borradores y no borra datos. Usa la base persistente existente, aplica únicamente el build del servicio y no recrea `pgdata` ni secretos.

## Comprobaciones

- `go test ./internal/booking/... ./cmd/api/...`
- `bash scripts/test-m06-local-booking-postgres.sh` para PostgreSQL desechable y el rol `espacigo_runtime`.
- `npm --prefix mock run test:profile-races`
- `go test ./...`, `go vet ./...`, validación del OpenAPI y `git diff --check` antes de presentar.

La evidencia final del corte queda en [evidencia M05 paginación](evidencia_m05_paginacion_catalogo_20261006.md).
