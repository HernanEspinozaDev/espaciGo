# espaciGo — definición documental de pagos con Checkout Pro

Fecha: 28-10-2026, America/Santiago. Complementa `contexto_pagos_marketplace_20261028.md` en la misma rama documental. No implementa pagos, firma, DTE, código, migraciones ni cambios de datos.

**Origen y estado:** Checkout Pro, el pago después del acuerdo contractual y la separación de arriendo/garantía son instrucciones del usuario. Las reglas operativas, redondeos y política para estancias cortas que siguen son definiciones propuestas por el asistente a petición del usuario para preparar el desarrollo. Se conservan en PR borrador para revisión; no se presentan como condiciones ya aceptadas por usuarios finales ni como asesoría tributaria individual. Las reglas de garantía siguen siendo las de cada contrato, conforme a la decisión previa.

## 1. Producto elegido y frontera de esta entrega

- **Mercado Pago Checkout Pro, API de Preferences, Split Payments 1:1 y OAuth por arrendador.** El comprador completa el pago en Mercado Pago. Checkout API con formulario propio queda para una evolución posterior; no se desarrolla ahora ni se incorpora una segunda integración Orders.
- Checkout Pro también utiliza APIs en backend: crear preferencia, consultar pagos, procesar notificaciones y gestionar reembolsos. La simplificación consiste en delegar la interfaz de cobro y captura de tarjeta; no elimina la integración de servidor.
- Flujo de referencia: `POST /checkout/preferences`, autorización OAuth del vendedor y `marketplace_fee` como importe CLP calculado por espaciGo. Cada cobro tiene su referencia interna y enlace de checkout. La documentación de marketplace confirma este modelo [MP-1]; la creación de aplicación identifica explícitamente Checkout Pro / API de Preferences [MP-2].
- Comisión de espaciGo: 3% + IVA, a cargo del arrendador; tarifa del procesador también a cargo del arrendador. La garantía tiene comisión de espaciGo cero. No introducir `application_fee` de Checkout API en el contrato de preferencias.
- Los cobros son íntegros por obligación: mensualidad, periodo corto o anticipo anual acordado. Sin abonos parciales, carrito entre arrendadores, débito automático, penalidades automáticas ni cobro de firmas en esta entrega. Un reembolso parcial sí está contemplado y es distinto de admitir pagos parciales.
- Conservar el contrato de negocio separado del proveedor para una futura integración, sin desarrollar ahora infraestructura para múltiples proveedores ni crear tablas por anticipado.

## 2. Acuerdo, habilitación de pago y ocupación

**Regla confirmada:** el pago se habilita después de que ambas partes hayan aceptado la misma versión del contrato de ocupación. No se cobra por contactar al arrendador, enviar una solicitud ni generar un borrador contractual. OAuth válido es requisito antes de que el arrendador acepte un arriendo con cobros.

Secuencia documental:

1. Las partes acuerdan espacio, periodo, precio, calendario y, si corresponde, garantía y política de cancelación.
2. Ambas aceptan la misma versión contractual; conservar el registro de aceptación disponible en el proyecto. Esto no se presenta como firma electrónica avanzada ni como certificación de validez jurídica.
3. Se habilita al arrendatario **Pagar arriendo**. Si se pactó garantía, aparece además **Pagar garantía**, con otro cobro y otro enlace. Sin garantía no aparece esa sección.
4. El servidor confirma cada pago con consulta al proveedor y notificación verificada. La pantalla de retorno no acredita pago.
5. Propuesta operativa: el arriendo queda financieramente habilitado para iniciar la ocupación cuando el pago inicial y, si se exige, la garantía estén acreditados. Un solo concepto pagado no oculta el otro pendiente. La fecha y efectos jurídicos del contrato siguen separados del estado financiero.
6. Una modificación de precio, fechas o garantía después de aceptar exige una nueva versión aceptada y anular/inutilizar los cobros pendientes anteriores, sin reescribir pagos ya aprobados. Las diferencias sobre pagos anteriores se resuelven mediante ajustes trazables, no cargos silenciosos.

