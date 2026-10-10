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
| Bloquear/desbloquear cuenta | Administrador activo; Backend revalida actor y sujeto bajo bloqueo de cuenta compartido con las operaciones protegidas. | Actor, cuenta interna, acción, resultado, motivo estructurado, fecha y correlación; evento append-only. Revoca sesiones y tokens normales, no cancela reservas ni toca fondos. No expone RUT ni contacto. | **Regla ratificada CU-43:** no publicar/cotizar/reservar; nueva autenticación restringida para consultar reservas propias y ejecutar solo acciones ya autorizadas para relaciones existentes. Allow-list alineada a los patrones de `cmd/api/main.go`, incluidos `GET .../disputes/{id}/history` y `GET .../reservations/{id}/evidence/{id}`; el módulo dueño sigue imponiendo participante. Desbloquear requiere una sesión nueva. |
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
| Decisión/captura/liberación/conciliación financiera fake | M10, #225/#224 | Brecha confirmada: el historial financiero persiste, pero la decisión admin y las operaciones admin de captura/liberación/conciliación no insertaban `evento_auditoria_local` en la transacción dueña. LOCAL-ADMIN-01D agrega solo esas tres acciones; la autorización iniciada por el arrendatario y los vencimientos automáticos permanecen como hechos de su módulo, no acciones admin. |
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

La auditoría es síncrona y atómica; no necesita dispatcher. Los usos locales de entrega ya tienen estructuras especializadas. Para esta línea base no hay un productor autorizado que justifique un outbox universal: un futuro productor debe demostrar consumidor asíncrono/durable, owner de cada transición, deduplicación, recuperación y retención propia antes de proponer DDL compartido. Un sistema de publicidad/promociones/NPS sigue diferido.

Si una futura decisión autoriza un producer que requiere outbox común, el dispatcher reclamaría con lease recuperable y confirmaría resultado/offset; cada consumer deduplicaría bajo su transacción. La garantía sería al menos una vez, no exactly-once; caída tras un efecto externo puede duplicar el envío. `espacigo_runtime` no obtiene UPDATE/DELETE sobre auditoría; el worker solo actualizaría estado técnico de outbox según grants revisados. No se mantendría una transacción abierta durante red. Pendiente/en procesamiento no se purgaría; terminal seguiría la retención del evento/owner y su finalidad; la regla de 30 días de credenciales/M09 no se generaliza. Un evento outbox no heredaría los cinco años del audit. Estos son requisitos para un diseño futuro, no una afirmación de que #112–#114 tengan hoy un publicador universal.

La decisión del corte #112–#114 es evitar migrar a un duplicado universal de los outboxes especializados. V43 solo amplía la allow-list de claves idempotentes del ledger append-only para tres acciones financieras admin; reusa el grant INSERT de `espacigo_runtime`, el índice único por recurso/acción/clave y el plazo RNF-043 de cinco años exclusivamente para auditoría. No modifica ni crea una tabla outbox. Las pruebas del productor financiero verifican atomicidad, persistencia e idempotencia en PostgreSQL aislado; la recuperación de envíos sigue probándose en sus outboxes dueños.

## Clasificación de criterios originales #112–#114

