# espaciGo — contexto de pagos marketplace

Fecha de traslado: 28-10-2026, America/Santiago.

Fuente: conversación ChatGPT «Guía pagos marketplace», ID `6ae23fd1-9560-83e9-943a-2977ddc462a6`, recuperada con `read_thread`. La lectura contiene tres turnos; algunos mensajes del asistente aparecen truncados por el servicio. Este documento consolida el contenido recuperable y el encargo actual; no es una transcripción literal completa.

Estado: documento de trabajo. Conserva decisiones del usuario, recomendaciones anteriores y asuntos abiertos sin convertir propuestas en capacidades implementadas. No modifica código, OpenAPI, esquema, migraciones ni datos. Las afirmaciones sobre proveedores y tributación se trasladan como antecedentes de la conversación; no se ha realizado una nueva validación documental/comercial en este traslado.

## Decisión de alcance de la entrega inicial — 28-10-2026

Por instrucción expresa del usuario, **Mercado Pago es el único proveedor de pagos y de vinculación de cuentas de cobro permitido en esta etapa de entrega**. El desarrollo de pagos se concentrará en Mercado Pago Split Payments 1:1 para facilitar la distribución al arrendador y el cobro de la comisión de espaciGo.

- Se permite publicar sin vincular Mercado Pago. Antes de aceptar un arriendo que genere cobros, el arrendador deberá tener una cuenta Mercado Pago habilitada y vinculada mediante OAuth. La aceptación y la creación de nuevos cobros requieren conexión válida; no basta con que haya existido una conexión en el pasado.
- La configuración de cobros de esta entrega solo permitirá vincular Mercado Pago. No se implementarán otros proveedores, vinculación de cuentas bancarias para abonos directos ni transferencias entre usuarios como alternativa de pago de la plataforma.
- Si la vinculación no existe, está vencida o revocada, no se habilitarán nuevos cobros hasta contar con una autorización válida. No habrá un método alternativo de respaldo en esta entrega.
- Esta exigencia se refiere al arrendador receptor. El arrendatario pagará a través del checkout de Mercado Pago con los medios que la integración habilite; no se deduce de esta decisión que deba vincular una cuenta de vendedor.
- Otros proveedores y modalidades se evaluarán a medida que avance la aplicación, mediante decisiones y alcances posteriores. Conservar separación entre negocio y proveedor no autoriza implementarlos ahora.
- El usuario intentará conseguir una alianza con Mercado Pago una vez desarrollado el flujo. Es una intención comercial futura: no existe aquí una alianza confirmada, condiciones preferenciales ni una habilitación técnica garantizada. Se deben validar los requisitos de integración disponibles para esta entrega sin dar por obtenida esa alianza.

Esta decisión sustituye las alternativas abiertas de proveedor para la entrega inicial descritas en la conversación anterior. Es alcance de desarrollo acordado, no evidencia de integración implementada.

### Reglas comerciales confirmadas por el usuario — 28-10-2026

1. **La comisión de espaciGo la asume el arrendador:** se descuenta del importe que recibe. El arrendatario no paga un recargo por esta comisión.
2. **El anticipo anual paga comisión una sola vez sobre el total anual:** no se generan nuevas comisiones mensuales sobre los periodos ya pagados. La asignación contable por periodos, si corresponde, es independiente de ese cobro único.
3. **Pago mensual iniciado por el arrendatario:** cada mensualidad se paga mediante un enlace de Mercado Pago. El débito automático y las suscripciones quedan fuera del alcance de esta entrega.
4. **El arrendador asume también la tarifa de procesamiento de Mercado Pago:** se descuenta de lo que recibe. No se agrega al arrendatario un recargo por esta tarifa. Su valor efectivo sigue sujeto a las condiciones del proveedor.
5. **Publicación sin vinculación permitida:** OAuth será obligatorio antes de aceptar un arriendo que genere cobros.
6. **Garantía opcional y separada:** el contrato permite indicar si se exige garantía. Cuando exista, se paga por Mercado Pago en un cobro separado del arriendo, sin comisión de espaciGo. El arrendador recibe, conserva y responde por su devolución; espaciGo no custodia ese dinero.

