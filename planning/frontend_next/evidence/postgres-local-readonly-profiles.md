# PostgreSQL local: inspección de perfiles y esquema

**Fecha:** 2026-10-27  
**Target local confirmado:** PostgreSQL nativo `127.0.0.1:5432`, base `espacigo_dms_stage_20261020`, rol `espacigo_runtime`. Autenticación vía archivo estándar `~/.pgpass`; su contenido no se leyó ni imprimió. No confundir con Cloud SQL ni con PostgreSQL Docker usado por la API en `127.0.0.1:18081`.

## Verificación de conexión

Comando ejecutado tal como se autorizó:

```sh
psql -X -h 127.0.0.1 -p 5432 -U espacigo_runtime -d espacigo_dms_stage_20261020 -w -c "SELECT current_database(), current_user;"
```

Resultado: `current_database=espacigo_dms_stage_20261020`, `current_user=espacigo_runtime`. `pg_isready` confirmó listener disponible; el servidor reportó PostgreSQL `18.6`.

Las consultas de esquema/datos de perfil se ejecutaron dentro de `BEGIN TRANSACTION READ ONLY` … `COMMIT`.

## Catálogo de perfiles dinámicos

- Esquemas accesibles: `pglogical`, `public`, `topology`.
- `public.categoria_espacio`: código/nombre/orden/activo.
- `public.categoria_perfil_atributos`: JSONB `perfil`, clave `(categoria_codigo, version)`.
- `public.espacio_caracteristicas`: JSONB `valores` y `perfil_version`, con FK a `(categoria_codigo, version)` del perfil.
- `public.espacio`: UUID, propietario, categoría, título, descripción, superficie, capacidad, precio, dirección, estado y campos públicos de ubicación. Solo se inspeccionó metadata/DDL; no se leyeron espacios ni datos personales.
- El catálogo `categoria_perfil_atributos` contiene **9 categorías**, todas con versión **1**. `oficina` v1 tiene 11 atributos; `climatizacion` es `enum_list` con opciones `aire_acondicionado`, `calefaccion`, `ventilacion_mecanica`. `estacionamiento` v1 tiene 7 atributos: números `largo_plaza_m`, `ancho_plaza_m`, `altura_maxima_m` (min 0,01; max 99.999.999,99; step 0,01; unidad m), `vehiculos_admitidos` `enum_list` (`motocicleta`, `automovil`, `suv`, `furgon`), `techado` boolean, `tipo_acceso` enum (`abierto`, `porton_manual`, `porton_automatico`) y `carga_electrica` boolean. El registro omite `required` y declara `filter_candidate: false` en estos campos; no añadir reglas ausentes. El formulario conserva `attribute_schema_version: 1`.
- El test HTTP de Go que devuelve oficina v2 usa un doble en memoria; el test de integración inserta v2 durante su propia prueba. Ninguno cambia el resultado del target local consultado.

## Observación de galería

El inventario local expone `espacio_galeria_sintetica_local`, con restricciones explícitas `fixture_code='synthetic-png-v1'`, MIME `image/png` y tamaño máximo `1 MiB`. Esto es una tabla/flujo sintético local y **no** demuestra integración de la galería real, su límite de 10 MiB ni el objetivo de 30 fotos. No se insertaron filas ni se modificaron constraints.

## Continuación y límites

El wizard Next consume perfiles dinámicos, pero `listing-wizard.tsx` no renderiza actualmente el tipo `enum_list`; se abrió un test E2E contra el perfil local v1 para cubrir esa brecha. La lectura SQL confirma catálogo y DDL local, no que un proceso Go concreto esté apuntando a esta base. No se ejecutaron migraciones, escrituras, borrados, creación de usuarios/cuentas ni consultas de filas de espacios; no se cambió credencial o permiso.
