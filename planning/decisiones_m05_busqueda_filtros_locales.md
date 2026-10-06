# M05-LOCAL-02 — búsqueda local por características y precio

**Issue:** #160, subentrega relacionada con #62–#68. Este corte usa solo fixtures sintéticos explícitamente habilitados y no cierra criterios generales M05/M06.

## Contrato de búsqueda

- La búsqueda recibe categoría opcional, intervalo opcional, límites inclusivos `min_total_clp` / `max_total_clp` opcionales, y filtros de atributos como objeto JSON URL-encoded. Si se consulta un intervalo, cada resultado incluye estimación CLP, unidad de tarifa y zona horaria. Si se indica un límite de precio, ambos instantes son obligatorios; mínimo y máximo pueden usarse por separado.
- Se calcula el total con la misma función de unidades por tarifa/zona que usa la cotización. La lista con intervalo se ordena por total ascendente y luego UUID ascendente. El monto se etiqueta como estimación, no precio comprometido.
- Categoría, disponibilidad, rango y atributos seleccionados se combinan mediante AND. La búsqueda excluye reservas/borradores fuera de la allowlist de fixtures y no inserta cotizaciones ni nuevas ocupaciones.
- Para atributos, se carga el perfil actual al seleccionar categoría. La API exige su versión y valida tipo, unidad implícita en el perfil, límites/opciones y código. Los filtros solo se aplican a espacios cuya versión guardada coincide exactamente; no se reinterpretan perfiles viejos.
- Booleanos y atributos escalares usan igualdad tipada; `false` es valor presente. `enum_list` usa contención: todos los elementos elegidos deben estar guardados. Un atributo ausente no coincide. Los controles solo expresan valores declarados por el perfil; los numéricos representan igualdad exacta en la unidad indicada.
- Cotizar vuelve a calcular unidades y tarifa actual y comprueba disponibilidad. Al reservar, el Backend revalida la vigencia de la tarifa snapshot y la disponibilidad bajo su transacción; una tarifa que cambió después de la cotización produce conflicto sin reserva ni ocupación.

## Persistencia y límites

No se requiere migración: perfiles versionados, atributos JSONB, tarifas, ocupaciones y fixtures ya cubren este corte. La consulta usa `jsonb @>` con parámetros, nunca SQL dinámico por código de atributo. El mecanismo ya existente puede materializar vencimientos antes de responder disponibilidad; esa transición libera únicamente retenciones vencidas. La búsqueda no persiste una cotización ni crea ocupación.

Fuera de alcance: publicación comercial, borradores no habilitados, geografía, mapas, ranking patrocinado, pagos reales, historial/avisos de DB02-09 y completar #62–#68 o M05/M06.