La aplicación no da por aceptado un contrato por el mero pago. La firma pagada de SimpleFirmas no es requisito de esta entrega ni condiciona habilitar el cobro con el mecanismo contractual vigente.

### Cuentas, permisos y continuidad

- Propuesta: una cuenta Mercado Pago activa por titular arrendador, cuya identidad/representación corresponda al beneficiario contractual; no habilitar compartir una cuenta entre titulares independientes en esta entrega. Una empresa requiere representación autorizada, sin inferir que cualquier administrador puede cambiar el receptor.
- El arrendatario consulta y paga sus obligaciones, solicita devoluciones y aporta/responde reclamos de su contrato. El arrendador consulta sus cobros, atiende reclamos y autoriza/solicita devoluciones conforme al contrato. Ninguno puede editar montos históricos, confirmar por sí solo un pago remoto ni operar contratos ajenos.
- Una operación de conciliación o gestión de incidencia requiere capacidad expresa ya definida o que se revise antes de implementar. No conceder facultades financieras a todos los administradores por nombre de rol. Registrar actor, motivo y resultado.
- La revocación externa de OAuth no puede impedirse desde espaciGo: renovar cuando sea posible; si falla, bloquear nuevos cobros/aceptaciones y solicitar reconexión, conservando historia y obligaciones. No detener consultas/reconciliación que sigan siendo autorizadas ni declarar reembolsos imposibles solo por un estado local.
- No sustituir receptor de cobros históricos al cambiar de cuenta. El cambio solo afecta obligaciones futuras no cobradas, con validación de titularidad y regeneración de enlaces pendientes; mantener los pagos/reembolsos ligados al vendedor original. Una desconexión voluntaria debe advertir obligaciones pendientes y no borrar sus referencias.
- Propuesta para garantías: permitir **despublicar** para no aceptar nuevos arriendos, conservando publicación, contrato e historial. Bloquear el cierre operativo de cuenta del arrendador y la eliminación de esa publicación mientras exista garantía pendiente o disputada. Desbloquear cuando consten devolución del saldo y/o aplicación a daños según acuerdo contractual o resolución competente, sin exigir devolver importes legítimamente aplicados. Una afirmación unilateral, el silencio o cerrar un reclamo sin respaldo no bastan para liquidarla. Revisar compatibilidad de estos bloqueos con las reglas de baja/privacidad antes de fondos reales.
- En reclamos, registrar presentación, respuesta, antecedentes, propuesta de liquidación, acuerdo o desacuerdo y resultado. Una discrepancia mantiene el expediente y la obligación en disputa; espaciGo facilita comunicación, no sustituye al órgano competente. La falta de respuesta dispara recordatorio/escalamiento operativo, no aprobación automática del descuento.

## 3. Calendario y vencimientos: definición operativa propuesta

- Fechas y vencimientos forman parte del contrato; se muestran en America/Santiago y se conservan como instantes inequívocos. Hora, día o noche se refieren al intervalo de ocupación pactado, no a una nueva categoría de publicación creada por este documento.
- Cada obligación indica concepto, periodo cubierto, importe CLP, vencimiento y arrendador beneficiario. Para un contrato mensual el calendario se acuerda al aceptarlo. Sin fecha/importe acordados no se genera el enlace; no asumir siempre día 1 ni meses de 30 días.
- Si un vencimiento mensual anclado a un día no existe en un mes, la propuesta es usar el último día de ese mes y volver al día ancla en el siguiente, dejando esta regla visible en el calendario aceptado.
- Primer periodo: usar el importe total o proporcional que figure en el contrato. No prorratear ni cambiar el precio después de la aceptación. Una modificación requiere nueva aceptación.
- Para el primer pago, proponer bloqueo temporal de disponibilidad de 30 minutos en horas/días/noches y 24 horas en arriendos mensuales/anuales, siempre limitado por el inicio pactado y cualquier plazo menor del contrato. Solo aplicarlo si encaja con las reglas de disponibilidad existentes; no crear reservas paralelas ni borrar un contrato al vencer el bloqueo.
- La preferencia y el botón de pago deben respetar el vencimiento de la obligación. Expirar un enlace no prueba que un pago en proceso haya fallado: consultar/cancelar lo cancelable y conciliar antes de liberar o volver a cobrar. La expiración general del proveedor no sustituye el plazo de la reserva.
- Pagos aprobados tardíamente: no confirmar una ocupación ya reasignada. Registrar incidencia, impedir doble asignación y gestionar devolución si no puede prestarse el arriendo. No borrar el cobro ni marcarlo rechazado.
- Mensualidades posteriores: enlace por cada obligación, aviso propuesto 3 días antes y al vencer; recordatorio al día siguiente y a los 3 días. Nunca generar otra obligación o un segundo cargo por enviar un recordatorio.
- Mora: no aplicar multas, desalojos, resolución contractual ni retenciones automáticas desde pagos. Registrar vencido y seguir las consecuencias pactadas por las partes. Para una fecha ya transcurrida, solo habilitar el cobro si la obligación sigue siendo exigible conforme al contrato y está conciliada.