El desarrollo de estas reglas sigue fuera del encargo actual: aquí solo se documentan.

## 1. Decisiones e intención del usuario

| Tema | Contexto que se conserva | Estado |
| --- | --- | --- |
| Modelo | Marketplace de arriendos con múltiples arrendadores; cada cobro debe identificar al beneficiario correcto | Requisito |
| Proveedor inicial | Solo Mercado Pago, con Split Payments 1:1 y vinculación OAuth por arrendador | Alcance definido por el usuario para esta entrega; habilitación y ensayo pendientes |
| Comisión | 3% del importe de arriendo sujeto a comisión, más 19% de IVA sobre esa comisión | Tasa indicada por el usuario; aplicación tributaria por validar |
| Continuidad | Comisión por mensualidad efectivamente pagada; anticipo anual comisionado una sola vez sobre el total anual | Confirmado por el usuario el 28-10-2026 |
| Quién paga la comisión de espaciGo | El arrendador, mediante descuento del importe que recibe | Confirmado por el usuario el 28-10-2026 |
| Pago mensual | El arrendatario paga mediante un enlace cada mes; sin débito automático | Confirmado para esta entrega |
| Tarifa Mercado Pago | Se descuenta al arrendador, además de la comisión de espaciGo | Confirmado por el usuario el 28-10-2026 |
| Publicación y OAuth | Puede publicar sin conectar; debe conectar antes de aceptar un arriendo con cobros | Confirmado por el usuario el 28-10-2026 |
| Garantía | Opcional en contrato, cobro separado por Mercado Pago, sin comisión de espaciGo; recepción y devolución a cargo del arrendador | Confirmado; liquidación y bloqueos requieren precisar reglas |
| Arrendadores sin Mercado Pago | Deben crear y vincular su cuenta antes de habilitar cobros; no habrá alternativa en esta entrega | Decisión expresa del usuario |
| Independencia de proveedor | Separar reglas de negocio de Mercado Pago para facilitar evoluciones posteriores | Criterio de diseño; solo se desarrollará el adaptador Mercado Pago en esta entrega |

Los ejemplos iniciales del asistente usaban **5% hipotético**. Quedan sustituidos por **3% + IVA** para este contexto. Tampoco se adopta la suposición anterior de Next.js/backend separado: la planificación del repositorio usa Go y Angular. Este traslado no cambia la arquitectura del proyecto.

## 2. Flujo propuesto con Split Payments 1:1

1. El arrendador puede publicar sin vincular Mercado Pago; debe completar su vinculación antes de aceptar un arriendo que genere cobros.
2. Para utilizar Split 1:1, cada arrendador dispone de una cuenta Mercado Pago habilitada y autoriza a espaciGo mediante OAuth. Agregar datos bancarios al perfil no sustituye esa vinculación.
3. espaciGo asocia el usuario o entidad arrendadora con el identificador externo de su cuenta receptora y el estado de la conexión.
4. Publicación → solicitud/acuerdo → contrato → obligación de pago. Una publicación por sí sola no identifica todos los términos del cobro.
5. Al cumplirse las condiciones pactadas, el backend identifica el contrato, la obligación, el pagador, el arrendador y su cuenta vinculada; calcula la comisión.
6. El backend crea el checkout con la autorización OAuth del arrendador correspondiente y la comisión de espaciGo. El arrendador no tiene que generar manualmente el enlace.
7. El arrendatario abre el enlace y paga con los medios habilitados. Mercado Pago procesa y distribuye el importe entre ese vendedor y el marketplace según el producto contratado.
8. El backend confirma el resultado con notificaciones verificadas y consulta del estado al proveedor; concilia importes y actualiza la obligación y los efectos contractuales definidos.

**1:1** describe un vendedor beneficiario y el marketplace por pago. No equivale a repartir una operación entre múltiples arrendadores ni a custodiar fondos en escrow.

### Vinculación OAuth por arrendador

