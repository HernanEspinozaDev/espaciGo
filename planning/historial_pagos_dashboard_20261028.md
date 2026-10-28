# espaciGo — historial de pagos y dashboard financiero

Fecha: 28-10-2026, America/Santiago. Complementa `contexto_pagos_marketplace_20261028.md` y `especificacion_pagos_checkout_pro_20261028.md`.

**Requisito confirmado por el usuario:** conservar historial de pagos, importes pagados y descuentos de comisión de espaciGo, y mostrarlo en un dashboard con detalle por cada espacio que genere ingresos. El diseño funcional y de registro que sigue se propone por encargo para dejar preparado el desarrollo. Solo documentación: no crea tablas, endpoints, pantallas ni cambios de datos.

## 1. Qué se conserva y cómo se relaciona

La cadena de consulta es **titular arrendador → espacio → contrato y versión → obligación de pago → intentos/transacciones → movimientos y ajustes**. Una obligación puede tener intentos rechazados y un pago aprobado; conservarlos no significa contar varias veces el ingreso. Un pago puede tener varios reembolsos parciales. Arriendo y garantía siguen siendo conceptos y cobros separados.

| Registro conceptual | Información necesaria |
| --- | --- |
| Obligación | Identificador interno, contrato/versión, espacio, partes autorizadas, concepto, periodo cubierto, vencimiento, importe acordado CLP y referencia a política de precio/comisión. |
| Intento de checkout | Obligación, identificador propio, preferencia/enlace referenciado sin secretos, cuenta receptora, creación/vencimiento y resultado. Enlaces caducados o intentos rechazados no son ingresos. |
| Pago confirmado | ID interno y `payment_id` externo, proveedor, cuenta vendedora, obligación/intento, concepto, moneda, bruto aprobado, fechas de creación/aprobación y estado remoto observado. Si el proveedor genera más de un pago para una operación, registrar y conciliar cada uno sin asumir relación uno a uno. |
| Desglose económico original | Base comisionable, comisión neta, tasa aplicada, IVA de comisión, total descontado por espaciGo, tarifa real Mercado Pago cuando se conozca, neto atribuible al arrendador y estado de conciliación. Conservar valores originales, no recalcular desde configuración actual. |
| Disponibilidad del pago | Condición efectiva de liberación de la operación y beneficiario, fecha prevista si la comunica Mercado Pago, estado observado, fecha/instante de confirmación y origen del dato. No confundir aprobación, disponibilidad y depósito bancario, ni inventar fechas sin información. |
| Periodos cubiertos | Intervalo o mensualidades que satisface el pago; distinguir fecha del cobro del periodo de uso. Un anticipo anual es un ingreso cobrado una vez con cobertura anual. |
| Devolución/contracargo/ajuste | Referencia al pago original, identificador externo cuando exista, concepto, importe, desglose por parte, motivo, fecha efectiva, estado, actor/origen y respaldo. No sustituir el pago original por un monto menor. |
| Garantía | Contrato, obligación/cobro separado, monto contractual/pagado/recibido, importes aplicados a daños conforme al contrato, saldo por liquidar y devolución acreditada. Distinguir pago o reembolso Mercado Pago de restitución registrada entre las partes. |
| Evento y conciliación | Identificador o clave deduplicable, tipo de evento, fecha ocurrida y recibida, referencia del proveedor, verificación/procesamiento, resultado y última consulta/conciliación. Guardar solo datos necesarios y autorizados. |
| Respaldo tributario | Referencia al documento autorizado y su emisor, tipo/folio, fecha, neto/IVA/total según corresponda y relación con ajustes/notas de crédito; no presentar un recibo interno como DTE emitido. |

Usar identificadores persistentes y una copia de los datos financieros esenciales al momento del cobro. Mostrar la identificación histórica mínima del espacio/contrato aun si cambia su título o se despublica. Una edición de publicación, precio, comisión o cuenta receptora no modifica el propietario, receptor, importe ni política de una operación pasada.

No guardar números completos de tarjeta, CVV, tokens OAuth, secretos ni datos bancarios innecesarios dentro del historial financiero, eventos de auditoría o dashboard.

## 2. Persistencia e integridad: definición para el diseño futuro

