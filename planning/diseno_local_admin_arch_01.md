# Diseño local ADMIN-ARCH-01 — permisos, auditoría y outbox

Estado: diseño ratificado para LOCAL-ADMIN-ARCH-01 (#232), hija de ADMIN-ARCH-01 (#111). Depende del corte local #230 aceptado (`ca259396`) y de CORE-ARCH-01 #19, ya cerrado. #111 conserva su dependencia original de DIS-MOCK-01 #110 y permanece abierta/bloqueada en Projects: este diseño permite avanzar un alcance local sin declarar satisfecho ese padre.

## Fuentes y límites

- Criterios originales ADMIN-ARCH-01 y ADMIN-DB/BE/API/TEST/MOCK #112–#118; CU-43–46/52, RQF-178–185/212/236, RNF-017/024/028/043.
- Contrato de ownership de [`contratos_entre_modulos.md`](contratos_entre_modulos.md) y [`mapa_dominio_y_ownership.md`](mapa_dominio_y_ownership.md): M11 posee el ledger común y el contrato outbox; los módulos dueños conservan sus invariantes y escriben intención en su propia transacción.
- Reutilizar LOCAL-ADMIN-01A/#226, LOCAL-ADMIN-01B/#228, LOCAL-DIS-01/#222, LOCAL-FIN-01/#224, LOCAL-COMM-01/#220, LOCAL-KYC-01/#200 y LOCAL-CORE-01/#180. Evidencia de la base local aceptada en [`local-dis-test-mock-20261009.md`](evidence/local-dis-test-mock-20261009.md).
- Alcance solo local. No se afirma inmutabilidad productiva RNF-017, no se configura GCP/BigQuery y BigQuery no es fuente transaccional.

## Política administrativa propuesta

El permiso se valida en Backend en cada acción; ocultar controles en el mock no es autorización. Solo una sesión activa con rol `administrador` entra a rutas privilegiadas. El rol no habilita lectura indiscriminada: cada endpoint limita recurso, propósito y campos, y el módulo dueño decide si la operación sobre ese recurso está permitida. No se introduce una jerarquía de roles nuevos.

| Acción | Actor permitido | Frontera y datos | Estado actual / brecha |
|---|---|---|---|
| Consulta/exportación de auditoría | Administrador activo | Periodo/filtros obligatorios; página/cursor ligado a actor y criterios; proyección minimizada; un evento por petición, fuera del propio conjunto leído/exportado. | Implementado para ledger local en #228. Cobertura solo para productores que ya escriben `evento_auditoria_local`. |
| Consulta admin de reserva/finanzas | Administrador activo | Proyección read-only, identificadores mínimos y vínculos retirados null; sin operaciones financieras ni carga de contenido privado. | Corte #226; reclamos/finanzas reutilizados en #222/#224. |
| Resolver reclamo o moderar reseña | Administrador activo mediante contrato del módulo dueño | Motivo estructurado, idempotencia/correlación e historial del dueño; no mutar tablas ajenas directamente. | Reclamo #222 y moderación #220. |
| Revisar/revocar elegibilidad sintética | Administrador activo mediante M03 | KYC/KYB separados, motivo estructurado, auditoría y datos minimizados. | #200. No equivale a gobierno de cuenta. |
| Bloquear/desbloquear cuenta | Administrador activo; Backend revalida actor y sujeto bajo bloqueo de cuenta compartido con las operaciones protegidas. | Actor, cuenta interna, acción, resultado, motivo estructurado, fecha y correlación; evento append-only. Revoca sesiones y tokens normales, no cancela reservas ni toca fondos. No expone RUT ni contacto. | **Regla ratificada CU-43:** no publicar/cotizar/reservar; nueva autenticación restringida para consultar reservas propias y ejecutar solo acciones ya autorizadas para relaciones existentes. Desbloquear requiere una sesión nueva. |
| Reportes de gestión locales | Administrador activo | Agregados de lectura con periodo obligatorio, TZ IANA, inicio inclusivo/fin exclusivo, máximo 31 días, CLP y aviso de ensayo; sin PII y sin mutar/conciliar. Una auditoría por petición. | **Catálogo parcial ratificado CU-45:** reservas creadas en periodo agrupadas por estado vigente al generar; hechos fake financieros confirmados separados y obligaciones pendientes separadas. No reconstruye estados pasados ni calcula comisión/liquidación. |

No se añade un bypass de administrador a conversaciones, archivos, evidencia KYC, pagos o reservas. Cuando se requiera consulta privilegiada, debe existir un endpoint/contrato read-only específico y dejar un evento de acceso mínimo. No se exportan correos, RUT, cuerpos de mensajes, tokens, archivos ajenos ni payloads de proveedores. La baja conserva referencias conforme a la matriz de privacidad; nunca reconstruir un identificador retirado.

## Cobertura de auditoría actual y faltante

`evento_auditoria_local` es distinto del historial de dominio, outbox y logs. Su proyección autorizada es: evento, fecha, actor técnico nullable, recurso nullable/estructurado, acción, resultado, motivo estructurado y correlación. El rol de ejecución puede insertar/consultar según su tarea; no modifica ni elimina auditoría. RNF-043 exige cinco años desde cada evento, ya ratificados. Los cinco años no se extienden automáticamente a otras tablas. Los grants locales append-only no prueban bloqueo ante superusuario o alteración de infraestructura: RNF-017 productivo sigue pendiente.

| Productor/capacidad existente | Evidencia y ownership | Tratamiento para M11 |
|---|---|---|
| Cambio/recuperación de credenciales y reapertura/cancelación de su intención durable | M01, PR #199/#198; migraciones V22/V27 | Reutilizar; no crear duplicado. |
| Solicitudes de derechos y ejecución de baja | M02, #193/#195; política ratificada | Reutilizar; eventos mínimos; las bajas no se declaran anonimizadas mientras persistan vínculos. |
| Revisión/revocación KYC | M03, #201/#200 | Reutilizar evento estructurado del owner M03. |
| Moderación de reseña | M09, #221/#220 | Reutilizar su evento auditado y reporte/history del owner. |
| Resolución de reclamo | M10, #223/#222 | Reutilizar evento estructurado; la resolución histórica no reemplaza decisión financiera. |
| Lectura de reserva y finanzas; lectura/export del ledger | M11 slices #227/#226 y #229/#228 | Reutilizar una auditoría mínima por petición; no un evento por fila. |
| Decisión/captura/liberación financiera fake | M10, #225/#224 | Historial financiero persistente existe. **Verificar e integrar cobertura de acción admin en el catálogo auditado**, sin copiar el payload ni el resultado completo. |
| Bloqueo/desbloqueo e informes agregados | RQF-178–185/212/236, CU-43/CU-45 | Subentrega local autorizada: gobierno aislado de baja/KYC; reportes por módulo owner; agregar solo los hechos definidos abajo y auditar una vez por petición. |
| Publicación/tarifa, reserva/transiciones, contratos y operación | Owners M04–M08 | Sus historiales de dominio no equivalen por sí solos a auditoría admin. #114 debe verificar cada acción privilegiada existente y anotar si se audita, se registra solo como hecho de dominio o queda fuera por no tener mutación admin. No insertar por cada lectura o fila. |

El catálogo auditado mínimo cubre acciones privilegiadas que leen o cambian información: actor autenticado, acción, recurso, resultado, motivo enum, instante del backend y correlación. Actor/recurso pueden quedar null cuando la baja haya retirado el vínculo. No guardar texto libre, correo, RUT, tokens, datos financieros completos ni cuerpos/respuestas. Un reintento lógico conserva idempotencia donde la operación muta; una consulta/export de colección registra un evento por solicitud.

## Outbox: ownership y contrato local

| Mecanismo existente | Propósito/owner | No confundir con |
|---|---|---|
| `outbox_evento_local` | Intención durable M01 para aviso de cambio de credencial, con reintentos, fallo terminal y recuperación administrativa; 30 días después del cierre terminal, según decisión ratificada. | Un outbox universal ni el ledger de auditoría. |
| `aviso_local` y ciclos de entrega | Avisos sintéticos M09 dirigidos a Mailpit, deduplicados por evento/destinatario; política local de ocho intentos/ciclo y 30 días tras entrega/fallo terminal cuando aplica. | Un publicador de todos los eventos de dominio. SMTP puede duplicar ante resultado incierto. |
| Inbox de pagos fake | Recepción y deduplicación de eventos entrantes del adaptador fake M06; conciliación tras reinicio. | Outbox de eventos salientes ni pago real. |
| `evento_auditoria_local` | Ledger de acciones y accesos privilegiados; retención de 5 años. | Cola de entrega o log técnico. |

El diseño general `outbox_evento` de #112–#114 solo se implementa para eventos que requieren entrega asíncrona o colaboración durable. El agregado owner escribe el envelope mínimo en la misma transacción de su hecho de dominio; M11 aporta contrato/worker, no escribe tablas ajenas para crear el hecho. Campos base propuestos: `event_id` estable, `aggregate_type/id`, `event_type`, `schema_version`, `occurred_at`, `correlation_id`, clave de deduplicación, estado/lease/intento y payload versionado mínimo. No incluir PII libre, secretos, tokens o payloads completos.

El dispatcher reclama mediante lease recuperable y confirma resultado/offset; consumidores hacen dedup persistente bajo su transacción. Garantía: entrega al menos una vez, no exactly-once; caída después del efecto externo puede duplicar el envío. `espacigo_runtime` no obtiene UPDATE/DELETE sobre auditoría; el worker podrá actualizar solo estado técnico de outbox según grants revisados. No se mantiene una transacción abierta durante red. Outbox pendiente/en procesamiento no se purga; terminal sigue la retención del evento/owner y una finalidad declarada; la regla 30 días de credenciales/M09 no se generaliza a otros eventos. Un evento outbox no hereda los cinco años del audit.

La propuesta evita migrar a un duplicado universal las tablas especializadas existentes. #112 debe confirmar el esquema compartido y, evento por evento, si se adopta el envelope común o el owner outbox ya existente; #113 migra solo tras esa revisión; #114 integra producers/workers en cambios incrementales; #117 prueba dedupe/crash/lease/retry y grants con PostgreSQL aislado.

Los catálogos diferidos no entran a esta consolidación: `campana` y `derecho_reporte` M04 siguen sujetos a MAP-07; orden/promoción M06 y NPS M09 siguen sin finalidad/entitlement aprobado. Outbox disponible no autoriza marketing, ventas ni NPS.

## Gaps y dependencia mínima para completar M11

| Paquete existente | Brecha que queda y alcance local mínimo | Dependencia registrada | Observación |
|---|---|---|---|
| #111 ADMIN-ARCH-01 / #232 LOCAL-ADMIN-ARCH-01 | Diseñar y ratificar matriz de permisos, auditoría/outbox y alcance local CU-43/CU-45. | #230 aceptada; #19 CORE-ARCH cerrado. CU-43/CU-45 ratificadas para alcance local; el issue nativo #111 conserva abierto su bloqueo original #110. | Diseño documentado; #232 no cierra #111. #233 ejecuta el slice local aprobado. |
| #112 ADMIN-DB-01 | Catalogar campos/acciones de audit; cerrar contrato del outbox y retención por owner; excluir catálogos diferidos. | Diseño #232 y CORE-DB #20/#21 cerrados. | No inventar plazo común de outbox. |
| #113 ADMIN-DB-02 | Migración incremental del outbox común solo si #112 lo justifica; grants separados; no renombrar tablas existentes. | #112; CORE-DB #21 cerrado. | No modificar migrations aplicadas; integración con DB desechable. |
| #114 ADMIN-BE-01 | Writer/dispatcher durable y auditoría de acciones privilegiadas faltantes, llamados por APIs/owners. | #113; CORE-BE #22 cerrado. | Exactly-once, GCP y promesa de inmutabilidad fuera. |
| #115 ADMIN-BE-02 | Gobierno general, cobertura de acciones privilegiadas y reportes requeridos según alcance restante. #233 solo cubre bloqueo CU-43 y dos reportes CU-45 locales. | #114; diseño #232 completado; alcance local de #233 aprobado. | No declarar completa #115 ni CU-45; el outbox general y el catálogo de reportes quedan pendientes. |
| #116 ADMIN-API-01 | Rutas versionadas faltantes de gobierno/informes; conservar audit read/export #228 y contratos existentes. | #115; CORE-API #23 cerrado. | Admin auth en Backend; owner auth/resource-specific. |
| #117 ADMIN-TEST-01 | Completar matriz de autorización, productores/audit atómico, duplicados/restarts y retención del mecanismo local elegido. | #116; CORE-TEST #24 cerrado. | Reutilizar suites aceptadas, no repetirlas sin un cambio afectado. |
| #118 ADMIN-MOCK-01 | Integrar controles de las brechas restantes de administración; #233 aporta bloqueo/reporte local en mock. | #117; CORE-ENV #25 cerrado. | #233 no completa el mock general ni M11; no dashboard final ni GCP. |

Las Issues #112–#118 siguen siendo la descomposición DB→Backend→API→test→mock de los criterios generales; #233 agrega solo una subentrega funcional local trazable, sin duplicar fases. El diseño #232 queda concluido para su alcance y no cierra ADMIN-ARCH-01 si #110 o los criterios del padre siguen incompletos; #233 tampoco cierra #115–#118. El Project conserva visibles estos estados.

## Implementación local en curso — #233

La Issue #233 es hija de #111. Su vínculo de bloqueo por #232 se retiró después de ratificar las decisiones de producto; #232 queda abierta En revisión por los ajustes documentales incorporados al PR. #233 incluye V41, Backend/API/OpenAPI, modo de sesión restringida, dos reportes agregados read-only, mock y pruebas. Sus resultados y límites se registran en [`evidence/local-admin-01c-20261009.md`](evidence/local-admin-01c-20261009.md). Su aceptación humana queda pendiente.

## Decisiones ratificadas y límites

CU-43 queda ratificado para prototipo sintético: bloquear y desbloquear requiere administrador activo, motivo estructurado, fecha, correlación y auditoría; revocar sesiones/tokens normales en la misma transacción. Se prohíben nuevas publicaciones, cotizaciones, reservas y administración de la cuenta bloqueada. Se permite una autenticación nueva en modo restringido, solo para reservas propias/historial y acciones existentes sobre esas reservas que el módulo dueño ya autorice (firma, check-in/out, mensajes, reclamos, cancelación, pago o devolución cuando estados/plazos lo permitan). El Backend vuelve a comprobar cuenta, sesión, participante, estado y operación en cada petición. No cancela, borra, libera ocupaciones ni altera contratos/fondos. Desbloquear no restaura credenciales ni sesiones revocadas. Este estado es distinto de la baja de privacidad, revocación KYC y bloqueo temporal por intentos de login.

CU-45 queda ratificado parcialmente con dos reportes admin read-only. **Reservas:** número de reservas creadas en el intervalo, agrupadas por el estado actual al instante de generación; no es una reconstrucción histórica. **Finanzas fake:** por separado, cobros de arriendo, devoluciones, autorizaciones de garantía, capturas y liberaciones confirmados durante el intervalo, más obligaciones pendientes al generar. Nunca se suma garantía autorizada a ingreso, importe previsto a confirmado ni reintento a una nueva operación. Ambos reportes exigen inicio inclusivo, fin exclusivo, zona IANA explícita, máximo 31 días, fecha de generación, CLP y `ENSAYO LOCAL — SIN MOVIMIENTOS REALES`; excluyen identificadores personales, contactos, texto libre y secretos. Consultar no muta ni reconcilia y genera una auditoría mínima por petición. Esta aceptación no completa CU-45 ni el catálogo general de reportes.

Quedan pendientes cobertura transversal de productores y permisos de todas las acciones M11, contrato/outbox general por owner, gobierno administrativo fuera del bloqueo definido, catalogación general de reportes, RNF-017 productivo, proveedores, respaldos externos no inventariados y GCP. No se cierran #111–#118 ni #185/#40 con este diseño.
