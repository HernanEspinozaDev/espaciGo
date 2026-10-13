# Decisiones de implementación MPV2-02

**Estado:** decisiones de ingeniería aplicadas en la rama de implementación; sujetas a revisión del PR. No equivalen a aceptación de producto.

## Ubicación pública

- Fuente de nombres y códigos: [SUBDERE, Códigos Únicos Territoriales actualizados al 6 de septiembre de 2018](https://www.subdere.gov.cl/documentacion/c%C3%B3digos-%C3%BAnicos-territoriales-actualizados-al-06-de-septiembre-2018), archivo `CUT_2018_v04.xls`; validación cruzada de cobertura con [IDE/MOP, capa DPA](https://rest-sit.mop.gob.cl/arcgis/rest/services/INTEROP/SERVICIO_DPA/MapServer/1).
- Snapshot versionado en `db/reference/dpa_cl_cut_2018.csv`: 16 regiones y 345 comunas; excluye el Territorio Chileno Antártico. SHA-256 `6d724d07ca9c9fa628394a6b74e6b413af444f8ab9f95068ba616d3ad11720db`.
- Los códigos se guardan como texto para preservar ceros iniciales. V55 agrega versión de catálogo, nombres y claves foráneas; los espacios existentes comienzan sin región ni comuna. No se geocodifica ni se infiere comuna desde la dirección privada. El catálogo de runtime solo tiene permiso de lectura.
- El primer endpoint de sugerencias devuelve catálogo oficial; el filtro de inventario exige códigos exactos. Los 345 registros caben en el límite público actual de 400.

## Texto y atributos públicos

- V55 agrega `unaccent` y configuración `espacigo_es_unaccent` copiada de `pg_catalog.spanish`, con stemming español, NFC y normalización de espacios. El documento indexado incluye título, descripción pública, nombre/código de categoría y comuna/región si fueron declaradas; nunca dirección ni identidad.
- La consulta usa `plainto_tsquery` (AND de lexemas) y GIN; `ts_rank` se expone solo para `sort=relevance`. Stopwords/puntuación sin lexemas producen página vacía, no una búsqueda distinta.
- El allowlist versionado inicial solo publica filtros booleanos aprobados por categoría (p. ej. wifi para oficina/sala multipropósito, acceso calle/vitrina para local flexible). No se publican objetos JSON completos ni valores privados de perfiles. El cliente debe enviar categoría y códigos del allowlist.

## Total por intervalo

- Se aceptan extremos RFC3339 con offset y ambos son obligatorios para filtrar u ordenar por total. SQL calcula unidades en la zona IANA y antes de paginar; Go recalcula lo mostrado mediante `booking.PriceUnits`. `estimated_total_clp` siempre aparece: será `null` sin intervalo, sin zona válida o ante desbordamiento int64.
- Es una comparación aproximada con la tarifa vigente, no una cotización, una reserva, una comprobación de disponibilidad ni una ocupación. No modifica estado de reservas.

## Límite del corte

La asignación de ubicación por el anfitrión corresponde a MPV2-03. SSR, fotografías públicas, edición de zonas, moderación nueva, coordenadas, geocodificación, disponibilidad/ocupación y aceptación de producto no se consideran implementadas en este corte.