| Issue / criterio | Clasificación | Evidencia o brecha concreta |
|---|---|---|
| #112 — ledger de auditoría, proyección minimizada y retención del hecho | **Aceptado localmente** como diseño/base y consumo en #228 | V22/V23+; ledger append-only con retención ratificada de cinco años, filtros/exportación minimizados. #228 aporta consulta, no toda la cobertura de productores. |
| #112 — cobertura de cada acción privilegiada existente | **Brecha funcional**, cubierta por este corte para finanzas fake | Inventario finito en esta decisión; se agrega decisión, captura/liberación y conciliación admin. Los demás productores listados arriba ya escriben auditoría. |
| #112 — envelope universal y catálogo de productores asíncronos | **Decisión resuelta para el alcance local actual**: no añadir esquema universal sin consumidor que lo requiera | Credenciales usan `outbox_evento_local`; avisos M09 usan `aviso_local`; pagos fake usan inbox/resultados de su owner. Sus límites, deduplicación y retenciones son distintos. No generalizar una retención. |
| #112 — campañas/promociones/NPS | **Decisión pendiente / diferido** | MAP-07 no habilita productores, finalidades, destinatarios ni retención. No implementar. |
| #113 — DDL/grants del outbox compartido | **Aceptado localmente como no aplicable a los productores vigentes; brecha funcional futura si aparece un caso justificado** | No se crea una tabla genérica. V43 añade solo restricciones de persistencia para idempotencia de los nuevos hechos auditados. Los grants runtime existentes ya permiten INSERT/SELECT del ledger sin UPDATE/DELETE. |
| #114 — auditoría transaccional de mutaciones admin locales | **Brecha funcional**, cerrada solo para las mutaciones financieras identificadas por este PR | Evento de auditoría entra en la transacción de decisión/operación/conciliación; claves derivadas de hash, actor/reserva/acción/código/correlación mínimos; replays no duplican. |
| #114 — publicador, worker y consumo de eventos de negocio universal | **Decisión resuelta para productores actuales**: no hay publicación transversal justificada | Las entregas asíncronas existentes mantienen sus workers/inbox en propiedad del módulo y evidencia propia. No se afirma exactly-once, inmutabilidad productiva ni publicación a nube. |
| #114 — crash/restart/lease/reintento de outbox | **Aceptado localmente por mecanismo y producer existentes; brecha de evidencia solo si un cambio los afecta** | #198/#199 cubren outbox de credenciales, #220 avisos Mailpit y #74 fake payments. Esta subentrega no modifica esos flujos y reutiliza sus pruebas. |
| Cobertura de auditoría/outbox para aceptación integral LOCAL-1 | **Brecha de evidencia pendiente en L5**, no un cambio que justifique otra tabla en este PR | Esta conciliación y sus pruebas acotadas no sustituyen `LOCAL-QA-03`; el recorrido integral M01–M11 todavía debe ejecutarse y aceptarse antes de declarar LOCAL-1 completo. |
| Pub/Sub/BigQuery, almacenamiento inmutable productivo, proveedores externos | **Integración externa** | Fuera del cierre local y de esta entrega; sin GCP. |

La clasificación no acepta #112/#113/#114 como Issues generales completas: #112 conserva el catálogo de decisiones diferidas; #113/#114 conservan criterios de cobertura general de M11 y cualquier nuevo productor definido posteriormente. No se cierran los Issues ni se cambia el bloqueo nativo #111 ← #110.

Los catálogos diferidos no entran a esta consolidación: `campana` y `derecho_reporte` M04 siguen sujetos a MAP-07; orden/promoción M06 y NPS M09 siguen sin finalidad/entitlement aprobado. Outbox disponible no autoriza marketing, ventas ni NPS.

## Gaps y dependencia mínima para completar M11

| Paquete existente | Brecha que queda y alcance local mínimo | Dependencia registrada | Observación |
|---|---|---|---|
| #111 ADMIN-ARCH-01 / #232 LOCAL-ADMIN-ARCH-01 | Diseñar y ratificar matriz de permisos, auditoría/outbox y alcance local CU-43/CU-45. | #230 aceptada; #19 CORE-ARCH cerrado. CU-43/CU-45 ratificadas para alcance local; el issue nativo #111 conserva abierto su bloqueo original #110. | #232 aceptada/cerrada por su alcance de diseño tras PR #234; no cierra #111. #233 ejecuta el slice local aprobado. |
| #112 ADMIN-DB-01 | Catálogo de audit por producer, decisión de outbox por owner/consumer, retención por finalidad y exclusión de catálogos diferidos. | Diseño #232; CORE-DB #20/#21; LOCAL-ADMIN-01D incorpora la brecha financiera observada. | Decisión de no tener tabla universal para productores actuales queda documentada; campañas/promociones/NPS difieren pendientes. No cerrar el Issue general. |
| #113 ADMIN-DB-02 | Migración incremental y grants únicamente cuando haya DDL local justificado; no renombrar estructuras existentes. | No se necesita tabla/outbox común para productores actuales; V43 extiende el constraint append-only del audit y reusa permisos runtime. | V43 probado desde base vacía con rol runtime. Mantener abierta #113 por criterios originales más amplios/futuro producer; no crear DDL por requisito nominal. |
| #114 ADMIN-BE-01 | Writer transaccional para acciones privilegiadas; publicador/consumer solo por entrega asíncrona justificada. | Productores existentes de credenciales, avisos, pagos inbox, privacidad, KYC, reputación, reclamo y gobierno tienen owner; este corte suma las acciones financieras que faltaban. | No hay publicador transversal justificado hoy; conservar pruebas de cada worker owner. Exactly-once, GCP e inmutabilidad productiva fuera. Issue sigue abierta. |
| #115 ADMIN-BE-02 | Gobierno general, cobertura de acciones privilegiadas y reportes requeridos según alcance restante. #233 solo cubre bloqueo CU-43 y dos reportes CU-45 locales. | #114; diseño #232 completado; alcance local de #233 aprobado. | No declarar completa #115 ni CU-45; el outbox general y el catálogo de reportes quedan pendientes. |
| #116 ADMIN-API-01 | Rutas versionadas faltantes de gobierno/informes; conservar audit read/export #228 y contratos existentes. | #115; CORE-API #23 cerrado. | Admin auth en Backend; owner auth/resource-specific. |
| #117 ADMIN-TEST-01 | Completar matriz de autorización, productores/audit atómico, duplicados/restarts y retención del mecanismo local elegido. | #116; CORE-TEST #24 cerrado. | Reutilizar suites aceptadas, no repetirlas sin un cambio afectado. |
| #118 ADMIN-MOCK-01 | Integrar controles de las brechas restantes de administración; #233 aporta bloqueo/reporte local en mock. | #117; CORE-ENV #25 cerrado. | #233 no completa el mock general ni M11; no dashboard final ni GCP. |