- Contemplar conectar, renovar autorización, desconectar y gestionar autorizaciones vencidas o revocadas. Validar el callback y proteger la correlación de autorización, por ejemplo con `state`.
- Tokens y secretos únicamente en backend, cifrados y fuera de logs, respuestas al navegador y repositorio.
- Validar ownership del contrato y de la cuenta receptora en servidor; no aceptar un beneficiario arbitrario enviado por el cliente.
- Definir representación de personas/empresas y administradores antes de fijar la relación cuenta–arrendador. No asumir que cualquier cuenta personal o empresarial será elegible: confirmar requisitos de identidad, actividad y habilitación para Chile.

### Checkout y confirmación

La recomendación anterior fue empezar con Checkout Pro. Su ejemplo de preferencias utilizaba `marketplace_fee` como **importe en moneda local**, calculado en backend, y `init_point` como enlace devuelto. Es referencia conceptual, no implementación aprobada. Elegir y validar el flujo/API vigente y compatible con Split en Chile antes de programar; no mezclar el contrato de preferencias con Orders API.

Usar referencias internas por obligación e intento, autenticidad de webhooks, consultas al proveedor e idempotencia para duplicados/reintentos. El retorno del navegador a una página de éxito no acredita el pago. Diferenciar pendiente, aprobado, rechazado, cancelado, reembolsado y contracargo, conservando historial.

## 3. Comisión 3% + IVA

Regla de cálculo propuesta, sobre base de arriendo `B`:

- Comisión neta: `B × 0,03`.
- IVA de la comisión: `comisión neta × 0,19`.
- Comisión total: comisión neta + IVA; equivale a `B × 0,0357` antes de redondeos.

| Concepto sobre $100.000 CLP | Importe |
| --- | ---: |
| Comisión neta 3% | $3.000 |
| IVA 19% sobre comisión | $570 |
| Total espaciGo | $3.570 |
| Arrendador tras descontar la comisión de espaciGo, antes de tarifa del procesador | $96.430 |

La tarifa de Mercado Pago es **otro costo** y no está incluida en el 3% + IVA. No se trasladan tarifas comerciales concretas ni se garantiza un neto final. El usuario confirmó que **también lo asume el arrendador**, mediante descuento de lo que recibe. Mostrar el desglose antes de aceptar/pagar; si el costo exacto solo se conoce al procesar, distinguir la estimación del importe real conciliado.

Para un arriendo de $100.000, el arrendatario paga $100.000, espaciGo recibe $3.570 y el neto del arrendador es **$96.430 menos la tarifa efectiva de Mercado Pago** y sus ajustes aplicables. No fijar una tarifa ficticia del proveedor.

La garantía no forma parte de la base de esta comisión: para un cobro de garantía `G`, la comisión de espaciGo y el IVA de esa comisión son cero. La tarifa del procesador sigue siendo un concepto independiente a cargo del arrendador. Definir explícitamente cómo se financia la diferencia entre lo recibido neto y la obligación contractual de devolver la garantía, sin reducirla automáticamente por esa tarifa.

Usar importes CLP enteros, regla explícita de redondeo y snapshot por cobro de base, tasa, IVA, reparto, tarifa del procesador conocida y neto conciliado. No recalcular cobros históricos cuando cambie una tasa. La suma mensual y la anual pueden diferir por redondeo en otros importes.

La conversación distingue comisión neta e IVA recaudado: el IVA no es ingreso neto de espaciGo ni necesariamente el importe final a enterar. No asumir que Mercado Pago lo reserva en una cuenta separada ni que cobrar IVA cubre todos los impuestos de la empresa. Validar con contabilidad el tratamiento de intermediación, arriendos, documentos tributarios y ajustes. Este traslado no resuelve esas obligaciones.

## 4. Mensualidades y anticipo anual

Ejemplo anterior: $200.000 mensuales, 12 meses, total $2.400.000.

