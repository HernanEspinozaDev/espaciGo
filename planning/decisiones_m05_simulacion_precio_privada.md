# Decisión: simulación privada de precio base en borradores propios

Estado: autorizado por la persona usuaria el 2026-10-05; Issue #146. Slice de M05 que reutiliza las entregas fusionadas de M04 y M06.

## Dependencias

- PR #134 fusionó autenticación de usuario, borradores privados propios y catálogo inicial con ocho categorías. Se reutilizan identidad, ownership y CRUD.
- PR #141 fusionó atributos configurables por las ocho categorías. El corte preserva ese catálogo sin cambios.
- PR #145 fusionó la zona horaria IANA por borrador y la consulta M06 de ocupación. La simulación llama a ese servicio; no crea calendario ni filas de `ocupacion`.
- PR #4–#9, #11 y #13–#15 fusionaron las fundaciones PostgreSQL, dominio, API, pruebas y entorno local.

No depende de búsqueda pública, publicación, KYC, reserva o pagos. #54/#55 y las Issues generales M05/M06 siguen abiertas por sus criterios originales; este slice no las completa ni cierra.

## Reglas confirmadas

El baseline define modalidad hora/día/mes y precio total como precio base por tiempo seleccionado, pero no fija la granularidad. La persona usuaria aprobó:

| Modalidad | Unidades facturadas | Ejemplo |
|---|---|---|
| Hora | Horas de 60 minutos transcurridas, redondeadas hacia arriba. | 90 min × $8.000/h = 2 × $8.000 = $16.000 CLP. |
| Día | Fechas calendario locales tocadas; el intervalo es semiabierto y un término exacto a medianoche no cuenta esa fecha final. | 1 día + 2 h × $40.000/día = 2 × $40.000 = $80.000 CLP. |
| Mes | Meses de calendario iniciados por aniversario local, con día de aniversario limitado al último día del mes. El término exacto en aniversario no inicia otra unidad. | 1 mes + 3 días × $300.000/mes = 2 × $300.000 = $600.000 CLP. |

La simulación solo presenta precio base en CLP. No incorpora comisión, impuestos, garantía ni condiciones comerciales.

## Privacidad y persistencia

El historial de tarifa es inmutable por espacio y versión. Cada simulación privada conserva snapshot exacto del intervalo UTC, zona horaria, versión de tarifa, unidad, precio unitario, unidades y subtotal CLP. Al actualizar el precio, una simulación guardada no se recalcula. El estado del borrador sigue `borrador`; no se crea retención/reserva ni se modifica `ocupacion`.
