# espaciGo — decisiones operativas, permisos y conservación de pagos

Fecha de investigación: 28-10-2026, America/Santiago. Complementa el contexto, la especificación Checkout Pro y el documento de historial/dashboard de esta rama.

**Alcance:** definición documental preparada a petición del usuario para concretar las propuestas comerciales, autorización financiera, condiciones del proveedor y conservación. Las instrucciones expresas del usuario siguen teniendo prioridad. Las decisiones de diseño que se fijan aquí son la recomendación resultante de esa investigación para el borrador de desarrollo; no se inventa aceptación contractual de terceros, habilitación comercial, informe de un contador ni funciones ya implementadas. No se cambia código, esquema, permisos reales ni infraestructura.

## 1. Base comercial propuesta para la entrega

| Tema | Definición para preparar el desarrollo | Origen |
| --- | --- | --- |
| Producto | Solo Mercado Pago Checkout Pro / Preferences y Split 1:1; OAuth por arrendador | Decisión expresa del usuario, respaldada por MP-1/MP-2 |
| Cobro | Después de aceptación mutua de la misma versión contractual; arriendo y garantía en enlaces separados | Decisión expresa del usuario |
| Comisión espaciGo | 3% + IVA de la comisión; la asume el arrendador, al igual que la tarifa Mercado Pago | Decisión expresa del usuario |
| Base | Precio final del concepto arriendo después de descuentos pactados, incluyendo los impuestos del arriendo que correspondan; garantía y firma excluidas | Definición comercial propuesta por encargo; mostrarla antes de aceptar |
| Anticipos/mensualidades | Comisión única sobre anticipo anual; mensualidades iniciadas por el cliente mediante enlaces | Decisión expresa del usuario |
| Garantía larga | Custodia/restitución a cargo del arrendador según contrato; espaciGo registra seguimiento/reclamos, no ejecuta ni financia la liquidación entre partes | Decisión expresa del usuario |
| Cancelación voluntaria corta | Propuesta única inicial: al menos 48 h, devolución 100%; de 24 h inclusive a menos de 48 h, 50%; menos de 24 h o inasistencia, sin devolución comercial automática | Política propuesta por encargo, no obligación impuesta por Mercado Pago |
| Incumplimiento/emergencia de uso | Aplicar lo previsto en sección 5 de la especificación: devolución del servicio no prestado/no utilizable o reprogramación aceptada, sin imponer cupón | Recomendación contractual; respetar derechos aplicables |
| Comisión ante reembolso | Revertir comisión neta e IVA proporcionalmente al arriendo efectivamente reembolsado; tarifas del procesador que no se reviertan, a cargo del arrendador | Política propuesta por encargo; conciliar reparto real |
| Cambios de tasa | Política de comisión congelada por versión contractual aceptada; cambios para renovación/nuevo acuerdo, sin recalcular historia | Definición operativa propuesta |
| Firmas | Posible cobro futuro SimpleFirmas a ambas partes, fuera de entrega; sin importe ni mecanismo ahora | Decisión expresa del usuario |

La política corta se incorpora a la versión contractual y al resumen previo a aceptación/pago. No aplica automáticamente a contratos largos ni sustituye las condiciones de garantía. No puede modificar un contrato ya aceptado de forma retroactiva.

SERNAC distingue el retracto de la política comercial y exige informar las exclusiones de servicios de forma previa y destacada [CON-1]. **Recomendación para simplificar la entrega: no incorporar una exclusión general de retracto ni un texto «sin devoluciones bajo ninguna circunstancia».** Cuando proceda un derecho legal, prevalece sobre los porcentajes comerciales. La calificación de cada tipo de arriendo y de relación de consumo se valida antes de publicar términos definitivos; el sistema conserva el motivo que distingue cancelación, incumplimiento, emergencia, retracto y garantía.

No se requiere un motor de múltiples políticas en esta entrega. La propuesta es una política corta inicial versionada y las condiciones particulares del contrato largo/garantía; el modelo no debe asumir que la política de 48/24 h es universal.

## 2. Permisos financieros concretos para el diseño

### 2.1 Principios y correspondencia con el proyecto