## 4. Base, redondeos y consistencia monetaria

Definición propuesta: `B` es el total CLP del **concepto arriendo efectivamente pactado y cobrado**, después de descuentos acordados y con los impuestos del arriendo que correspondan ya incorporados al precio final. Se excluyen garantía, firmas futuras y cargos de terceros. El módulo de pagos no calcula por su cuenta el IVA del inmueble. Si se quiere comisionar solo renta neta sin ese IVA, será un cambio comercial expreso antes de implementar.

- Todos los importes enviados/cobrados en CLP son pesos enteros. La regla de la aplicación es **redondeo al peso más cercano, con 0,5 hacia arriba**, mediante cálculo decimal o enteros; no usar coma flotante binaria ni redondeo bancario por defecto.
- Comisión neta `C = redondear(B × 3 / 100)`; IVA comisión `I = redondear(C × 19 / 100)`; total marketplace `F = C + I`. Calcular el IVA sobre la comisión ya redondeada. No usar directamente `redondear(B × 3,57%)`, porque puede dar otro resultado.
- Neto del arrendador: `B − F − tarifa efectiva Mercado Pago`. No estimar el neto como garantizado antes de conocer esa tarifa. El precio pagado por el arrendatario no aumenta por estas comisiones.
- Ejemplos: `B=100.000 → C=3.000, I=570, F=3.570`; `B=150 → C=5, I=1, F=6`; `B=350 → C=11, I=2, F=13`. Los ejemplos pequeños prueban aritmética; no garantizan que el proveedor acepte esos mínimos.
- Guardar base, tasas, orden de redondeo y desglose por obligación/transacción. Propuesta: congelar la política de comisión en la versión contractual aceptada para sus cobros; las renovaciones o cambios de comisión requieren nueva aceptación. No recalcular historia por cambios de configuración.
- No cobrar importes cero/negativos ni superiores a los límites efectivos de Mercado Pago. La habilitación comercial debe verificar mínimos/máximos y que exista saldo de venta suficiente para las comisiones; no inventar límites numéricos.
- Descuentos posteriores a un pago son ajustes/reembolsos. Nuevas tarifas o servicios adicionales requieren una obligación expresamente acordada; no se incorporan ocultamente al precio de una mensualidad.

### Redondeo de comisión en devoluciones parciales

Para una transacción original de base `B`, comisión `C` e IVA `I`, y devolución acumulada aprobada de arriendo `R` (`0 ≤ R ≤ B`):

