# LOCAL-CLOSE-01 — matriz final de requisitos y brechas locales

Fecha de conciliación: 2026-10-11. `main` integrado hasta PR #255, merge `036718dfd80549d0aae415a1d404d07419cacbec`. QA-03/#244 y las subentregas #247–#255 están aceptadas únicamente por sus cortes; #246 queda **En curso** y el propietario no ha aceptado LOCAL-1. La regresión integral existente se conserva como evidencia y no se repite hasta resolver las brechas locales que siguen. Este documento no declara LOCAL-1 terminado.

## Regla de clasificación

- **Satisfecho con evidencia:** solo el slice local y el criterio citado, no el Issue padre entero.
- **Pendiente funcional local:** el comportamiento está definido y se puede implementar/probar con PostgreSQL, fake, Mailpit y mock. No se atribuye al proveedor externo.
- **Exclusión expresamente ratificada:** la instrucción del propietario limita explícitamente el prototipo. La obligación productiva puede seguir abierta.
- **Decisión pendiente:** el requisito no concreta un catálogo/regla necesaria; no se inventa alcance.
- **Brecha de evidencia:** no demuestra ausencia de funcionalidad; requiere ejecutar/documentar un escenario sin cambiar producto.
- **Integración externa:** exclusivamente la comunicación, certificación, documento o efecto de un tercero real.

QA-03 demuestra los recorridos incluidos en su manifiesto, no cada aceptación de los padres. Para cada requisito que no se declara satisfecho aquí, se conserva la clasificación pendiente indicada y la traza original en [`trazabilidad_requisitos_backend_local.md`](../trazabilidad_requisitos_backend_local.md). El detalle del runner y de los escenarios ya aceptados está en [`local-qa-03-20261010.md`](local-qa-03-20261010.md); se reutiliza sin repetirlo.

## Brechas funcionales locales obligatorias aún pendientes

La revisión requisito → handler/servicio → mock del 2026-10-11 encontró los criterios siguientes en ES1 sin comportamiento equivalente aceptado. Por tanto, **todavía no se puede solicitar el cierre de LOCAL-1**. Estos son pendientes locales implementables; no se clasifican como externos por asociación a su Issue padre.

| ID | Brecha concreta observada | Trazabilidad / dependencia | Clasificación |
|---|---|---|---|
| RQF-077 | La galería sintética permite generar, listar, consultar y retirar PNG; la subentrega #256 agrega seleccionar y persistir una portada sintética privada propia. | M04, #55/#56; galería aceptada #210; implementación #256 pendiente de revisión/aceptación. | En implementación; no aceptar el criterio antes de fusionar y recorrerlo. |
| RQF-091, RQF-092 | No existe operación autenticada de eliminación permanente de una publicación ni su precondición transaccional. La regla local de conservación histórica ya fue ratificada el 2026-10-11; falta implementar y probar el corte. | M04, #52–#61; #258, Listo tras #257. | Pendiente funcional local. |
| RQF-094 | La búsqueda de catálogo no acepta texto libre ni busca publicaciones por texto; solo admite filtros estructurados. | M05, #62–#68; subentrega #257. | Pendiente funcional local. |
| RQF-095 | El mock muestra distancia numérica, pero no presenta mapa interactivo con ubicaciones sintéticas. No se requiere geocodificación ni ubicación del navegador para cerrar esta brecha local. | M05, #62–#68; subentrega #257 reutiliza coordenadas sintéticas autorizadas. | Pendiente funcional local. |
| RQF-099 | El catálogo no recibe ni aplica un filtro de superficie mínima (`area_m2`), aunque la superficie se guarda en el espacio. | M05, #62–#68; #257 agrega filtro y control. | Pendiente funcional local. |
| RQF-102 | No hay acción del mock para limpiar de una vez los filtros de búsqueda; “Volver al inicio” solo reinicia la paginación. | M05, #62–#68; #257 restablece criterios y cursores. | Pendiente funcional local. |
| RQF-177 | Tras liquidación fake confirmada, la operación queda `liquidada`, pero la reserva conserva `finalizada`; no se registra la transición requerida a `cerrada`. | M10, #109/#110; subentrega #259. | Pendiente funcional local. |
| RQF-198 | La subentrega #256 permite versionar modalidad hora/día/mes en espacio propio borrador, activo u oculto; horario semanal activo impide cambio incompatible. Cotizaciones y reservas históricas retienen su snapshot. | M04, #55/#52–#61; implementación #256 pendiente de revisión/aceptación. | En implementación; no aceptar el criterio antes de fusionar y recorrerlo. |