El ADR de administración global y su matriz dejan `finance.read/operate` como propuestas y prohíben que el rol global sea un bypass. Este documento **define un catálogo futuro**, sin afirmar que ya exista ni conceder permisos a administradores actuales. No crea roles de soporte ni de contador. Las capacidades finales deben mapearse a servicios/contratos existentes en el corte de implementación.

- Denegar por defecto y autorizar cada recurso/operación en backend, revalidando cuenta, titularidad, capacidad y estado financiero al ejecutar.
- Ser arrendador concede operaciones propias previstas por contrato; no acceso financiero a otros arrendadores. Tener `administrador` o `administrador_global` no concede por sí solo las capacidades nuevas.
- Las credenciales OAuth solo las utiliza el servicio del proveedor; ningún permiso de lectura financiera permite ver tokens ni actuar como otro usuario.
- Separar capacidad y alcance: el actor autorizado para un contrato/espacio no puede cambiar un identificador para acceder a otro. El servidor determina titular, vendedor, montos y referencias.

### 2.2 Catálogo propuesto y asignación

| Capacidad / acción | Quién y para qué | Límites |
| --- | --- | --- |
| Consultar/pagar obligaciones propias | Arrendatario del contrato | Solo su historial/importes y conceptos; garantía separada; sin desglose privado completo del arrendador ni edición de deuda. |
| Consultar cobros propios | Arrendador titular o representante autorizado del contrato | Espacios y contratos propios, bruto/comisiones/tarifa/neto; no exponer secretos ni operar ingresos de otro titular. |
| Solicitar devolución / presentar o responder reclamo propio | Participantes del contrato | Solicitar no equivale a aprobar ni ejecutar devolución; el arrendador puede aportar daños y el arrendatario controvertirlos conforme al contrato. |
| Autorizar devolución sobre un cobro propio | Arrendador receptor, cuando proceda según contrato | Importe no superior al saldo reembolsable, sin duplicidad; no puede aprobar unilateralmente un descuento de garantía disputado como si hubiese acuerdo. |
| `finance.read` | Actor de espaciGo al que se asigne expresamente consulta financiera | Lectura consolidada y detalle necesario, sin ejecución ni facultad para modificar pagos. El acceso personal detallado se registra y debe ser necesario para la gestión. |
| `finance.export` | Actor expresamente autorizado de espaciGo | Exportación acotada por fechas/finalidad/columnas; no se hereda por poder ver un KPI o ser administrador. Copias para contabilidad mediante este alcance, sin dar al contador un acceso global implícito. |
| `finance.refunds.approve` | Actor financiero designado de espaciGo | Validar operación documentada cuando haya autorización del receptor, política contractual aceptada aplicable o resolución competente; no arbitrar libremente contratos ni disponer de fondos ajenos sin fundamento. |
| `finance.refunds.execute` | Servicio de pagos, tras autorización válida; actor designado solo por comando de dominio auditado | Ejecutar en el proveedor, con idempotencia y límite de saldo; no alterar destinatario ni usarlo como transferencia libre. Tener capacidad de aprobación no concede ejecución automáticamente. |
| `finance.reconcile` | Servicio de conciliación y actor de espaciGo expresamente autorizado para revisión | Consultar/conciliar hechos externos y registrar diferencias; no inventar pago aprobado ni cambiar importes originales para cuadrar el dashboard. |
| `finance.policies.manage` | Responsable de producto/finanzas designado expresamente | Crear futuras versiones de política, sin modificar contratos aceptados ni cobros previos; nueva aceptación cuando corresponda. |
| `audit.read` / `audit.export` con alcance financiero | Actor al que se asigne ese alcance | Trazabilidad minimizada; no concede aprobación/ejecución de reembolsos. Mantener las fronteras de auditoría existentes. |

No asignar este catálogo a una cuenta real en la tarea documental. La lista queda determinada; falta vincular las capacidades a la identidad designada por el propietario cuando se autorice la implementación/operación. El modelo no exige que existan nuevos empleados ni que un contador acceda directamente a la aplicación.

### 2.3 Controles de ejecución

