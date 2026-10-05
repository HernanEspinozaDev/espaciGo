# M04-ATTR-01 — catálogo extensible de características

Fecha: 2026-10-05. Estado: implementado en V6 y presentado para revisión en un PR; no marca las Issues como Hecho ni habilita publicación comercial.

## Decisiones aplicadas

- Las ocho categorías conservan sus códigos estables. V6 añade `categoria_perfil_atributos` y `espacio_caracteristicas`; V7 completa el orden determinista de atributos. V5/V6 y sus checksums no se modifican. Los perfiles son JSONB de datos, versionados por categoría y versión, de solo lectura para el rol runtime.
- El perfil persistido es la fuente única para códigos, etiqueta, tipo, unidad, opciones, límites y candidato a filtro. Backend lee ese perfil para validar; el mock genera controles con el mismo endpoint. Agregar una categoría dentro de los tipos soportados no requiere cambiar el CRUD.
- El borrador conserva sus columnas comunes y agrega `attribute_schema_version` y un objeto `attributes`. Los valores nuevos son opcionales. `false` y `0` son valores declarados, `null`, opciones duplicadas/desconocidas, tipos incorrectos, claves no definidas y objetos mayores a 16 KiB se rechazan. El mock deja expresar “no declarado” sin convertirlo en `false`.
- Los ocho perfiles iniciales tienen versión 1 y límites del catálogo de planificación. Cantidades usan `integer` con límite PostgreSQL int4; dimensiones positivas usan precisión de centésimas y máximo compatible con numeric(10,2). Las reglas combinadas explícitas verifican parrilla/cantidad y superficies parciales de eventos contra la superficie común.
- Cada escritura del borrador y sus características se confirma en una transacción. El FK compuesto amarra categoría a espacio y perfil. Al cambiar categoría, el conjunto anterior se sustituye por el objeto declarado para el perfil destino; no se reinterpretan campos.
- Borradores pre-V6 sin fila de características se leen con perfil v1 y `{}`. No se rellenan atributos ficticios. Una futura versión nueva no debe sobrescribir una versión ya usada.
- La ausencia de `attributes` en un request es equivalente a `{}` para creación y reemplazo. El mock siempre envía la versión y el objeto explícitos.

## Comprobación y uso local

La suite de persistencia usa `scripts/test-m04-attributes-postgres.sh [paquetes...]`. Crea PostgreSQL/PostGIS fijado por digest en red aislada, solo socket Unix, almacenamiento temporal y sin puertos; la limpieza elimina el contenedor y directorio temporal. El contenedor no accede al volumen `espacigo_pgdata`.

Tras levantar el prototipo con `scripts/dev-env.sh up -d`, inicia sesión y abre `http://localhost:8081`. En “Espacios propios”, selecciona una categoría, declara opcionalmente características, crea y vuelve a abrir el borrador para comprobar persistencia. Cambia la categoría para comprobar que el formulario y el conjunto guardado usan el perfil destino. La API expone los perfiles en `GET /api/v1/spaces/categories/{categoryCode}/attributes`; `planning/openapi.yaml` documenta el contrato.

## Pendientes y límites

La entrega contiene borradores privados; no habilita publicación, búsqueda, reservas, tarifas nuevas, pagos, galería/almacenamiento, geocodificación ni calendario. Los perfiles v1 no implican permiso legal ni aforo certificado. Persisten por separado los pendientes M02/M03, DB02-09 y la limpieza del servidor #123. El historial/aviso durable dependiente de DB02-09 sigue pendiente.
