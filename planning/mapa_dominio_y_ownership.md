# Mapa global de dominio y ownership lógico

Estado: mapa de ownership y disposiciones MAP-01–MAP-10 ratificados por el usuario el 2026-10-01 y documentados en el PR #1 (aprobado y fusionado a `main`: https://github.com/HernanEspinozaDev/espaciGo/pull/1). Es línea base de planificación, no evidencia de implementación ni aprobación académica; no autoriza DDL fuera de las tarjetas y dependencias del backlog. Corte documental: ES1 snapshots congelados y ES2 Anexo B v2.0 (29-09-2026). Prevalecen ES1 A–E para requisitos y ES2 Anexo B para contrato lógico de datos; cambios requieren ticket y trazabilidad, no edición silenciosa de anexos.

## Decisión de arquitectura

Se mantiene el catálogo funcional ES1 M01–M11 sin crear ni renombrar módulos. La solución objetivo es un monolito modular Go (ES2 3.3): los módulos son dueños lógicos de sus escrituras y exponen operaciones explícitas; otros módulos consultan mediante contratos/interfaces, no SQL/escrituras arbitrarias a tablas ajenas. PostgreSQL es fuente de verdad operativa. El alcance es el producto completo, con implementación incremental según prioridad actual: DB → backend → API → pruebas → mock. Esta línea base fue ratificada por el usuario y el PR #1; su aprobación no demuestra implementación ni cumplimiento de RNF.

## Matriz de ownership de las 43 tablas ES2

“Dueño” significa módulo lógico autorizado a definir invariantes/escrituras. Un consumidor puede leer mediante contrato explícito, sin adquirir propiedad de la tabla.

