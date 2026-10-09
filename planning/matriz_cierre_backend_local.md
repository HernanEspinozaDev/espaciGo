# Matriz inicial de cierre del backend local

Fecha de conciliación: 2026-10-09. Línea base posterior al merge #217 (`LOCAL-CONT-01` aceptado); se conserva la evidencia del corte en `evidence/local-cont-01-matrix.md`. LOCAL-OPS-01/#218 está en curso en PR actual. M06 LOCAL-BOOK-02 (#214) quedó aceptada en #217; véase `evidence/local-book-02-20261009.md`. Los padres generales permanecen abiertos. Fuente operativa: Issues y Project `EspaciGo — Desarrollo`, consultados con HernanMEC. Esta matriz forma parte del cierre LOCAL-1; las trazas por ID se actualizan dentro de entregas verticales.

## Lectura

- **Completo (slice):** criterios concretos de una subentrega aceptada; no implica que el Issue padre ni el módulo estén terminados.
- **Parcial:** existe un recorrido local útil, pero quedan criterios originales verificables.
- **Pendiente:** no hay implementación/evidencia suficiente para los criterios locales del Issue.
- **Diferido/bloqueado:** la parte depende de una decisión vigente o de un proveedor real; solo queda bloqueada esa operación.

Los estados GitHub y Projects son independientes. Se conservan Issues abiertas cuando su aceptación general sigue incompleta, aunque sus slices estén fusionados. Los snapshots Hermes/backlog históricos no prevalecen sobre los Issues actuales.

## Requisitos, Issues, código y evidencia