| Modalidad | Base por pago | Comisión neta | IVA comisión | Comisión total |
| --- | ---: | ---: | ---: | ---: |
| Pago mensual, cada mes efectivamente pagado | $200.000 | $6.000 | $1.140 | $7.140 |
| Pago anual anticipado, una sola operación | $2.400.000 | $72.000 | $13.680 | $85.680 |
| Suma de 12 pagos mensuales del ejemplo | $2.400.000 | $72.000 | $13.680 | $85.680 |

**Decisión confirmada:** el pago anual anticipado genera una comisión única de $85.680 en este ejemplo. No se vuelve a cobrar comisión por los mismos meses. El arrendador recibe $2.314.320 antes de la tarifa del procesador. Cualquier asignación contable mensual deberá definirse con contabilidad y no crea nuevos cargos.

Para mensualidades, generar obligaciones e intentos independientes y aplicar la comisión a cada pago efectivamente procesado. **El usuario confirmó el pago iniciado por el arrendatario mediante un enlace cada mes.** El débito automático y las suscripciones quedan fuera de esta entrega; no se condiciona el lanzamiento a su compatibilidad con Split. Una renovación crea nuevas obligaciones; no se cobra de nuevo por periodos ya cubiertos.

Distinguir comisión correspondiente a pagos procesados, comisión prevista de mensualidades impagas y comisión revertida por reembolso. No confundir ese seguimiento con una política contable de devengo ya aprobada.

## 5. Personas sin Mercado Pago

### Arrendatario que paga

La conversación señala que Checkout Pro permite pagar como invitado con los medios habilitados; no tener cuenta no implica necesariamente necesitar otro proveedor. Confirmar disponibilidad efectiva para el checkout, país y medios elegidos antes de describirlo como comportamiento garantizado.

### Arrendador que recibe

**Decisión vigente:** en esta entrega debe crear/conectar Mercado Pago mediante OAuth para habilitar cobros. Las alternativas siguientes se conservan únicamente como antecedentes para etapas futuras; no son opciones disponibles ni desarrollo autorizado en la entrega inicial. El usuario confirmó que puede publicar sin vincular, pero debe vincular antes de aceptar un arriendo que genere cobros.

El flujo Split 1:1 descrito requiere cuenta Mercado Pago y autorización OAuth del vendedor. No permite inferir un abono a cualquier cuenta bancaria de un arrendador sin cuenta Mercado Pago.

| Alternativa | Requisitos y límites a conservar |
| --- | --- |
| Crear/conectar Mercado Pago | Única opción de esta entrega, definida por el usuario: habilitar cobro online al completar OAuth y elegibilidad. Se permite publicar sin conexión; la vinculación es obligatoria antes de aceptar un arriendo que genere cobros. |
| Otro proveedor con abonos bancarios | Fuera de esta entrega. En una etapa futura, verificar recaudación, identificación del beneficiario, reparto/comisión, KYC/KYB, tarifas, plazos, devoluciones, conciliación y admisibilidad contractual para marketplace chileno. |
| Transferencia directa al arrendador | Fuera de esta entrega. Una evaluación futura requiere verificar/conciliar el pago y cobrar aparte la comisión a quien corresponda; no garantiza recaudación automática de comisión. Definir mora, evidencia, disputas y eventual acceso restringido. |

Fintoc se mencionó como candidato a evaluar, **no como proveedor seleccionado ni split equivalente verificado**. Poder enviar transferencias a terceros no demuestra que un producto recaude y divida pagos para este marketplace. Tampoco queda aprobada una solución donde espaciGo reciba/custodie todo el dinero y luego lo distribuya.

## 6. Diseño agnóstico de proveedores

Separar las reglas de contrato, calendario, base/tasa de comisión, IVA, distribución comercial y conciliación del adaptador de Mercado Pago. Solo se implementará Mercado Pago en esta entrega; los adaptadores adicionales y su configuración quedan fuera de alcance y podrán evaluarse posteriormente.

Flujo conceptual: contrato → obligación de pago → servicio de pagos → adaptador del proveedor → transacción/eventos → conciliación → efectos sobre contrato y registros financieros.

Responsabilidades conceptuales a revisar frente al modelo existente:

- Catálogo/capacidades de proveedores y métodos habilitados.
- Cuentas receptoras vinculadas, ownership, elegibilidad y estado de autorización.
- Obligaciones y periodos cubiertos, intentos y referencias externas.
- Política de comisión versionada y desglose neto/IVA/costos.
- Transacciones, webhooks procesados, reembolsos, contracargos y conciliación.
- Documentos tributarios y trazabilidad de cambios/reversiones.

La conversación propuso tablas llamadas `payment_providers`, `payment_accounts` (antes `payment_provider_accounts`), `payment_methods`, `payment_orders`, `payment_transactions`, `commission_policies`, `payment_commissions`, `payment_refunds`, `payment_webhook_events` y `payment_reconciliations`. Son un **inventario conceptual**, no instrucciones para crear diez tablas. Antes de cambiar persistencia, mapear a entidades y puertos ya existentes y justificar cualquier delta. Conservar las diferencias entre reserva, pago, garantía, liquidación y movimiento financiero.

## 7. Contratos, garantías y devoluciones

### 7.1 Garantía opcional: requisitos confirmados

- El contrato de arriendo debe permitir indicar **si exige garantía** y su monto. No todos los arriendos la requieren. Sin garantía, no aparece la sección de seguimiento ni se genera un cobro de garantía. La configuración del contrato debe permitir escoger esa condición.
- Si hay garantía, mostrar un desglose separado del arriendo: importe pactado, pago de garantía y recepción. No incluirla en la base de comisión del arriendo ni mezclar su estado con el de una mensualidad.
- **La garantía se paga por Mercado Pago en un cobro independiente del arriendo.** No se cobra comisión de espaciGo sobre la garantía; por ello tampoco hay IVA de una comisión de espaciGo sobre ese concepto. No se promete exención de cargos del procesador ni se decide aquí el tratamiento tributario propio de la garantía.
- Los fondos de garantía se entregan al **arrendador**, quien los conserva y se hace responsable de su eventual devolución. espaciGo registra y muestra el seguimiento; no mantiene esos fondos en custodia, escrow o una cuenta propia para liberarlos al finalizar.
- Ambas partes deben tener la sección de garantía vinculada al mismo contrato. El arrendatario puede ver su garantía pagada y el arrendador la garantía recibida, además del estado compartido de seguimiento. Pago y recepción deben sustentarse en hechos confirmados del proveedor, distinguiendo aprobación y disponibilidad; no habilitar confirmación ficticia solo por un retorno del navegador.
- Conservar el importe contractual, monto efectivamente pagado, referencia de cobro, receptor y seguimiento de devolución/liquidación. Esto describe información necesaria, sin ordenar tablas ni cambios de esquema.
- El usuario pide **impedir el cierre de cuenta del arrendador y la eliminación de la publicación vinculada mientras la garantía esté pendiente de resolución**. También pide que una salida del arrendador no borre su responsabilidad. Es una regla funcional solicitada para definir, no un mecanismo de recuperación de fondos ya implementado ni una garantía de devolución efectiva.

### 7.2 Liquidación de garantía: puntos que permanecen abiertos

El usuario expresó duda sobre si se devuelve íntegra siempre o se liquida conforme al contrato. **Todavía no se ha aprobado una devolución total incondicional ni una autorización al arrendador para retenerla unilateralmente.** Deben definirse:

1. Condiciones del contrato que originan devolución total, eventual descuento o retención; justificación, evidencia, aceptación y resolución de desacuerdos.
2. Plazo desde la entrega/restitución del espacio para proponer y completar la liquidación; responsables y avisos. No fijar días por suposición.
3. Canal de devolución: reembolso del cobro original u otra operación admitida, ventanas/límites del proveedor, ausencia de saldo y tarifas no recuperables. La recepción de la garantía no prueba que el arrendador conserve saldo suficiente en Mercado Pago al finalizar.
4. Cómo se acredita la devolución, quién confirma recepción y qué sucede si no responde o discrepa. Separar devolución solicitada, procesada y confirmada.
5. Regla exacta de desbloqueo: devolución acreditada o liquidación contractual resuelta, según la decisión pendiente. Una garantía recibida no está por ello liquidada; un contrato finalizado tampoco la liquida automáticamente.
6. Si se permite despublicar el espacio para impedir nuevos arriendos conservando la publicación, contrato y obligaciones; bloquear eliminación no debe interpretarse automáticamente como prohibición de despublicar. El usuario aún debe escoger ese comportamiento.
7. Correspondencia de estos bloqueos con las políticas de baja, privacidad y conservación existentes; validación jurídica antes de operación real. No prometer que una restricción de la aplicación impide a una persona abandonar el inmueble o garantiza recuperar dinero.
8. Quién cubre cualquier diferencia entre garantía contractual y neto recibido tras cargos del proveedor. La decisión de que el arrendador absorbe las tarifas no autoriza disminuir automáticamente lo que deba devolver conforme al contrato.