| # | Tabla ES2 | Dueño lógico | Colaboración / límite |
|---:|---|---|---|
| 1 | `usuario` | M01 Identidad y cuenta | M02 consume identidad; cambios de cuenta coordinan sesión y solicitud de derechos. |
| 2 | `rol_usuario` | M01 Identidad y cuenta | Concesión de administrador auditada por M11; autorización por recurso es transversal. |
| 3 | `perfil_usuario` | M02 Perfil y privacidad | M01 referencia la cuenta; no duplicar perfil dentro de identidad. |
| 4 | `sesion` | M01 Identidad y cuenta | Revocación en baja/bloqueo coordinada con M02/M11. |
| 5 | `token_accion` | M01 Identidad y cuenta | Tokens de un solo uso; notificación desacoplada vía M09. |
| 6 | `version_terminos` | M01 Identidad y cuenta | M04/M07 consumen versión publicada según finalidad; documento binario gestionado por M09 (ver `documento`). |
| 7 | `aceptacion_terminos` | M01 Identidad y cuenta | Evidencia de aceptación; M02 aplica solicitudes de derechos sin borrado en cascada. |
| 8 | `verificacion` | M03 Verificación KYC/KYB | Revisión manual por administrador M11; archivos sensibles usan M09. |
| 9 | `cuenta_cobro` | M02 Perfil y privacidad | Solo referencias protegidas; M06/M10 consumen estado autorizado, nunca credenciales. |
| 10 | `vinculo_proveedor_vendedor` | M06 Reservas y pagos | Vínculo externo de recepción; M10 consume para conciliación/liquidación. Sin secreto en BD. |
| 11 | `categoria_espacio` | M04 Publicaciones y disponibilidad | Catálogo configurable; M05 lectura. |
| 12 | `espacio` | M04 Publicaciones y disponibilidad | M05 consulta filtrada; M06/M10 reciben referencias, sin escritura cruzada. |
| 13 | `politica_cancelacion` | M04 Publicaciones y disponibilidad | M06 aplica y congela versión aceptada en reserva. |
| 14 | `tramo_cancelacion` | M04 Publicaciones y disponibilidad | M06 calcula devolución contra snapshot, no política mutable. |
| 15 | `regla_tarifa` | M04 Publicaciones y disponibilidad | M05 cotiza y M06 congela referencia/snapshot; publicación/versionado pertenece a M04. |
| 16 | `regla_comision` | M04 Publicaciones y disponibilidad | M05 consulta la regla mediante interfaz de M04 para cotizar; M06 consume la cotización aceptada y guarda referencia/snapshot, sin ser dueño de la regla; M11 controla permisos administrativos y audita los cambios. Detalle y propagación RNF-020 en MAP-10. |
| 17 | `cotizacion` | M05 Búsqueda y cotización | M06 valida vigencia/uso único y la referencia al crear reserva. |
| 18 | `reserva` | M06 Reservas y pagos | Agregado transaccional principal; M07–M10 colaboran mediante comandos/eventos explícitos. |
| 19 | `ocupacion` | M06 Reservas y pagos | Único dueño del calendario transaccional, retenciones y reservas. M04 solicita/consulta bloqueos manuales por contrato; no mantiene calendario paralelo. |
| 20 | `reserva_transicion` | M06 Reservas y pagos | Historial append-only de máquina de estados; M07/M08/M10 solicitan transición autorizada, no escriben directamente. |
| 21 | `pago` | M06 Reservas y pagos | Intentos idempotentes; M10 consume hechos confirmados para conciliar. |
| 22 | `evento_proveedor` | M06 Reservas y pagos | Inbox autenticado/deduplicado; M10 procesa efectos financieros mediante contrato. |
| 23 | `movimiento_financiero` | M10 Disputas, liquidación y tributación | Hechos económicos inmutables; M06 entrega referencia de pago, no altera movimientos contabilizados. |
| 24 | `garantia` | M10 Disputas, liquidación y tributación | Estado observado de autorización/captura/liberación; M06 inicia operación mediante adaptador y comunica resultado. No equivale a escrow. |
| 25 | `liquidacion` | M10 Disputas, liquidación y tributación | Conciliación del resultado observado; M06 aporta operaciones externas correlacionadas. |
| 26 | `documento_tributario` | M10 Disputas, liquidación y tributación | Emisor/criterio fiscal pendientes de validación competente; archivo binario mediante M09. |
| 27 | `contrato` | M07 Contratos y firma | Consume snapshot autorizado de M06; M09 conserva objeto final. |
| 28 | `firma_contrato` | M07 Contratos y firma | Respuesta externa autenticada/correlacionada; cambio de estado de reserva solicitado a M06. |
| 29 | `operacion_arriendo` | M08 Operación del arriendo | Actos de check-in/out/recepción; evidencia binaria en M09; transición de reserva coordinada con M06. |
| 30 | `disputa` | M10 Disputas, liquidación y tributación | M08/M09 aportan referencias de evidencia/mensajes; decisión administrativa auditada por M11. |
| 31 | `documento` | M09 Comunicación y reputación (capacidad transversal de archivos) | Única propiedad/metadata de objeto para todos los dominios; el dominio consumidor autoriza dueño/propósito. Exactamente un FK dueño conforme Anexo B. No copiar archivos a tablas de módulo. |
| 32 | `mensaje_reserva` | M09 Comunicación y reputación | M06 valida participantes y estado de reserva mediante contrato; texto privado no sale íntegro a analítica. |
| 33 | `resena` | M09 Comunicación y reputación | M06 confirma elegibilidad de reserva; M11 modera por flujo explícito. |
| 34 | `reporte_resena` | M09 Comunicación y reputación | M11 decide/modera; decisión auditada. |
| 35 | `notificacion` | M09 Comunicación y reputación (servicio transversal) | Otros módulos emiten intención tipada; destinatario/recurso autorizados en origen. No asignar una copia/owner a M10. |
| 36 | `entrega_notificacion` | M09 Comunicación y reputación (servicio transversal) | Módulo de origen no controla proveedor/reintentos; entrega desacoplada e idempotente. |
| 37 | `evento_auditoria` | M11 Administración y auditoría | Escritor transversal por interfaz append-only; consulta restringida y con motivo. Separado de logs y outbox. |
| 38 | `solicitud_titular` | M02 Perfil y privacidad | M11 puede recibir asignación administrativa; M02 coordina ejecución en dominios/derivados y evidencia, sin cascada. |
| 39 | `outbox_evento` | M11 Administración y auditoría (plataforma/eventos) | Escrita por adaptador transaccional dentro del mismo commit del agregado dueño; worker publica. Consumidores deduplican; no es auditoría ni fuente de verdad financiera. |
| 40 | `campana` | M04 Publicaciones y disponibilidad | Publicación/alcance comercial; M06 procesa su orden de pago; M11 controla autorización/reportes según alcance. |
| 41 | `orden_promocion` | M06 Reservas y pagos | Orden/cobro idempotente; M04 consume estado para activar campaña. No mezclada con reserva ni comisión por arriendo. |
| 42 | `derecho_reporte` | M04 Publicaciones y disponibilidad | Entitlement sobre publicación; M11/M05 autorizan consulta agregada, BigQuery no es fuente operativa. |
| 43 | `respuesta_nps` | M09 Comunicación y reputación | Finalidad/seguimiento explícitos y minimización; analítica recibe agregado, no texto íntegro. |

## Fronteras y flujo de valor