| Módulo y traza original | Estado real de Issues/Project | Código y evidencia aceptada reutilizable | Conciliación y brecha local restante | Dependencia que sí bloquea |
|---|---|---|---|---|
| **M01 Identidad** · #26–35; CU-01–06/50; RQF-213–218 | #26–30, #32–33 y #35 cerradas; #31 y #34 abiertas. #182 (LOCAL-AUTH-01), #180 (LOCAL-CORE-01) y #181 (LOCAL-PLAN-01) aceptadas/cerradas con #183. | `internal/identity/`; PR #124/#125/#183/#199; V22 y V27, pruebas PostgreSQL de historial/ventana/atomicidad/outbox/reapertura. Evidencia V22 `planning/evidence/local-auth-01-v22-20261007.md`; evidencia V27 `planning/evidence/local-priv-01a5-outbox-terminal-20261008.md`. | Preferencia independiente, historial de hash con ventana desde cambio, aviso durable de cambio y ciclo de fallo terminal/reapertura administrativa del outbox aceptados en su alcance local. V27: ocho intentos por ciclo, recuperación con auditoría, baja coordinada y retención de 30 días. **M01 general parcial:** #31/#34 y criterios originales restantes siguen abiertos; SMTP puede duplicar en caída post-envío/pre-ack. DB02-09 no se considera resuelto globalmente. | D-AUTH ratificada para local; correo externo y otros criterios M01 continúan pendientes. |
| **M02 Perfil/privacidad** · #36–43; CU-07–09 | #36 cerrada; #37–43 abiertas. #186/#192/#194/#196/#198 aceptadas; #202 cerrada y Hecho tras PR #203. #185/#40 permanecen abiertas. | `internal/privacy/`, `internal/m02local/`; PR #187/#190/#191/#193/#195/#197/#199/#203. V30 aplicada incrementalmente; evidencia `planning/evidence/local-m02-photo-payout-20261008.md`. | **LOCAL-M02-01 aceptado:** foto PNG sintética privada y cuenta de cobro fake con gate KYC, exportación ZIP, sustitución/revocación y eliminación. Verificación posterior al merge limitada a integración real HTTP/ZIP; los padres y criterios generales siguen abiertos, sin afirmación de anonimización o supresión integral. | Proveedor real, RUT, archivos reales y criterios generales M02 siguen fuera de este slice. |
| **M03 Verificación** · #44–51; CU-10–14; #142 | #44, #46, #47, #51 cerradas; #45/#48–50 abiertas; #142 conserva obligaciones productivas. #200 aceptada/cerrada tras PR #201. | `internal/verification/`, `internal/adapters/postgres/verification/`, almacenamiento privado sintético; PR #127/#143/#201. Evidencia `planning/evidence/local-kyc-01-20261008.md`. | LOCAL-KYC-01 aceptado: historial, subsanación tipada, reintento idempotente, elegibilidad persistente por tipo e integración con baja. Nuevas reservas validan KYC efectivo para ambas cuentas personales. #204 conecta ahora el gate en la subentrega de publicación local; documentos reales, RUT, proveedor y avisos durables de resultado siguen pendientes (#142 y padres). | #142 bloquea solo proveedor/datos reales; las transiciones M04 restantes continúan bajo #52–#61. |
| **M04 Oferta** · #52–61; CU-15–18 | #52–61 siguen abiertas; #204, #206, #208 y #210 cerradas/Hecho por sus cortes, sin cerrar padres. | `internal/spaces/`, V31/V32/V33, handlers JSON/OpenAPI, mock, tarifas versionadas y galería privada; PR #205/#207/#209/#211; evidencia `local-m04-gallery-20261008.md`. #200/#201 aporta elegibilidad KYC efectiva. | **Parcial:** publicación/ocultación owner-only con KYC, historial y conflicto de fixture; edición individual con snapshots; galería sintética privada con exportación y limpieza recuperable. #212 integra la publicación activa a descubrimiento como alcance M05 y no completa los criterios generales de oferta. Modalidades/políticas comerciales y criterios M04 restantes siguen pendientes. | #142 bloquea solo datos/proveedor productivos. La galería reutiliza #47/#143, #202/#203 y publicación #204/#205. Mantener los padres M04 abiertos; la integración de descubrimiento se traza en #212. |
| **M05 Descubrimiento** · #62–68; CU-19–21 | #62–68 siguen abiertas; LOCAL-M05-DISC-01 (#212) cerrada/Hecho como slice. | `internal/booking/`, handlers, OpenAPI y mock; PR #147/#152/#161/#163/#165/#167/#213. Evidencia `evidence/local-m05-published-catalog-20261009.md`. | **Parcial:** fixtures autorizados y publicaciones activas con KYC efectivo se listan/detallan; filtros, precio/geo/paginación y revalidación de cotización/reserva se conservan. Borradores/ocultos/no elegibles quedan privados. #212 solo acepta este corte, no cierra M05 ni certifica rendimiento general. | Dependencias implementables de publicación/KYC, tarifas, disponibilidad y filtros integradas; criterios comerciales generales y rendimiento cloud siguen fuera. |
| **M06 Reserva/pagos** · #69–79; CU-22–28/47/51 | #70–72/#74 cerradas; #69/#73/#75/#77 abiertas. #175/#177 slices aceptadas; #214 LOCAL-BOOK-02 en revisión. #76/#78 requieren proveedor; #79 conserva criterios generales. | `internal/booking/`, `internal/adapters/postgres/booking/`, `internal/adapters/fakebooking/`; PR #149/#152/#157/#159/#161/#167/#169/#170/#172/#174/#176/#178/#213; evidencia anterior y `evidence/local-book-02-20261009.md`. | **Parcial:** el nuevo slice une oferta activa no-fixture → API catálogo/detalle/horario/cotización/solicitud → pago fake → decisión del anfitrión, con historial por participante. Pruebas PostgreSQL añaden rechazo, vencimiento, cancelación/refund e invalidación ante ocultación, revocación KYC o cambio de tarifa; reserva/ocupación siguen atómicas e idempotentes. Esto no satisface todos los criterios originales ni completa #79. | Dependencias del slice (#212, AUTH/CORE, KYC, snapshots, ocupación, fake durable, `local_flexible_v1`) satisfechas. #76/#78 bloquean solo integración real; restantes requisitos de #73/#75/#77/#79 continúan abiertos. |
| **M07 Contratos/firma** · #80–87; CU-29–32 | #216 LOCAL-CONT-01 cerrada/Hecho por PR #217; #80–87 siguen abiertas por criterios generales. | `internal/contract/`, `internal/adapters/postgres/contracts/`, migración V34, rutas/OpenAPI y mock; evidencia `evidence/local-cont-01-matrix.md`. | **Parcial:** snapshot versionado, artefacto privado cifrado, firma/rechazo fake por participante, expiración transaccional y cancelación/refund local probados. Proveedor/callback real, avisos durables, exportación del artefacto, retención general y validez jurídica siguen pendientes. | Dependencias locales de operaciones están satisfechas; proveedor real y criterios legales no bloquean el corte sintético M08. |
| **M08 Operación** · #88–94; CU-33–34/48 | #218 LOCAL-OPS-01 en curso; padres abiertos. | `internal/operation/`, `internal/damageclaim/`, adaptadores PostgreSQL, V35, endpoints/OpenAPI y mock (PR actual). | **Parcial al integrar:** check-in/out y recepción con evidencia PNG privada y ubicación explícita; reclamo formal M10 separado de observación, con descargo básico. Notificación durable, fotos de descargo, resolución M10 y proveedor/cuestiones legales permanecen pendientes. | LOCAL-CONT-01 y LOCAL-BOOK-02 aceptadas; el actor/plazo de reclamo se ratificó. No requiere GCP ni M10 financiero. |
| **M09 Comunicación** · #95–102; CU-35–38/49 | Todas abiertas/Bloqueadas pese a slices de conversación/lectura aceptados. | `internal/conversation/`, `internal/adapters/postgres/conversation/`, handlers; PR #157/#159; evidencia `evidencia_m09_local_conversacion_20261006.md`, `evidencia_m09_lectura_local_20261006.md`. | Conversación segura/idempotente/paginada y cursor no leído independiente aceptados como slices. **Parcial:** #98 general, reseñas elegibles, reportes/moderación y notificación durable/reintentos siguen pendientes; Mailpit directo no demuestra outbox. | D-COMM bloquea política de reseña/moderación y retención; chat y avisos tipados locales pueden avanzar según contratos aprobados. |
| **M10 Disputas/cierre económico** · #103–110; CU-39–42 | Todas abiertas; #218 enlaza el ingreso sintético inicial del reclamo por daño. | `internal/damageclaim/`, V35, evidencia de #218; separado de `disputa_ensayo_local` (solo bloqueador privacidad). | **Parcial:** apertura por anfitrión dentro de 24h desde check-out, referencia a evidencia sintética y descargo de arrendatario. Fotos propias de descargo, avisos durables, revisión/resolución, garantía/ledger, fondos y liquidación siguen pendientes. | Política local de actor/plazo ratificada; D-DIS/ADMIN y decisiones contables bloquean solo adjudicación/efectos financieros. |
| **M11 Administración** · #111–118; CU-43–46/52 | Todas abiertas/Bloqueadas; existe revisión restringida M03. #180 (LOCAL-CORE-01) en curso como fundamento, enlazado a #112. | `internal/verification/` incluye acción administrativa acotada; esta entrega agrega tablas locales append-only/outbox y permisos mínimos para el uso de identidad. | **Pendiente en módulo general:** gobierno/moderación, consultas/reportes y mock; el evento de auditoría/outbox de credenciales cubre solo un consumidor local. | D-PRIV y D-DIS/ADMIN ratificadas para datos sintéticos locales; D-COMM sigue limitando nuevos tipos de notificación. RNF-017 productivo sigue pendiente. |