Como propuesta para cerrar el diseño, distinguir garantía pendiente de pago, pago en proceso, pagada/recibida, liquidación pendiente, devolución en proceso, liquidada y en disputa. Los nombres y transiciones requieren validación; no son estados implementados. Un eventual descuento contractual y una devolución deben quedar desglosados para ambas partes.

### 7.3 Relación con el arriendo y reembolsos

- Definir cuándo se acepta/firma el contrato y qué pago confirma la reserva o habilita el arriendo. No deducir validez contractual de la creación de un enlace. Si hay garantía, decidir si su pago es requisito para confirmar la reserva o entregar el espacio.
- Definir cancelación antes/después del inicio, reembolsos totales/parciales del arriendo, reversión de comisión/IVA, notas o documentos de ajuste, contracargos y disputas. La garantía tiene proceso separado y comisión de espaciGo igual a cero.
- La guía anterior advirtió que el reembolso Split puede afectar vendedor y marketplace y depender del saldo disponible. Verificar condiciones exactas para arriendo y para un cobro de garantía sin comisión; no prometer devolución completa sin validar su operativa.
- Mantener conciliación de bruto pagado, comisión neta, IVA, costos, neto al arrendador y ajustes, separando arriendo y garantía. Un comprobante cargado o evento recibido no equivale por sí solo a liquidación conciliada.

## 8. Registro de decisiones necesarias para cerrar la especificación

Estado documental: **alcance parcialmente definido; todavía no listo para implementar sin decisiones adicionales**. No completar los pendientes por suposición ni confundir una propuesta con aprobación del usuario. Las respuestas posteriores deben actualizar este registro y el texto afectado, conservando fecha y origen.

### 8.1 Decisiones del usuario ya cerradas

| ID | Regla confirmada | Evidencia |
| --- | --- | --- |
| PAG-01 | Solo Mercado Pago y Split 1:1 en esta entrega; OAuth obligatorio para cobrar; otros proveedores en etapas futuras | Instrucción expresa del usuario del 28-10-2026 |
| PAG-02 | Comisión neta 3% + IVA 19% sobre la comisión, descontada al arrendador | Conversación original y respuesta expresa del usuario del 28-10-2026 |
| PAG-03 | Anticipo anual: comisión única sobre el total pagado, sin nuevos cargos por los mismos meses | Respuesta expresa del usuario del 28-10-2026 |
| PAG-04 | Mensualidades pagadas por el arrendatario mediante enlaces; sin débito automático/suscripciones en esta entrega | Respuesta expresa del usuario del 28-10-2026 |
| PAG-05 | Posible alianza futura: intención del usuario, sin alianza ni beneficios comerciales confirmados | Instrucción expresa del usuario del 28-10-2026 |
| PAG-06 | El arrendador asume la tarifa de Mercado Pago, descontada adicionalmente de lo que recibe | Respuesta expresa del usuario del 28-10-2026; tarifa efectiva por verificar |
| PAG-07 | Garantía opcional, separada del arriendo, cobrada por Mercado Pago sin comisión de espaciGo, en manos del arrendador; seguimiento visible para ambas partes y bloqueos solicitados mientras esté pendiente | Respuestas expresas del usuario del 28-10-2026; liquidación y alcance de bloqueos pendientes en PAG-18 |
| PAG-08 | Se permite publicar sin OAuth; vinculación válida obligatoria antes de aceptar arriendo que genere cobros | Respuesta expresa del usuario del 28-10-2026 |