- Un reembolso requiere motivo, referencia al contrato/política/autorización, concepto y monto, saldo remoto validado y estado apto. Conservar actor solicitante, autorización y ejecución como hechos separados, aunque intervenga un único servicio técnico.
- Propuesta: exigir reautenticación reciente de hasta 5 minutos para una autorización manual sensible, cambio de cuenta receptora o cambio de política; la sesión normal no basta por sí sola. No se configura nada ahora.
- Evitar que un actor resuelva unilateralmente una disputa propia aprovechando permisos globales. Si no hay un responsable independiente habilitado, conservar el desacuerdo y canal contractual/competente; no fabricar una aprobación.
- Bloquear nueva ejecución si ya hay devolución/contracargo incompatible o resultado incierto. Consultar antes de repetir; no aprobar por silencio de la contraparte.
- No existe acción ordinaria para editar/borrar ledger, marcar pago remoto como recibido sin comprobación, cambiar receptor de una devolución ni transferir saldo entre usuarios.
- Revocar una capacidad debe impedir su uso en la siguiente operación. La interfaz refleja permisos confirmados por servidor; no concede autoridad por mostrar un botón.
- Una rectificación financiera genera ajuste trazable y evidencia externa/tributaria cuando corresponda; no es edición libre de registros históricos.

## 3. Mercado Pago: qué se verificó y qué depende de cada cuenta

### 3.1 Información pública confirmada

La documentación oficial consultada con país MLC soporta Checkout Pro y OAuth por vendedor en Split 1:1, publica requisitos de identificación del vendedor y contempla reparto proporcional de reembolsos con dependencia del saldo [MP-1, MP-2]. No es evidencia de que la aplicación concreta de espaciGo o todos sus arrendadores estén ya habilitados.

La página comercial chilena consultada publica referencias estándar de Checkout: **3,19% + IVA al instante** y **2,89% + IVA en 10 días**, junto con ofertas para nuevos clientes [MP-3]. Son referencias de esa página al 28-10-2026; **no se adoptan como tarifa contratada para Split, ni se presupone elegibilidad promocional**. No se accedió al panel de ninguna cuenta, se creó aplicación ni se solicitaron credenciales.

Definición operativa: no codificar una tarifa universal de Mercado Pago en el cálculo de comisión de espaciGo. La disponibilidad y tarifa del procesador se conservan por cuenta/operación cuando se conozcan, diferenciando estimación y real. No prometer que «al instante» significa depósito bancario inmediato o disponibilidad garantizada para toda cuenta.

Mercado Pago documenta un reporte de ventas Split con bruto, comisiones y dinero recibido, utilizable para conciliación mediante API [MP-4]. Definición: el dashboard utiliza el historial local confirmado y se reconcilia con los datos/reportes del proveedor; no depende de descargar manualmente un archivo cada vez que se abre. Sin tarifa confirmada, el neto sigue pendiente/estimado.

### 3.2 Ficha obligatoria de habilitación real, todavía sin completar

| Evidencia por aplicación/cuenta | Criterio de aceptación antes de fondos reales |
| --- | --- |
| País MLC, moneda CLP, producto y modelo | Checkout Pro / Preferences y Split 1:1 habilitados para la aplicación; no inferirlo de una alianza prevista. |
| Identidad de beneficiario / elegibilidad | Cuenta vendedora y representación correctas, identificación requerida por el proveedor y validación tributaria aplicable. OAuth vincula al beneficiario contractual. |
| Tarifas y disponibilidad efectivas | Captura/referencia fechada de las condiciones de la cuenta o confirmación del proveedor; impuestos, plazos, eventuales promociones y restricciones. Nunca incluir secretos en esa evidencia. |
| Medios y límites | Comprobar medios disponibles, invitado si se ofrece, límites CLP y validez de cobro de garantía con comisión cero; no asumir opciones de otro país o producto. |
| Reembolsos | Verificar flujo Preferences, autorización, reparto, saldo insuficiente y ventana disponible; las condiciones públicas no garantizan reembolso ilimitado de contratos largos. |
| Tarifas al devolver y contracargos | Confirmar cargos que revierten/no revierten y reparto efectivo. Hasta conocerlo, no prometer tarifa devuelta: registrar solo hechos conciliados. |
| Datos de conciliación | Confirmar acceso a pagos/tarifas y, cuando se use, reporte Split para esa aplicación; trazabilidad consistente con el historial local. |

Esta ficha está **pendiente de evidencia de cuenta/ensayo**, no de elegir otra vez la arquitectura. Se puede preparar el diseño a partir de las reglas definidas; no declarar validación comercial concluida. No se programa una integración ni se mueve dinero por esta investigación.