## Conciliación de las 43 entidades de ES2

La matriz de ownership autorizada contiene 43 entidades. La línea base previa a V30 tenía 34 declaraciones de tabla; V30 incorpora cuatro tablas específicas, para 38 tablas migradas en esta rama, con nombres/propósitos de ensayo y soporte; no es una correspondencia 1:1. **Parcial** indica tabla física local específica o capacidad equivalente, no materialización completa de la entidad global. La fuente detallada de owner, colaboración y alcance sigue siendo [`mapa_dominio_y_ownership.md`](mapa_dominio_y_ownership.md); la revisión de contrato es [`revision_diccionario_datos.md`](revision_diccionario_datos.md).

| # | Entidad / owner | Conciliación contra migraciones y código actuales |
|---:|---|---|
| 1 | `usuario` / M01 | Parcial: tabla y operaciones de cuenta autenticada; DB02-09/retención siguen pendientes. |
| 2 | `rol_usuario` / M01 | Parcial: conserva roles de autorización; la preferencia no excluyente RQF-213 se almacena separadamente tras V22 y no concede roles. |
| 3 | `perfil_usuario` / M02 | Parcial: perfil propio; faltan campos/acciones de alcance M02 pendientes. |
| 4 | `sesion` / M01 | Implementada para sesiones locales, idle/absoluto y revocación según evidencia M01. |
| 5 | `token_accion` / M01 | Implementada para acciones de identidad; el token no se expone en logs. |
| 6 | `version_terminos` / M01 | Implementada como catálogo versionado de aceptación local. |
| 7 | `aceptacion_terminos` / M01 | Implementada para aceptación asociada a cuenta. |
| 8 | `verificacion` / M03 | Parcial: caso/revisión local; faltan criterios generales de elegibilidad/retención. |
| 9 | `cuenta_cobro` / M02 | Parcial: V30 añade una referencia fake local con historial, sin vínculo bancario ni proveedor; el modelo general y pagos reales siguen ausentes. |
| 10 | `vinculo_proveedor_vendedor` / M06 | Sin modelo general; proveedor real/credenciales pendientes. |
| 11 | `categoria_espacio` / M04 | Implementada como catálogo de categorías; extensiones/versiones complementan perfiles. |
| 12 | `espacio` / M04 | Parcial: borradores y fixtures; no ciclo general de oferta/publicación. |
| 13 | `politica_cancelacion` / M04 | Parcial: `local_flexible_v1` snapshot del ensayo, no catálogo comercial general. |
| 14 | `tramo_cancelacion` / M04 | Sin modelo general de tramos/reglas de cancelación. |
| 15 | `regla_tarifa` / M04 | Parcial: `tarifa_espacio` versionada para prototipo; no contrato general completo. |
| 16 | `regla_comision` / M04 | Sin modelo; su dueño y consumidor M05/M06 están fijados en MAP-10. |
| 17 | `cotizacion` / M05 | Parcial: `cotizacion_reserva_ensayo`, ligada a fixtures y reserva local. |
| 18 | `reserva` / M06 | Parcial: `reserva_ensayo_local`; no incluye integración contractual/operación/disputa. |
| 19 | `ocupacion` / M06 | Implementada como calendario único con rango `[)`, exclusión y tipos manual/reserva; LOCAL-BOOK-01 añade evidencia directa de constraint/adyacencia. |
| 20 | `reserva_transicion` / M06 | Parcial: `reserva_ensayo_transicion` secuencial, append-only para flujo local. |
| 21 | `pago` / M06 | Parcial: intentos/operaciones fake de ensayo durables; no movimiento real. |
| 22 | `evento_proveedor` / M06 | Parcial: inbox de evento fake autenticado/deduplicado; no contrato ni callback de proveedor real. |
| 23 | `movimiento_financiero` / M10 | Sin modelo/ledger financiero. |
| 24 | `garantia` / M10 | Sin modelo general; reembolso fake no equivale a garantía. |
| 25 | `liquidacion` / M10 | Sin modelo de conciliación/liquidación. |
| 26 | `documento_tributario` / M10 | Sin modelo; emisión y reglas fiscales fuera del fake local. |
| 27 | `contrato` / M07 | Parcial: contrato snapshot versionado y artefacto privado sintético en `contrato_ensayo_local`; proveedor/retención general pendientes. |
| 28 | `firma_contrato` / M07 | Parcial: `contrato_ensayo_firma` y su historial conservan firma/rechazo fake; firma legal/proveedor pendientes. |
| 29 | `operacion_arriendo` / M08 | Parcial en PR #218: check-in/out/recepción sintéticos en `operacion_arriendo_ensayo_local`; criterios de aviso y operación general pendientes. |
| 30 | `disputa` / M10 | Parcial en PR #218: `reclamo_dano_ensayo_local`/descargo para ingreso local; `disputa_ensayo_local` sigue siendo únicamente bloqueador de privacidad, sin resolución financiera M10. |
| 31 | `documento` / M09 | Sin entidad genérica de metadata; archivos sintéticos M03 tienen tabla específica, almacenamiento privado no equivale a esta entidad. |
| 32 | `mensaje_reserva` / M09 | Parcial: `mensaje_reserva_ensayo`, autorizado por participantes, paginado/idempotente y con secuencia. |
| 33 | `resena` / M09 | Sin modelo. |
| 34 | `reporte_resena` / M09 | Sin modelo. |
| 35 | `notificacion` / M09 | Sin modelo general M09; existe un outbox acotado a avisos de credenciales con V22/V27, Mailpit no acredita entrega externa exactly-once. |
| 36 | `entrega_notificacion` / M09 | Sin modelo de entrega/reintento durable. |
| 37 | `evento_auditoria` / M11 | Sin modelo de auditoría operacional general. |
| 38 | `solicitud_titular` / M02 | Parcial: registro/estado inicial; falta ejecutar exportación/supresión con resolución y retención. |
| 39 | `outbox_evento` / M11 | Sin outbox general M11; el outbox local de credenciales y el inbox fake de pagos tienen ownership y alcance distintos. |
| 40 | `campana` / M04 | Diferida por MAP-07: finalidad/reglas/permisos/retención no aprobados para activar. |
| 41 | `orden_promocion` / M06 | Diferida con reglas comerciales de promoción; no hay cobro de promoción. |
| 42 | `derecho_reporte` / M04 | Diferida por MAP-07 hasta definir finalidad y autorización. |
| 43 | `respuesta_nps` / M09 | Diferida por MAP-07; no hay finalidad/plazo ratificados. |

