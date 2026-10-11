# LOCAL-CLOSE-01 — matriz final de requisitos y brechas locales

Fecha de conciliación: 2026-10-11. `main` integrado hasta PR #245, merge `74b10dbe655d4f6ae3957a6de81f895ab887e57d`. QA-03/#244 está aceptada, cerrada y Hecho únicamente por su corrida integral. LOCAL-CLOSE-01/#246 queda **En curso**; este documento no declara LOCAL-1 terminado ni solicita su aceptación.

## Regla de clasificación

- **Satisfecho con evidencia:** solo el slice local y el criterio citado, no el Issue padre entero.
- **Pendiente funcional local:** el comportamiento está definido y se puede implementar/probar con PostgreSQL, fake, Mailpit y mock. No se atribuye al proveedor externo.
- **Exclusión expresamente ratificada:** la instrucción del propietario limita explícitamente el prototipo. La obligación productiva puede seguir abierta.
- **Decisión pendiente:** el requisito no concreta un catálogo/regla necesaria; no se inventa alcance.
- **Brecha de evidencia:** no demuestra ausencia de funcionalidad; requiere ejecutar/documentar un escenario sin cambiar producto.
- **Integración externa:** exclusivamente la comunicación, certificación, documento o efecto de un tercero real.

QA-03 demuestra los recorridos incluidos en su manifiesto, no cada aceptación de los padres. Para cada requisito que no se declara satisfecho aquí, se conserva la clasificación pendiente indicada y la traza original en [`trazabilidad_requisitos_backend_local.md`](../trazabilidad_requisitos_backend_local.md). El detalle del runner y de los escenarios ya aceptados está en [`local-qa-03-20261010.md`](local-qa-03-20261010.md); se reutiliza sin repetirlo.

## Matriz RQF/CU por criterio