Las Issues #112–#118 siguen siendo la descomposición DB→Backend→API→test→mock de los criterios generales; #233 agrega solo una subentrega funcional local trazable, sin duplicar fases. El diseño #232 quedó aceptado/cerrado por su alcance y no cierra ADMIN-ARCH-01 si #110 o los criterios del padre siguen incompletos; #233 tampoco cierra #115–#118. Las Issues están cerradas por sus alcances; la sincronización de sus campos Project queda pendiente porque la API GraphQL no permitió acceder al tablero.

## Implementación local aceptada — #233

La Issue #233 es hija de #111 y quedó aceptada/cerrada por este corte, sin completar M11. Su vínculo de bloqueo por #232 se retiró después de ratificar las decisiones de producto. #233 incluye V41, Backend/API/OpenAPI, modo de sesión restringida, dos reportes agregados read-only, mock y pruebas. PR #234 agregó CORS en denegaciones del middleware, completó las rutas reales de evidencia/historial y la exportación/minimización en baja; V42 hace nullable el actor solo para retirar ese vínculo. La aceptación de #233 se limita a este corte; la retención del historial sigue pendiente de ratificación y no se le asigna plazo. Los resultados y límites se registran en [`evidence/local-admin-01c-20261009.md`](evidence/local-admin-01c-20261009.md).

## Decisiones ratificadas y límites

CU-43 queda ratificado para prototipo sintético: bloquear y desbloquear requiere administrador activo, motivo estructurado, fecha, correlación y auditoría; revocar sesiones/tokens normales en la misma transacción. Se prohíben nuevas publicaciones, cotizaciones, reservas y administración de la cuenta bloqueada. Se permite una autenticación nueva en modo restringido, solo para reservas propias/historial y acciones existentes sobre esas reservas que el módulo dueño ya autorice (firma, check-in/out, mensajes, reclamos, cancelación, pago o devolución cuando estados/plazos lo permitan). El Backend vuelve a comprobar cuenta, sesión, participante, estado y operación en cada petición. No cancela, borra, libera ocupaciones ni altera contratos/fondos. Desbloquear no restaura credenciales ni sesiones revocadas. Este estado es distinto de la baja de privacidad, revocación KYC y bloqueo temporal por intentos de login.

CU-45 queda ratificado parcialmente con dos reportes admin read-only. **Reservas:** número de reservas creadas en el intervalo, agrupadas por el estado actual al instante de generación; no es una reconstrucción histórica. **Finanzas fake:** por separado, cobros de arriendo, devoluciones, autorizaciones de garantía, capturas y liberaciones confirmados durante el intervalo, más obligaciones pendientes al generar. Nunca se suma garantía autorizada a ingreso, importe previsto a confirmado ni reintento a una nueva operación. Ambos reportes exigen inicio inclusivo, fin exclusivo, zona IANA explícita, máximo 31 días, fecha de generación, CLP y `ENSAYO LOCAL — SIN MOVIMIENTOS REALES`; excluyen identificadores personales, contactos, texto libre y secretos. Consultar no muta ni reconcilia y genera una auditoría mínima por petición. Esta aceptación no completa CU-45 ni el catálogo general de reportes.

Quedan pendientes cobertura transversal de productores y permisos de todas las acciones M11, contrato/outbox general por owner, gobierno administrativo fuera del bloqueo definido, catalogación general de reportes, RNF-017 productivo, proveedores, respaldos externos no inventariados y GCP. No se cierran #111–#118 ni #185/#40 con este diseño.
