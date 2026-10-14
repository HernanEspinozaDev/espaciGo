# Auditoría de atributos comerciales públicos — MPV2-11

Fecha: 2026-10-13. Corte técnico iniciado sobre `main` en `5029a1ab73f4e9c365d5352299849b7dbe80991f`, posterior a la fusión de MPV2-10 en PR #304. Esta auditoría no constituye aceptación de producto.

## Fuente y regla de exposición

Los perfiles V6 versionados por categoría son la fuente canónica de `code`, etiqueta, tipo, orden, unidad, opciones y límites. En los ocho perfiles inspeccionados `filter_candidate` es `false`; ese campo no concede ni deniega exposición. El endpoint solo devuelve campos enumerados expresamente en `atributo_filtro_publico` y nunca devuelve el objeto JSON de perfil ni valores privados de espacios. V57 conserva la política booleana existente y agrega la selección comercial explícita de la tabla siguiente.

Se excluyen identidad, propiedad, dirección precisa, contacto, KYC, reservas, contratos, pagos, evidencias y todo campo no enumerado. Los filtros numéricos conservan precisión decimal y unidad del perfil; CLP continúa siendo `int64`.

## Allowlist por categoría

`bool` = booleano; `int` = entero; `num` = decimal; `enum` = igualdad; `lista` = multiselección con OR interno. Etiquetas, orden, unidad, opciones y cotas se leen directamente del perfil versionado por Go; no se mantienen duplicados en Angular.

| Categoría | Campos públicos filtrables |
| --- | --- |
| oficina | `wifi` (bool), `mobiliario_incluido` (bool), `puestos_trabajo` (int), `tipo_uso_oficina` (enum), `escritorios` (int), `climatizacion` (lista) |
| sala_multiproposito | `wifi` (bool), `proyector` (bool), `disposiciones_admitidas` (lista), `sillas_disponibles` (int), `mesas_disponibles` (int), `pantalla` (bool), `videoconferencia` (bool) |
| bodega | `carro_carga_disponible` (bool), `muelle_carga` (bool), `altura_util_m` (num, m), `volumen_util_m3` (num, m³), `acceso_carga` (enum), `ancho_acceso_m` (num, m), `altura_acceso_m` (num, m), `tipo_almacenamiento` (enum) |
| estacionamiento | `techado` (bool), `carga_electrica` (bool), `largo_plaza_m` (num, m), `ancho_plaza_m` (num, m), `vehiculos_admitidos` (lista), `altura_maxima_m` (num, m), `tipo_acceso` (enum) |
| local_flexible | `acceso_nivel_calle` (bool), `vitrina` (bool), `frente_m` (num, m), `probadores` (int), `bodega_apoyo_m2` (num, m²), `climatizacion` (lista) |
| stand | `montaje_incluido` (bool), `wifi` (bool), `ancho_stand_m` (num, m), `fondo_stand_m` (num, m), `lados_abiertos` (int), `altura_maxima_m` (num, m), `potencia_disponible_w` (int, W), `mesas_incluidas` (int), `sillas_incluidas` (int) |
| quincho | `techado` (bool), `agua_potable` (bool), `tipo_parrilla` (enum), `asientos_disponibles` (int), `parrillas_disponibles` (int), `mesas_disponibles` (int), `lavaplatos` (bool), `refrigerador` (bool) |
| parcela_eventos | `salon_cubierto` (bool), `quincho_disponible` (bool), `superficie_exterior_util_m2` (num, m²), `superficie_cubierta_util_m2` (num, m²), `estacionamientos_disponibles` (int), `cocina_disponible` (bool), `piscina_disponible` (bool), `acceso_bus` (bool) |

## Contrato de predicados

- Booleanos conservan `attribute=code:true|false`; false solo coincide con un valor presente `false`, nunca con una clave ausente.
- Enteros/decimales usan cotas inclusivas. Go verifica tipo, límites y paso del perfil con aritmética decimal exacta.
- Enum usa igualdad contra una opción declarada. Enum-list usa OR entre opciones seleccionadas; atributos diferentes se combinan con AND.
- Cada atributo se expresa una vez; duplicados, operadores, tipos, valores, categorías, rangos y opciones inválidos se rechazan. El máximo es 16 atributos y 32 valores repetidos de URL.
- Los cursores firmados pasan a versión 3 e incluyen los predicados tipados en su fingerprint; un cursor v2 previo se rechaza explícitamente. Las URLs booleanas antiguas siguen aceptándose.

## Estado de verificación

La implementación, la migración V57 desde base vacía y V56 con V1–V56 intactas, el fixture de ocho categorías, Go/PostgreSQL, Angular Router/Atrás-Adelante, contexto de identidad, SSR, y capturas con hash se registraron en [evidencia MPV2-11](evidencia/mpv2_11/README.md). Incluye límites del EXPLAIN local y del entorno sintético. Esto acredita pruebas técnicas, no aceptación de producto.