## Subentregas ordenadas de brechas obligatorias

1. **#256 M04 — portada y modalidad tarifaria:** en curso; V49 agrega referencia de portada a imagen propia activa y exige compatibilidad con horario semanal. Snapshots existentes permanecen inmutables.
2. **#257 M05 — búsqueda textual, mapa sintético, superficie mínima y limpiar filtros:** Listo después de aceptar #256.
3. **#258 M04 — eliminación y conservación histórica:** Listo tras #257; propietario ratificó el 2026-10-11 la verificación transaccional de bloqueadores, preservación de snapshots/hechos, no cascada, fechas originales y limpieza durable recuperable, acotada a esa publicación. Implementar y probar conforme a [la Issue](https://github.com/HernanEspinozaDev/espaciGo/issues/258).
4. **#259 M10/M06 — cerrar reserva tras liquidación fake:** Listo después de #258; requiere obligaciones resueltas e idempotencia.

Las Issues son hijas de #246 y preservan referencias a padres originales. #246 permanece abierta hasta la aceptación explícita de LOCAL-1.

RQF-077 y RQF-198 se implementan en la subentrega #256 descrita en [su evidencia](local-m04-cover-rate-20261011.md); los demás criterios anteriores continúan pendientes. Ninguna parte de #256 se considera aceptada antes de la revisión y el merge. El alcance productivo o expresamente diferido se separa abajo.

## Matriz RQF/CU por criterio