## 4. Circuito tributario: responsables y evidencia pendiente

Se mantiene la separación ya definida: arriendo documentado por el arrendador según su situación; comisión de intermediación documentada por espaciGo al arrendador; garantía restituible con respaldo separado; tarifa del procesador documentada por su emisor. No tratar todo el volumen procesado como venta de espaciGo ni presentar IVA como ingreso neto.

Para la entrega inicial, la emisión podrá realizarse en el sistema autorizado que corresponda y registrar sus referencias. No desarrollar ahora emisión DTE automática ni una API SII. La ausencia de automatización no dispensa de emitir los documentos en la oportunidad que corresponda.

| Validación del responsable tributario antes de operación real | Definición documental / evidencia necesaria |
| --- | --- |
| Identidad y actividad de espaciGo | Razón social/RUT y giro aplicables; modelo de intermediación y base comercial 3% sobre total final de arriendo, con IVA sobre la comisión. No inventar datos de empresa. |
| Documento de comisión | Confirmar factura/boleta electrónica afecta según operación/receptor, emisor autorizado, oportunidad de emisión, modelo de boleta/voucher y tratamiento del neto/IVA/reembolsos. |
| Arriendo por categoría y titular | Confirmar si corresponde IVA, exención/no afectación y documento del arrendador. Que un inmueble esté en la plataforma no fija su tratamiento. |
| Garantía y deducciones | Validar respaldo del depósito y tratamiento cuando se aplica a daños u otra obligación contractual; no convertir todo descuento de garantía en ingreso de espaciGo. |
| Ajustes y conciliación | Cada emisor corrige sus propios documentos cuando corresponde; enlazar notas de crédito y evitar doble contabilización con voucher. El reembolso técnico no anula el DTE automáticamente. |
| Formalización y reportes de plataforma | Confirmar aplicabilidad de acreditación de inicio de actividades, excepciones e informes como DJ 1966; conservar la evidencia exigible conforme a la situación, sin presumir que Mercado Pago cumple todas las obligaciones de espaciGo. |
| Retención fiscal y cierres | Ratificar el calendario por documento/ejercicio y cualquier extensión por revisión, efectos tributarios pendientes o procedimiento. No autorizar una purga universal por el mero transcurso de seis años. |

Estado: **criterios definidos y matriz preparada, sin revisión individual de contador realizada**. La evidencia puede incorporarse posteriormente al expediente privado de habilitación; no publicar datos tributarios personales ni documentos completos innecesarios en Git. No se ha contactado a terceros ni se ha emitido documento tributario por esta tarea.

## 5. Conservación: matriz productiva propuesta por clase

### 5.1 Base y límites

El SII indica **seis años para conservar DTE** [TAX-1]. El Código Tributario exige conservar libros y respaldos mientras esté pendiente su revisión; el SII describe situaciones con necesidad de conservar antecedentes más allá de seis años [TAX-2, TAX-3]. La Ley 21.719 incorpora proporcionalidad y excepciones a supresión por obligaciones/contrato o defensa de reclamaciones; su entrada en vigencia es el **01-12-2026**. Al 28-10-2026 se utiliza como criterio de diseño futuro, sin atribuirle vigencia anticipada [DATA-1, DATA-2].

Los plazos operativos siguientes son una **política propuesta de producto**, no todos plazos legales. No modifican `privacidad_local_v1`, el purgador sintético ni la retención local ratificada. Cada clase tendrá finalidad, fecha ancla, vencimiento, referencia de regla y suspensión motivada de purga cuando corresponda.