- **Fuente operacional única:** el registro persistente de pagos y movimientos del backend del proyecto. El dashboard consulta esa fuente o resúmenes derivados reconstruibles; no se obtiene sumando el estado del navegador ni leyendo únicamente el último webhook.
- Antes de diseñar persistencia, mapear estas responsabilidades al modelo existente de `pago`, `evento_proveedor`, `movimiento_financiero`, `garantia`, `liquidacion` y `documento_tributario` que describe la planificación. Son referencias de dominio, no evidencia de cobertura implementada. No crear tablas paralelas ni migraciones por esta especificación.
- Separar la proyección del estado actual del pago y su historial de hechos. La proyección puede actualizarse; los movimientos financieros confirmados se preservan y una corrección se representa mediante un nuevo ajuste enlazado, con actor, motivo y fecha. Esto no afirma que la base sea criptográficamente inmutable.
- Confirmar el resultado con el proveedor. Registrar la recepción/verificación de eventos y aplicar de forma transaccional e idempotente los cambios financieros locales. Repetir una notificación, reintento o consulta no agrega otro ingreso ni otra devolución.
- Deduplicar por proveedor, cuenta receptora y referencia remota según el contrato API. La obligación y cada intento tienen identidad propia. No confiar solo en el ID del contrato ni crear un movimiento de ingreso por cada webhook.
- Conservar fechas distintas de ocurrencia, recepción y conciliación para notificaciones tardías. No hacer retroceder un estado confirmado por un evento antiguo sin reconciliar el pago real.
- Si la tarifa del procesador o la liquidación no se conoce todavía, usar **pendiente/no conciliado**, nunca cero ficticio. El bruto pagado puede estar confirmado mientras el neto definitivo o la disponibilidad de fondos siguen pendientes.
- Los totales derivados se pueden recalcular desde hechos originales y ajustes. Un error de dashboard se corrige en la consulta/proyección, sin editar el historial para hacerlo coincidir con una cifra esperada.
- Separar ambientes y datos sintéticos de pagos reales. La historia fake existente no se convierte en ventas reales ni se mezcla con indicadores de producción.
- Eliminar/despublicar un espacio o cambiar de cuenta no borra automáticamente el historial ni rompe la trazabilidad. Su conservación, acceso posterior a una baja y minimización siguen las políticas de privacidad/retención aprobadas; este documento no prescribe conservación indefinida ni un plazo tributario universal.

## 3. Dashboard del arrendador

### Resumen general y por espacio

Permitir ver todos los espacios propios o seleccionar uno. Un espacio sin ingresos puede aparecer con cero, diferenciado de uno con pagos pendientes o datos aún no conciliados. Filtrar por rango de fechas, espacio, contrato, concepto y estado. Proponer mes actual como vista inicial y permitir otros periodos.

| Indicador | Significado |
| --- | --- |
| Arriendos cobrados brutos | Pagos de arriendo aprobados/confirmados en el periodo, antes de comisiones. No incluye garantías ni pagos todavía pendientes. |
| Devoluciones de arriendo | Reembolsos efectivamente confirmados en el periodo; separados de solicitudes o autorizaciones sin ejecución. |
| Descuentos de espaciGo | Comisión neta, IVA y total descontado, mostrando aparte las reversiones que correspondan. |
| Tarifas Mercado Pago | Cargos reales del proveedor atribuibles a esos cobros/movimientos y sus eventuales reversiones; pendientes identificados. |
| Neto de cobros del arrendador | Parte económica confirmada atribuible al arrendador tras descuentos y ajustes. No equivale a utilidad contable, saldo bancario ni garantía de fondos disponibles para retiro. |
| Disponibilidad en Mercado Pago | Del neto atribuible a los cobros consultados: pendiente de disponibilidad, disponible confirmado, retenido/bloqueado o no informado. No es el saldo completo de la cuenta ni confirma retiro bancario. |
| Por cobrar | Obligaciones vigentes y exigibles aún no satisfechas; separar futuras, vencidas y en proceso. No usar simplemente valor total del contrato menos pagos sin considerar sus cambios. |
| Garantías | Bloque separado: recibidas, pendientes de pago, por liquidar, aplicadas a daños, devueltas y en disputa. **No sumarlas como ingreso por arriendo.** |
| Incidencias | Reembolsos pendientes/bloqueados, contracargos y diferencias de conciliación, sin tratarlos como devoluciones ya realizadas. |

Debajo del resumen, mostrar una lista por espacio: identificación, bruto cobrado de arriendo, comisión/IVA, tarifa del proveedor, reversiones/ajustes y neto. Desde cada espacio se accede a sus contratos y pagos; desde un pago se consulta el detalle original y todos sus movimientos relacionados.

### Fila y detalle de cada pago