- Objetivo acumulado de reversión de comisión: `Cr = redondear(C × R / B)`.
- Objetivo acumulado de reversión de IVA: `Ir = redondear(I × R / B)`.
- Cada devolución revierte la diferencia entre esos objetivos y lo ya revertido. El último reembolso total devuelve exactamente los residuos originales: nunca más de `C` e `I`.
- El importe reembolsado al arrendatario es el acordado sobre el pago original; **no se le descuenta otra vez la comisión de espaciGo**. La reversión de comisión/IVA compensa internamente al arrendador y marketplace.
- Registrar los importes reales que Mercado Pago distribuya; si su distribución proporcional difiere por redondeo del objetivo contable, registrar la diferencia y resolverla. No declarar que la API permite ordenar por separado cualquier reversión de comisión que el modelo no soporte.
- Garantía: comisión e IVA de comisión son cero, incluso en devoluciones parciales. Los cargos del procesador los absorbe el arrendador; no reducen automáticamente la obligación de devolver la garantía.

## 5. Normas propuestas para cancelaciones y problemas de uso

**Son una propuesta comercial de espaciGo solicitada por el usuario para contratos cortos; deben mostrarse antes de aceptar/pagar e incorporarse a la versión contractual. No son porcentajes impuestos por Mercado Pago ni una afirmación de exigencia legal.** Los descuentos de garantía por daños siguen exclusivamente las condiciones del contrato. No aplicar la tabla retroactivamente a contratos existentes.

| Situación | Propuesta para horas/días/noches |
| --- | --- |
| El arrendador cancela, no entrega acceso o no tiene disponible el espacio | Devolver 100% del arriendo no prestado; si nada pudo utilizarse, 100% del pago. Garantía sin ocupación se devuelve íntegra salvo obligación contractual válida distinta que corresponda evaluar. |
| El inmueble presenta un problema grave que impide el uso pactado o es inseguro | Suspender el uso afectado y abrir incidencia. Devolver la parte no prestada/no utilizable; si el uso completo perdió su finalidad, evaluar devolución total conforme al contrato y derechos aplicables. No obligar a ocupar un lugar inseguro. |
| Emergencia objetiva que impide usar el espacio: evacuación, cierre de autoridad, incendio u otra afectación comprobable | Ofrecer reprogramación aceptada por ambas partes o devolución del tramo no utilizable. No imponer un cupón como única solución ni cobrar doble comisión por cambiar fecha sin nuevo pago. |
| Cancelación voluntaria del arrendatario al menos 48 horas antes del inicio | Devolución de 100% del arriendo. |
| Cancelación voluntaria entre 24 horas inclusive y menos de 48 horas antes | Devolución de 50% del arriendo. |
| Cancelación voluntaria a menos de 24 horas o inasistencia sin incidencia acreditada | Sin devolución automática por la política comercial propuesta; revisar acuerdos de excepción y derechos legales que procedan. No confundir con incumplimiento del arrendador. |
| Emergencia personal acreditada | Abrir revisión excepcional, proponer reprogramación sin penalidad o devolución pactada. No prometer que cualquier motivo personal genera automáticamente devolución total; no exigir diagnósticos ni datos de salud innecesarios. |
| Uso parcialmente prestado y luego interrumpido por causa ajena al arrendatario | Desglosar unidades útiles y afectadas usando precios del contrato; devolver las no utilizables. Si una parte esencial vuelve inútil el conjunto, revisión del importe total. |

Las franjas se calculan desde la recepción de la solicitud hasta el inicio pactado, con instantes registrados. Para reservas hechas a menos de 48 horas del inicio, mostrar de forma destacada la franja aplicable antes de contratar; no anunciar una cancelación gratuita inexistente.

**Arriendos mensuales/anuales:** la terminación y devolución de periodos no usados se rigen por lo pactado y la normativa aplicable; no trasladar automáticamente las franjas de 48/24 horas. Un contrato no significa que nunca pueda corresponder devolución, por ejemplo ante pago duplicado, cobro indebido o incumplimiento. Un anticipo anual puede requerir reembolso parcial: se calcula contra el pago original, no contra una mensualidad ficticia.

**Derechos de consumidores:** cancelación comercial, incumplimiento y retracto son conceptos diferentes. Cuando exista relación de consumo, deben respetarse los derechos aplicables aunque el contrato diga algo incompatible. SERNAC describe el retracto en contratación a distancia y las condiciones de información [CON-1, CON-2]. Esta entrega no establecerá una exclusión general automática de retracto; la aplicación concreta por tipo de operación requiere revisión jurídica antes de operar con fondos reales. La referencia de SERNAC sobre viajes suspendidos orienta la atención de emergencias, pero no establece por sí sola una regla idéntica para todo arriendo [CON-3].