Cuenta/roles (M01) → perfil/derechos (M02) y verificación (M03) → publicación/tarifas/política (M04) → búsqueda/cotización (M05) → reserva + ocupación atómica + pago (M06) → contrato/firma (M07) → check-in/uso (M08) → comunicación/reputación (M09) → disputa/garantía/liquidación/tributación (M10) → gobierno/auditoría (M11). M09 presta archivos y notificaciones como capacidades comunes mediante contratos, no como propiedad compartida de tablas. M11 consume lecturas acotadas y no es dueño de datos operacionales ajenos.

Las escrituras de reserva y ocupación, transición, y evento outbox requerido comparten una transacción local. No se llama a terceros dentro de la transacción; pagos/firma/webhooks son adaptadores con idempotencia, correlación, conciliación y compensaciones como hechos nuevos. Outbox se publica al menos una vez hacia Pub/Sub/BigQuery; el plano analítico no modifica estados OLTP. Auditoría de negocio es distinta de logs técnicos, notificaciones y eventos analíticos.

## Hallazgos, impacto y tratamiento

| ID | Hallazgo/decisión propuesta | Trazabilidad afectada | Tratamiento antes de DDL |
|---|---|---|---|
| MAP-01 | `ocupacion` aparece bajo M04 y M06 en la propuesta de visión. Un calendario duplicado permitiría sobreventa. Se fija un único owner M06; M04 conserva calendario administrativo de publicación solo vía contrato. | RQF-083–085, 108–112, 120; CU-17, CU-19, CU-22; HU05/HU20/HU21/HU23; RNF-001/002/006/009/011. | Adoptar frontera de ownership; probar exclusión/adyacencia/concurrencia en DDL (MD-02/PT-01/PT-02 según plan vigente). No ampliar HU24 recurrente: sin RQF/CU y explícitamente fuera de alcance ES1. |
| MAP-02 | `documento` y sus binarios se comparten entre perfil, KYC, espacios, contratos, reservas, operación, disputa, términos y DTE. El módulo de archivos M09 posee metadatos/objeto; el recurso de negocio conserva autorización/propósito. | RQF-074–078, 137, 145, 151, 161, 165, 211; RNF-013/014/018/029/042; CU-12/15/30/34/40; HU04/HU05/HU31/HU33/HU35. | Mantener una tabla; exigir exactamente un dueño, coherencia categoría-FK-rol, bucket privado y ciclo de vida. Detallar contrato de carga en fase CORE-ARCH-01; no elegir proveedor adicional aquí. |
| MAP-03 | `notificacion`/`entrega_notificacion` son capacidad común de M09, no propiedad repetida de M09 y M10. Módulos de negocio originan intención; M09 controla entrega/reintento. | RQF-047, 121–122, 139, 149, 164, 194, 201, 210, 218, 231; RNF-023/024; HU30; CU vinculados en ES1 E. | Aprobar contrato de evento tipado, datos minimizados, correlación y reintentos; sin PII/cuerpo en outbox/logs. |
| MAP-04 | `evento_auditoria` (M11) es registro operacional de decisiones críticas, distinto de outbox y logs. Una política de retención bloqueada no está lista para activación. | RQF-178–185, 212; RNF-017/043; CU-43–46/CU-52; HU26/HU32. | Auditar actor, recurso, acción, resultado, motivo/correlación minimizados; acordar plazo/alcance y validar privacidad antes de retención irreversible; prueba de inmutabilidad sigue pendiente (ES2 3.3 y Anexo B). |
| MAP-05 | `outbox_evento` es capacidad de plataforma M11 con escritura por adaptadores dentro de la transacción del dueño del agregado; no representa auditoría ni analítica de interacción de alto volumen. | RNF-012/024/028; RQF-116/139 como eventos relacionados, sin convertirlos en tabla genérica; ES2 3.3.7. | Contrato de esquema/versionado, payload mínimo, deduplicación/orden por agregado, leases/reintentos/retención en CORE-ARCH-01 antes de publicar. |
| MAP-06 | `garantia`/`liquidacion`/movimientos son resultado observado, no promesa de Escrow. ES1 A usa terminología de custodia/proveedor pero ES2 limita expresamente afirmaciones sin evidencia. | RQF-114–118, 159–177, 227–231; RNF-020/027/028; CU-22–28/CU-39–42/CU-51; HU17/HU19/HU28. | Mantener contrato neutral y estado por conciliar; sandbox/prueba de proveedor, contrato y revisión contable pendientes. No renombrar requisitos ES1 cerrados ni declarar capacidad disponible. |
| MAP-07 | Campañas, derecho de reporte y NPS están en el diccionario de producto completo, pero con reglas/finalidad/comercialización aún abiertas; promoción no es core transaccional inicial. | `campana`, `orden_promocion`, `derecho_reporte`, `respuesta_nps`; RQF-236 y RNF-018/026/029 cuando aplique; HU26/CU-52 y HU29/HU24 sin RF. | Diseñar ahora como contrato; diferir migración física/activación hasta validar finalidad, política comercial y permisos. HU24/HU29 permanecen excluidas; no inventar requisitos. |
| MAP-08 | Asignación RQF↔CU↔HU de ES1 prevalece sobre agrupación de HUs por épica. La visión declara correctamente HU24 sin RQF/CU y HU29 sin RQF/CU fuera del core. | ES1 A–E completos; matriz de visión M01–M11. | En backlog/implementación enlazar IDs primarios de anexos; no inferir trazabilidad de un nombre de módulo o de una HU transversal. |
| MAP-09 | Retención, base jurídica, plazos para identidad/documentos/finanzas/chat/analítica, y exigencias RNF de cinco años requieren separar finalidad y excepción. | RNF-018/026/029/042/043; PT-16; Anexo B.7. | No automatizar supresión ni bloqueo irreversible con plazo no aprobado; decisión de privacidad/contable pendiente según Anexo B. |
| MAP-10 | **Hallazgo y evidencia:** `planning/vision_y_modulos.md` § “Módulos funcionales y trazabilidad” asigna `regla_comision` a M04. ES2 Anexo B (`planning/referencias/ES2/B_diccionario_datos.md`, § `regla_comision`) especifica versionado, vigencia, motivo y aprobador; `cotizacion` referencia `comision_id` y el cálculo reproducible; `reserva` conserva la versión aceptada y los importes snapshot. ES1 RQF-105 exige calcular la comisión y RNF-020 exige reflejar cambios en máximo 60 s sin reiniciar el servidor. **Decisión ratificada por el usuario el 2026-10-01:** M04 es dueño de la regla; M05 la consulta mediante interfaz de M04 para cotizar; M06 consume la cotización aceptada y guarda referencia/snapshot, sin ser dueño ni recalcular la regla; M11 autoriza administrativamente y registra auditoría al cambiarla. Para RNF-020, propuesta operativa: invalidación/refresco de caché o lectura de versión vigente con TTL máximo de 60 s, sin redespliegue ni reinicio; verificar el SLA en aceptación/prueba. Esto no es evidencia de implementación. | RQF-105; RQF-223; RNF-020; CU-18/CU-21; HU16; ES2 Anexo B, `regla_comision`, `cotizacion` y `reserva`. | Ownership M04 y lectura/consumo/snapshot por M05/M06 ratificados; cotizaciones emitidas y reservas aceptadas conservan referencia/snapshot; cambios posteriores aplican por vigencia a nuevas cotizaciones, sin alterar snapshots existentes. Requiere autorización, actor/aprobador, motivo, auditoría M11 y verificar propagación ≤60 s. No modifica ES1/ES2 ni inicia DDL. Evidencia de ratificación: aprobación de MAP-01–MAP-10 en PR #1, fusionado a `main` el 2026-10-01; URL: https://github.com/HernanEspinozaDev/espaciGo/pull/1. |