La fila debe mostrar fecha de aprobación, espacio, contrato, periodo cubierto, concepto, importe aprobado, comisión neta, IVA, total de comisión, tarifa Mercado Pago, neto y estado de pago/conciliación. En pagos de garantía la comisión de espaciGo es cero, sin mezclar su saldo con rentas.

El detalle conserva los intentos, referencia externa pertinente, desglose original, reembolsos/contracargos/ajustes con fecha y estado, saldo reembolsable del pago y referencias a documentos. Un intento fallido se muestra como tal y no genera una fila de ingreso confirmado.

Un gráfico por periodos y comparación por espacios puede usar los mismos indicadores, con definición visible de bruto, neto y fecha. El listado y los totales constituyen el alcance mínimo; el estilo visual y la biblioteca de gráficos no se definen aquí.

## 4. Vista del arrendatario y de espaciGo

- **Arrendatario:** historial de lo que pagó y debe pagar por sus contratos/espacios: arriendo, periodo cubierto, garantía separada, estado, fecha, devoluciones y respaldo disponible. Los descuentos que asume el arrendador no se muestran como una deuda o recargo del arrendatario. La visibilidad del detalle económico interno del arrendador se limita a lo necesario y autorizado; no se expone su contabilidad privada por defecto.
- **Arrendador:** solo operaciones y espacios en los que es parte autorizada. Una cuenta vinculada o ID de espacio enviado por el cliente no basta para conceder acceso.
- **EspaciGo:** vista consolidada para el actor que disponga de capacidad financiera expresamente autorizada: volumen de arriendos procesado, comisión neta propia, IVA de comisión, reversiones, importes cobrados incluyendo IVA y diferencias pendientes. Desglosar por espacio, contrato, cuenta receptora y periodo cuando ese acceso esté autorizado. No habilitar esa vista a todos los administradores por su nombre de rol ni crear un módulo de soporte.
- Los fondos del arrendador y las garantías no son ingresos propios de espaciGo. El IVA recaudado de la comisión se muestra separado de la comisión neta; el total de ambas magnitudes no se presenta como utilidad.
- Cambios de titularidad/administración de un espacio deben respetar la autorización histórica de cada operación. Un nuevo administrador o propietario no hereda automáticamente todo el detalle financiero personal de titulares anteriores.

## 5. Reglas de cálculo, periodos y devoluciones

1. **Cobro aprobado una sola vez:** un pago de arriendo de $100.000 deja comisión neta $3.000, IVA $570 y total espaciGo $3.570. El neto inicial del arrendador es $96.430 menos tarifa real Mercado Pago. Un webhook repetido no duplica esos importes.
2. **Anticipo anual:** $2.400.000 cobrados en octubre aparecen una vez como cobro en octubre, con los 12 meses cubiertos indicados en el detalle. No registrar doce ingresos mensuales nuevos. Una eventual vista de distribución por periodos se rotula separadamente y no se usa como otro cobro ni como política contable de devengo ya aprobada.
3. **Criterio de fechas:** el indicador «cobrado» usa fecha de aprobación del pago; devoluciones, contracargos y ajustes se agrupan por su fecha efectiva confirmada. Guardar instantes y presentar rangos en America/Santiago, con límites de periodo inequívocos. El periodo de ocupación se ofrece como filtro distinto.
4. **Periodos distintos:** una devolución de noviembre sobre un pago de octubre aparece como movimiento negativo en noviembre, con vínculo al cobro de octubre. Consultar el pago original conserva su bruto y muestra sus ajustes acumulados. Si se ofrece una vista «cobros de octubre actualizados», rotular su criterio para no confundirla con flujo de caja de octubre.
5. **Reembolso parcial:** mantener el bruto original, sumar devolución confirmada y revertir comisión/IVA según la fórmula acumulada de la especificación Checkout Pro. El neto del arrendador se ajusta por la parte de devolución/contracargo que efectivamente le corresponde y las reversiones reales de tarifa. No restar automáticamente el reembolso completo de su neto y además volver a restarle la comisión.
6. **Conciliación del neto:** para el periodo, sumar los movimientos firmados confirmados atribuibles al arrendador (netos iniciales, cargos y reversiones). El bruto cobrado menos devoluciones es un indicador distinto; no basta por sí solo para conocer el neto. Mostrar diferencias cuando la distribución del proveedor no coincida aún con el cálculo esperado.
7. **Importes pendientes:** no sumar devoluciones solicitadas como devueltas ni garantías declaradas por una parte como reembolsos confirmados de Mercado Pago. Indicar origen y grado de confirmación de una restitución contractual entre partes.
8. **Deudas:** un reembolso o contracargo no recrea automáticamente una obligación de cobro del mismo importe; su exigibilidad resulta del contrato y del ajuste registrado. No mostrar como moroso al arrendatario solo porque recibió una devolución autorizada.
9. **Estimación frente a definitivo:** sin tarifa real, el neto se muestra pendiente o estimado explícitamente. Los totales deben indicar cuántas operaciones faltan por conciliar y la fecha de actualización. No mezclar importes estimados con un neto rotulado «definitivo».