## 6. Cómo se procede a una devolución

1. El interesado solicita cancelación/devolución desde el contrato y selecciona concepto y motivo: arriendo no prestado, cancelación voluntaria, emergencia, cobro duplicado o devolución de garantía. Reclamos por daños/garantía conservan su opción ya acordada.
2. Se identifican contrato y versión/política aplicable, pagos afectados, montos aprobados y reembolsos previos. Presentar la solicitud no devuelve dinero ni autoriza descuentos de garantía.
3. El arrendador responde; en casos de acuerdo se registra importe y motivo. Si hay desacuerdo, queda en disputa, con antecedentes y vía de resolución aplicable; espaciGo no se atribuye un arbitraje vinculante ni facultades no pactadas. Las intervenciones operativas requieren un responsable autorizado y auditoría, sin inventar un nuevo rol de soporte.
4. Propuesta de atención: respuesta inicial en 2 días hábiles y envío del reembolso al proveedor dentro de 2 días hábiles desde que quede autorizado y técnicamente ejecutable. Son objetivos operativos; no un plazo de extinción del derecho ni una promesa del tiempo que tomará el banco. Informar bloqueo por saldo, vencimiento o revisión en lugar de marcar completado.
5. Para un pago aprobado, ejecutar reembolso total o parcial del **pago original**, con credencial autorizada, operación idempotente y límite por saldo reembolsable. Consultar la referencia de reembolsos de **Checkout Pro Preferences** [MP-3], no copiar endpoints de Orders. Conservar identificadores de pago/reembolso y resultado; evitar doble devolución por reintento o concurrencia con contracargo.
6. Para pagos pendientes/en proceso, gestionar cancelación cuando corresponda; antes de actuar reconsultar el estado. Un pago que acaba de aprobarse pasa por el flujo de reembolso, no se borra como si nunca hubiese existido.
7. Distinguir solicitud, autorización, envío, en proceso, confirmado y fallido/bloqueado. Confirmar por consulta/evento del proveedor y conciliar el importe; luego informar a ambas partes y reflejar el ajuste contractual y tributario correspondiente. La anulación de un contrato y la devolución de dinero son hechos separados.

Mercado Pago documenta reembolsos totales/parciales, plazo de **180 días desde la aprobación**, saldo suficiente y abono según el medio original [MP-3]. No prometer devolución instantánea ni que el destinatario pueda escogerse libremente.

En Split 1:1, el reembolso se distribuye proporcionalmente entre vendedor y marketplace; la falta de saldo del vendedor puede impedir reembolsar todo [MP-1]. La propuesta es revertir proporcionalmente la comisión de espaciGo y su IVA junto al arriendo devuelto, incluso en cancelación voluntaria, manteniendo solo la parte asociada al importe retenido. Los costos de Mercado Pago que no revierta el proveedor siguen a cargo del arrendador, sin reducir el reembolso aprobado al cliente. La devolución de garantía no tiene comisión de espaciGo que revertir.

Un contracargo es una controversia del medio de pago: registrar el débito/notificación real, suspender devoluciones duplicadas sobre el mismo importe y aportar evidencia de contrato, aceptación y uso cuando exista. Propuesta comercial: el arrendador responde por la parte del arriendo/garantía que recibió; espaciGo por su comisión revertida, sin financiar por defecto la deuda del otro. Cargos adicionales o responsabilidad por fraude se determinan conforme a las condiciones efectivas del proveedor y las obligaciones aplicables, no por una promesa universal de esta especificación. La resolución del contracargo no resuelve por sí sola el contrato ni acredita un daño al inmueble.

### Estados y recuperación propuestos