| Módulo y requisitos exactos | Estado local | Implementación/evidencia o brecha concreta |
|---|---|---|
| **M01** RQF-001–015, 017–023, 188, 213–218; CU-01–06/50 | **Satisfecho con evidencia** para el ciclo sintético | Registro/verificación Mailpit, sesión/logout, recuperación/cambio de clave, controles/ventana de historial, preferencia no excluyente y outbox V22/V27/V46. `local-auth-01-v22-20261007.md`, `local-priv-01a5-outbox-terminal-20261008.md`, `local-close-01-notices-20261011.md`, QA-03. |
| M01 RQF-016 | **Aceptado localmente (#247)** | V46 añade alerta durable de Mailpit por bloqueo sin verificar; integración PostgreSQL/Mailpit enfocada reutilizada en #251. El correo productivo sigue externo. |
| **M02** RQF-024–032, 034–037, 189–191; CU-07–09 | **Satisfecho con evidencia** solo para los recursos modelados | Perfil, foto PNG, cuenta de cobro ficticia con KYC, exportación ZIP, solicitud/revisión y baja con bloqueadores/minimización están en `local-m02-photo-payout-20261008.md`, `local-priv-01a4-export-20261008.md`, QA-02 y QA-03. Los padres #185/#40 siguen abiertos. |
| M02 RQF-033 | **Exclusión expresamente ratificada para este prototipo** | No se recogen RUT ni datos bancarios reales; el adaptador de cuenta de cobro usa referencias ficticias (#202). No equivale a satisfacer verificación bancaria productiva. |
| M02, privacidad_local_v1 | **Aceptada localmente (#248)** | V47 añade ejecución por lotes de metadata/blob KYC, cotizaciones no convertidas, aceptaciones y solicitudes vencidas. La cola de archivos persiste lease/reintento; ante fallo se conserva el archivo y su referencia, y ante éxito se retira la referencia antes de que venza la metadata. PR #252 fusionado, V47 aplicado incrementalmente y evidencia `local-close-01-retention-20261011.md`. |
| M02 copias de datos | **Exclusión de inspección ratificada; pendiente de inventario externo** | El inventario conocido y el replay de bajas tras restauración están documentados en `inventario-copias-locales-20261008.md` y `local-qa-02-20261010.md`. El propietario limitó la inspección a ubicaciones conocidas; discos, sync externo y otros equipos no se afirman cubiertos. No borrar respaldos. |
| **M03** RQF-038, 041, 045–046, 054–059, 192–193, 219–220; CU-10–14 | **Satisfecho con evidencia** para casos y resultados sintéticos | Ownership, revisión manual, rechazo/reintento, elegibilidad por tipo, historial, revocación y gates locales: `local-kyc-01-20261008.md`, PR #201/QA-03. |
| M03 RQF-047/194 | **Aceptado localmente (#247)** | V46 añade aviso durable Mailpit de resultado KYC/KYB; las pruebas de productor/entrega publicadas en #251 se reutilizan. |
| M03 RQF-039–040, 042–044, 048–053; #142 | **Exclusión expresamente ratificada para este prototipo** / **integración externa** | No documentos/RUT personales, OCR, Registro Civil/SII ni proveedor real. Casos sintéticos y su elegibilidad local sí existen; #142 conserva lo productivo. |
| **M04** RQF-060–076, 078–090, 093, 195–197, 221–226; CU-15–18 | **Satisfecho con evidencia** para los criterios identificados de publicación local | Borrador, ocho categorías, atributos/perfiles, tarifa inicial, zona, edición de campos admitidos, reglas de horario, publicar/ocultar/reactivar, historial y galería sintética: evidencias M04/QA-03. Excepciones: RQF-077, RQF-091/092 y RQF-198 son pendientes locales (tabla anterior). RQF-080 es geocodificación excluida. |
| M04 RQF-080 | **Exclusión expresamente ratificada para este prototipo** | No geocodificación de dirección privada. Las coordenadas se cargan solo como ubicación sintética explícita de fixtures autorizados (M05-LOCAL). |
| **M05** RQF-096–098, 100–101, 103–107; CU-19–21 | **Satisfecho con evidencia** para el alcance de catálogo aceptado | Oferta publicada, tarjetas, filtros categoría/atributos/precio/geo, disponibilidad, paginación, selector, detalle y recotización con datos vigentes: `local-m05-published-catalog-20261009.md`, QA-03. RQF-094/095/099/102 permanecen como brechas funcionales locales; no se afirma rendimiento/escala productiva. |
| **M06** RQF-108–120, 123–129, 199–200, 227–230; CU-22–28/47/51 | **Satisfecho con evidencia** para flujo fake local | Ocupación transaccional, snapshots/idempotencia, inbox autenticado, rechazo/vencimiento, aprobación, cancelación y devolución `local_flexible_v1`; garantía y settlement según cortes aceptados. QA-03 `TestLocalBookingTrialPostgresLifecycleAndConcurrentRetry`, `local-book-02-20261009.md`, `local-fin-01-20261009.md`, `local-fin-settlement-20261010.md`. |
| M06 RQF-121/122/201 | **Aceptado localmente (#247)** | V46 emite avisos durables para el pago confirmado/rechazado y la solicitud pagada; la garantía rechazada no crea un aviso de pago rechazado. Pruebas enfocadas de Mailpit publicadas en #251. |
| M06 RQF-114–118 y reembolsos | **Satisfecho con evidencia** en adaptador fake; **integración externa** pendiente | Las reglas y estados fake son reproducibles; PSP, sandbox, credenciales y movimientos reales no lo son. Mantener #76/#78 abiertos; #69/#73/#75/#77/#79 retienen otros criterios generales. |
| M10 RQF-177 | **Pendiente funcional local** | La liquidación fake se confirma, pero la reserva no pasa de `finalizada` a `cerrada` con historial en la misma operación confirmada. |
| M04 RQF-198 | **Pendiente funcional local** | La edición de tarifa no permite cambiar su modalidad; véase detalle en la lista de brechas. |
| **M07** RQF-130–132, 136–138, 140–142, 202; CU-29–32 | **Satisfecho con evidencia** para contrato sintético | Snapshot, artefacto privado, firmas/rechazo de ambos titulares, vencimiento/ocupación y devolución local; `local-cont-01-matrix.md`, QA-03. No tiene validez jurídica. |
| M07 RQF-134/135/139 | **Aceptado localmente (#247)** | V46 envía invitación local a cada firmante y aviso durable al completar firmas. El enlace lleva solo el ID de recurso y exige autenticación/autorización. Prueba de respuesta tardía y Mailpit publicada en #251. |
| M07 RQF-131/133 y validez de firma | **Exclusión expresamente ratificada** / **integración externa** | Datos legales reales, proveedor/callback de firma y efecto jurídico permanecen fuera; no se confunde con los avisos locales pendientes. |
| **M08** RQF-143–152, 203–206; CU-33/34/48 | **Satisfecho con evidencia** para operación sintética | Check-in/out, fecha IANA, fotos generadas en backend, recepción, estados, conversación en curso y reloj; `local-ops-01-matrix.md`, QA-03. |
| **M09** RQF-153–158, 164, 207, 231–235; CU-35–38/49 | **Satisfecho con evidencia** por slices locales | Chat/cursores, reseñas recíprocas, promedio, reporte y moderación; aviso local de check-in, apertura de reclamo y cancelación; privacidad/outbox D-COMM. `local-comm-01-20261009.md`, QA-03. No añadir aviso por mensaje ni realtime, decisiones expresamente fuera del slice. |
| **M10** RQF-159–163, 166–177, 208–209; CU-39–42 | **Satisfecho con evidencia** para disputa/finanzas fake | Evidencia sintética de apertura, resolución, deducción dentro de autorización, captura/liberación, obligación durable/conciliación y liquidación fake versionada: `local-dis-01-20261009.md`, `local-dis-test-mock-20261009.md`, `local-fin-01-20261009.md`, `local-fin-settlement-20261010.md`, QA-03. |
| M10 RQF-165 | **Aceptado localmente (#249)** | Evidencia PNG sintética privada de descargo, recuperación tras fallo, exportación del autor, autorización de participantes/admin y tratamiento por baja/retención: PR #253 y sus pruebas enfocadas. No implica fotos reales ni completa M10. |
| M10 RQF-210 | **Satisfecho localmente, evidencia aceptada** | V38 añade trigger transaccional de aviso a ambos participantes al insertar resolución (`reclamo_resuelto`); `aviso_local` tiene ciclo durable Mailpit y reintentos. Integración de Mailpit y cierre M10 se cubrieron en los cortes #222/#223 y QA-03. No notifica movimientos financieros adicionales. |
| M10 RQF-173–175/208 | **Satisfecho con evidencia** solo bajo settlement/garantía fake ratificados | No se atribuye movimiento real a fake. |
| M10 RQF-176/211 y pago/garantía reales | **Exclusión expresamente ratificada** / **integración externa** | Solo `comprobante_ensayo` no fiscal y adaptadores fake. Sin boleta tributaria válida, PSP, captura, liberación ni transferencia real. |
| **M11** RQF-178–182, 184–185; CU-43–46/52 | **Satisfecho con evidencia** para bloqueo, moderación y ledger auditado | Bloqueo restringido, reportes CU-45 ratificados, consulta/export de auditoría con cursor y auditoría transaccional de acciones financieras. `local-admin-01a/b/c/d-20261009/10.md`, `diseno_local_admin_arch_01.md`, QA-03. |
| M11 RQF-212/236 | **Aceptado localmente (#250)** | API administrativa de búsqueda exacta UUID y lista/detalle de publicaciones activas con proyección mínima y auditoría por petición. PR #254 integrado; pruebas PostgreSQL desechables y controles del mock documentados en `local-close-01d-admin-queries-20261011.md`. No requiere infraestructura externa. |
| M11 RQF-183 / CU-45 | **Tres reportes locales aceptados por alcance** | Dos reportes previamente aceptados y conteo agregado de disputas por estado vigente para reclamos abiertos durante el período, de solo lectura y sin texto, evidencia ni contacto; período IANA acotado a 31 días, auditado por petición. Integración PostgreSQL desechable cubre apertura/cierre. Otros reportes no especificados se difieren conforme a ratificación. |
| M11 RQF-184 (búsqueda por RUT) | **Exclusión expresamente ratificada para este prototipo** | La consulta auditada usa actor UUID y campos minimizados. No se introduce RUT personal en el ledger local; se conserva fuera el requisito productivo correspondiente. |
| MAP-07 campañas, promociones, NPS | **Exclusión/diferimiento expresamente ratificado** | Sin finalidad, permisos o retención ratificados; no implementar. |

## Privacidad: inventario específico de brechas locales

- **Implementado:** baja protegida con revalidación transaccional; bloqueadores; minimización y retención residual explícitas; exportación ZIP de datos propios implementados; archivo durable de replay de bajas y restauración SQL+blobs aislada; eliminación recuperable de evidencia cuando la baja o el propietario la inicia; purga de vínculos de reservas solo cuando ya no hay obligaciones; retención de historial de claves, outbox de credenciales, reseñas/reportes/avisos locales; ledger de auditoría de cinco años. Evidencia en `local-priv-01a4-export-20261008.md`, `inventario-copias-locales-20261008.md`, `local-qa-02-20261010.md`, `local-comm-01-20261009.md`, `local-fin-settlement-20261010.md`.
- **Aceptado localmente (#248):** V47 aplica esos plazos con purga por lotes. La evidencia PostgreSQL enfocada demuestra reloj simulado, archivo KYC, fallo del borrado, recuperación con servicio recreado, retención de metadata y posterior eliminación de case/historial/elegibilidad; también verifica los vencimientos de términos, solicitud y cotización no convertida, conservando el snapshot convertido. PR #252 está fusionado y V47 aplicado incrementalmente; no se repitió QA-02 completo.
- **Runner integral:** PASS para el código/manifiesto del commit probado `5d635f826f6b1afeeb514148f5bc4c68c9b1550c`, después del merge #254. La evidencia detallada está en [`local-close-01-final-20261011.md`](local-close-01-final-20261011.md). Esta corrida demuestra las pruebas incluidas; no cubre los RQF locales pendientes enumerados arriba ni sustituye la aceptación del propietario.
- **Límite inventariado:** ubicaciones externas al checkout/home confirmado siguen desconocidas según autorización; no se inspecciona todo el PC, no se borran respaldos y no se afirma que copias desconocidas se hayan reaplicado. Esto no bloquea el código local autorizado, pero impide afirmar supresión integral de esas copias.

## RNF y exclusiones transversales

| Grupo | Clasificación | Evidencia/límite |
|---|---|---|
| RNF verificables localmente: integridad, auth/ownership, errores, idempotencia, migración, roles/grants y estabilidad de workers | **Satisfecho con evidencia**, dentro de paquetes citados | QA-01 `local-qa-01-20261010.md`, QA-02, QA-03, y pruebas enfocadas por módulo. No equivale a certificación productiva. |
| RNF-043 auditoría | **Satisfecho localmente** | Cinco años desde cada evento para las filas de audit, no extensible a otras tablas; test y vista admin. |
| RNF-017 inmutabilidad de infraestructura | **Integración externa/exclusión expresa** | Grants append-only del rol runtime no prueban tamper-proof ante superusuario o almacenamiento productivo inmutable. Sin GCP. |
| Performance/SLA, disponibilidad de email real, PSP/firma real, fiscales/jurídicos, documentos reales | **Integración externa o exclusión ratificada**, según requisito | Mantener funciones fake pendientes como locales cuando la fila funcional lo indica; no etiquetarlas externas por asociación. |

## Estado de cierre

LOCAL-CLOSE-01 espera la aceptación del propietario tras la regresión integral post-merge. Las brechas locales identificadas ya tienen slices aceptados: avisos (#247), retención/purgadores (#248), evidencia de descargo (#249) y consultas administrativas/reportes (#250). La ratificación amplía de dos a tres los reportes locales de CU-45: agregado de disputas por período/estado. RQF-212 se limita a búsqueda exacta por UUID técnico; RQF-236 incluye lista/consulta de espacios publicados. Ninguna aceptación cierra por sí sola los padres. Los padres #185/#40, #109/#110 y #111–#118 permanecen abiertos por criterios generales; sus cortes aceptados no los cierran.

## Decisiones ratificadas para cerrar funciones locales

| Requisito | Estado tras ratificación | Regla acotada |
|---|---|---|
| RQF-212 | Aceptado localmente (#250/#254) | Búsqueda exacta de cuenta sintética por UUID técnico; respuesta mínima, administrador activo y auditoría. Sin RUT ni contacto. PR #254 y prueba HTTP/PostgreSQL en `local-close-01d-admin-queries-20261011.md`. |
| RQF-183 / CU-45 | Tres reportes locales aceptados por su alcance | Incluye el agregado de disputas por período/estado, solo lectura; validado por prueba PostgreSQL. Otros reportes no especificados se difieren según ratificación. |
| RQF-016/047/194/121/122/201/134/135/139 | Aceptado localmente (#247) | V46/PR #251 implementa los productores/outbox local Mailpit; #247 cerrada/Hecho después de aplicar V46. SMTP conserva posibilidad de duplicado ante resultado incierto. RQF-210 ya cuenta con trigger y entrega probados. |
| RQF-165 | Aceptado localmente (#249, PR #253) | Evidencia sintética privada de descargo, exportación acotada, baja y purga recuperable; recorrido y pruebas aceptados. |
| Retención de privacidad ratificada | Aceptada localmente (#248, PR #252) | V47 aplicado incrementalmente con evidencia de fallos/reintentos y referencias KYC en `local-close-01-retention-20261011.md`. No se añade ni generaliza ningún plazo. |
| RQF-236 | Aceptado localmente (#250/#254) | Lista y consulta administrativa autenticada/auditada de publicaciones activas, proyección mínima; no muta publicación. |
| RQF-091/092 · privacidad al eliminar publicación | Regla local ratificada; implementación pendiente (#258) | Bloqueadores se revisan transaccionalmente antes de cambios; si es elegible, retirar solo esa oferta y contenido sin finalidad, conservar snapshots/hechos históricos mínimos sin cascada, reutilizar fechas/plazos originales y dejar limpieza de archivos durable. No cancelar reservas ni afirmar anonimización. |

Las integraciones reales siguen en una columna separada y no excusan estos pendientes. No se inicia trabajo de GCP.
