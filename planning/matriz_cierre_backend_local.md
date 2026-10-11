# Matriz inicial de cierre del backend local

Fecha de conciliación: 2026-10-11. Línea base actual: PR #252 merge `6e96d8fac10ce52f3f61a9300a1a05b43f867fa6`; #247 y #248 están aceptadas/Hecho tras aplicar V46/V47 y reutilizar la evidencia publicada. LOCAL-QA-01/#238, LOCAL-CORE-02/#240, LOCAL-QA-02/#242 y LOCAL-QA-03/#244 siguen aceptadas por sus alcances; los padres generales permanecen abiertos. LOCAL-CLOSE-01/#246 sigue En curso; no se declara LOCAL-1 terminado. #249 es la brecha funcional activa; #250 sigue pendiente, con dependencias locales satisfechas.

## Lectura

- **Completo (slice):** criterios concretos de una subentrega aceptada; no implica que el Issue padre ni el módulo estén terminados.
- **Parcial:** existe un recorrido local útil, pero quedan criterios originales verificables.
- **Pendiente:** no hay implementación/evidencia suficiente para los criterios locales del Issue.
- **Diferido/bloqueado:** la parte depende de una decisión vigente o de un proveedor real; solo queda bloqueada esa operación.

Los estados GitHub y Projects son independientes. Se conservan Issues abiertas cuando su aceptación general sigue incompleta, aunque sus slices estén fusionados. Los snapshots Hermes/backlog históricos no prevalecen sobre los Issues actuales.

## Requisitos, Issues, código y evidencia