| Hecho confirmado | Efecto financiero de la plataforma |
| --- | --- |
| Obligación acordada, sin pago | Pendiente; enlace disponible solo si contrato, cuenta, importe y vencimiento son válidos. |
| Pago remoto pendiente/en proceso | En proceso; no dar por pagado ni generar otro intento concurrente sin revisar/cancelar el anterior. |
| Intento rechazado/cancelado | Conservar intento y permitir reintento si la misma obligación sigue vigente. No crear otra deuda. |
| Aprobación acreditada | Pagada; impedir nuevos cobros de la misma obligación. No confundir con fondos disponibles para retiro. |
| Devolución autorizada pero aún no confirmada | Devolución en proceso o bloqueada; conservar por separado que el pago fue aprobado. |
| Devolución parcial/total confirmada | Parcialmente reembolsada/reembolsada; conservar importe original, acumulado y saldo. No recrear automáticamente una deuda ni resolver el contrato por inferencia. |
| Reclamo o contracargo sin resolver | Incidencia/disputa separada del historial de pago; no inventar devolución ni nueva aprobación. |

Verificar autenticidad y correlación de notificaciones, procesar duplicados sin repetir efectos y consultar el estado remoto antes de aceptar transiciones ante eventos fuera de orden. Ante timeout, consultar el resultado antes de repetir una devolución. Conciliar pagos/reembolsos inconclusos al recuperar servicio y antes de permitir un nuevo cargo sobre la misma obligación. La caída de Mercado Pago deja un estado pendiente de verificación; no habilita una transferencia directa alternativa ni una confirmación manual sin evidencia del proveedor. El mapeo de nombres exactos a los estados existentes se revisará antes de implementar, sin alterar los hechos descritos.

### Garantías largas y ausencia de saldo

- El cobro separado no garantiza saldo reservado en la cuenta del arrendador. El contrato debe prever su obligación de restitución, sin presentarlo como custodia de espaciGo.
- Si el cobro de garantía supera la ventana de 180 días, el reembolso ordinario documentado puede no estar disponible. La obligación contractual no desaparece ni se resuelve con un nuevo enlace para cobrar al arrendatario.
- Propuesta de excepción operativa para restituciones fuera de ventana: el arrendador devuelve por un medio lícito acordado con el arrendatario fuera de la integración de cobro; espaciGo registra evidencia y recepción con estado separado de «reembolso Mercado Pago». No es un segundo método de cobro ofertado ni integración de transferencias bancarias en esta entrega. Requiere validación contractual/operativa antes de habilitar garantías que excedan esa ventana; si no se admite esta excepción, no prometer el ciclo completo para esas garantías.
- Con falta de saldo, informar, requerir reposición al responsable y mantener la obligación pendiente; no financiar silenciosamente con fondos de espaciGo ni descontar de pagos ajenos. Un contracargo tampoco se presenta como devolución de garantía acordada.

## 7. Dinero en cuenta y transferencias

No se desarrollará una billetera, saldo interno ni botón de transferencia entre usuarios. Arriendo y garantía se cobran con enlaces de Checkout Pro. Los medios concretos deben estar habilitados para la cuenta chilena y el producto Split; no asumir todos los medios de un checkout genérico o de WooCommerce.

La guía Split menciona pagos con dinero en cuenta entre cuentas Mercado Pago y excluye transferencias desde instituciones externas en ese flujo [MP-1]. Eso no acredita una API genérica para transferir libremente saldo entre arrendador y arrendatario desde espaciGo. El saldo del comprador que el checkout admita es un medio de pago del cobro; devolver un pago es otro proceso. La opción de invitado, tarjetas y saldo deben validarse en la integración elegida antes de mostrarlas como disponibles; no anunciar una transferencia bancaria directa alternativa.

## 8. Documentación tributaria y respaldos

Definición documental: separar las siguientes operaciones y sus emisores. La emisión se realiza por un sistema autorizado o el SII según corresponda; **no se incorpora ahora un desarrollo de facturación electrónica ni integración SII**. La preparación de pagos debe conservar referencias que permitan conciliación y documentación posterior; antes de fondos reales, el circuito de emisión sí debe estar habilitado y validado por el responsable tributario.

