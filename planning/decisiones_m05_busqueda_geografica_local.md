# Decisión — búsqueda geográfica local de fixtures M05

Fecha: 2026-10-06. Issue #162. La autorización del corte define decisiones provisionales para el prototipo; no resuelve búsqueda comercial general (#62–#68).

## Contrato

- Solo el catálogo autenticado de fixtures habilitados explícitamente participa. Las ubicaciones se almacenan en `reserva_ensayo_local_ubicacion_sintetica`, con FK a la allowlist y marca sintética obligatoria. El punto PostGIS es `geography(Point,4326)`; la tabla no altera `espacio.direccion`, no geocodifica direcciones y el rol runtime tiene solo lectura.
- La búsqueda acepta `latitude`, `longitude`, `radius_km`. Los tres son opcionales en conjunto: si se informa alguno, se requieren los tres. Latitud y longitud son finitas y pertenecen a [-90,90] y [-180,180]; el radio entero debe ser uno de 1, 3, 5, 10 o 25 km. El mock preselecciona 5 km al habilitar proximidad. Los valores son provisionales del prototipo y se pueden cambiar después.
- Ubicación ausente excluye al fixture solo con filtro geográfico; sin él se conserva la respuesta previa. La búsqueda combina proximidad mediante AND con categoría, disponibilidad, características versionadas y precio de #160.
- PostGIS usa `ST_DWithin` inclusivo para el radio y `ST_Distance` en metros sin redondear para ordenar. La API no devuelve dirección, latitud, longitud ni distancia cruda; devuelve distancia aproximada `distance_km` redondeada a una décima y `distance_kind: direct`. El mock explica que es distancia directa, no distancia por carretera.
- Con filtro geográfico: ordenar por metros ascendente; en empate, total estimado cuando hay intervalo; finalmente UUID. Sin filtro geográfico se conserva el orden de #160.
- Las lecturas mantienen el proceso de vencimiento existente. La búsqueda no inserta cotizaciones, reservas u ocupaciones y no altera los contratos de cotización/reserva.

## Ubicaciones sintéticas reproducibles

Centro de ejemplo: Santiago sintético (-33.4560, -70.6693). La migración nueva asigna coordenadas explícitas a las filas allowlisted existentes, con ubicaciones por categoría suficientemente separadas para observar inclusiones/exclusiones en radios 1/3/5/10/25 km. No usa direcciones. El comando local de habilitación asigna el punto fijo de su categoría también a fixtures futuros; puede repetirse para reparar un punto sintético ausente.

Las pruebas exactas de límite no dependen de esas separaciones aproximadas: construyen puntos con `ST_Project` a la distancia exacta del radio y validan inclusión. Se agregan ejemplos por categorías existentes en el mock; radio 1 km separa la sala sintética próxima de la bodega más lejana y radio 5 km las incluye.

## Esquema y seguridad

V018 es incremental; V1–V17 permanecen intactas. Índice GiST sobre geography para acotar radios. Bootstrap otorga solo `SELECT` a `espacigo_runtime`, sin INSERT/UPDATE/DELETE; las coordenadas provienen de migración de datos sintéticos o del comando local explícito ejecutado por administración. El endpoint usa parámetros y la serialización omite lat/lon y punto.