### Ejemplo de devolución, sin inventar tarifa del proveedor

Para un arriendo de $100.000 reembolsado por $50.000, la política propuesta revierte comisión neta $1.500 e IVA $285. El historial conserva el cobro $100.000, la devolución $50.000, comisión neta restante $1.500 e IVA restante $285. La parte económica de devolución atribuible al arrendador es $48.215 antes de los ajustes reales de tarifa. Si el proveedor distribuye otro redondeo, se registra el hecho real y la diferencia a conciliar. La solicitud de esa devolución no altera los totales hasta confirmar su ejecución.

## 6. Conservación, permisos y verificación de correspondencia

El historial financiero no ofrece una acción normal de «editar/borrar pago» para corregir importes. Una regularización autorizada genera ajuste o nueva versión con auditoría, preservando los hechos y sus respaldos conforme a las políticas de conservación. El dashboard es una consulta del historial, no un libro separado susceptible de modificar saldos manualmente.

La [matriz de decisiones operativas y conservación](decisiones_operativas_y_conservacion_pagos_20261028.md) concreta las capacidades de consulta/exportación y operación financiera, permisos por titular y plazos productivos propuestos por clase. Para DTE se toma la referencia fiscal de seis años; los calendarios de hechos financieros, contratos y archivos técnicos se distinguen y se justifican como propuesta de diseño. No se extiende ese plazo a todos los datos ni se cambia la retención sintética local.

Queda por verificar al autorizar implementación/operación: correspondencia de estos registros con entidades y permisos existentes, asignación de capacidades a la identidad designada, disponibilidad real de tarifas/liquidaciones de Mercado Pago y validación tributaria individual. Son evidencias de encaje/habilitación, no campos ni indicadores financieros pendientes de definir.

La especificación ya define qué información guardar, relaciones, filtros, indicadores, cálculos y reglas de actualización; esos puntos no quedan delegados a una elección improvisada durante el desarrollo. El esquema físico, índices y contratos de consulta se diseñarán en el corte autorizado de implementación, sin confundirlos con requisitos comerciales aún abiertos.

## 7. Criterios de aceptación del desarrollo futuro

- Un pago aprobado aparece en el historial del contrato, espacio y titulares correctos, con el mismo desglose que en los totales.
- Intentos rechazados, checkout sin completar, eventos duplicados o fuera de orden no crean ingresos adicionales.
- Cambio de título/precio/comisión/cuenta receptora y despublicación conservan la operación histórica y sus permisos.
- Un anticipo anual se cobra/comisiona una vez y muestra correctamente su cobertura; pagos de garantía no inflan ingresos de arriendo ni comisión de espaciGo.
- Reembolso parcial/total y contracargo conservan el original, muestran movimientos y no generan cargos o devoluciones duplicadas.
- El dashboard distingue pendiente, confirmado, reembolsado y conciliado, y no presenta neto definitivo con tarifas desconocidas.
- Filtros por espacio/contrato/periodo y totales generales coinciden con los movimientos persistidos, incluso con devoluciones en otro mes.
- Arrendatario, arrendador y actor autorizado de espaciGo solo acceden a su alcance; no se filtran otros contratos ni secretos.
- La restitución de una garantía larga entre partes se registra sin aparentar transferencia ejecutada por espaciGo o reembolso de Mercado Pago fuera de plazo.
- Datos sintéticos y pruebas permanecen separados de operaciones reales; no se afirma integración o dashboard implementados a partir de esta documentación.

## 8. Plazos del contrato frente a disponibilidad del proveedor

Se agrupan pagos por espacio y contrato, pero las condiciones de disponibilidad se observan por cuenta receptora/beneficiario y operación. El calendario de pago lo acuerdan las partes; no se usa para imponer al proveedor una fecha de liberación. Para esta entrega no hay configuración de disponibilidad por espacio ni elección del arrendatario. Las condiciones de cada cuenta pueden variar y se conserva su aplicación histórica por pago. Ver el apartado «Tres plazos distintos» de la matriz de decisiones operativas.
