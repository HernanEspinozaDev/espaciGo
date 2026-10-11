# LOCAL-M05-SEARCH-GAPS-01 — evidencia de búsqueda y filtros locales

Estado al 2026-10-11: implementación en revisión en la misma entrega DB/Backend/API/mock de #257, publicada en [PR #261](https://github.com/HernanEspinozaDev/espaciGo/pull/261). El commit probado y publicado es `6b244fa7ca4e8a2134be7b21e1b2e66c57a5a746` (autor de publicación HernanMEC). #246 sigue abierta y LOCAL-1 no está aceptado. Sin geocodificación real, mapas de calles, nuevas migraciones ni GCP.

## Criterios y decisiones implementadas

- **RQF-094:** `q` aplica búsqueda literal insensible a mayúsculas en título, descripción y nombre de categoría pública. La consulta parametrizada no incluye dirección/contacto. El Backend normaliza espacios, limita a 120 caracteres y rechaza controles; `%` y `_` son caracteres literales, no comodines SQL.
- **RQF-095:** el mock presenta un plano SVG esquemático, con acercar/alejar, desplazamiento y reinicio. Solo se dibujan fixtures habilitados con ubicación declarada sintética. El Backend transforma puntos a posiciones relativas `x/y` de 0 a 100 y no devuelve latitud, longitud, dirección ni calles. No consulta ubicación del navegador ni geocodifica.
- **RQF-099:** `min_area_m2` es inclusivo y valida el rango/precisión admitidos por PostgreSQL. Se combina con categoría, texto, disponibilidad, atributos, precio y proximidad. No había que crear columna ni migración: se reutiliza `espacio.superficie_m2`.
- **RQF-102:** “Limpiar filtros” invalida peticiones pendientes antes de restablecer controles, atributos de categoría, cursor, página, selección, disponibilidad, cotización y viewport; luego inicia la consulta inicial con los criterios vacíos. La identidad/cursor evita aplicar resultados de una solicitud anterior.
- El fingerprint del cursor incluye el texto normalizado y el mínimo de superficie; un cursor de otros filtros no continúa la nueva búsqueda. El catálogo no crea cotizaciones ni ocupaciones.

Un mapa se calcula para puntos de ensayo autorizados; las publicaciones generales sin coordenadas sintéticas no reciben una posición inventada. Las posiciones son esquemáticas y relativas al resultado filtrado, no sirven para medir distancias. El cálculo geográfico y el orden por proximidad vigentes no cambiaron.

## Pruebas ejecutadas

```text
npm --prefix mock run test:search-filters
PASS TypeScript build; 7 tests del mock/paginación; 0 fallos.
  Incluye siguiente deshabilitado durante carga/fin, descarte de respuestas antiguas,
  integración con action(), limpieza ordenada/invalidadora y viewport sintético.

go test ./internal/booking ./internal/booking/transport/http
PASS

GO_TEST_RUN='^TestLocalBookingTrialPostgresLifecycleAndConcurrentRetry$' \
  bash scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/booking
PASS en PostgreSQL/PostGIS desechable y rol espacigo_runtime.
  La integración prueba búsqueda combinada texto/categoría/superficie, coincidencia
  de mayúsculas/espacios, frontera inclusiva, exclusión justo sobre la superficie,
  resultado vacío, porcentaje literal (no SQL wildcard), no búsqueda de dirección y JSON/API con posición sintética sin
  coordenadas crudas. También comprueba 422 para min_area_m2 fuera de rango.

python3 (PyYAML 6.0.3): safe_load(planning/openapi.yaml)
PASS
```

La integración amplía `TestLocalBookingTrialPostgresLifecycleAndConcurrentRetry`, ya incluida en `scripts/local-required-integrations.txt`; no se añadió una integración que pueda omitirse en el runner.

## Entorno local y límites de la comprobación

Se reconstruyó y arrancó el stack mediante `./scripts/dev-env.sh up`, conservando `espacigo_pgdata`, secretos y datos. Los servicios quedaron healthy y la página sirvió los nuevos controles de búsqueda, superficie, mapa sintético y limpieza. No se autenticó una cuenta desde el navegador en esta comprobación; la interacción del botón, el orden de invalidación y el viewport están cubiertos por las pruebas enfocadas del mock, mientras la API real y el repositorio se verificaron en PostgreSQL desechable.

La regresión integral LOCAL-1 se reserva para cuando #258 y #259 estén integradas. #246 permanece abierta; los criterios generales M04/M05 tampoco quedan completos por este corte.

## Ajustes de revisión en PR #261 (2026-10-11)

- Se corrigió `CatalogDetail` para seleccionar y devolver `espacio.superficie_m2`. La integración PostgreSQL compara `area_m2` del resultado de búsqueda con el detalle autenticado del mismo espacio; además consulta el endpoint HTTP y valida el JSON recibido contra `LocalCatalogDetailEnvelope` de OpenAPI.
- `SyntheticMapViewport.zoomBy` ahora recorta `panX/panY` al rango válido para el nuevo zoom. La prueba acercar → desplazar → alejar verifica que, al volver a zoom 1, el desplazamiento queda en cero y reaparecen ambos pines sin pulsar “Restablecer mapa”.

Comprobaciones enfocadas posteriores a estos cambios:

```text
GO_TEST_RUN='^TestLocalBookingTrialPostgresLifecycleAndConcurrentRetry$' \
  bash scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/booking
PASS en PostgreSQL/PostGIS desechable con rol runtime.

npm --prefix mock run test:search-filters
PASS TypeScript build y 8 pruebas del mock; 0 fallos.

go test ./internal/booking ./internal/booking/transport/http ./internal/openapicontract -count=1
PASS
```

No se ejecutó la regresión integral LOCAL-1. #257 permanece abierta/en revisión y #246 sigue abierta.