| Operación | Responsable y respaldo previsto |
| --- | --- |
| Arriendo/alojamiento | El arrendador documenta la operación con el arrendatario según su situación y el inmueble: factura/boleta afecta, exenta/no afecta u otro respaldo que legalmente corresponda. No emitir siempre el mismo DTE ni aplicar IVA del 19% a todo arriendo por defecto. |
| Comisión de espaciGo | espaciGo documenta al arrendador su servicio de intermediación: comisión neta 3%, IVA sobre ella y total. Factura o boleta electrónica afecta según corresponda al receptor/operación. No facturar a nombre de espaciGo todo el canon como si fuera su ingreso. |
| Garantía genuinamente restituible | Recibo y trazabilidad separados, vinculados al contrato; el SII indica que la garantía de arriendos de inmuebles amoblados/con instalaciones no está gravada con IVA en cuanto no es venta ni servicio [TAX-3]. Si después se aplica a otra obligación, revisar la naturaleza de esa aplicación antes de asignarle tratamiento tributario. No convertir automáticamente el recibo en DTE exento. |
| Tarifa Mercado Pago | Conservar liquidación y documento del proveedor para el receptor que efectivamente lo soporte. Es independiente del DTE de comisión de espaciGo. |
| Reembolso/ajuste | Cada emisor regulariza sus documentos cuando corresponda; espaciGo ajusta su comisión y el arrendador su operación. Vincular nota de crédito/documento correctivo con folio original, motivo y montos. Un reembolso de Mercado Pago no anula por sí solo el DTE [TAX-5]. |

La regla general de servicios afectos a IVA tiene excepciones [TAX-1]. El SII confirma IVA para arriendos amoblados [TAX-2]; no extrapolar sin revisar inmuebles sin amoblar, instalaciones, hospedaje, exenciones ni reglas específicas de base imponible. No mezclar ese impuesto con el IVA de la comisión.

Conservar para documentación: identidad tributaria de emisor/receptor cuando corresponda, concepto, contrato/obligación, fecha, base neta, IVA/exención aplicable, monto total, tipo/folio y referencia del emisor autorizado, más vínculos de ajustes. Acceso por titular/parte autorizada y política de conservación del proyecto; no inventar aquí un plazo universal de retención.

Un comprobante electrónico puede reemplazar una boleta bajo el modelo de emisión informado al SII [TAX-4]; no asumir que cualquier pantalla o recibo de checkout sustituye la factura o documenta simultáneamente arriendo y comisión. Validar ese modelo para cada emisor y evitar duplicar ventas en el registro tributario.

**Formalización del marketplace:** el SII informó la exigencia de acreditar inicio de actividades por plataformas desde el 02-01-2026 [TAX-6]. Revisar aplicabilidad y excepciones por arrendador antes de habilitar operaciones reales; OAuth/KYC no sustituye esa validación. La Resolución 115/2026 incorpora la DJ 1966 para plataformas respecto de quienes declaran no requerir inicio de actividades, desde AT 2027 sobre operaciones de 2026 [TAX-7]. Es una dependencia tributaria de lanzamiento, no permiso para construir ahora una integración SII ni para exigir a todos una categoría tributaria sin analizar su caso.

## 9. Firma pagada: dependencia futura explícita

El usuario prevé un pago por firma legal de contrato mediante **SimpleFirmas**, con cobro a ambas partes si se elige ese servicio. **No está implementado y queda fuera del desarrollo inicial de pagos.** No definir todavía importe, reparto, proveedor de cobro, momento de cargo, flujo de autorización o devoluciones por firma.

Una vez finalizado el desarrollo de pagos, abrir una definición separada para ese concepto. La arquitectura debe distinguir arriendo, garantía y eventual firma; no agregar ahora enlaces, servicios, pantallas, obligaciones cobrables ni una dependencia de SimpleFirmas para completar arriendo/garantía. No se ha consultado ni validado la API o condiciones comerciales de SimpleFirmas en esta investigación.