### 8.2 Decisiones de producto que aún requieren respuesta

| ID | Decisión pendiente | Qué debe quedar escrito para poder ejecutar |
| --- | --- | --- |
| PAG-09 | Momento del pago y confirmación de reserva | Si el primer pago se solicita tras aceptación o firma; cuándo se reserva el espacio y qué ocurre si el pago no se acredita. Si se exige garantía, definir si su pago condiciona reserva o entrega. Mantener separado el estado contractual del financiero. |
| PAG-10 | Calendario y vencimiento | Día/regla de vencimiento, periodo cubierto, primer mes proporcional o completo, zona horaria, plazo para pagar y expiración del enlace. Precisar si esta entrega incluye arriendos por hora/día además de mes/año. |
| PAG-11 | Mora e incumplimiento | Plazo de gracia, recordatorios, posibilidad de pagar vencido, efectos sobre reserva/contrato y si existen multas. No cancelar un contrato ni generar recargos por inferencia. |
| PAG-12 | Base comisionable y modalidades admitidas | Precio pactado, descuentos y quién los financia, servicios/cargos adicionales e impuestos del arriendo; permitir o excluir abonos parciales, pago de varios periodos y anticipos parciales. La garantía queda excluida de la base y la comisión nunca se calcula sobre sí misma. |
| PAG-13 | Cancelaciones y reembolsos | Condiciones antes/después del inicio; importe de arriendo devuelto; devolución total/parcial de comisión e IVA; responsable de tarifas no recuperables; quién solicita/autoriza y plazo. Referenciar una política contractual vigente cuando aplique. |
| PAG-14 | Contracargos, disputas y saldo insuficiente | Quién asume el perjuicio, reúne evidencia y autoriza acciones; qué ocurre si no puede ejecutarse un reembolso y cómo se informa. No representar un reembolso solicitado como completado. |
| PAG-15 | Cuenta receptora y cambios de titular | Personas/empresas incluidas; una cuenta activa por titular o regla distinta; si puede compartirse; posibilidad y efecto de cambiar/desvincular con cobros, reembolsos o contratos pendientes. Ningún cambio debe reescribir transacciones históricas. |
| PAG-16 | Política de tasas y cambios contractuales | Si la comisión se congela por contrato o por obligación; fecha efectiva de cambios futuros, aceptación requerida y efecto de renovaciones, descuentos y modificaciones del contrato. Lo cobrado no se recalcula. |
| PAG-17 | Operación y visibilidad | Qué ve cada parte del desglose/estado, quién gestiona conciliaciones y devoluciones y qué intervenciones manuales se permiten con autorización y auditoría. No permitir marcar pagado solo por comprobante o retorno del navegador. |
| PAG-18 | Liquidación de garantía y desbloqueos | Contrato, descuentos permitidos, plazos, evidencia, controversias, canal de devolución, diferencia por tarifas y condición de cierre; definir si se permite despublicar conservando contrato y obligación. Ver sección 7.2. |

Estos pendientes pueden resolverse por tandas. No requieren programar ni modificar el esquema. No se presenta como pendiente de esta entrega elegir un segundo proveedor ni configurar suscripciones: ambos quedaron fuera de alcance.

### 8.3 Comprobaciones documentales/técnicas que no debe resolver el usuario por intuición