Las tablas locales adicionales —perfiles versionados, simulación privada, fixtures, ubicación sintética, horario semanal, reembolsos, mensajes/lectura e inbox fake— son extensiones del prototipo y no aumentan las 43 entidades autorizadas ni las reemplazan. No se crea DDL global por conteo.

## Dependencias y decisiones para L0–L5

- La decisión local `local_flexible_v1` queda ratificada solo para fixtures/pagos fake; D-BOOK no bloquea su evidencia de persistencia.
- #69 continúa abierta/Bloqueada por su dependencia general `DISC-MOCK-01` y `CORE-ARCH-01`; los cortes M05 locales no completan la oferta general de #65–68.
- #70–72 se cerraron/Hecho con criterios propios satisfechos en PR #170. #74 está cerrada/Hecho solo para eventos/pagos fake durables por PR #172; #173/#175 se aceptaron como cortes locales de API/mock según Projects. #76/#78 siguen abiertas para proveedor real/sandbox; #79 conserva alcance general pendiente.
- #73/#75/#77 permanecen abiertas y no se marcan completas. LOCAL-BOOK-01 es su evidencia hija, no una mutación de sus bloqueos.
- D-AUTH conserva DB02-09; D-PRIV conserva retención/exportación/supresión; D-KYC/LIST separa elegibilidad local de proveedor/documentos reales; D-CONT/OPS, D-COMM y D-DIS/ADMIN se resolverán antes del criterio dependiente. La incertidumbre de un grupo no bloquea módulos independientes.
- GitHub no tiene todavía Issues operativas para LOCAL-PLAN-01/LOCAL-DEC-01 ni los paquetes L1–L5; #177 registra el primer corte autorizado. No se importan tarjetas del board Hermes.

