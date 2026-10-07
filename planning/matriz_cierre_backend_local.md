# Matriz inicial de cierre del backend local

Fecha de conciliación: 2026-10-07. Línea base de código actualizada: `main` `716a4a38512ddbd693d1541620786a75d20432ca` (PR #178). Fuente operativa: Issues #26–#120 y Project `EspaciGo — Desarrollo`, consultados con HernanMEC. LOCAL-BOOK-01 (#177) se aceptó en ese merge. Esta matriz abre el seguimiento LOCAL-1; las trazas por ID están en [`trazabilidad_requisitos_backend_local.md`](trazabilidad_requisitos_backend_local.md), y se actualizarán dentro de entregas verticales, no mediante PR documental aislado.

## Lectura

- **Completo (slice):** criterios concretos de una subentrega aceptada; no implica que el Issue padre ni el módulo estén terminados.
- **Parcial:** existe un recorrido local útil, pero quedan criterios originales verificables.
- **Pendiente:** no hay implementación/evidencia suficiente para los criterios locales del Issue.
- **Diferido/bloqueado:** la parte depende de una decisión vigente o de un proveedor real; solo queda bloqueada esa operación.

Los estados GitHub y Projects son independientes. Se conservan Issues abiertas cuando su aceptación general sigue incompleta, aunque sus slices estén fusionados. Los snapshots Hermes/backlog históricos no prevalecen sobre los Issues actuales.

## Requisitos, Issues, código y evidencia

| Módulo y traza original | Estado real de Issues/Project | Código y evidencia aceptada reutilizable | Conciliación y brecha local restante | Dependencia que sí bloquea |
|---|---|---|---|---|
| **M01 Identidad** · #26–35; CU-01–06/50; RQF-213–218 | #26–30, #32–33 y #35 cerradas; #31 y #34 abiertas. #182 (LOCAL-AUTH-01) bajo #31, en curso. | `internal/identity/`; PR #124/#125; `planning/evidence/auth-be-01-postgresql.md`, `auth-be-02-registration-session.md`, `m01-api-e2e-20261005.log`; esta rama añade V22 y pruebas de LOCAL-AUTH-01 / CORE. | Registro, verificación, sesión, recuperación/cambio tienen cortes aceptados. **Parcial:** la preferencia, historial de hashes/ventana, outbox del aviso y su auditoría están implementados en esta entrega, todavía En revisión; el resto de criterios de #31/#34 y entregas M01 siguen abiertos. DB02-09 solo se considera resuelto para la regla ratificada, no para cada criterio pendiente del registro histórico. | D-AUTH se ratificó el 2026-10-07 para local; correo externo y los criterios M01 restantes siguen fuera de esta aceptación. |
| **M02 Perfil/privacidad** · #36–43; CU-07–09 | #36 cerrada; #37–43 abiertas/En curso. | `internal/privacy/`, identidad y `internal/adapters/postgres/identity/`; PR #126 y decisiones `decisiones_m02_perfil_privacidad.md`. | Perfil propio y registro de solicitudes aceptados parcialmente. **Pendiente:** cuenta de cobro/foto donde aplique; exportación y resolución efectiva/motivada de acceso/supresión; minimización, retención y copias. #43 permanece abierta. | D-PRIV bloquea solo exportación, supresión/retención y datos sensibles que requieren finalidad/plazo. |
| **M03 Verificación** · #44–51; CU-10–14; #142 | #44, #46, #47, #51 cerradas; #45/#48–50 abiertas; #142 mantiene obligaciones productivas. | `internal/verification/`, `internal/adapters/postgres/verification/`, almacenamiento privado sintético; PR #127/#143; `planning/evidence/m03-evidencia-sintetica-local.md`. | Slice de PNG sintético privado y revisión básica aceptado. **Parcial:** esquema/retención de evidencia bajo #45 y recorrido general de subsanación/eligibilidad/endpoints/pruebas fake en #48–50. Documento real, proveedor y verificación productiva siguen excluidos en #142. | D-KYC/LIST bloquea el uso general de elegibilidad; documentos reales/proveedor continúan bloqueados separadamente por #142. |
| **M04 Oferta** · #52–61; CU-15–18 | #52–61 abiertas/Bloqueadas. | `internal/spaces/`, repositorio PostgreSQL; borradores, ocho categorías, perfiles versionados, tarifas, calendario privado y horarios por hora. PR #134/#141/#145/#169; evidencia `m04-drafts-20261005.md`, `m04-attributes-20261005.md`, `evidencia_m06_horario_semanal_local.md`. | Varios slices aceptados, **módulo parcial**: borradores y disponibilidad no equivalen a publicación. Faltan ciclo general de publicar/despublicar, gating de elegibilidad, galerías versionadas/privadas y criterios de tarifa/política general. | #43/#142 bloquean solo operaciones que exigen perfil/KYC; elegibilidad sintética aprobada puede resolverse localmente bajo decisión D-KYC/LIST. |
| **M05 Descubrimiento** · #62–68; CU-19–21 | #62–68 abiertas/Bloqueadas; slices #151/#160/#162/#164/#166 cerrados tras PRs aceptados. | `internal/booking/service.go`, `internal/adapters/postgres/booking/repository.go`, handlers HTTP; PR #147/#152/#161/#163/#165/#167. Evidencias `m05-local-catalog-20261006.md`, `evidencia_m05_filtros_locales_20261006.md`, `evidencia_m06_selector_intervalos_local_20261006.md`. | **Parcial:** fixtures con autorización explícita, filtros, precio, geo, paginación y selector existen. Falta búsqueda/detalle/cotización general sobre oferta publicada/elegible y límites/índices del contrato amplio; una allowlist de ensayo no reemplaza publicación. | Depende de LIST para oferta publicada; reglas de geografía/producto no bloquean filtros locales independientes. |
| **M06 Reserva/pagos** · #69–79; CU-22–28/47/51 | #70–72 y #74 cerradas; #69/#73/#75/#77 abiertas; #76/#78/#79 abiertas/Bloqueadas. #175 y #177 cerradas como slices locales. | `internal/booking/`, `internal/adapters/postgres/booking/`, `internal/adapters/fakebooking/`, handlers; PR #149/#152/#157/#159/#161/#167/#169/#170/#172/#174/#176/#178. Evidencia `matriz_consolidacion_m06_reservas.md`, `evidence/m06-booking-consolidation-20261006.md`, `evidence/m06-booking-payment-inbox-20261007.md`, `evidence/m06-local-payment-api-20261007.md`, `evidence/m06-local-mock-payment-20261007.md`, `evidence/local-book-01-20261007.md`. | **Parcial:** flujo fake de fixtures, transacciones, snapshots, estado/historial, ocupación, vencimientos, cancelación local/refund y recuperación fake; mock de pago cubierto. **LOCAL-BOOK-01 aceptado:** prueba PostgreSQL directa de `23P01`, dos reservas adyacentes persistidas y traducción HTTP 409 sin filas parciales. Esto satisface exclusivamente su subentrega; #73/#75/#77 conservan criterios generales pendientes. #76/#78 requieren integración real/sandbox; #79 mezcla mock general y proveedor. | La aceptación de LOCAL-BOOK-01 no elimina dependencias nativas de #77/#75/#73/#65. #76/#78 solo bloquean proveedor real. D-BOOK bloquea únicamente reglas aún no ratificadas. |
| **M07 Contratos/firma** · #80–87; CU-29–32 | Todas abiertas/Bloqueadas. | No hay paquete de dominio/persistencia/handler específico identificado; capacidades privadas de M03/M09 no son contrato. | **Pendiente:** snapshot/versiones y participantes, documento privado, firma fake con estado durable, callbacks/reintentos, conflictos y mock. | D-CONT/OPS para contenido y reglas; fake permite avanzar el mecanismo una vez fijados firmantes/transiciones locales. |
| **M08 Operación** · #88–94; CU-33–34/48 | Todas abiertas/Bloqueadas. | Calendario, ocupaciones, conversación y storage privado son componentes base, no el ciclo de operación. | **Pendiente:** check-in/out, entrega/recepción, objeción/devolución, evidencias sintéticas y transiciones verificables. | D-CONT/OPS bloquea ventanas y actos cuyo contrato no esté especificado; no bloquea otros módulos. |
| **M09 Comunicación** · #95–102; CU-35–38/49 | Todas abiertas/Bloqueadas pese a slices de conversación/lectura aceptados. | `internal/conversation/`, `internal/adapters/postgres/conversation/`, handlers; PR #157/#159; evidencia `evidencia_m09_local_conversacion_20261006.md`, `evidencia_m09_lectura_local_20261006.md`. | Conversación segura/idempotente/paginada y cursor no leído independiente aceptados como slices. **Parcial:** #98 general, reseñas elegibles, reportes/moderación y notificación durable/reintentos siguen pendientes; Mailpit directo no demuestra outbox. | D-COMM bloquea política de reseña/moderación y retención; chat y avisos tipados locales pueden avanzar según contratos aprobados. |
| **M10 Disputas/cierre económico** · #103–110; CU-39–42 | Todas abiertas/Bloqueadas. | Devolución fake local de M06 no es ledger, garantía ni resolución de M10. | **Pendiente:** reclamo/descargo, resolución motivada, movimientos/garantía/liquidación conciliables y documento tributario sintético claramente marcado. | D-DIS/ADMIN y revisión contable/fiscal bloquean solo reglas financieras/jurídicas dependientes. |
| **M11 Administración** · #111–118; CU-43–46/52 | Todas abiertas/Bloqueadas; existe revisión restringida M03. #180 (LOCAL-CORE-01) en curso como fundamento, enlazado a #112. | `internal/verification/` incluye acción administrativa acotada; esta entrega agrega tablas locales append-only/outbox y permisos mínimos para el uso de identidad. | **Pendiente en módulo general:** gobierno/moderación, consultas/reportes y mock; el evento de auditoría/outbox de credenciales cubre solo un consumidor local. | D-PRIV y D-DIS/ADMIN ratificadas para datos sintéticos locales; D-COMM sigue limitando nuevos tipos de notificación. RNF-017 productivo sigue pendiente. |

## Conciliación de las 43 entidades de ES2

La matriz de ownership autorizada contiene 43 entidades. Las migraciones actuales tienen 34 declaraciones de tabla, con nombres/propósitos de ensayo y soporte; no es una correspondencia 1:1. **Parcial** indica tabla física local específica o capacidad equivalente, no materialización completa de la entidad global. La fuente detallada de owner, colaboración y alcance sigue siendo [`mapa_dominio_y_ownership.md`](mapa_dominio_y_ownership.md); la revisión de contrato es [`revision_diccionario_datos.md`](revision_diccionario_datos.md).

| # | Entidad / owner | Conciliación contra migraciones y código actuales |
|---:|---|---|
| 1 | `usuario` / M01 | Parcial: tabla y operaciones de cuenta autenticada; DB02-09/retención siguen pendientes. |
| 2 | `rol_usuario` / M01 | Parcial: roles de autorización; no representa preferencia de uso RQF-213. |
| 3 | `perfil_usuario` / M02 | Parcial: perfil propio; faltan campos/acciones de alcance M02 pendientes. |
| 4 | `sesion` / M01 | Implementada para sesiones locales, idle/absoluto y revocación según evidencia M01. |
| 5 | `token_accion` / M01 | Implementada para acciones de identidad; el token no se expone en logs. |
| 6 | `version_terminos` / M01 | Implementada como catálogo versionado de aceptación local. |
| 7 | `aceptacion_terminos` / M01 | Implementada para aceptación asociada a cuenta. |
| 8 | `verificacion` / M03 | Parcial: caso/revisión local; faltan criterios generales de elegibilidad/retención. |
| 9 | `cuenta_cobro` / M02 | Sin modelo general; cuenta real no autorizada. |
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
| 27 | `contrato` / M07 | Sin modelo de contrato/versionado. |
| 28 | `firma_contrato` / M07 | Sin modelo ni adaptador de firma. |
| 29 | `operacion_arriendo` / M08 | Sin modelo de entrega/recepción. |
| 30 | `disputa` / M10 | Sin modelo de reclamo/descargo/resolución. |
| 31 | `documento` / M09 | Sin entidad genérica de metadata; archivos sintéticos M03 tienen tabla específica, almacenamiento privado no equivale a esta entidad. |
| 32 | `mensaje_reserva` / M09 | Parcial: `mensaje_reserva_ensayo`, autorizado por participantes, paginado/idempotente y con secuencia. |
| 33 | `resena` / M09 | Sin modelo. |
| 34 | `reporte_resena` / M09 | Sin modelo. |
| 35 | `notificacion` / M09 | Sin entidad/outbox de notificación durable; Mailpit local actual no demuestra entrega recuperable. |
| 36 | `entrega_notificacion` / M09 | Sin modelo de entrega/reintento durable. |
| 37 | `evento_auditoria` / M11 | Sin modelo de auditoría operacional general. |
| 38 | `solicitud_titular` / M02 | Parcial: registro/estado inicial; falta ejecutar exportación/supresión con resolución y retención. |
| 39 | `outbox_evento` / M11 | Sin outbox general/worker; inbox fake de pago no la sustituye. |
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

## Actualización de LOCAL-BOOK-01

Issue hija #177 bajo #77, añadida al Project con HernanMEC. Mantiene abiertas las relaciones generales #77 → #75 → #73 → #65.

El commit `aa06d3759312623830d172d1eabd6c07feeb42b8` añade la integración seleccionada: dos reservas persistidas adyacentes, `23P01` directo con runtime y traducción a HTTP 409 sin filas parciales. Resultado y límites en [`evidence/local-book-01-20261007.md`](evidence/local-book-01-20261007.md). No se ejecutó prueba GCP. La integración usa únicamente una instancia PostgreSQL desechable; la base `espacigo_pgdata` y sus secretos no participan.