| Clase | Plazo/ancla propuestos | Destino al vencer y fundamento |
| --- | --- | --- |
| DTE de espaciGo, notas de crédito y respaldo fiscal propio indispensable | Como mínimo seis años conforme al régimen aplicable. Propuesta de calendario operativo: seis años calendario completos siguientes al año de emisión, sin purgar mientras exista obligación/revisión que exija más | Archivo restringido y posterior eliminación segura cuando se confirme elegibilidad. SII fija seis años; el calendario y extensión se ratifican con contabilidad. No implica custodiar por defecto todos los DTE ajenos. |
| Hechos financieros mínimos: pago confirmado, comisión/IVA, tarifa, liquidación, devolución y conciliación | Durante obligaciones abiertas y, como política propuesta, seis años calendario completos siguientes al año del movimiento para respaldo financiero; extender solo los antecedentes necesarios cuando exista causa documentada | Conservar IDs, importes, fechas, emisor/receptor mínimo y vínculos necesarios. Al vencer, eliminar/desvincular datos identificables según evaluación y conservar solo estadísticas efectivamente anónimas si se justifican. |
| Contrato aceptado y condiciones que sustentan el pago/garantía | Mientras existan obligaciones activas y cinco años desde el cierre final de obligaciones; si una parte es respaldo fiscal imprescindible, respetar el plazo de esa clase para el extracto/documento necesario | Los cinco años son propuesta de producto para trazabilidad/defensa, no declaración de prescripción universal. Minimizar copias y datos accesorios; una reclamación pendiente suspende la purga del expediente pertinente. |
| Resultado y evidencia pertinente de reclamo/garantía | Mientras esté abierto y cinco años desde su cierre final, salvo vínculo fiscal o suspensión motivada de purga | Conservar solo evidencia necesaria para el caso y acceso restringido. No retener indiscriminadamente fotos completas, conversaciones o datos de salud. Una solicitud de emergencia no exige un diagnóstico. |
| Auditoría financiera que respalda autorización de un movimiento | Mismo plazo del hecho financiero mínimo al que da soporte | Subconjunto necesario para probar actor/motivo/operación; no extender seis años a todos los logs. Auditoría administrativa genérica conserva su regla existente de cinco años desde evento. |
| Intento fallido/cancelado sin pago ni disputa | 90 días desde su cierre terminal | Eliminar datos del intento no necesarios; conservar solo claves mínimas si justificadamente se necesitan para evitar duplicados o sostener una incidencia. No borrar un pago real por confundirlo con el intento. |
| Payload técnico minimizado de webhook / archivo bruto de conciliación | 90 días desde procesamiento y conciliación terminal | Extraer antes el hecho durable y clave de deduplicación necesaria; eliminar copia técnica. No almacenar secretos ni payload libre por defecto. Incidencia abierta conserva únicamente evidencia pertinente con motivo. |
| Registro técnico de error sin finalidad de auditoría financiera | 30 días desde evento | Purga automática; sin tarjeta, tokens, documentos o texto privado. Un error no debe copiar el expediente financiero entero. |
| Descarga/exportación temporal preparada por la plataforma | Enlace/archivo temporal hasta 24 horas desde generación | Retirar copia temporal y revocar acceso; la fuente conserva su plazo. Registrar la exportación minimizada conforme a auditoría; la copia ya descargada por un tercero autorizado tiene su propia responsabilidad. |
| Copia de seguridad rotativa de la futura solución productiva | Ventana propuesta de 35 días desde creación | Rotación sin tratar backups como archivo fiscal permanente. Mantener el archivo fiscal por su propia clase; reejecutar supresión/restricciones al restaurar antes de servir datos. No cambiar backups ni recursos actuales por este documento. |

Ejemplo del calendario propuesto de seis años completos: un documento/movimiento de octubre de 2026 se conserva, al menos según esa política, hasta el 31-12-2032 y solo se evalúa purga desde el 01-01-2033, sin obligaciones abiertas ni suspensión de purga. Esto es un calendario de producto conservador para revisar, no una instrucción de que cualquier libro, contrato o dato pueda destruirse en esa fecha.

### 5.2 Suspensión de purga, derechos y cierre de cuenta

- Suspender solo la purga de registros necesarios ante garantía/reembolso pendiente, disputa, fiscalización, procedimiento o deber vigente. Registrar causa, ámbito, responsable y próxima revisión; proponer revisión cada 90 días y al cerrar el asunto. Consultas, reintentos y exportaciones no reinician plazos.
- Separar **despublicar o cerrar comercialmente** de **tramitar una solicitud de derechos sobre datos**. Una garantía pendiente no autoriza rechazar todas las solicitudes de privacidad ni conservar todo el perfil para siempre.
- Ante baja/derechos, conservar lo imprescindible por la obligación o causa aplicable, restringir su uso y minimizar el resto según la política vigente. No utilizar archivo fiscal o contractual para marketing. No afirmar anonimización si un ID o vínculo todavía permite identificar a una persona.
- La liquidación contractual de garantías largas sigue a cargo de las partes; el archivo y sus bloqueos son seguimiento, no custodia ni ejecución financiera de espaciGo.
- Los responsables del servicio no disponen de un botón para saltarse una suspensión fundada ni para borrar manualmente la evidencia financiera. La purga requiere criterio de elegibilidad, ejecución auditable y control sobre copias/derivados. No implementar el purgador por este documento.