| Módulo y traza original | Estado real de Issues/Project | Código y evidencia aceptada reutilizable | Conciliación y brecha local restante | Dependencia que sí bloquea |
|---|---|---|---|---|
| **M01 Identidad** · #26–35; CU-01–06/50; RQF-213–218 | #26–30, #32–33 y #35 cerradas; #31 y #34 abiertas. #182, #180 y #181 aceptadas/cerradas con #183. | `internal/identity/`; PR #124/#125/#183/#199; V22/V27 y pruebas PostgreSQL de historial/ventana/outbox/reapertura. Evidencia `local-auth-01-v22-20261007.md`, `local-priv-01a5-outbox-terminal-20261008.md`, QA-03. | Preferencia, hash, recuperación/cambio y aviso durable de cambio aceptados localmente. **Brecha local:** RQF-016, alerta durable por bloqueo de cuenta. SMTP puede duplicar tras aceptación incierta. DB02-09 no se considera resuelto globalmente. | La alerta local usa Mailpit; correo productivo queda externo. |
| **M02 Perfil/privacidad** · #36–43; CU-07–09 | #36 cerrada; #37–43 abiertas. #186/#192/#194/#196/#198 aceptadas; #202 Hecho tras #203. #185/#40 abiertas. #247 y #248 aceptadas/Hecho; #249 en curso. | `internal/privacy/`, `internal/m02local/`; PR #187/#190/#191/#193/#195/#197/#199/#203/#251/#252 y trabajo actual. V30/V46/V47; evidencia `local-m02-photo-payout-20261008.md`, `local-priv-01a4-export-20261008.md`, `local-close-01-notices-20261011.md`, `local-close-01-retention-20261011.md`. | Perfil, foto sintética, cuenta de cobro fake, ZIP, solicitud/baja/replay y avisos durables de #247 aceptados para sus cortes. **Purgador V47 aceptado en #248:** aplica vencimientos independientes ratificados y limpieza durable archivo/referencia. #185/#40 continúan abiertas; no anonimización/supresión integral. | RUT/datos bancarios reales externos o excluidos; el purgador sintético no depende de ellos. |
| **M03 Verificación** · #44–51; CU-10–14; #142 | #44, #46, #47, #51 cerradas; #45/#48–50 abiertas; #142 conserva obligaciones productivas. #200 aceptada/cerrada tras PR #201. #247 y #248 aceptadas/Hecho; #249 en curso. | `internal/verification/`, storage sintético; PR #127/#143/#201/#251/#252 y trabajo actual; `local-kyc-01-20261008.md`, `local-close-01-notices-20261011.md`, `local-close-01-retention-20261011.md`, QA-03. | Historial, subsanación, revisión fake, elegibilidad, gates KYC y avisos durables aceptados por sus cortes. **Purgador V47 aceptado en #248:** retira archivos hasta 90 días, metadata/historial al vencimiento de 2 años. Documentos/RUT/validaciones reales y proveedor siguen fuera por #142. | El purgador sintético no depende de proveedor. #142 bloquea solo la porción real. |
| **M04 Oferta** · #52–61; CU-15–18 | #52–61 siguen abiertas; #204, #206, #208 y #210 cerradas/Hecho por sus cortes, sin cerrar padres. | `internal/spaces/`, V31/V32/V33, handlers JSON/OpenAPI, mock, tarifas versionadas y galería privada; PR #205/#207/#209/#211; evidencia `local-m04-gallery-20261008.md`. #200/#201 aporta elegibilidad KYC efectiva. | **Parcial:** publicación/ocultación owner-only con KYC, historial y conflicto de fixture; edición individual con snapshots; galería sintética privada con exportación y limpieza recuperable. #212 integra la publicación activa a descubrimiento como alcance M05 y no completa los criterios generales de oferta. Modalidades/políticas comerciales y criterios M04 restantes siguen pendientes. | #142 bloquea solo datos/proveedor productivos. La galería reutiliza #47/#143, #202/#203 y publicación #204/#205. Mantener los padres M04 abiertos; la integración de descubrimiento se traza en #212. |
| **M05 Descubrimiento** · #62–68; CU-19–21 | #62–68 siguen abiertas; LOCAL-M05-DISC-01 (#212) cerrada/Hecho como slice. | `internal/booking/`, handlers, OpenAPI y mock; PR #147/#152/#161/#163/#165/#167/#213. Evidencia `evidence/local-m05-published-catalog-20261009.md`. | **Parcial:** fixtures autorizados y publicaciones activas con KYC efectivo se listan/detallan; filtros, precio/geo/paginación y revalidación de cotización/reserva se conservan. Borradores/ocultos/no elegibles quedan privados. #212 solo acepta este corte, no cierra M05 ni certifica rendimiento general. | Dependencias implementables de publicación/KYC, tarifas, disponibilidad y filtros integradas; criterios comerciales generales y rendimiento cloud siguen fuera. |
| **M06 Reserva/pagos** · #69–79; CU-22–28/47/51 | #70–72/#74 cerradas; #69/#73/#75/#77 abiertas. #177/#214 y #224 aceptadas por alcance; #76/#78 proveedor real; #79 conserva criterios generales. | `internal/booking/`, PostgreSQL/fake adapters; PR #149…#225; `local-book-02`, `local-fin-01`, `local-fin-settlement`, QA-03. | Reserva, snapshots, ocupación, pagos/refunds fake, garantía/settlement, vencimiento y conciliación aceptados localmente. **Brecha local:** avisos durables RQF-121/122/201; el mail best-effort no es intención durable. | PSP real #76/#78 separado; no bloquea los avisos Mailpit locales. |
| **M07 Contratos/firma** · #80–87; CU-29–32 | #216 Hecho por #217 en su alcance; #80–87 siguen generales. | `internal/contract/`, PostgreSQL, OpenAPI/mock; `local-cont-01-matrix.md`, QA-03. | Firma/rechazo fake, artefacto privado y cancelación integrada aceptados. **Brecha local:** enlaces/invitaciones e intención durable de aviso de firma completa (RQF-134/135/139). Proveedor/callback y validez jurídica sí quedan externos/excluidos. | Mock puede entregar por Mailpit; no requiere firma comercial. |
| **M08 Operación** · #88–94; CU-33–34/48 | #218 Hecho solo por su slice; #88–94 siguen generales. #249 en revisión por corte local. | `internal/operation/`, `internal/damageclaim/`, V35/V48, rutas/OpenAPI/mock; evidencia `local-close-01c-defense-evidence-20261010.md`. | Check-in/out/recepción aceptados; #249 añade PNG generado por servidor a descargo persistido, con acceso participante/admin, ZIP solo del autor y cleanup por baja/retención. No implica fotos reales ni completa M08 general. | Storage y worker de limpieza sintética existente; sin proveedor externo. |
| **M09 Comunicación** · #95–102; CU-35–38/49 | #95–#102 abiertas; #156/#158/#220 Hecho por slices. | `internal/conversation/`, `internal/reputation/`, `internal/localnotice/`; V36/V37; `local-comm-01-20261009.md`, QA-03. | Chat/cursores, reseñas, reportes/moderación y eventos locales de check-in, reporte, reclamo abierto y cancelación aceptados. Sin realtime ni notificaciones por mensaje. El aviso de resolución de disputa RQF-210 pertenece a M10 y falta. | SMTP productivo externo; Mailpit entrega fake local. |
| **M10 Disputas/cierre económico** · #103–110; CU-39–42 | #103/#106–#110 abiertos; #222/#224/#230/#236 Hecho solo por slices; #249 en revisión para evidencia privada del descargo. | `damageclaim`, `operation`, V35/V38/V39/V40/V44/V45/V48; evidencia `local-close-01c-defense-evidence-20261010.md`. | Apertura, evidencia del reclamo, resolución y finanzas fake aceptadas; #249 cierra la brecha de evidencia del descargo para su corte con privacidad/retención. Efectos financieros generales, fiscal y transferencia real permanecen fuera; padres generales abiertos. | El corte es local y no depende de PSP/doc tributario externos. |
| **M11 Administración** · #111–118; CU-43–46/52 | #111–#118 abiertas por alcance general; #226/#228/#233/#235 y decisiones #232 aceptadas solo por slices. | `adminlocal`, auditoría en `booking`/owners, lectura/export auditada, bloqueo y dos reportes ratificados; `local-admin-01a/b/c/d`, QA-03. | **Brechas locales:** buscar cuenta por UUID y listar espacios publicados (RQF-212/236); falta el tercer reporte CU-45 ratificado: agregado de disputas por período/estado, minimizado. No implementar outbox universal; los productores finitos están en #247. | #110/#111 siguen grafo padre; no cerrar automáticamente. RNF-017 productivo y providers no bloquean las funciones locales. |

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
| 12 | `espacio` / M04 | Parcial: publicación/ocultamiento/reactivación owner-only y gate KYC local implementados (#204); #212 conecta ofertas activas al catálogo, pero criterios comerciales generales siguen abiertos. |
| 13 | `politica_cancelacion` / M04 | Parcial: `local_flexible_v1` snapshot del ensayo, no catálogo comercial general. |
| 14 | `tramo_cancelacion` / M04 | Sin modelo general de tramos/reglas de cancelación. |
| 15 | `regla_tarifa` / M04 | Parcial: `tarifa_espacio` versionada para prototipo; no contrato general completo. |
| 16 | `regla_comision` / M04 | Sin modelo; su dueño y consumidor M05/M06 están fijados en MAP-10. |
| 17 | `cotizacion` / M05 | Parcial: `cotizacion_reserva_ensayo`, ligada a fixtures y reserva local. |
| 18 | `reserva` / M06 | Parcial: `reserva_ensayo_local` integra snapshots/estados, contrato sintético, operación y reclamo; garantía/deducción fake M10 se relacionan por tablas owner separadas. Pago real/liquidación siguen fuera. |
| 19 | `ocupacion` / M06 | Implementada como calendario único con rango `[)`, exclusión y tipos manual/reserva; LOCAL-BOOK-01 añade evidencia directa de constraint/adyacencia. |
| 20 | `reserva_transicion` / M06 | Parcial: `reserva_ensayo_transicion` secuencial, append-only para flujo local. |
| 21 | `pago` / M06 | Parcial: intentos/operaciones fake de ensayo durables; no movimiento real. |
| 22 | `evento_proveedor` / M06 | Parcial: inbox de evento fake autenticado/deduplicado; no contrato ni callback de proveedor real. |
| 23 | `movimiento_financiero` / M10 | Sin modelo/ledger financiero. |
| 24 | `garantia` / M10 | Parcial local: V40 aporta snapshot y operaciones fake de autorización/captura/liberación; no es custodia ni proveedor de garantía real. |
| 25 | `liquidacion` / M10 | Slice local aceptado en #236/PR #237: V44 `reserva_liquidacion_ensayo_local` conserva intención, política/monto snapshot, resultado fake, reconciliación y auditoría; no mueve dinero real ni completa #109/#110. |
| 26 | `documento_tributario` / M10 | Slice de ensayo aceptado en #236/PR #237: `reserva_comprobante_ensayo_local` produce JSON `comprobante_ensayo` con aviso no fiscal; no es documento tributario ni folio válido. |
| 27 | `contrato` / M07 | Parcial: contrato snapshot versionado y artefacto privado sintético en `contrato_ensayo_local`; proveedor/retención general pendientes. |
| 28 | `firma_contrato` / M07 | Parcial: `contrato_ensayo_firma` y su historial conservan firma/rechazo fake; firma legal/proveedor pendientes. |
| 29 | `operacion_arriendo` / M08 | Parcial en PR #218: check-in/out/recepción sintéticos en `operacion_arriendo_ensayo_local`; criterios de aviso y operación general pendientes. |
| 30 | `disputa` / M10 | Parcial: #218 inicia reclamo/descargo; LOCAL-DIS-01 (#222) añade adjudicación administrativa local e historial, sin resolución financiera. `disputa_ensayo_local` sigue siendo el bloqueador de privacidad M02 independiente. |
| 31 | `documento` / M09 | Sin entidad genérica de metadata; archivos sintéticos M03 tienen tabla específica, almacenamiento privado no equivale a esta entidad. |
| 32 | `mensaje_reserva` / M09 | Parcial: `mensaje_reserva_ensayo`, autorizado por participantes, paginado/idempotente y con secuencia. |
| 33 | `resena` / M09 | Parcial local: `resena_ensayo_local`, unicidad reserva/autor, reciprocidad, promedio y retención; no perfil público ni moderación general. |
| 34 | `reporte_resena` / M09 | Parcial local: `reporte_resena_ensayo_local` e historial estructurado con moderación admin; sin apelación ni flujo general. |
| 35 | `notificacion` / M09 | Parcial local: `aviso_local` para cuatro eventos de ensayo, deduplicado por evento/destinatario; V22/V27 de credenciales sigue separado. |
| 36 | `entrega_notificacion` / M09 | Parcial local: ciclos/intentos durables en `aviso_local_ciclo`, ocho por ciclo y fallo terminal; SMTP puede duplicar ante resultado incierto. |
| 37 | `evento_auditoria` / M11 | Parcial local: `evento_auditoria_local` append-only para runtime, retención cinco años, productores específicos y consulta/export #228; este PR agrega auditoría transaccional a decisión/operación/conciliación financiera admin. No es garantía de inmutabilidad productiva ni completa #111–#114. |
| 38 | `solicitud_titular` / M02 | Parcial: registro/estado inicial; falta ejecutar exportación/supresión con resolución y retención. |
| 39 | `outbox_evento` / M11 | Sin outbox general M11: existen outbox de credenciales, avisos M09 e inbox fake de pagos, con owner, lifecycle y retenciones separados. No se duplica como esquema universal sin un productor/consumidor justificado. |
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
- D-AUTH conserva DB02-09; D-PRIV conserva retención/exportación/supresión; D-KYC/LIST separa elegibilidad local de proveedor/documentos reales; D-CONT/OPS, D-DIS/ADMIN y D-COMM ratificadas para slices locales. Criterios generales siguen abiertos. La incertidumbre de un grupo no bloquea módulos independientes.
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


## LOCAL-FIN-SETTLEMENT-01 (#236) — slice fake local aceptado

PR #237 fusionado como `09f03f60061ebd3dacb679661ad0180e5901b054`; V44/V45 aplicadas incrementalmente al entorno local conservando `espacigo_pgdata`, secretos y datos. La comprobación posterior al merge confirmó DB/backend/mock/mailpit saludables, migraciones 44/45 y `scripts/dev-env.sh verify-http`. #236 aceptada, cerrada y Hecho en Project 1 por su alcance fake local. Evidencia funcional PostgreSQL previa reutilizada en [`local-fin-settlement-20261010.md`](evidence/local-fin-settlement-20261010.md). #109/#110 siguen abiertas; proveedor real/sandbox, comisión productiva y documento fiscal siguen fuera. MAP-07 difiere campañas/promociones/NPS; no se marcan satisfechos.

## LOCAL-QA-01 (#238) — aceptada

PR #239 merge `c6769e3c73643a6f555e50125f18efdae599285a`; Issue #238 cerrada y Hecho en Project 1. [`local-qa-01-20261010.md`](evidence/local-qa-01-20261010.md) registra el comando, suite completa, 76 integraciones obligatorias sin omisiones y cinco repeticiones estables del recorrido PostgreSQL de check-in. No se repitió esa corrida únicamente por el merge. LOCAL-QA-01 no equivale a LOCAL-QA-03 ni al cierre LOCAL-1.

## LOCAL-CORE-02 (#240) — continuidad de trabajo durable, aceptado localmente

PR #241 se fusionó como `f9200fc200b2b11d76f9e15266c6807ea07c970e`; #240 quedó aceptada/cerrada/Hecho únicamente por este slice. La evidencia de [`local-core-02-20261010.md`](evidence/local-core-02-20261010.md) y sus integraciones se reutiliza sin repetirlas por el merge. M11/#111–#118, #109/#110, #185/#40 y los demás padres conservan sus criterios incompletos y permanecen abiertos. LOCAL-QA-02/#242 quedó aceptada por PR #243.

## LOCAL-QA-02 (#242) — actualización y restauración aisladas, aceptada

Se reutilizan migraciones vacía/repetida y checksum del runner QA-01; upgrade representativo de verificación/reputación; y el restore aislado con reaplicación idempotente del registro externo de bajas. Este corte agrega restore de snapshots/relaciones de cotización, reserva y ocupación con permisos runtime, y restore coordinado de metadata/blob privado de galería con aislamiento por titular. Evidencia de implementación/pruebas enfocadas y aceptación: [`local-qa-02-20261010.md`](evidence/local-qa-02-20261010.md). PR #243 se fusionó en `65e5a08`; #242 quedó aceptada/Cerrada/Hecho únicamente por este alcance. Los ensayos no usaron `espacigo_pgdata` ni tocaron secretos. LOCAL-QA-03/#244 se aceptó por PR #245; esto no implica que los criterios generales de sus módulos estén completos ni que LOCAL-1 haya terminado.