| ID | Verificación necesaria | Evidencia o especificación de cierre |
| --- | --- | --- |
| VAL-01 | Mercado Pago Chile | Documentación oficial vigente y condiciones de habilitación para Split 1:1, tipos de cuenta/identidad y requisitos de la aplicación. Distinguir integración disponible de una eventual alianza. |
| VAL-02 | Checkout elegido | Seleccionar y documentar Checkout Pro y su API/flujo compatible con Split, campos de comisión (incluido cobro de garantía con comisión cero), medios aceptados, pago invitado, límites y expiración. No mezclar contratos de APIs. |
| VAL-03 | Tarifas y disponibilidad de fondos | Condiciones comerciales efectivas, impuestos del servicio, plazos de disponibilidad y abonos; explicar que pago aprobado y fondos disponibles son hechos distintos. Sin valores inventados. Verificar también tarifas del cobro de garantía y efecto sobre su neto, sin modificar por ello la obligación contractual. |
| VAL-04 | Tributación | Validación contable de base, IVA y documentos separados de arriendo/intermediación, emisor y destinatario conforme a PAG-02, emisión, ajustes y notas de crédito; tratamiento propio de garantía y cargos del procesador. Identificar qué es requisito previo a operación real. |
| VAL-05 | Dinero y redondeo | Especificar fórmula y orden de redondeo a CLP enteros, mínimos/máximos y reparto de diferencias; ejemplos de bordes y reembolsos parciales acumulados. |
| VAL-06 | Ciclo de vida y recuperación | Matriz de estados/transiciones de obligación, intento y pago; duplicados, eventos fuera de orden, confirmaciones tardías, pago duplicado y carrera entre vencimiento, cancelación y aprobación. Especificar conciliación y recuperación ante caída del proveedor. |
| VAL-07 | Seguridad e identidad | Contrato OAuth, titularidad, renovación/revocación, protección de secretos, verificación de notificaciones, permisos por operación y datos mínimos visibles; compatibilidad de los bloqueos de garantía con baja y conservación. Nunca incluir tokens reales en el documento. |
| VAL-08 | Correspondencia con el proyecto | Inventario acotado de entidades, puertos y políticas actuales; diferencias respecto del fake; contratos y escenarios de aceptación. No crear tablas ni alterar políticas aprobadas solo por esta investigación. |

### 8.4 Condición para declarar la documentación lista para desarrollo

- Todas las decisiones pendientes PAG-09–PAG-18 deben tener respuesta o exclusión explícita para esta entrega, fecha y origen; ninguna pregunta abierta debe ocultarse tras frases como «según corresponda».
- VAL-01–VAL-08 deben quedar documentadas con evidencia o con un bloqueo externo concreto y las tareas que pueden avanzar sin él. La alianza futura no sustituye habilitación ni validaciones.
- Incluir un desglose reproducible para pago mensual, anticipo anual, rechazo/reintento, duplicado, pago tardío, devolución parcial/total, contracargo, OAuth revocado y garantía (sin garantía, pagada, liquidación, devolución y disputa); cuando una modalidad se excluya, definir su bloqueo en lugar de inventar su comportamiento.
- La matriz de aceptación futura debe distinguir simulación local, pruebas de integración y habilitación para fondos reales. Este trabajo solo prepara documentación: no ejecuta pagos, pruebas de aplicación ni cambios de código/datos.

## 9. Relación con la documentación existente y alcance del traslado

- `planning/decisiones_y_hallazgos.md`: antecedente de contrato backend neutral y de investigación parcial de Split 1:1; no afirmar que Split sea escrow.
- `planning/decisiones_m02_foto_cuenta_cobro_fake.md`: cuenta de cobro sintética `fake-local-v1`, que no es vinculación OAuth ni cuenta bancaria real.
- `planning/decisiones_m06_cancelacion_local.md`: devolución sintética local sin comisiones; no se cambia retroactivamente con este contexto.
- `planning/vision_y_modulos.md`: referencias del dominio y arquitectura; revisar documentación vigente antes de un futuro corte.

Se conserva el marco existente y se añade este documento para continuar decisiones. No se declara integración real, aceptación de propuestas, migración aplicada ni validación tributaria. No se accedió a cuentas ni se movieron fondos.

Preparado en una copia de trabajo aislada del proyecto, rama `codex/docs/chore/pagos-01-contexto-de-pago-espacigo`, basada en `origin/main` `3956d2f3459b0bf4b15511f4e1a74ffd06a29975`. El usuario autorizó subir el contexto a una rama. Su publicación documental no implica aprobación de pendientes, incorporación a `main` ni autorización de desarrollo de pagos.
