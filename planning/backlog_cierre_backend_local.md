# Backlog propuesto para cerrar el backend local

Fecha inicial: 2026-10-07; conciliado con `main` #199 el 2026-10-08. Complementa [el plan de cierre](plan_cierre_backend_local.md). Los paquetes son planificación local; GitHub Issues/Projects conserva los estados operativos.

## Convenciones

- `LOCAL-*` identifica paquetes de cierre. Tipo `VERTICAL` agrupa las etapas ARCH → DB → BE → API → TEST → MOCK de las tarjetas originales para evitar un PR por etapa. No sustituye la trazabilidad de esas tarjetas.
- Reutilizar las Issues originales; crear un hijo local cuando sea necesario distinguir criterios locales de proveedores/GCP o ampliar una entrega ya cerrada. LOCAL-KYC-01 es Issue #200 y subissue de #45, relacionada con #48–#50. No importar de nuevo el backlog Hermes.
- LOCAL-PLAN-01, LOCAL-CORE-01 y LOCAL-AUTH-01 cuentan con entregas aceptadas (#181/#180/#182). LOCAL-BOOK-01 también fue aceptada (#177). LOCAL-PRIV-01 permanece parcial con cortes #186/#192/#194/#196/#198 aceptados; #202 (LOCAL-M02-01) añade foto PNG sintética y referencia de cobro fake, sin cerrar el padre. LOCAL-KYC-01 (#200) fue aceptada y cerrada tras PR #201; KYC para nuevas reservas personales de anfitrión y arrendatario está implementado. El gate de publicación queda como dependencia explícita de LOCAL-LIST/M04, porque su endpoint aún no existe. El resto de los paquetes no cambia por sí mismo Issues ni Projects.
- Una dependencia significa contrato/entrega aceptada y disponible en la rama base. Las puertas D-* del plan bloquean solo las operaciones que necesitan su decisión; no esperar a resolver toda la lista de decisiones para avanzar trabajo independiente.
- Toda tarjeta VERTICAL incluye Backend/API → pruebas → tarjeta final de mock de su módulo. El mock no queda `ready` antes de existir sus APIs y pruebas necesarias.
- Aceptación común: código en PR revisable, comportamiento/errores documentados, permisos efectivos, evidencia del commit y sin secretos; aceptación humana y merge para `Hecho`. Conservar volumen y migraciones previas.
- Los RNF aplicables se extraen de referencias/contratos al convertir los paquetes en Issues. No atribuir un número de requisito a una decisión nueva sin comprobar su fuente.

## L0 — Conciliación y primera entrega

### LOCAL-PLAN-01 — Conciliar requisitos y entregas locales

- **Módulo/tipo/estado:** transversal / ARCH-DOC / `ready`.
- **Objetivo y motivo:** definir el trabajo que falta realmente; fuentes ES1/ES2, M01–M11, Issues #17–120 y subentregas aceptadas.
- **Alcance:** matriz RQF/RNF/CU/HU → módulo → criterio local → código/PR/evidencia → brecha → Issue; conciliación de las 43 entidades; distinguir fakes, obligaciones externas, ideas fuera de alcance y estados históricos.
- **Fuera:** implementar, cerrar criterios sin evidencia o decidir requisitos nuevos unilateralmente.
- **Dependencias:** ninguna. **Desbloquea:** LOCAL-DEC-01, LOCAL-CORE-01 y LOCAL-BOOK-01.
- **Aceptación:** todo requisito tiene clasificación justificada; cada brecha local tiene owner y tarea; relacionar Issues existentes sin duplicarlas; estimación solo tras identificar trabajo restante.
- **Verificación prevista:** revisión documental de cobertura, referencias y grafo sin ciclos; no ejecutar suites de aplicación.
- **Riesgo:** confundir aceptación parcial con módulo completo o perder obligaciones al excluirlas como externas.

### LOCAL-DEC-01 — Preparar y ratificar decisiones pendientes por grupo

- **Módulo/tipo/estado:** transversal / ARCH / `todo`.
- **Objetivo y motivo:** resolver DB02-09 y reglas faltantes de CU-07–14/29–42/43–52 sin inventar políticas; usar grupos D-* del plan.
- **Alcance:** presentar fuente, recomendación, alternativa, impacto y operaciones bloqueadas; registrar ratificación por grupo. Reutilizar las decisiones aceptadas de identidad, tarifas, tiempo y cancelación local.
- **Fuera:** proveedores, efectos jurídicos/financieros reales, nuevas funcionalidades o decisión de frontend definitivo.
- **Dependencias:** LOCAL-PLAN-01. **Desbloquea:** criterios de AUTH/PRIV/KYC/LIST/BOOK/CONT/OPS/COMM/DIS/ADMIN que requieran la decisión ratificada.
- **Aceptación:** cada decisión tiene estado, actor y alcance; los pendientes bloquean solo tareas afectadas; no se declara completa esta tarjeta ni el cierre local con decisiones locales obligatorias pendientes.
- **Verificación prevista:** contrastar CU/RQF y contratos; revisión de impactos, sin pruebas de aplicación.
- **Riesgo:** un bloqueo global de decisiones vuelve a impedir trabajo independiente.

### LOCAL-BOOK-01 — Consolidar invariantes de reservas existentes

- **Módulo/tipo/estado:** M06 / TEST-DOC / `todo`.
- **Objetivo y motivo:** completar evidencia concreta de #77 y aceptación local de #73/#75, CU-22–28/51, sin añadir otra función al prototipo.
- **Alcance:** prueba PostgreSQL directa de `23P01`, traducción de conflicto a servicio/API, dos reservas adyacentes creadas y solape estricto con rollback; matriz de criterios ya satisfechos.
- **Fuera:** proveedor, nuevas políticas, cerrar padres generales pendientes o cambiar constraints si ya satisfacen el requisito.
- **Dependencias:** LOCAL-PLAN-01 y registro de subentrega/dependencias locales de código fusionado. Las dependencias generales #77 → #75 → #73 → #65 permanecen visibles. **Desbloquea:** consolidación de descubrimiento/reserva general y parte de LOCAL-QA-03.
- **Aceptación:** adyacentes confirman ambas; solape deja una sola ocupación válida; error público no filtra SQLSTATE; no hay reserva/historial parcial; un PR agrupa evidencia y matriz.
- **Pruebas previstas:** PostgreSQL desechable con rol runtime, transacciones/barreras controladas, servicio/API y regresión enfocada.
- **Riesgo:** confundir horarios ofrecidos con reservas persistidas o probar solo el mapper de errores.

## L1 — Fundamentos y confianza

### LOCAL-CORE-01 — Completar fundamentos locales comunes

- **Módulo/tipo/estado:** transversal, base M11 / DB-BE-API / `todo`.
- **Objetivo y motivo:** soportar persistencia y contratos generales según CORE-ARCH/CORE-BE y #112–114, sin duplicar infraestructura por módulo.
- **Alcance:** mapa físico/ownership; transacciones; separación de rutas/casos de uso de los habilitadores de fixtures; selección explícita de adaptadores fake; metadata/almacenamiento privado; auditoría y outbox atómicos con payload mínimo; contrato de reintentos/leases.
- **Fuera:** GCP, microservicios, proveedor real, sustituir código aceptado completo o construir toda M11 antes de M01.
- **Dependencias:** LOCAL-PLAN-01; contratos vigentes y decisiones aplicables de datos/auditoría. **Desbloquea:** LOCAL-QA-01 y los módulos que usan storage/eventos.
- **Aceptación:** migraciones incrementales trazadas; evento e intención de envío se guardan con el cambio de dominio; dominio sin importación de herramientas de ensayo; secreto no aparece en logs/respuestas; SQL de cada módulo respeta owner.
- **Pruebas previstas:** rollback/evento atómico, permisos runtime, acceso privado y selección de adaptadores; PostgreSQL aislado.
- **Riesgo:** convertir las tablas específicas de ensayo en modelo global sin conciliación o producir ciclos entre módulos.

### LOCAL-QA-01 — Unificar ejecución reproducible de pruebas locales

- **Módulo/tipo/estado:** transversal / TEST / `todo`.
- **Objetivo y motivo:** completar CORE-TEST-01 con ejecución real, evitando suites verdes con integraciones omitidas.
- **Alcance:** un comando para DB desechable, migración y rol runtime; Go/contratos/mock e integraciones exigidas; fixtures deterministas; reporte de pruebas ejecutadas/omitidas; limpieza verificable. Reutilizar scripts actuales.
- **Fuera:** pruebas cloud, sandbox, ejecutar suite completa por edición documental o umbral arbitrario de cobertura.
- **Dependencias:** LOCAL-CORE-01. **Desbloquea:** gates de entregas y LOCAL-QA-02/03.
- **Aceptación:** falta de DSN/servicio requerido falla el gate, no lo convierte en éxito por `Skip`; comando reproducible documentado; ninguna limpieza toca la DB persistente de desarrollo.
- **Pruebas previstas:** comprobar preparación, fallo por precondición ausente y limpieza; futuras suites del proyecto.
- **Riesgo:** utilizar el DSN de desarrollo por defecto o contabilizar un parseo YAML como validación del contrato.

### LOCAL-AUTH-01 — Completar identidad y credenciales

- **Módulo/tipo/estado:** M01 / VERTICAL / **entrega local aceptada parcialmente** (#182 mediante #183; outbox terminal/recovery #198 mediante #199).
- **Objetivo y motivo:** cerrar alcance local de #26–35, CU-01–06/50 y DB02-09 (RQF-213/217/218).
- **Alcance:** preferencias independientes de roles; historial de hashes/ventana de tres meses ratificada; avisos durables de credenciales; completar matriz de registro, verificación, reemisión, sesión, recuperación, cambio y revocación. Conservar decisiones vigentes de enumeración/TTL.
- **Fuera:** correo real, redefinir reglas aprobadas, tokens en logs o preferencias como permisos.
- **Dependencias:** LOCAL-CORE-01 y D-AUTH ratificada; satisfechas para este corte. **Desbloquea:** LOCAL-PRIV-01, LOCAL-KYC-01 y validación final M01.
- **Aceptación:** recuperación/cambio de clave impiden reutilización según ventana; hash nunca se expone; sesiones idle/absoluto y reemisión mantienen comportamiento; notificación se recupera tras reinicio sin enviar secretos.
- **Pruebas previstas:** fronteras temporales, contraseñas reutilizadas, autorización, consumo/concurrencia de tokens y outbox; finalizar con LOCAL-AUTH-MOCK-01.
- **Riesgo:** comparar hashes salados por igualdad o conservar historial/PII más allá del plazo ratificado.

### LOCAL-PRIV-01 — Completar perfil y derechos de datos

- **Módulo/tipo/estado:** M02 / VERTICAL / **parcial**, con cortes aceptados #186, #192, #194, #196 y #198.
- **Objetivo y motivo:** #37–43 y CU-07–09; pasar de registrar solicitudes a ejecutarlas conforme al contrato local.
- **Alcance:** perfil/foto sintética y cuenta de cobro local según requisitos; estados, actor/motivo, acceso/exportación y supresión/desidentificación con trazabilidad; coordinar referencias y retención con otros módulos.
- **Fuera:** datos bancarios reales, entrega a proveedor o afirmar cumplimiento jurídico; eliminación irreversible indiscriminada de reservas/ledger.
- **Dependencias:** LOCAL-AUTH-01, fundamentos privados/eventos de CORE y D-PRIV. La exportación actual, baja local, retención/replay y recuperación de outbox tienen evidencia aceptada; quedan criterios de los padres. **Desbloquea:** LOCAL-LIST-01 y administración de derechos solo según cada criterio.
- **Aceptación:** solo titular/administrador autorizado accede; una resolución produce el efecto acordado sobre datos o un motivo trazado de conservación/rechazo; exportación acotada; actuación no rompe integridad.
- **Pruebas previstas:** solicitud duplicada, cambio de estado, ownership, datos exportados/suprimidos/conservados y replay; finalizar LOCAL-PRIV-MOCK-01.
- **Riesgo:** cerrar solicitudes sin actuar o borrar evidencias que deben conservarse por política ratificada.

### LOCAL-PRIV-M02-01 — Foto sintética y cuenta de cobro fake (Issue #202)

- **Módulo/tipo/estado:** M02 / subentrega VERTICAL / `En curso`; vinculada a #37–#43, bajo seguimiento de #185/#40.
- **Objetivo:** añadir los dos recursos M02 faltantes que se pueden verificar localmente sin proveedor productivo.
- **Alcance:** PNG fijo generado por Backend y guardado en storage privado; leer/reemplazar/retirar foto propia; referencia `fake-local-v1` con alta/consulta/cambio/revocación, gate KYC sintético efectivo, ZIP, baja y limpieza recuperable. Integrar controles en mock existente.
- **Fuera:** imagen real/carga arbitraria, RUT, cuentas bancarias reales, pagos/transferencias, proveedor real, cierre de padres ni publicación M04.
- **Dependencias:** almacenamiento privado, auth/ownership, elegibilidad local, export ZIP, baja bloqueada/worker recuperable satisfechos por entregas aceptadas; permiso de escritura sincronizado por account lock. No depende de #142 ni desbloquea M04.
- **Aceptación:** migración incremental; privacidad de archivo; aislamiento; reintentos y concurrencia con baja; KYC requerido al crear/cambiar el payout fake; ZIP incluye datos propios; limpieza falla/reintenta; pruebas PostgreSQL desechables y mock.

### LOCAL-KYC-01 — Elegibilidad y ciclo de revisión sintéticos (Issue #200)

- **Módulo/tipo/estado:** M03 / VERTICAL / `En curso`; hija nativa de #45, relacionada con #48–#50.
- **Objetivo y motivo:** completar en local los casos, subsanaciones y resultados fake reutilizando #47/#51; conservar la aprobación sintética por tipo y permitir revocación explícita auditada conforme a la decisión ratificada.
- **Alcance:** V28 incremental; historial append-only; código de corrección compatible con el motivo; reintento idempotente; lista de casos pendientes y rechazados; elegibilidad `kyc`/`kyb` persistente e independiente; revocación admin bajo bloqueo de cuenta; API/OpenAPI y mock existente.
- **Fuera:** documentos reales, RUT, proveedor, consentimiento/retención productivos de #142; no conceder roles. No implementar el ciclo general de publicación M04 ni la reserva comercial general M06 en esta Issue.
- **Dependencias:** identidad/CORE y almacenamiento sintético ya integrados. El tipo de gate está ratificado: cuentas personales requieren KYC; KYB no lo sustituye. La transacción de nueva reserva lo verifica para anfitrión y arrendatario junto a los bloqueos de baja/aprobación. LOCAL-LIST-01 deberá conectar el gate cuando cree el endpoint general de publicar/despublicar; #142 bloquea solo proveedor/datos reales. **Desbloquea:** LOCAL-LIST-01.
- **Aceptación:** aprobar concede solo el tipo indicado; casos nuevos pendientes/rechazados no remueven una concesión; revocar la retira con motivo/auditoría y no altera reservas; una aprobación nueva puede restablecer. Historial estable, aislamiento, reintento y concurrencia probados en PostgreSQL descartable; mock compilado y recorrido validado.
- **Pendientes originales:** notificación de resultados KYC mediante contrato durable de dominio, consentimiento/cuotas M03, retención/supresión integral de expedientes y todo proveedor/dato real permanecen abiertos; no cerrar #45/#48–#50.
- **Riesgo:** interpretar una aprobación fixture como identidad real o asignar el tipo a operaciones comerciales sin un vínculo de negocio definido.

## L2 — Oferta y reserva generales

### LOCAL-LIST-01 — Completar ciclo local de publicaciones

- **Módulo/tipo/estado:** M04 / VERTICAL / `todo`.
- **Objetivo y motivo:** #52–61, CU-15–18; completar borrador → publicación local con controles de negocio.
- **Alcance:** publicar/despublicar y edición autorizada; elegibilidad; ocho categorías/perfiles versionados; galería sintética privada; tarifa/comisión/políticas versionadas conforme al owner ratificado. Reutilizar calendario M06.
- **Fuera:** publicación comercial, archivos reales sin autorización, cambiar categoría semilla o elegir frontend final.
- **Dependencias:** LOCAL-PRIV-01, LOCAL-KYC-01, CORE y D-KYC/LIST/D-BOOK. **Desbloquea:** LOCAL-DISC-01.
- **Aceptación:** publicación requiere datos/estados aprobados; borradores quedan privados; despublicar corta nuevas operaciones según regla sin borrar reservas anteriores; galería conserva autorización y datos validados; snapshots anteriores sobreviven edición.
- **Pruebas previstas:** transición inválida, dueño/tercero, versión de perfil, files sintéticos, permisos y reglas de calendario; finalizar LOCAL-LIST-MOCK-01.
- **Riesgo:** duplicar `ocupacion` o confundir una oferta local sintética con lanzamiento público.

### LOCAL-DISC-01 — Completar búsqueda y cotización de oferta local

- **Módulo/tipo/estado:** M05 / VERTICAL / `todo`.
- **Objetivo y motivo:** #62–68, CU-19–21; reemplazar la dependencia funcional de allowlists por oferta publicada/elegible.
- **Alcance:** filtros existentes, disponibilidad, detalle autorizado, proximidad y paginación; cotización versionada por oferta seleccionada; índices y límites para dataset local acordado.
- **Fuera:** geocodificador real, ranking patrocinado, mapa definitivo o garantía de capacidad cloud.
- **Dependencias:** LOCAL-LIST-01 y LOCAL-BOOK-01. **Desbloquea:** LOCAL-BOOK-02.
- **Aceptación:** oferta despublicada/no elegible no aparece; búsquedas no retienen disponibilidad; total/intervalo/snapshot correctos; paginación y orden documentados; no exponer dirección/coordenada privada por error.
- **Pruebas previstas:** filtros combinados, atributos ausentes, redondeo, fronteras DST, precio/radio inclusivo, páginas y cambio de estado/tarifa; finalizar LOCAL-DISC-MOCK-01.
- **Riesgo:** permitir reserva de precio estimado obsoleto o declarar estable un catálogo que cambia entre páginas.

### LOCAL-BOOK-02 — Completar dominio local general de reservas

- **Módulo/tipo/estado:** M06 / VERTICAL / `todo`.
- **Objetivo y motivo:** #69/#73/#75/#77/#79, CU-22–28/47/51; ciclo general con pago fake/durable ya aceptado.
- **Alcance:** reglas de elegibilidad y cotización, reserva/ocupación atómica, solicitud/aprobación/rechazo, vencimientos y cancelación/refund local; contratos de transición para M07/M08/M10; inbox/conciliación se reutilizan.
- **Fuera:** proveedor real/sandbox #76/#78, políticas financieras nuevas no ratificadas o custodiar fondos.
- **Dependencias:** LOCAL-DISC-01, AUTH/CORE y D-BOOK. **Desbloquea:** LOCAL-CONT-01 y operación posterior.
- **Aceptación:** mismos servicios atienden oferta general local; estados/actores/plazos trazados; cambios de tarifa/elegibilidad y reloj revalidados bajo bloqueo; eventos tardíos no reactivan reserva; reintento no cobra ni devuelve dos veces.
- **Pruebas previstas:** concurrencia, autorización, timeout/reinicio, adyacencia, transiciones y callbacks; finalizar LOCAL-BOOK-MOCK-01. Reutilizar evidencia válida de #170–176.
- **Riesgo:** doble ocupación, dinero simulado no trazado o dependencia circular entre reserva y módulos posteriores.

## L3 — Ejecutar el arriendo

### LOCAL-CONT-01 — Construir contrato y firma simulada

- **Módulo/tipo/estado:** M07 / VERTICAL / `todo`.
- **Objetivo y motivo:** #80–87, CU-29–32; asegurar contenido y resultado antes del uso del espacio.
- **Alcance:** plantilla/versiones, snapshot de partes/condiciones, documento sintético privado/hash, estados por firmante y firma fake; rechazo, vencimiento, callback y recuperación.
- **Fuera:** validez jurídica, firma real, proveedor o documentos personales reales.
- **Dependencias:** LOCAL-BOOK-02, storage/eventos CORE y D-CONT/OPS. **Desbloquea:** LOCAL-OPS-01.
- **Aceptación:** edición de perfil/tarifa no cambia contrato histórico; firmantes y acceso autorizados; callback autenticado según contrato local; finalización idempotente y transiciones de reserva válidas.
- **Pruebas previstas:** contenido/hash, acceso de tercero, firma parcial/final/rechazo, replay y reinicio; finalizar LOCAL-CONT-MOCK-01.
- **Riesgo:** regenerar retrospectivamente el snapshot o representar el fake como firma legal.

### LOCAL-OPS-01 — Construir entrega, recepción y devolución

- **Módulo/tipo/estado:** M08 / VERTICAL / `todo`.
- **Objetivo y motivo:** #88–94, CU-33–34/48; completar el uso del arriendo y sus evidencias.
- **Alcance:** check-in/out, confirmación/objeción, evidencia sintética y ventanas aprobadas; actor, fecha y transiciones de reserva. No cerrar el arriendo solo porque pasó la hora si requiere confirmaciones.
- **Fuera:** geolocalización real o política de daño/garantía no acordada; resolución de disputa pertenece a M10.
- **Dependencias:** LOCAL-CONT-01, BOOK/CORE y D-CONT/OPS. **Desbloquea:** LOCAL-COMM-01 y LOCAL-DIS-01.
- **Aceptación:** acciones por participante/estado/plazo; evidencia privada; reintentos conservan resultado; objeción inicia el recorrido autorizado sin inventar adjudicación automática.
- **Pruebas previstas:** reloj bajo bloqueo, actor incorrecto, confirmaciones/objeciones, repetición y estado final; finalizar LOCAL-OPS-MOCK-01.
- **Riesgo:** concluir una reserva por tiempo sin cumplir confirmación o perder pruebas de entrega.

### LOCAL-COMM-01 — Completar reputación y avisos durables

- **Módulo/tipo/estado:** M09 / VERTICAL / `todo`.
- **Objetivo y motivo:** #95–102, CU-35–38/49; completar reseñas/reportes y envío durable, conservando chat/cursores aceptados.
- **Alcance:** elegibilidad y unicidad de reseña según requisito, moderación/reportes; notificaciones, entregas y reintentos mediante outbox/Mailpit; cobertura de eventos de otros módulos.
- **Fuera:** tiempo real, push, campana interna u otras funciones sin requisito, canales externos reales.
- **Dependencias:** LOCAL-OPS-01, AUTH/CORE y D-COMM. **Desbloquea:** LOCAL-DIS-01 y administración de contenido.
- **Aceptación:** tercero no lee hilo ni evalúa reserva ajena; reseña cumple estado/plazo; destinatario/payload mínimos; un fallo de correo deja intención recuperable y reintento no duplica entrega confirmada.
- **Pruebas previstas:** roles, elegibilidad, reportes, replay, workers y regresión chat/cursores; finalizar LOCAL-COMM-MOCK-01.
- **Riesgo:** reseñar una reserva no ejecutada o considerar un SMTP directo como outbox durable.

## L4 — Resolver, liquidar y administrar

### LOCAL-DIS-01 — Construir resolución local de disputas

- **Módulo/tipo/estado:** M10 / VERTICAL / `todo`.
- **Objetivo y motivo:** #103–106/#108–110, CU-39–42; reclamo, descargo y resolución motivada del caso.
- **Alcance:** estados, partes, pruebas sintéticas privadas, plazos/actor y resolución administrativa; eventos para garantía/liquidación y avisos.
- **Fuera:** decisión jurídica automática, dinero real o política nueva deducida de un ejemplo.
- **Dependencias:** LOCAL-OPS-01, LOCAL-COMM-01, BOOK/CORE y D-DIS/ADMIN. **Desbloquea:** LOCAL-FIN-01.
- **Aceptación:** participantes ven solo su expediente; resolución exige autoridad y motivo; no hay decisiones simultáneas incompatibles ni movimiento duplicado; efectos de reserva trazados.
- **Pruebas previstas:** ownership, plazos, descargo, concurrencia, replay y acceso a evidencia; finalizar parte de LOCAL-DIS-MOCK-01.
- **Riesgo:** resolver disputa sin política ratificada o filtrar evidencia entre cuentas.

### LOCAL-FIN-01 — Completar cierre económico simulado

- **Módulo/tipo/estado:** M10 e integración M06 / DB-BE-API-TEST / `todo`.
- **Objetivo y motivo:** #107/#108–110 y criterios financieros de CU-39–42/47; separar movimiento observado fake de una mera transición de reserva.
- **Alcance:** garantía, ledger/movimientos, comisión y liquidación; refund/compensación bajo reglas aprobadas, evidencia conciliable fake y documento tributario sintético inequívoco; todo con persistencia e idempotencia.
- **Fuera:** emitir documentos legales, dinero real, Escrow/custodia o contabilidad validada.
- **Dependencias:** LOCAL-DIS-01, BOOK, snapshots LIST y D-BOOK/D-DIS. **Desbloquea:** LOCAL-ADMIN-01 y validación completa M10.
- **Aceptación:** sumas coherentes y snapshot de reglas; intentos/resultados observados, no saldo inferido de estado; reintentos/fallos conservan una operación; cierre no pierde trazabilidad; documento marcado simulado.
- **Pruebas previstas:** importes, redondeo, disputa/garantía, timeout y resultado tardío, deduplicación y reconexión; terminar LOCAL-DIS-MOCK-01 tras este paquete.
- **Riesgo:** confundir comisión con fondos de terceros o simular liquidación como pago confirmado real.

### LOCAL-ADMIN-01 — Completar gobierno y administración

- **Módulo/tipo/estado:** M11 / VERTICAL / `todo`.
- **Objetivo y motivo:** #111–118, CU-43–46/52 y requisitos de reportes/accesos vigentes.
- **Alcance:** gobierno de cuentas, moderación, consultas de reserva/pago/disputa, reportes requeridos y auditoría; permisos, filtros y exportaciones mínimas. Reusar infraestructura CORE, no duplicar eventos.
- **Fuera:** panel visual de producción, nuevas métricas/promociones sin requisito o configuración cloud.
- **Dependencias:** LOCAL-FIN-01 y capacidades de PRIV/KYC/LIST/DIS/CORE; D-DIS/ADMIN. **Desbloquea:** LOCAL-CORE-02 y validación final.
- **Aceptación:** cada función administrativa requiere permiso y motivo cuando corresponde; transiciones válidas; exportación minimizada; auditoría registra actor/recurso/correlación sin secretos; actor sin permiso recibe rechazo.
- **Pruebas previstas:** matriz por rol, alcance de consultas, acciones concurrentes, auditoría/append y exportación; finalizar LOCAL-ADMIN-MOCK-01.
- **Riesgo:** un rol administrativo con acceso indiscriminado o auditoría modificable por el rol runtime.

### LOCAL-CORE-02 — Consolidar continuidad del trabajo durable

- **Módulo/tipo/estado:** transversal / BE-TEST / `todo`.
- **Objetivo y motivo:** RNF de continuidad/idempotencia y contratos M11; verificar pagos, firmas, notificaciones y auditoría integrados.
- **Alcance:** leases/reintentos/checkpoints, recuperación tras interrupción, deduplicación y diagnósticos mínimos; correlación sin PII. Pruebas con varios workers locales si el contrato lo requiere.
- **Fuera:** escalado GCP, HA cloud, WAF o infraestructura productiva.
- **Dependencias:** LOCAL-ADMIN-01 y workers de módulos anteriores. **Desbloquea:** LOCAL-QA-02.
- **Aceptación:** trabajo pendiente recuperable; worker detenido no retiene lease indefinidamente; dos workers no finalizan dos veces; errores/correlación permiten diagnosticar sin filtrar secretos.
- **Pruebas previstas:** interrupciones antes/después de cada persistencia/efecto, expiración de lease, replay y continuidad local.
- **Riesgo:** efecto externo simulado realizado pero no persistido o retry ilimitado sin política.

## L5 — Aceptación final local

### LOCAL-QA-02 — Verificar evolución y recuperación de datos

- **Módulo/tipo/estado:** transversal / DB-TEST / `todo`.
- **Objetivo y motivo:** cierre DB de todos los módulos y RNF locales de integridad/continuidad.
- **Alcance:** migración vacía/repetida, upgrade con datos representativos, checksums, backup/restore en otra DB, metadata/blobs privados y permisos restaurados; conciliación final de entidades.
- **Fuera:** probar sobre/borrar el volumen de desarrollo o medir RPO/RTO de GCP.
- **Dependencias:** LOCAL-QA-01 y LOCAL-CORE-02. **Desbloquea:** LOCAL-QA-03.
- **Aceptación:** datos/hashes/relaciones coinciden después de restaurar; cambios conservan snapshots; migraciones aplicadas intactas; resultados y límites de la restauración documentados.
- **Pruebas previstas:** exclusivamente entornos desechables identificados; secuencia completa y actualización con fixture previo.
- **Riesgo:** respaldar solo SQL y perder blobs o usar una base restaurada como prueba de conservación en desarrollo.

### LOCAL-QA-03 — Ejecutar aceptación integral M01–M11

- **Módulo/tipo/estado:** transversal / TEST-MOCK / `todo`.
- **Objetivo y motivo:** demostrar LOCAL-1, casos originales aplicables y RNF verificables localmente.
- **Alcance:** unitarias/integración/contratos; recorrido completo feliz y alternativos; roles/tercero; concurrencia, expiración, errores y fakes; límites/consultas con dataset/máquina declarados; mocks finales de módulos completos.
- **Fuera:** GCP, sandbox, SLA extrapolado o nueva UI.
- **Dependencias:** LOCAL-QA-02, todos los paquetes funcionales y once tarjetas finales MOCK. **Desbloquea:** LOCAL-CLOSE-01.
- **Aceptación:** integraciones exigidas sin omisiones; cuenta → publicación → reserva/pago → firma → operación → reputación o disputa → liquidación, más privacidad/admin, reproducibles; fallos críticos resueltos y evidencia por commit.
- **Pruebas previstas:** comando consolidado y recorridos visuales mínimos; repetir solo lo necesario por correcciones y una regresión final.
- **Riesgo:** dar por completo el backend porque pasa una suite que no ejecutó un módulo o su integración.

### LOCAL-CLOSE-01 — Registrar aceptación y detener desarrollo local

- **Módulo/tipo/estado:** transversal / DOC / `todo`.
- **Objetivo y motivo:** cerrar LOCAL-1 y distinguir pendientes externos del alcance local entregado.
- **Alcance:** matriz final por criterio, evidencias, guía de arranque/uso, estados Issues/Project y seguimientos de proveedores/producto global; revisión humana.
- **Fuera:** desplegar/probar GCP, afirmar cumplimiento legal o cerrar criterios externos por su fake.
- **Dependencias:** LOCAL-QA-03. **Desbloquea:** solamente la posibilidad de solicitar una nueva planificación de nube después de aceptación; no autoriza ejecutarla.
- **Aceptación:** cero brechas funcionales locales obligatorias; exclusiones ratificadas; todos los cambios publicados/fusionados y evidencia trazada; propietario acepta; agente se detiene.
- **Verificación prevista:** revisión de matriz, enlaces, commits y estados; no repetir suites por actualizar documentación.
- **Riesgo:** aceptar un cierre parcial como integral o iniciar nube automáticamente.

## Tarjetas finales de mock por módulo

Estas tarjetas son tareas de aceptación incluidas en el paquete vertical, preferentemente usando/ampliando la tarjeta MOCK original. Si la original está cerrada por un corte parcial, abrir un hijo local con el delta; no invalidar su aceptación anterior. **Estado inicial propuesto de todas: `todo`.**

**Ficha común:** tipo MOCK; objetivo comprobar visualmente CU del módulo; fuera de alcance frontend definitivo/archivos o proveedores reales; dependencia Backend/API y pruebas del paquete; desbloquea aceptación del módulo y LOCAL-QA-03; riesgo falsas conclusiones por sesión/selección/respuesta obsoleta; comprobación funcional en navegador. Todas deben mostrar resultados/datos y errores HTTP, respetar auth/roles, consumir solo la API pública y ejecutarse desde su contenedor HTML/CSS/TS sin framework ni acceso a DB.

| ID / módulo / referencia | Depende de código y pruebas de | Operaciones exactas de aceptación |
| --- | --- | --- |
| LOCAL-AUTH-MOCK-01 / M01 / #35 | LOCAL-AUTH-01 | Registro/duplicado, aceptar versión, verificación/reemisión Mailpit, login/sesión/logout, recuperar/cambiar clave, rechazo de reutilización y permisos/preferencias sin alterar roles. |
| LOCAL-PRIV-MOCK-01 / M02 / #43 | LOCAL-PRIV-01 | Consultar/editar perfil y foto sintética, dato de cobro local; crear/seguir solicitud; exportar datos propios; revisor autorizado resuelve y comprueba efecto/motivo. |
| LOCAL-KYC-MOCK-01 / M03 / #51 | LOCAL-KYC-01 | Crear/consultar caso, gestionar evidencia sintética admitida, revisión/subsanación/aprobación/rechazo/reintento y visualizar elegibilidad; tercero rechazado. |
| LOCAL-LIST-MOCK-01 / M04 / #61 | LOCAL-LIST-01 | Crear/editar borrador con atributos y perfiles, galería sintética, tarifas/políticas y calendario; publicar/despublicar; mostrar impedimentos de elegibilidad y ownership. |
| LOCAL-DISC-MOCK-01 / M05 / #68 | LOCAL-DISC-01 | Buscar oferta publicada con filtros/precio/intervalo/proximidad, paginar, consultar detalle/horarios y cotizar selección; cambios de filtros/espacio invalidan resultados anteriores. |
| LOCAL-BOOK-MOCK-01 / M06 / #79 | LOCAL-BOOK-02 | Solicitar, consultar bandejas/detalle/historial, pago fake/timeout/retry, aprobar/rechazar, vencer y cancelar/refund según política; mismo resultado tras reintento. |
| LOCAL-CONT-MOCK-01 / M07 / #87 | LOCAL-CONT-01 | Consultar documento privado/versionado y firmantes, firmar/rechazar simuladamente, mostrar estado parcial/final/vencido y conflictos; tercero sin acceso. |
| LOCAL-OPS-MOCK-01 / M08 / #94 | LOCAL-OPS-01 | Check-in, confirmación/objeción, evidencia sintética, check-out/devolución, fechas/roles y estado; repetición y acción fuera de ventana muestran resultado/error correcto. |
| LOCAL-COMM-MOCK-01 / M09 / #102 | LOCAL-COMM-01 | Hilo/lectura existente, publicar/consultar reseña elegible, reportar/moderar con rol adecuado y comprobar avisos por Mailpit/estado de entrega autorizado. No añadir un centro de notificaciones. |
| LOCAL-DIS-MOCK-01 / M10 / #110 | LOCAL-DIS-01 y LOCAL-FIN-01 | Reclamo/descargo/evidencia, resolución motivada, movimientos/garantía/liquidación simulada, reintento y documento fiscal marcado sintético; permisos de partes y revisor. |
| LOCAL-ADMIN-MOCK-01 / M11 / #118 | LOCAL-ADMIN-01 | Consultar cuentas/reservas/pagos/disputas, acciones administrativas permitidas, moderación, reportes/exportación mínima y auditoría/correlación; negar rol insuficiente. |

No crear un mock antes de que existan sus operaciones Backend/API. Se pueden usar secciones del mock existente, sin routing complejo, layouts ni componentes de producción.

## Traslado futuro a GitHub Projects

1. Ejecutar LOCAL-PLAN-01: revisar registro vigente antes de editar. Preservar IDs, historial y relaciones originales.
2. Asociar estos paquetes a Issues existentes o hijos locales necesarios; el tipo VERTICAL no requiere crear una nueva tarjeta por cada etapa ya planificada.
3. Registrar puertas de decisión y dependencias de código reales; verificar grafo sin ciclos. No borrar dependencias generales para fingir que están satisfechas.
4. Actualizar estado operativo solo mediante mecanismos soportados y leerlo de vuelta. Esta propuesta no cambia el Project por sí misma.
5. Estimar y reportar por hito/criterio, no por cantidad de PRs. Un PR puede completar varias tarjetas del mismo corte si cada aceptación queda demostrada.
6. Detenerse tras aceptación LOCAL-1. Las pruebas de GCP quedan fuera del backlog ejecutable local.