## 10. Condiciones y escenarios antes de ejecutar

- Validar aplicación/país, requisitos de vendedor (la documentación de Split exige identificación KYC), OAuth, comisión cero para garantía y medios disponibles; la alianza futura no sustituye esta habilitación [MP-4].
- Confirmar contrato/API de reembolsos y comportamiento real del reparto, redondeos y tarifas con cuentas de prueba; probar también OAuth revocado, saldo insuficiente y pago fuera de ventana. No ejecutar pruebas ni mover fondos por este documento.
- Antes de implementación, mapear estas reglas al modelo existente de contratos, disponibilidad, reclamos y pagos fake. Los nombres conceptuales no ordenan crear entidades duplicadas.
- La propuesta para estancias cortas, reservas temporales, recordatorios, permisos operativos y excepción de devolución externa requiere revisión del borrador. Las cláusulas jurídicas/tributarias individuales y disponibilidad comercial no pueden darse por certificadas solo por investigación documental.
- Escenarios de aceptación futuros: aceptación unilateral no habilita pago; aceptación mutua habilita enlace; arriendo y garantía separados; garantía inexistente no aparece; mensualidad y anticipo anual sin doble comisión; rechazo/reintento sin doble pago; pago tardío sin doble ocupación; devoluciones total/parcial con redondeo acumulado; reclamación por daño sin retención automática; garantía fuera de 180 días; documentos tributarios separados; ninguna dependencia de firma pagada.

## Fuentes oficiales consultadas el 28-10-2026

La documentación de Mercado Pago se consultó mediante su buscador oficial con país **MLC**. Algunas páginas web devolvieron 403; se usó el contenido del conector, que remite a la documentación global. No se trasladaron ejemplos de Pix/BRL ni condiciones de otro país a Chile. Las fuentes de normativa deben revisarse contra el caso concreto antes de operación real.

- [MP-1 — Integrar checkout en Split 1:1](https://www.mercadopago.com/developers/es/docs/split-payments/split-1-1/integration-configuration/integrate-marketplace).
- [MP-2 — Checkout Pro Preferences: crear aplicación](https://www.mercadopago.com/developers/es/docs/checkout-pro-preferences/create-application).
- [MP-3 — Checkout Pro Preferences: reembolsos y cancelaciones](https://www.mercadopago.com/developers/es/docs/checkout-pro-preferences/additional-settings/refunds-and-cancellations).
- [MP-4 — Requisitos Split 1:1](https://www.mercadopago.com/developers/es/docs/split-payments/split-1-1/prerequisites).
- [CON-1 — SERNAC: derecho a retracto](https://www.sernac.cl/portal/617/w3-article-57413.html).
- [CON-2 — SERNAC: comercio electrónico y deberes de información](https://www.sernac.gob.cl/portal/604/w3-article-65050.html).
- [CON-3 — SERNAC: viajes suspendidos y alternativas](https://www.sernac.cl/portal/604/w3-article-62173.html).
- [TAX-1 — SII: IVA a servicios desde 2023 y excepciones](https://www.sii.cl/noticias/2022/211222noti01aav.htm).
- [TAX-2 — SII: IVA del arriendo amoblado](https://www.sii.cl/preguntas_frecuentes/impuestos_mensuales/001_130_0792.htm).
- [TAX-3 — SII: garantía de arriendo e IVA](https://www.sii.cl/preguntas_frecuentes/impuestos_mensuales/001_130_3300.htm).
- [TAX-4 — SII: boleta y comprobante electrónico](https://www.sii.cl/destacados/boletas_electronicas/index.html).
- [TAX-5 — SII: anulación de boleta electrónica](https://www.sii.cl/preguntas_frecuentes/factura_electronica/001_003_5352.htm).
- [TAX-6 — SII: acreditación de inicio de actividades desde enero 2026](https://www.sii.cl/noticias/2025/311225noti01pcr.htm).
- [TAX-7 — SII: Resolución 115/2026, DJ 1966](https://www.sii.cl/normativa_legislacion/resoluciones/2026/reso115.pdf).