## Verificación de cobertura documental

El inventario de la sección B.2–B.5 del Anexo B enumera 43 nombres de tabla; esta matriz presenta los 43, sin tablas físicas adicionales. Las 11 filas M01–M11 de la tabla de visión se conservan y cada entidad tiene un único owner lógico; las capacidades compartidas se expresan como colaboración. Cobertura funcional contrastada contra ES1 Anexo A y su resumen de módulos; el cruce exacto de IDs sigue en Anexos B/D/E de ES1. La lista de asignación de visión ya incluía las 43 tablas salvo `respuesta_nps`, incorporada aquí como M09; se reubicaron `ocupacion` (un owner), `documento` (servicio común), `notificacion` (servicio común) y `regla_comision`/finanzas con owner único.

Fuentes autorizadas revisadas: `referencias/ES1/A_actores_y_modulos.md`, `B_requerimientos_funcionales.md`, `C_requerimientos_no_funcionales.md`, `D_casos_de_uso.md`, `E_historias_de_usuario.md`; `referencias/ES2/B_diccionario_datos.md` v2.0 y `03_03_componentes.md`; `vision_y_modulos.md`, `base_de_datos.md`, `decisiones_y_hallazgos.md`. La matriz de trazabilidad ES2 indica que el diccionario es especificación de diseño, no evidencia de implementación o aprobación.

## Aprobación y pendientes

El usuario ratificó el mapa de ownership y las disposiciones MAP-01–MAP-10 el 2026-10-01; la aprobación quedó registrada en el PR #1, `APPROVED` y fusionado a `main`. La línea base permite diseñar contratos, pero no es evidencia de implementación ni autoriza DDL fuera de las tarjetas y dependencias del backlog. Pendientes externos (privacidad, contador, proveedores) siguen abiertos y pueden condicionar retención y flujos financieros/productivos.