## 6. Qué queda definido y qué requiere evidencia externa

Quedan preparados para revisión de diseño: política comercial inicial propuesta, fronteras de pagos/garantías, matriz de autorización, catálogo de capacidades, reglas de ejecución, responsables tributarios y matriz productiva de conservación. El historial y dashboard deben aplicar estas definiciones sin borrar hechos ni dar acceso por rol genérico.

No se consideran decisiones de producto abiertas la elección de Checkout Pro, quién asume costos, separación de garantía, pago tras contrato o implementación de firma: ya se fijaron. Tampoco se exige una alianza para empezar a diseñar.

Siguen pendientes **evidencias de habilitación**, no datos que pueda inventar el asistente:

1. Condiciones efectivas y disponibilidad de la aplicación/cuentas Mercado Pago, medios/límites y resultado de ensayos futuros.
2. Identidad concreta a la que el propietario asignará capacidades financieras, cuando autorice esa asignación. No concederlas ahora.
3. Revisión del responsable tributario sobre la entidad/actividad real, emisión, reportes y calendario fiscal, con evidencia privada y fecha.
4. Revisión del borrador comercial/retención antes de convertirlo en términos de usuarios o una política productiva aplicada. La publicación del documento no modifica contratos existentes ni ratifica por sí sola propuestas preparadas por el asistente.

La documentación no autoriza iniciar desarrollo, operar fondos reales ni afirmar cumplimiento certificado. No es necesario reabrir todas las decisiones en cada chat: las modificaciones posteriores deben indicar qué regla/versionan y su motivo.

## Fuentes oficiales y referencias del proyecto

Consultadas el 28-10-2026. Mercado Pago Developers se consultó también por su buscador oficial con país MLC; ejemplos/medios de otro país no se adoptan. Los precios públicos son una referencia fechada, no un contrato de espaciGo.

- [MP-1 — Integrar Split 1:1](https://www.mercadopago.com/developers/es/docs/split-payments/split-1-1/integration-configuration/integrate-marketplace).
- [MP-2 — Requisitos de Split 1:1](https://www.mercadopago.com/developers/es/docs/split-payments/split-1-1/prerequisites).
- [MP-3 — Mercado Pago Chile: Checkout y tarifas públicas](https://www.mercadopago.cl/herramientas-para-vender/check-out).
- [MP-4 — Reporte de ventas Split para conciliación](https://www.mercadopago.com/developers/es/docs/split-payments/split-1-1/additional-content/reports/sales-report/introduction).
- [CON-1 — SERNAC: derechos en viajes y deber de informar retracto](https://www.sernac.cl/portal/618/w3-propertyvalue-21019.html).
- [TAX-1 — SII: conservación de DTE](https://www.sii.cl/preguntas_frecuentes/factura_electronica/001_003_2356.htm).
- [TAX-2 — SII: Código Tributario, artículo 17](https://www.sii.cl/normativa_legislacion/dl830.pdf).
- [TAX-3 — SII: conservación y antecedentes necesarios más allá de seis años](https://www.sii.cl/pagina/jurisprudencia/adminis/2006/ventas/ja1118.htm).
- [DATA-1 — BCN: Ley 21.719](https://www.bcn.cl/leychile/navegar?idNorma=1209272).
- [DATA-2 — BCN: síntesis y vigencia de Ley 21.719](https://obtienearchivo.bcn.cl/obtienearchivo?id=repositorio/10221/37137/1/Informe_12_25_Ley_Datos_Personales_rev.pdf).
- [ADR de permisos de administración global](administracion_global/adr_permisos_compatibilidad.md).
- [Matriz de capacidades y límites existentes](administracion_global/matriz_capacidades_pruebas_cortes.md).
- [Privacidad sintética local: preservar sus reglas separadas](clasificacion_privacidad_local.md).