## Reconciliación posterior a #199 — 2026-10-08

- **M01:** #183 fusionado integra V22; #198 fusionado como PR #199 integra V27 para fallo terminal, recuperación administrativa, auditoría y retención del outbox de credenciales. El paquete local de identidad de #182 está aceptado; #31/#34 y el ciclo general de M01 siguen abiertos. SMTP puede duplicar si se cae el Backend tras aceptar el correo y antes de persistir el acuse.
- **M02/exportación:** #187 fue una exportación limitada; #197 (Issue #196) amplió a ZIP con JSON versionado y archivos sintéticos propios según `planning/evidence/local-priv-01a4-export-20261008.md`. El alcance cubre los datos implementados, no entidades ausentes ni supresión integral. #185/#40 siguen abiertas.
- **Retención/restauración:** #195/Issue #194 aceptada con V26: purga sintética tras vencimiento y reaplicación desde registro externo al restaurar en destino aislado; no se restauró sobre el volumen activo. El registro externo de bajas permanece fuera del volumen; el volumen `espacigo_pgdata` se conserva. La ubicación de secretos permanece separada de copias de datos. Solo las rutas acotadas del inventario se comprobaron; ubicaciones externas/no revisadas siguen desconocidas.
- **Outbox V27:** #199 fusionado en `1beae923d1af4891e4c7bd1ce62b7952a260cdc3`. Aceptado #198 con fallo terminal tras ocho intentos por ciclo, ciclo administrativo idempotente, auditoría y retención de 30 días. La evidencia publicada usa PostgreSQL desechable y Mailpit; la comprobación de salud/admin posterior al merge está registrada sin repetir suites.
- **LOCAL-KYC-01 (#200):** historial, elegibilidad persistente y revocable por tipo; baja la desactiva; nuevas reservas requieren KYC para ambas cuentas personales. #204 conecta KYC efectivo al paso local de publicar espacio propio. #142 permanece separada para documentos/proveedor/identidad real. #45/#48–#50 siguen abiertos.

## Conciliación posterior a #205 — 2026-10-08

- **#202 / LOCAL-M02-01:** PR #203 fusionado (`ca64f11860dfd8c60e3998cc2754b3b4915ece26`). V30 aplicada incrementalmente, entorno saludable y recorrido API/ZIP con dos cuentas sintéticas confirmó foto generada/consulta/reemplazo/retirada, cuenta fake crear/consultar/cambiar/revocar, exportación de ambos recursos y rechazo 409 al crear cuenta fake sin KYC. #202 aceptada, cerrada y Hecho en Projects. Padres #37–#43 y #185/#40 permanecen abiertos.
- **LOCAL-M04-PUB-01 (#204):** subentrega bajo #55 para completar una brecha local habilitada tras #200. V31 añade los estados `borrador/activa/oculta` y el historial inmutable local. Endpoint exige rol `arrendador`, y al activar consulta elegibilidad KYC efectiva dentro de la transacción protegida por el mismo bloqueo de cuenta; KYB no sustituye KYC. El estado activo de este slice no publica automáticamente en catálogo general ni altera reservas. #52–#61 y padres permanecen abiertos; #142 no bloquea este alcance sintético.
- **Aceptación #204:** PR #205 integrado en `main` como `545a8a9c2b08a0f7183d6d816d43504c185a1912`; V31 aplicada incrementalmente a `espacigo_pgdata`. Salud HTTP confirmada. El recorrido API local de espacio sin fixture comprobó 409 sin KYC/no-mutación, publicación, consulta, ocultación y reactivación; el ZIP descargado y abierto contiene eventos propios ordenados. La prueba PostgreSQL desechable publicada comprueba el conflicto 409 del fixture habilitado y la continuidad del recorrido fixture de catálogo/cotización/reserva. #204 cerrada y Hecho en Projects por estos criterios únicamente. #52–#61 continúan abiertas y el catálogo general no se amplió.

## Actualización de LOCAL-BOOK-01

Issue hija #177 bajo #77, añadida al Project con HernanMEC. Mantiene abiertas las relaciones generales #77 → #75 → #73 → #65.

El commit `aa06d3759312623830d172d1eabd6c07feeb42b8` añade la integración seleccionada: dos reservas persistidas adyacentes, `23P01` directo con runtime y traducción a HTTP 409 sin filas parciales. Resultado y límites en [`evidence/local-book-01-20261007.md`](evidence/local-book-01-20261007.md). No se ejecutó prueba GCP. La integración usa únicamente una instancia PostgreSQL desechable; la base `espacigo_pgdata` y sus secretos no participan.