| Módulo y requisitos exactos | Estado local | Implementación/evidencia o brecha concreta |
|---|---|---|
| **M01** RQF-001–015, 017–023, 188, 213–218; CU-01–06/50 | **Satisfecho con evidencia** para el ciclo sintético | Registro/verificación Mailpit, sesión/logout, recuperación/cambio de clave, controles/ventana de historial, preferencia no excluyente y outbox V22/V27/V46. `local-auth-01-v22-20261007.md`, `local-priv-01a5-outbox-terminal-20261008.md`, `local-close-01-notices-20261011.md`, QA-03. |
| M01 RQF-016 | **Aceptado localmente (#247)** | V46 añade alerta durable de Mailpit por bloqueo sin verificar; integración PostgreSQL/Mailpit enfocada reutilizada en #251. El correo productivo sigue externo. |
| **M02** RQF-024–032, 034–037, 189–191; CU-07–09 | **Satisfecho con evidencia** solo para los recursos modelados | Perfil, foto PNG, cuenta de cobro ficticia con KYC, exportación ZIP, solicitud/revisión y baja con bloqueadores/minimización están en `local-m02-photo-payout-20261008.md`, `local-priv-01a4-export-20261008.md`, QA-02 y QA-03. Los padres #185/#40 siguen abiertos. |
| M02 RQF-033 | **Exclusión expresamente ratificada para este prototipo** | No se recogen RUT ni datos bancarios reales; el adaptador de cuenta de cobro usa referencias ficticias (#202). No equivale a satisfacer verificación bancaria productiva. |
| M02, privacidad_local_v1 | **Implementación en revisión (#248)** | V47 añade ejecución por lotes de metadata/blob KYC, cotizaciones no convertidas, aceptaciones y solicitudes vencidas. La cola de archivos persiste lease/reintento; ante fallo se conserva el archivo y su referencia, y ante éxito se retira la referencia antes de que venza la metadata. Evidencia `local-close-01-retention-20261011.md`; revisión y merge pendientes. |
| M02 copias de datos | **Exclusión de inspección ratificada; pendiente de inventario externo** | El inventario conocido y el replay de bajas tras restauración están documentados en `inventario-copias-locales-20261008.md` y `local-qa-02-20261010.md`. El propietario limitó la inspección a ubicaciones conocidas; discos, sync externo y otros equipos no se afirman cubiertos. No borrar respaldos. |
| **M03** RQF-038, 041, 045–046, 054–059, 192–193, 219–220; CU-10–14 | **Satisfecho con evidencia** para casos y resultados sintéticos | Ownership, revisión manual, rechazo/reintento, elegibilidad por tipo, historial, revocación y gates locales: `local-kyc-01-20261008.md`, PR #201/QA-03. |
| M03 RQF-047/194 | **Aceptado localmente (#247)** | V46 añade aviso durable Mailpit de resultado KYC/KYB; las pruebas de productor/entrega publicadas en #251 se reutilizan. |
| M03 RQF-039–040, 042–044, 048–053; #142 | **Exclusión expresamente ratificada para este prototipo** / **integración externa** | No documentos/RUT personales, OCR, Registro Civil/SII ni proveedor real. Casos sintéticos y su elegibilidad local sí existen; #142 conserva lo productivo. |
| **M04** RQF-060–079, 081–093, 195–198, 221–226; CU-15–18 | **Satisfecho con evidencia** para publicación local | Borrador, ocho categorías, atributos/perfiles, tarifa, zona, edición parcial, reglas de horario, publicación/ocultamiento/reactivación, historial y galería privada sintética: evidencias `local-m04-publication-20261008.md`, `local-m04-edit-20261009.md`, `local-m04-edit-details-20261009.md`, `local-m04-gallery-20261008.md`; QA-03 integra su consumo. Los padres conservan políticas comerciales generales. |
| M04 RQF-080 | **Exclusión expresamente ratificada para este prototipo** | No geocodificación de dirección privada. Las coordenadas se cargan solo como ubicación sintética explícita de fixtures autorizados (M05-LOCAL). |
| **M05** RQF-094–107; CU-19–21 | **Satisfecho con evidencia** para catálogo local | Oferta publicada, filtros categoría/atributos/precio/geo, paginación, selector, detalle y recotización con datos vigentes: `local-m05-published-catalog-20261009.md`, QA-03. Sin afirmación de rendimiento/escala de producción. |
| **M06** RQF-108–120, 123–129, 199–200, 227–230; CU-22–28/47/51 | **Satisfecho con evidencia** para flujo fake local | Ocupación transaccional, snapshots/idempotencia, inbox autenticado, rechazo/vencimiento, aprobación, cancelación y devolución `local_flexible_v1`; garantía y settlement según cortes aceptados. QA-03 `TestLocalBookingTrialPostgresLifecycleAndConcurrentRetry`, `local-book-02-20261009.md`, `local-fin-01-20261009.md`, `local-fin-settlement-20261010.md`. |
| M06 RQF-121/122/201 | **Aceptado localmente (#247)** | V46 emite avisos durables para el pago confirmado/rechazado y la solicitud pagada; la garantía rechazada no crea un aviso de pago rechazado. Pruebas enfocadas de Mailpit publicadas en #251. |
| M06 RQF-114–118 y reembolsos | **Satisfecho con evidencia** en adaptador fake; **integración externa** pendiente | Las reglas y estados fake son reproducibles; PSP, sandbox, credenciales y movimientos reales no lo son. Mantener #76/#78 abiertos; #69/#73/#75/#77/#79 retienen otros criterios generales. |
| **M07** RQF-130–132, 136–138, 140–142, 202; CU-29–32 | **Satisfecho con evidencia** para contrato sintético | Snapshot, artefacto privado, firmas/rechazo de ambos titulares, vencimiento/ocupación y devolución local; `local-cont-01-matrix.md`, QA-03. No tiene validez jurídica. |
| M07 RQF-134/135/139 | **Aceptado localmente (#247)** | V46 envía invitación local a cada firmante y aviso durable al completar firmas. El enlace lleva solo el ID de recurso y exige autenticación/autorización. Prueba de respuesta tardía y Mailpit publicada en #251. |
| M07 RQF-131/133 y validez de firma | **Exclusión expresamente ratificada** / **integración externa** | Datos legales reales, proveedor/callback de firma y efecto jurídico permanecen fuera; no se confunde con los avisos locales pendientes. |
| **M08** RQF-143–152, 203–206; CU-33/34/48 | **Satisfecho con evidencia** para operación sintética | Check-in/out, fecha IANA, fotos generadas en backend, recepción, estados, conversación en curso y reloj; `local-ops-01-matrix.md`, QA-03. |
| **M09** RQF-153–158, 164, 207, 231–235; CU-35–38/49 | **Satisfecho con evidencia** por slices locales | Chat/cursores, reseñas recíprocas, promedio, reporte y moderación; aviso local de check-in, apertura de reclamo y cancelación; privacidad/outbox D-COMM. `local-comm-01-20261009.md`, QA-03. No añadir aviso por mensaje ni realtime, decisiones expresamente fuera del slice. |
| **M10** RQF-159–163, 166–177, 208–209; CU-39–42 | **Satisfecho con evidencia** para disputa/finanzas fake | Evidencia sintética de apertura, resolución, deducción dentro de autorización, captura/liberación, obligación durable/conciliación y liquidación fake versionada: `local-dis-01-20261009.md`, `local-dis-test-mock-20261009.md`, `local-fin-01-20261009.md`, `local-fin-settlement-20261010.md`, QA-03. |
| M10 RQF-165 | **Pendiente funcional local** | El descargo no acepta/admite todavía adjuntar evidencia fotográfica sintética privada, aunque el storage privado ya soporta los documentos del reclamo. Requiere integration y mock del recurso, no evidencia real. |
| M10 RQF-210 | **Satisfecho localmente, evidencia aceptada** | V38 añade trigger transaccional de aviso a ambos participantes al insertar resolución (`reclamo_resuelto`); `aviso_local` tiene ciclo durable Mailpit y reintentos. Integración de Mailpit y cierre M10 se cubrieron en los cortes #222/#223 y QA-03. No notifica movimientos financieros adicionales. |
| M10 RQF-173–175/208 | **Satisfecho con evidencia** solo bajo settlement/garantía fake ratificados | No se atribuye movimiento real a fake. |
| M10 RQF-176/211 y pago/garantía reales | **Exclusión expresamente ratificada** / **integración externa** | Solo `comprobante_ensayo` no fiscal y adaptadores fake. Sin boleta tributaria válida, PSP, captura, liberación ni transferencia real. |
| **M11** RQF-178–182, 184–185; CU-43–46/52 | **Satisfecho con evidencia** para bloqueo, moderación y ledger auditado | Bloqueo restringido, reportes CU-45 ratificados, consulta/export de auditoría con cursor y auditoría transaccional de acciones financieras. `local-admin-01a/b/c/d-20261009/10.md`, `diseno_local_admin_arch_01.md`, QA-03. |
| M11 RQF-212/236 | **Pendiente funcional local** | Falta consulta/búsqueda acotada de cuenta por clave técnica y listado administrativo de espacios publicados. No requiere infraestructura externa. |
| M11 RQF-183 / CU-45 | **Parcial + pendiente funcional local** | Dos reportes ratificados están implementados; falta el tercero ratificado ahora: agregado de disputas por período y estado, solo lectura y minimizado. Se difieren reportes adicionales no especificados. |
| M11 RQF-184 (búsqueda por RUT) | **Exclusión expresamente ratificada para este prototipo** | La consulta auditada usa actor UUID y campos minimizados. No se introduce RUT personal en el ledger local; se conserva fuera el requisito productivo correspondiente. |
| MAP-07 campañas, promociones, NPS | **Exclusión/diferimiento expresamente ratificado** | Sin finalidad, permisos o retención ratificados; no implementar. |

## Privacidad: inventario específico de brechas locales

- **Implementado:** baja protegida con revalidación transaccional; bloqueadores; minimización y retención residual explícitas; exportación ZIP de datos propios implementados; archivo durable de replay de bajas y restauración SQL+blobs aislada; eliminación recuperable de evidencia cuando la baja o el propietario la inicia; purga de vínculos de reservas solo cuando ya no hay obligaciones; retención de historial de claves, outbox de credenciales, reseñas/reportes/avisos locales; ledger de auditoría de cinco años. Evidencia en `local-priv-01a4-export-20261008.md`, `inventario-copias-locales-20261008.md`, `local-qa-02-20261010.md`, `local-comm-01-20261009.md`, `local-fin-settlement-20261010.md`.
- **Implementación en revisión (#248):** V47 aplica esos plazos con purga por lotes. La evidencia PostgreSQL enfocada demuestra reloj simulado, archivo KYC, fallo del borrado, recuperación con servicio recreado, retención de metadata y posterior eliminación de case/historial/elegibilidad; también verifica los vencimientos de términos, solicitud y cotización no convertida, conservando el snapshot convertido.
- **Pendiente de aceptación:** revisión/merge de #248 y regresión final LOCAL-CLOSE-01; no se vuelve a correr QA-02 completo por este cambio.
- **Límite inventariado:** ubicaciones externas al checkout/home confirmado siguen desconocidas según autorización; no se inspecciona todo el PC, no se borran respaldos y no se afirma que copias desconocidas se hayan reaplicado. Esto no bloquea el código local autorizado, pero impide afirmar supresión integral de esas copias.

## RNF y exclusiones transversales

| Grupo | Clasificación | Evidencia/límite |
|---|---|---|
| RNF verificables localmente: integridad, auth/ownership, errores, idempotencia, migración, roles/grants y estabilidad de workers | **Satisfecho con evidencia**, dentro de paquetes citados | QA-01 `local-qa-01-20261010.md`, QA-02, QA-03, y pruebas enfocadas por módulo. No equivale a certificación productiva. |
| RNF-043 auditoría | **Satisfecho localmente** | Cinco años desde cada evento para las filas de audit, no extensible a otras tablas; test y vista admin. |
| RNF-017 inmutabilidad de infraestructura | **Integración externa/exclusión expresa** | Grants append-only del rol runtime no prueban tamper-proof ante superusuario o almacenamiento productivo inmutable. Sin GCP. |
| Performance/SLA, disponibilidad de email real, PSP/firma real, fiscales/jurídicos, documentos reales | **Integración externa o exclusión ratificada**, según requisito | Mantener funciones fake pendientes como locales cuando la fila funcional lo indica; no etiquetarlas externas por asociación. |

## Estado de cierre

LOCAL-CLOSE-01 **no es aceptable aún**: la tabla enumera brechas locales obligatorias de avisos, evidencia de descargo, retención y consulta admin. La autorización para cerrar implica resolverlas en entregas funcionales revisables, no darles estado Hecho con esta matriz. La ratificación posterior amplía de dos a tres los reportes locales de CU-45: se agrega el agregado de disputas por período/estado. RQF-212 se limita a búsqueda exacta por UUID técnico; RQF-236 incluye lista/consulta de espacios publicados. Los Issue slices #247–#250 trazan las brechas; no implican aceptación. Los padres #185/#40, #109/#110 y #111–#118 permanecen abiertos por criterios generales; sus cortes aceptados no los cierran.

## Decisiones ratificadas para cerrar funciones locales

| Requisito | Estado tras ratificación | Regla acotada |
|---|---|---|
| RQF-212 | Pendiente funcional local (#250) | Búsqueda exacta de cuenta sintética por UUID técnico; respuesta mínima, administrador activo y auditoría. Sin RUT ni contacto. |
| RQF-183 / CU-45 | Parcial aceptado; brecha local (#250) | Dos reportes ya implementados más agregado de disputas por período/estado. Solo lectura, no incluye texto/evidencia/contacto. Otros reportes no especificados se difieren. |
| RQF-016/047/194/121/122/201/134/135/139 | Aceptado localmente (#247) | V46/PR #251 implementa los productores/outbox local Mailpit; #247 cerrada/Hecho después de aplicar V46. SMTP conserva posibilidad de duplicado ante resultado incierto. RQF-210 ya cuenta con trigger y entrega probados. |
| RQF-165 | Pendiente funcional local (#249) | Evidencia sintética privada generada por servidor en descargo, reutilizando storage, exportación, baja y limpieza recuperable. |
| Retención de privacidad ratificada | En revisión (#248) | V47 añade purgadores recuperables para los vencimientos ratificados aún sin consumidor; evidencia de fallos/reintentos y referencias KYC en `local-close-01-retention-20261011.md`. No se añade ni generaliza ningún plazo. |
| RQF-236 | Pendiente funcional local (#250) | Consulta administrativa autenticada/auditada de publicaciones, proyección mínima; no muta publicación. |

Las integraciones reales siguen en una columna separada y no excusan estos pendientes. No se inicia trabajo de GCP.
