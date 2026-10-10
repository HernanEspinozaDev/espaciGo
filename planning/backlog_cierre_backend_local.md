# Backlog propuesto para cerrar el backend local

Fecha inicial: 2026-10-07; conciliación actualizada tras los merges #219/#221 el 2026-10-09. #216 LOCAL-CONT-01, #214 LOCAL-BOOK-02 y #218 LOCAL-OPS-01 fueron aceptadas por sus alcances locales; #218 quedó cerrada/Hecho en Projects. Los padres generales siguen abiertos. Complementa [el plan de cierre](plan_cierre_backend_local.md). Los paquetes son planificación local; GitHub Issues/Projects conserva los estados operativos.

## Convenciones

- `LOCAL-*` identifica paquetes de cierre. Tipo `VERTICAL` agrupa las etapas ARCH → DB → BE → API → TEST → MOCK de las tarjetas originales para evitar un PR por etapa. No sustituye la trazabilidad de esas tarjetas.
- Reutilizar las Issues originales; crear un hijo local cuando sea necesario distinguir criterios locales de proveedores/GCP o ampliar una entrega ya cerrada. LOCAL-KYC-01 es Issue #200 y subissue de #45, relacionada con #48–#50. LOCAL-M04-PUB-01 (#204), EDIT-01 (#206) y EDIT-02 (#208) quedaron aceptadas tras PR #205/#207/#209; sus recorridos y límites constan en `planning/evidence/local-m04-publication-20261008.md`, `planning/evidence/local-m04-edit-20261009.md` y `planning/evidence/local-m04-edit-details-20261009.md`. LOCAL-M04-GALLERY-01 (#210, hija de #56) cubre ahora la galería sintética privada. No importar de nuevo el backlog Hermes.
- LOCAL-PLAN-01, LOCAL-CORE-01 y LOCAL-AUTH-01 cuentan con entregas aceptadas (#181/#180/#182). LOCAL-BOOK-01 (#177) y LOCAL-M02-01 (#202, PR #203) también fueron aceptadas sin cerrar sus padres generales. LOCAL-KYC-01 (#200) fue aceptada tras PR #201. LOCAL-M04-PUB-01 (#204), EDIT-01 (#206) y EDIT-02 (#208) son slices aceptados; #209 confirma descripción/capacidad/reglas. LOCAL-M04-GALLERY-01 (#210) cubre galería privada; LOCAL-M05-DISC-01 (#212) integra publicaciones activas al catálogo. LOCAL-BOOK-02 (#214) reúne el ciclo de reserva/pago fake sobre ofertas activas; #73/#75/#77 permanecen abiertas por alcance general. Los padres/issues no cambian de estado automáticamente.
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

- **Módulo/tipo/estado:** M02 / subentrega VERTICAL / `Aceptada y Hecho` tras PR #203; vinculada a #37–#43, bajo seguimiento de #185/#40.
- **Objetivo:** añadir los dos recursos M02 faltantes que se pueden verificar localmente sin proveedor productivo.
- **Alcance:** PNG fijo generado por Backend y guardado en storage privado; leer/reemplazar/retirar foto propia; referencia `fake-local-v1` con alta/consulta/cambio/revocación, gate KYC sintético efectivo, ZIP, baja y limpieza recuperable. Integrar controles en mock existente.
- **Fuera:** imagen real/carga arbitraria, RUT, cuentas bancarias reales, pagos/transferencias, proveedor real, cierre de padres ni publicación M04.
- **Dependencias:** almacenamiento privado, auth/ownership, elegibilidad local, export ZIP, baja bloqueada/worker recuperable satisfechos por entregas aceptadas; permiso de escritura sincronizado por account lock. No depende de #142 ni desbloquea M04.
- **Aceptación:** migración incremental; privacidad de archivo; aislamiento; reintentos y concurrencia con baja; KYC requerido al crear/cambiar el payout fake; ZIP incluye datos propios; limpieza falla/reintenta; pruebas PostgreSQL desechables y mock.
- **Evidencia de integración posterior al merge:** V30 aplicada incrementalmente sin borrar volumen; API y ZIP reales con dos cuentas sintéticas: photo PNG create/read/replace/remove, payout fake create/read/change/revoke, ambos incluidos en exportación, y `eligibility_required` al crear payout sin KYC. #202 cerrada y Projects Hecho. No cierra #37–#43 ni #185/#40.

### LOCAL-KYC-01 — Elegibilidad y ciclo de revisión sintéticos (Issue #200)

- **Módulo/tipo/estado:** M03 / VERTICAL / `En curso`; hija nativa de #45, relacionada con #48–#50.
- **Objetivo y motivo:** completar en local los casos, subsanaciones y resultados fake reutilizando #47/#51; conservar la aprobación sintética por tipo y permitir revocación explícita auditada conforme a la decisión ratificada.
- **Alcance:** V28 incremental; historial append-only; código de corrección compatible con el motivo; reintento idempotente; lista de casos pendientes y rechazados; elegibilidad `kyc`/`kyb` persistente e independiente; revocación admin bajo bloqueo de cuenta; API/OpenAPI y mock existente.
- **Fuera:** documentos reales, RUT, proveedor, consentimiento/retención productivos de #142; no conceder roles. No implementar el ciclo general de publicación M04 ni la reserva comercial general M06 en esta Issue.
- **Dependencias:** identidad/CORE y almacenamiento sintético ya integrados. El tipo de gate está ratificado: cuentas personales requieren KYC; KYB no lo sustituye. La transacción de nueva reserva lo verifica para anfitrión y arrendatario; LOCAL-M04-PUB-01 (#204) lo conecta al cambio de estado local de borrador a activa. #142 bloquea solo proveedor/datos reales. **Habilita:** siguientes criterios de LOCAL-LIST-01, sin darlos por satisfechos.
- **Aceptación:** aprobar concede solo el tipo indicado; casos nuevos pendientes/rechazados no remueven una concesión; revocar la retira con motivo/auditoría y no altera reservas; una aprobación nueva puede restablecer. Historial estable, aislamiento, reintento y concurrencia probados en PostgreSQL descartable; mock compilado y recorrido validado.
- **Pendientes originales:** notificación de resultados KYC mediante contrato durable de dominio, consentimiento/cuotas M03, retención/supresión integral de expedientes y todo proveedor/dato real permanecen abiertos; no cerrar #45/#48–#50.
- **Riesgo:** interpretar una aprobación fixture como identidad real o asignar el tipo a operaciones comerciales sin un vínculo de negocio definido.

## L2 — Oferta y reserva generales

### LOCAL-LIST-01 — Completar ciclo local de publicaciones

- **Módulo/tipo/estado:** M04 / VERTICAL / `parcial`; #204 y #206 son slices aceptados, no completan este paquete.
- **Objetivo y motivo:** #52–61, CU-15–18; completar borrador → publicación local con controles de negocio.
- **Alcance restante:** #204 cubre transición local owner-only, gate KYC e historial. #206 cubre título/precio con snapshots, #208 edición de descripción/capacidad/reglas y #210 galería privada sintética; esos slices están aceptados. Continúan pendientes conexión al catálogo general, modalidad tarifaria y políticas/comisiones generales; reutilizar calendario M06.
- **Fuera:** publicación comercial, archivos reales sin autorización, cambiar categoría semilla o elegir frontend final.
- **Dependencias:** LOCAL-PRIV-01, LOCAL-KYC-01, CORE y D-KYC/LIST/D-BOOK. El gate KYC sintético de publicación/solicitud personal está ratificado; #204 y #206 lo conservan en los puntos de entrada existentes. #142 queda fuera para la parte sintética local. La relación de publicaciones con descubrimiento general sigue pendiente. No cerrar #55/#52–#61 para desbloquear. **Desbloquea:** LOCAL-DISC-01 al completar contratos mínimos de oferta y acceso; #204/#206 por sí solos no bastan.
- **Aceptación:** publicación requiere datos/estados aprobados; borradores quedan privados; despublicar corta nuevas operaciones según regla sin borrar reservas anteriores; galería conserva autorización y datos validados; snapshots anteriores sobreviven edición.
- **Pruebas previstas:** transición inválida, dueño/tercero, versión de perfil, files sintéticos, permisos y reglas de calendario; finalizar LOCAL-LIST-MOCK-01.
- **Riesgo:** duplicar `ocupacion` o confundir una oferta local sintética con lanzamiento público.

### LOCAL-M04-PUB-01 — Publicar y ocultar espacios propios con gate KYC sintético (Issue #204)

- **Módulo/tipo/estado:** M04 / subentrega VERTICAL de LOCAL-LIST-01 / Aceptada y Hecho; Issue hija de #55, cerrada tras PR #205.
- **Alcance:** transición de borrador/oculto a activo y activo a oculto; solo rol arrendador y titular; KYC sintético efectivo para cuentas personales, sin KYB sustituto; revalidación bajo bloqueo de cuenta/espacio; historial append-only; OpenAPI y controles en mock.
- **Fuera:** aparición automática en catálogo general, edición de oferta activa, galería, políticas comerciales, cancelación de reservas, KYC productivo/#142 y GCP.
- **Dependencias reales:** #200/#201, ownership/rol, V30 y la tabla `espacio` están integrados en main. #204 se implementa sobre ellos; no depende de terminar padres #185/#40 ni cierra Issues M04.

### LOCAL-M04-EDIT-01 — Editar título y tarifa de una publicación propia (Issue #206)

- **Módulo/tipo/estado:** M04 / VERTICAL DB-API-TEST-MOCK / aceptada y Hecho, hija de #55.
- **Alcance:** editar título y/o precio base solo en publicación propia activa u oculta; mantener estado; cambio de precio crea nueva versión de tarifa; cotizaciones y reservas conservan snapshots. El mock ofrece edición contextual al titular.
- **Dependencias reales:** #204/#205 publica y delimita ownership/estados; #134/#141 integra tarifa versionada y snapshots; #200/#201 mantiene gate KYC en reserva/publicación. Sin dependencia de catálogo general, galería, proveedor real o GCP.
- **Validación focalizada:** unidad y handler/OpenAPI; build del mock; PostgreSQL desechable comprueba titularidad, estados activa/oculta, precio versionado, título sin nueva versión y snapshots inmutables. Sin migración nueva (V31 basta).
- **Aceptación:** PR #207 merge `42e0744515baa0b9b191e3aacd03a0bd480b5370`; #206 cerrada/Hecho tras el mock posterior al merge. #55 y demás padres M04 permanecen abiertos.
- **Evidencia:** `planning/evidence/local-m04-edit-20261009.md`; comprobación del mock activo propio con cambio de título y tarifa, reutilizando pruebas publicadas.

### LOCAL-M04-EDIT-02 — Editar descripción, capacidad y reglas propias (Issue #208)

- **Módulo/tipo/estado:** M04 / VERTICAL DB-API-TEST-MOCK / aceptada y Hecho tras PR #209, subissue de #55.
- **Alcance:** edición owner-only de descripción, capacidad y reglas de uso para publicaciones activa/oculta; partial update conserva estado, título y tarifa. El mock genera únicamente el body con valores cambiados.
- **Dependencias reales:** #204/#205 y #206/#207 están fusionadas; reutiliza el modelo V5/V31 y la política de titularidad existente. Sin nueva migración, galería o conexión al catálogo general.
- **Validación:** unitarias de límites, handler que valida body/respuesta OpenAPI, build y test enfocado del mock, integración PostgreSQL desechable con snapshots preservados.
- **Fuera:** cambiar modalidad tarifaria (interactúa con calendario/disponibilidad), galería/archivos, ubicación/categoría/atributos, comercialización y GCP.
- **Aceptación:** merge #209 integrado en `main`; prueba del mock guardó por separado descripción, capacidad y reglas, confirmó persistencia y preservó título, tarifa y estado originales. #208 cerrada/Hecho por su alcance; #55 y #52–#61 siguen abiertas.
- **Evidencia:** `planning/evidence/local-m04-edit-details-20261009.md`.

### LOCAL-M04-GALLERY-01 — Administrar galería sintética privada (Issue #210)

- **Módulo/tipo/estado:** M04 / VERTICAL DB-API-TEST-MOCK / aceptada y Hecho tras PR #211, hija de #56.
- **Alcance:** Backend genera solo `synthetic-png-v1`; titular lista, consulta, añade idempotentemente y retira hasta 10 imágenes de sus espacios. Archivos quedan fuera del repositorio y de rutas públicas. Incluye limpieza recuperable, ZIP propio y coordinación con la baja existente. La galería no se expone al catálogo general.
- **Dependencias reales:** storage privado #47/#143, baja/exportación #202/#203 y ownership/publicación #204/#205 integrados. La edición de contenido #208/#209 no es dependencia funcional. No espera a cerrar #55/#56 ni al catálogo general.
- **Fuera:** archivos arbitrarios/reales, moderación, URL pública, plazo nuevo de retención, proveedor productivo y GCP.
- **Validación:** PostgreSQL desechable cubre límite/idempotencia/competencia, ownership y limpieza recuperable; integración de exportación/baja cubre inclusión del PNG propio y limpieza coordinada; build y pruebas enfocadas del mock. PR actual añade evidencia y recorrido mínimo propietario.
- **Aceptación:** PR #211 integrado en `47b04cf`; V33 incremental, health/mock health y endpoints HTTP verificados. El mock permitió generar PNG, consultarlo desde API autenticada y retirarlo; galería quedó en 0/10. #210 cerrada y Hecho; padres M04 siguen abiertos.
- **Evidencia:** `planning/evidence/local-m04-gallery-20261008.md`: PostgreSQL descartable con `espacigo_runtime` (candidato antiguo bloqueado durante Put, foto confirmada accesible, recuperación tras reinicio), checksums V32/V33, verify-http y smoke del mock. Se conservaron volumen/secretos y no se ejecutó GCP.

### LOCAL-M05-DISC-01 — Descubrir publicaciones locales activas (Issue #212)

- **Módulo/tipo/estado:** M05 / subentrega VERTICAL DB-API-TEST-MOCK / aceptada como slice (#212, PR #213), hija de #65; no completa M05 ni M04.
- **Alcance:** extender catálogo/detalle existente para incluir espacios `activa` con KYC sintético efectivo, además de conservar fixtures explícitos; borrar/ocultar/no elegible permanece excluido. Reusar filtros, precio estimado, disponibilidad, zona, paginación, cotización snapshot y reserva; no exponer dirección, coordenadas, titular ni galería privada.
- **Dependencias reales:** #200/#201, #204/#205, #206/#207, #208/#209, #210/#211 y slices M05/M06 de filtro, geografía, paginación, selector e integridad están fusionados. No exige completar #55/#56/#62–68 ni proveedores/GCP.
- **Fuera:** tarifas/comisiones generales, promociones, galería pública, modificación/cancelación de reservas por ocultación o cambio de elegibilidad, pagos reales y rendimiento cloud.
- **Aceptación:** borradores/ocultos/no elegibles no aparecen; detalle no filtra datos privados; catálogo no crea cotización/ocupación; filtros y paginación se conservan; cotización ligada al espacio y reserva revalida snapshots/eligibilidad/disponibilidad. Pruebas PostgreSQL de visibilidad/ownership y recorrido mock búsqueda→detalle→cotización.
- **Estado operativo:** Issue #212 cerrada y Hecho por su alcance aceptado; parent #65 y #55/#56/#62–68 permanecen abiertos.

### LOCAL-DISC-01 — Completar búsqueda y cotización de oferta local

- **Módulo/tipo/estado:** M05 / VERTICAL / `todo`.
- **Objetivo y motivo:** #62–68, CU-19–21; reemplazar la dependencia funcional de allowlists por oferta publicada/elegible.
- **Alcance:** filtros existentes, disponibilidad, detalle autorizado, proximidad y paginación; cotización versionada por oferta seleccionada; índices y límites para dataset local acordado.
- **Fuera:** geocodificador real, ranking patrocinado, mapa definitivo o garantía de capacidad cloud.
- **Dependencias:** LOCAL-LIST-01 y LOCAL-BOOK-01. **Desbloquea:** LOCAL-BOOK-02.
- **Aceptación:** oferta despublicada/no elegible no aparece; búsquedas no retienen disponibilidad; total/intervalo/snapshot correctos; paginación y orden documentados; no exponer dirección/coordenada privada por error.
- **Pruebas previstas:** filtros combinados, atributos ausentes, redondeo, fronteras DST, precio/radio inclusivo, páginas y cambio de estado/tarifa; finalizar LOCAL-DISC-MOCK-01.
- **Riesgo:** permitir reserva de precio estimado obsoleto o declarar estable un catálogo que cambia entre páginas.

### LOCAL-BOOK-02 — Consolidar reservas locales sobre publicaciones activas (Issue #214)

- **Módulo/tipo/estado:** M06 / VERTICAL / en implementación; hija de #73 y relacionada con #75/#77. La subentrega cubre el recorrido no-fixture.
- **Objetivo y motivo:** #69/#73/#75/#77/#79, CU-22–28/47/51; ciclo general con pago fake/durable ya aceptado.
- **Alcance:** reutilizar elegibilidad/snapshot, reserva/ocupación atómica, solicitud/aprobación/rechazo, vencimientos y cancelación/refund `local_flexible_v1` sobre publicación activa sin habilitador de fixture; registrar puntos de integración futura con M07/M08/M10; inbox/conciliación se reutilizan.
- **Fuera:** proveedor real/sandbox #76/#78, políticas financieras nuevas no ratificadas o custodiar fondos.
- **Dependencias:** publicación/detalle/cotización de #212, AUTH/CORE y `local_flexible_v1` ratificada; #74 fake durable aceptada. Satisfechas para este alcance. Proveedor real sigue aislado en #76/#78.
- **Criterios satisfechos en este slice:** API de publicación activa no-fixture; snapshots de tarifa/política; reintentos idempotentes de solicitud/pago/refund; aprobación/rechazo; historial y aislamiento; expiración, cancelación/refund; cambios oculto/KYC/tarifa antes de solicitar sin filas parciales; recorrido mock de dos participantes.
- **Criterios pendientes de los padres:** conciliación general, proveedor/sandbox real, aceptación amplia de #79, políticas generales y criterios restantes de #73/#75/#77. No declarar completos esos Issues ni M06.
- **Contratos futuros:** M07 consume reserva aprobada y snapshots por contrato versionado e idempotente; M08 ejecuta check-in/out sobre la misma reserva sin autocompletar por reloj; M10 vincula disputas a reserva y conserva pagos/refunds fake como hechos, no como ledger. Ningún consumidor muta snapshots ni crea ocupaciones alternativas. Los nuevos eventos/transiciones se definen al implementar esos módulos; aquí quedan registrados los puntos de integración.
- **Pruebas previstas:** reutilizar cobertura previa y añadir recorrido API de publicación activa, transiciones y regresiones de tarifa/elegibilidad en PostgreSQL desechable; verificar el mock con las dos cuentas.
- **Riesgo:** doble ocupación, dinero simulado no trazado o dependencia circular entre reserva y módulos posteriores.

## L3 — Ejecutar el arriendo

### LOCAL-CONT-01 — Construir contrato y firma simulada

- **Módulo/tipo/estado:** M07 / VERTICAL / `Aceptado` (#216 / PR #217 fusionado).
- **Objetivo y motivo:** #80–87, CU-29–32; asegurar contenido y resultado antes del uso del espacio.
- **Alcance del corte:** snapshot versionado desde reserva aprobada; PDF sintético privado/hash cifrado; firmas fake; rechazo terminal no cancelatorio; vencimiento transaccional M06 con liberación de ocupación y obligación fake del 100%; recuperación idempotente después de aprobación. Rutas JSON, OpenAPI y controles del mock.
- **Fuera:** validez jurídica, firma real, proveedor o documentos personales reales.
- **Dependencias:** LOCAL-BOOK-02/#214, identidad, precio y M06 pagos/devoluciones fake fusionados. Las decisiones de firma/rechazo/vencimiento fueron ratificadas en este corte. LOCAL-OPS-01/#218 quedó habilitada tras aceptación.
- **Aceptación del corte:** edición de perfil/tarifa no cambia contrato histórico; firman solo participantes; tercero no accede; firma parcial/final y rechazo registrado; inicio exacto con firma faltante cancela y crea una devolución única dentro del lock; contrato completamente firmado no vence; descarga solo tras firma total; contenido cifrado en reposo.
- **Pendiente del padre M07:** proveedor real/sandbox y callback externo; notificaciones durables de contrato; exportación del nuevo artefacto; retención general/productiva y criterios restantes de firma legal.
- **Pruebas ejecutadas/previsibles:** lifecycle PostgreSQL con reloj inyectado, lock/carrera, replay y ocupación/refund fake; cifrado PDF y acceso participante; acciones mock. Integración completa en DB desechable; no usar el volumen de desarrollo.
- **Riesgo:** regenerar retrospectivamente el snapshot o representar el fake como firma legal.

### LOCAL-OPS-01 — Construir entrega, recepción y devolución

- **Módulo/tipo/estado:** M08/M10 intake / VERTICAL / `Hecho` para su alcance local (#218; PR #219 fusionado y aceptado 2026-10-09).
- **Objetivo y motivo:** #88–94, CU-33–34/48; completar el uso del arriendo y sus evidencias.
- **Alcance:** check-in/out, recepción/observaciones, evidencia PNG privada sintética, ubicación identificada de ensayo, actor/fecha y transiciones de reserva. Incluye apertura local del reclamo formal M10 por anfitrión dentro de las 24 horas desde el check-out persistido y descargo textual del arrendatario. No adjudica daños, mueve fondos ni cierra el arriendo por reloj.
- **Fuera:** geolocalización real o política de daño/garantía no acordada; resolución de disputa pertenece a M10.
- **Dependencias:** LOCAL-CONT-01/#216 aceptado, BOOK/CORE y decisión D-CONT/OPS; decisión de reclamante/ventana ya ratificada. **Desbloquea:** LOCAL-COMM-01 y el siguiente corte de evidencias/resolución LOCAL-DIS-01.
- **Aceptación:** acciones por participante/estado/fecha; firma completa requerida para check-in; evidencia privada generada por Backend; idempotencia/reintentos; check-out inicia el plazo persistido; recepción no acorta plazo; observación separada del reclamo formal; apertura y descargo se autorizan por rol y ventana, sin resolución automática.
- **Pruebas previstas:** reloj bajo bloqueo, actor incorrecto, confirmaciones/objeciones, repetición y estado final; finalizar LOCAL-OPS-MOCK-01.
- **Riesgo:** confundir la incidencia de privacidad M02 con el reclamo de daños M10, presentar un fake como resolución legal/financiera, o completar la reserva solo porque pasó la hora.

### LOCAL-COMM-01 — Reseñas, moderación y avisos durables locales (Issue #220)

- **Módulo/tipo/estado:** M09 / VERTICAL / Hecho para LOCAL-COMM-01; #220 cerrada tras merge PR #221 (`8d3157c`), hija de #95 y relacionada con #96–#102. Los padres y criterios generales siguen abiertos.
- **Objetivo y motivo:** #95–102, CU-35–38/49; completar reseñas/reportes y avisos durables sin cerrar padres generales.
- **Alcance D-COMM ratificado:** una reseña por participante y reserva `finalizada`, arrendatario→espacio y anfitrión→arrendatario; nota entera 1–5 obligatoria, comentario opcional; no hay ventana adicional, edición ni nuevas reseñas en `en_disputa`; reintento idéntico reutiliza. Reseñas visibles del espacio y promedio únicamente para publicación activa, sin datos privados; reputación del arrendatario solo en vistas autenticadas autorizadas. El anfitrión reporta reseñas de su espacio con catálogo estructurado; el reporte marca `reportada` pero no oculta ni modifica promedio. Admin desestima u oculta con motivo y auditoría; ocultas no listan ni promedian; sin apelación.
- **Avisos:** intención en transacción, deduplicación por evento/destinatario y entrega Mailpit para check-in→anfitrión, reporte→administradores autorizados, reclamo abierto→arrendatario y cancelación→ambas partes. Sin aviso por mensaje. Ocho intentos por ciclo, fallo terminal persistido y reapertura administrativa motivada; SMTP puede duplicar si el resultado es incierto tras envío.
- **Privacidad local ratificada:** exportación propia y minimización coordinada con baja; reseñas 24 meses desde creación; reportes hasta resolución y 24 meses posteriores; entregas pendientes se conservan hasta resolución y terminales 30 días. Chat conserva su tratamiento vigente. Son decisiones locales, no plazos legales.
- **Dependencias satisfechas para el corte:** LOCAL-BOOK-02/#214 (reserva/participantes), LOCAL-CONT-01/#216, LOCAL-OPS-01/#218 (check-in, check-out, disputa, cancelación), conversación/#156/#158, identidad/auditoría/outbox/Mailpit. #79 y los padres #88–94/#103–110 conservan criterios generales y no se cierran ni bloquean este recorrido local.
- **Fuera:** mensajería por cada mensaje, push/campana, canales externos, perfiles públicos de arrendatarios, apelaciones y criterios generales productivos.
- **Aceptación:** autorización por rol/participante/propietario/admin; reseñas recíprocas, promedio activo, reporte/moderación/auditoría; cuatro tipos de intención durable; fallos/reapertura y retención; exportación/baja aisladas; pruebas PostgreSQL y recorrido mock.
- **Pruebas:** `go test` focalizado PostgreSQL con runtime, aislamiento, idempotencia, concurrencia, promedio, moderación y recuperación del outbox; recorrido mock con ambas partes y administrador.
- **Riesgo:** filtrar datos privados o confundir aceptación SMTP con entrega exactamente una vez.
- **Corrección previa a revisión:** V37 permite minimizar reseñas recibidas conforme a sus checks; coordina escrituras/despacho con baja; recupera leases vencidos del intento 8 como terminales sin noveno envío; y conserva marca opaca de unicidad hasta que deje de estar permitida la reseña, incluida reserva sin vencimiento de vínculos. Altas de reseña/reporte ahora comparten con reclamos el orden cuentas ordenadas → reserva → reseña/reporte. La integración desechable aplica V36→V37, comprueba Mailpit SMTP/API, la carrera reseña/reclamo sin deadlock y el purgado a 25 meses con unicidad retenida.

## L4 — Resolver, liquidar y administrar

### LOCAL-DIS-01 — Resolución administrativa local de disputas (Issue #222)

- **Módulo/tipo/estado:** M10 / DB-BE-API-TEST-MOCK / PR de esta rama para revisión; Issue #222 pasa a `En revisión` tras publicar; hija de #106, relacionada con #103. No cierra #103/#106 ni los padres #103–110.
- **Objetivo y motivo:** completar adjudicación sintética de reclamo/descargo siguiendo CU-39–41, RQF-159–171 y PT-07, después de LOCAL-OPS-01/#218 y LOCAL-COMM-01/#220.
- **Alcance:** anfitrión abre según plazo vigente; participante contrario consulta y presenta texto de descargo; ambos participantes acceden a PNG sintéticos de check-in/out/recepción vinculados a la reserva; administrador tiene cola/detalle/evidencia privados y registra `acogido`/`rechazado`, motivo estructurado, actor, instante e historial; cierre transaccional/idempotente y aviso outbox a ambos. Cerrar retira solo el bloqueador `disputa_abierta` de este reclamo; la evaluación de privacidad conserva otras obligaciones. No ejecuta baja.
- **Separación financiera ratificada:** no asignar montos ni actualizar reserva/ocupación/pagos. Garantía, deducciones, captura, liberación y devoluciones derivadas del fallo pertenecen a LOCAL-FIN-01; su importe/fuente no se simulan aquí.
- **Fuera:** validez jurídica, proveedores/PSP, efectos financieros, reglas comerciales no aprobadas y GCP. La foto propia del descargo y la retención/purga general más allá de la política local ratificada siguen pendientes; V39 sí cubre el texto M10 y la desvinculación de vínculos históricos de este recorrido tras 24 meses.
- **Dependencias satisfechas para este corte:** LOCAL-OPS-01/#218, LOCAL-COMM-01/#220, identidad/roles, auditoría, outbox y almacenamiento privado local. **Desbloquea:** solo consumo posterior por LOCAL-FIN-01; #107–110 mantienen sus criterios generales.
- **Aceptación prevista:** autorización de administrador/partes/tercero; decisión estructurada e idempotente; sin cambios financieros; historial/auditoría/avisos únicos; PostgreSQL con rol runtime y mock compilado/recorrido.
- **Riesgo:** presentar adjudicación sintética como resolución jurídica o como saldo/garantía disponible.

### LOCAL-FIN-01 — Garantía y deducciones fake del cierre local (Issue #224)

- **Módulo/tipo/estado:** M06/M10 / DB-BE-API-TEST-MOCK / aceptada tras PR #225 fusionado el 2026-10-09; #224 cerrada/Hecho. Issue hija de #185/#40 y relacionada con #103/#107–110. El corte local no cierra criterios generales.
- **Objetivo y motivo:** #107/#108–110 y criterios financieros de CU-39–42/47; separar movimiento observado fake de una mera transición de reserva.
- **Alcance ratificado 2026-10-09:** exclusivamente cuentas sintéticas y reservas fake. `garantia_local_fija_v1` = CLP 50.000 por reserva nueva; política/moneda/monto quedan en snapshot de cotización y reserva. Autorización, captura, liberación y devolución son obligaciones distintas y persistidas; autorización no es cobro ni ingreso. Tras pago fake confirmado, preautorización separada/idempotente con vencimiento min(15 min desde primer intento, inicio); pending impide aprobar, firmar y check-in. Rechazo o vencimiento cancela, libera ocupación y genera devolución fake completa del arriendo confirmado. Timeout se concilia sobre misma operación; autorización tardía no reactiva y crea liberación compensatoria pendiente. Deducción administrativa inmutable 0..autorizado; reclamo rechazado exige 0; positiva exige motivo `dano_acreditado`/`faltante_acreditado` y evidencia sintética de la reserva. Captura confirmada antes de marcar aplicada; luego liberar saldo. Sin reclamo, liberar tras 24 h desde checkout; reclamo abierto suspende. Cancelar reserva cierra/libera autorización sin borrar obligaciones inciertas. Todos los inciertos bloquean baja.
- **Fuera:** proveedor real/sandbox, liquidación al anfitrión, comisión, boleta, fondos reales/custodia y completar M06/M10 generales. Los hechos V40 se exportan con alcance propio y se incluyen en bloqueadores de privacidad.
- **Dependencias:** LOCAL-DIS-01, BOOK, snapshots LIST y D-BOOK/D-DIS. **Desbloquea:** LOCAL-ADMIN-01 y validación completa M10.
- **Dependencias:** LOCAL-DIS-01/#222 aceptado, LOCAL-BOOK-01/#177 y BOOK-02/#214, snapshots de tarifa/listado, LOCAL-CONT-01/#216 y LOCAL-OPS-01/#218. No depende del proveedor real #76/#78. **Desbloquea:** solo cierre económico local fake; no cerrar #103/#107–110 ni desbloquear liquidación/proveedor.

### LOCAL-DIS-TEST-MOCK-01 — Consolidación de pruebas de disputa y mock financiero (Issue #230)

- **Módulo/tipo/estado:** M10 / VERTICAL TEST-API-MOCK / `En revisión` durante PR de implementación; subentrega bajo #109 DIS-TEST-01 y #110 DIS-MOCK-01, no cierre de M10.
- **Propósito:** conciliar criterios originales de #109/#110 con LOCAL-DIS-01 y LOCAL-FIN-01; verificar evidencia/descargo, resolución admin y decisiones financieras fake mediante resultados persistidos, no solo proyecciones.
- **Alcance local:** recorrido de anfitrión, arrendatario y administrador independientes; resolución y evidencia privada; límite de deducción al monto autorizado; captura/liberación fake, timeout/conciliación e historial observado; autorización, concurrencia e idempotencia ya cubiertas por pruebas aceptadas, añadiendo solo el contraste de proyección/resultado/historial que faltaba.
- **Dependencias satisfechas para este corte:** LOCAL-DIS-01 (#222/#223), LOCAL-FIN-01 (#224/#225, V40), CORE-TEST-01 (#24) y CORE-ENV-01 (#25). Los endpoints del alcance de DIS-API-01 (#108) están disponibles; los criterios generales de #108 siguen abiertos.
- **Matriz de criterios:** [`evidence/local-dis-test-mock-20261009.md`](evidence/local-dis-test-mock-20261009.md). La fila test del #109 queda completa solo dentro del fake local: valores observados, reintentos/terminales y audit trail. #110 queda parcial porque no hay payout/liquidación al anfitrión, comisión ni documento/boleta fixture.
- **Pendientes fuera del corte:** credenciales, contrato y sandbox de proveedor auténtico; liquidación, comisión y documento fiscal requieren regla/modelo y siguen pendientes aunque exista proveedor. No representar garantía autorizada como ingreso ni inventar dichos artefactos. No GCP.
- **Trazabilidad:** conservar #109/#110 abiertas hasta revisión; #108 general abierta; #111 queda bloqueada hasta satisfacer #110 completo. Tras la aceptación de la subentrega se podrá continuar el alcance local de #111 con dependencias explícitas, sin cerrar #109/#110 si persisten sus criterios generales.
- **Aceptación:** snapshot CLP fijo sin retroactividad; operaciones fake idempotentes/durables; gates transaccionales; importes enteros exactos; reintentos/fallos/resultado tardío no duplican; cierre no pierde trazabilidad y bloqueadores de baja consideran toda incertidumbre. No registrar garantía autorizada como ingreso.
- **Pruebas/evidencia:** límites (0, 1, 50.000 y >50.000), reclamo aceptado/rechazado, rechazo/vencimiento/timeouts, resultado tardío, cierre 24 h, cancelación, carreras, reinicio/conciliación y rol runtime están cubiertos por la integración desechable publicada y sus pruebas focalizadas; evidencia de aceptación/migración V40 y HTTP posterior al merge: [`local-fin-01-20261009.md`](evidence/local-fin-01-20261009.md). El recorrido de negocio se verifica por integración API; no se afirma automatización de navegador sobre los datos persistentes.
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

### LOCAL-ADMIN-01A — Consulta administrativa local de reservas y finanzas (Issue #226)

- **Módulo/tipo/estado:** M11 / VERTICAL DB-BE-API-TEST-MOCK / `aceptada y Hecho` para esta subentrega tras PR #227; subentrega acotada de LOCAL-ADMIN-01, no aceptación del padre.
- **Objetivo y motivo:** proporcionar una bandeja de consulta para reserva, participantes mínimos, estado/historial, pago/devolución fake, garantía, deducción y reclamo, reutilizando los ownerships existentes.
- **Alcance:** filtros por ID/estado/fecha de creación; cursor estable versionado y ligado a cuenta/filtros; default 25, máximo 100, sin conteo. Sólo rol administrador en Backend. Proyección privada y minimizada; una auditoría estructurada por solicitud de lista/detalle, correlacionada y sin guardar respuestas. Mock que limpia selección, detalle y cursores al cambiar/perder sesión. Lecturas sin vencimiento materializado, conciliación, pagos ni transiciones.
- **Fuera:** gobierno/moderación general, acciones financieras, historial de mensajes/documentos, proveedor real y GCP. No desbloquea LOCAL-CORE-02 ni cierra #111–118.
- **Dependencias satisfechas para esta subentrega:** LOCAL-CORE-01/#180 (sesión/roles/auditoría), LOCAL-DIS-01/#222 (reclamo/resolución mínima), LOCAL-FIN-01/#224 (garantía/deducción fake) y LOCAL-BOOK-02/#214 (reservas/snapshots/historial). Padres #185/#40 y criterios generales M11 permanecen abiertos; no son una dependencia técnica para la vista acotada, aunque siguen siendo gates de privacidad/módulo completo.
- **Criterios:** administrador puede listar/abrir; filtros y cursor sin duplicados con datos estables; identificadores retirados son null; terceros reciben 403; respuesta excluye contacto, credenciales, documentos y contenido privado; cada lectura exitosa genera un evento mínimo de auditoría; ninguna lectura muta estados/importes; el mock descarta respuestas atrasadas después de cambio/logout/relogin.
- **Pruebas/evidencia:** unitarias de autorización/parámetros/cursor y sesión mock; integración PostgreSQL desechable con rol runtime, auditoría, historial minimizado y conteos/estado antes-después; pasos de navegador documentados en `planning/evidence/local-admin-01a-20261009.md`.

### LOCAL-ADMIN-01B — Consulta y exportación mínima de auditoría local (Issue #228)

- **Módulo/tipo/estado:** M11 / VERTICAL DB-BE-API-TEST-MOCK / aceptada y Hecho tras PR #229 (merge `994c91f`); subentrega acotada, no aceptación de LOCAL-ADMIN-01 ni #111–#118.
- **Objetivo:** permitir inspección local del ledger ya existente sin ampliar su cobertura de productores ni modificar/borrar eventos.
- **Contrato:** administrador activo obligatorio en Backend; periodo RFC3339 requerido desde inclusivo/hasta exclusivo normalizado UTC y máximo 31 días; filtros exactos opcionales por actor UUID, tipo/ID recurso, acción y resultado; lista por `ocurrido_en DESC, id DESC`, 25 por defecto y 100 máximo, cursor URL-safe ligado a cuenta/filtros/tamaño/corte; exportación JSON v1 con máximo 10.000, sin truncamiento. Proyección única: ID, fecha, actor técnico nullable, recurso, acción, resultado, motivo estructurado y correlación. Un evento mínimo por petición administrativa autenticada; scope de lectura/exportación se fija antes de su evento de acceso.
- **Persistencia/retención:** reusa `evento_auditoria_local`, V22/V23+ y grants append-only existentes. No requiere DDL; conserva retención de cinco años y no añade UPDATE/DELETE.
- **Dependencias satisfechas para este corte:** #180 roles/auditoría local y #226 lectura admin integrados; el ledger existente permite lectura con rol `espacigo_runtime`. #111–#114 son relacionadas y contienen criterios generales de gobierno, modelado transversal, outbox y durabilidad aún abiertos; no se declara su dependencia completa ni se desbloquean padres.
- **Fuera:** nuevos productores transversales, búsqueda textual, mutaciones, purga, outbox general, inmutabilidad productiva y GCP. LOCAL-CORE-02, #111–#118 y los padres M11 siguen abiertos. Siguiente bloqueo de la cadena: DIS-TEST-01/#109 → DIS-MOCK-01/#110 antes de ADMIN-ARCH-01/#111.
- **Evidencia:** [`local-admin-01b-20261009.md`](evidence/local-admin-01b-20261009.md); contraste de productores/cobertura, integración PostgreSQL desechable, OpenAPI, pruebas mock y smoke HTTP posterior al merge.

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
| LOCAL-OPS-MOCK-01 / M08 / #94 | LOCAL-OPS-01 | Check-in/out, recepción/observación, reclamo formal y descargo sintético, evidencia, fechas/roles y estado; repetición y plazo vencido muestran resultado/error correcto. |
| LOCAL-COMM-MOCK-01 / M09 / #102 | LOCAL-COMM-01 | Hilo/lectura existente, publicar/consultar reseña elegible, reportar/moderar con rol adecuado y comprobar avisos por Mailpit/estado de entrega autorizado. No añadir un centro de notificaciones. |
| LOCAL-DIS-TEST-MOCK-01 / M10 / #230, bajo #109/#110 | LOCAL-DIS-01/#222–223, LOCAL-FIN-01/#224–225, #24, #25 | Verificar resultados fake observados, historial, resolución/evidencia/descargo, deducción dentro de garantía, conciliación e integración de mock. Proveedor real/sandbox y payout/boleta/liquidación comercial no se simulan; #109/#110 permanecen abiertos por aceptación y criterios incompletos. |
| LOCAL-ADMIN-MOCK-01 / M11 / #118 | LOCAL-ADMIN-01 | Consultar cuentas/reservas/pagos/disputas, acciones administrativas permitidas, moderación, reportes/exportación mínima y auditoría/correlación; negar rol insuficiente. |

No crear un mock antes de que existan sus operaciones Backend/API. Se pueden usar secciones del mock existente, sin routing complejo, layouts ni componentes de producción.

## Traslado futuro a GitHub Projects

1. Ejecutar LOCAL-PLAN-01: revisar registro vigente antes de editar. Preservar IDs, historial y relaciones originales.
2. Asociar estos paquetes a Issues existentes o hijos locales necesarios; el tipo VERTICAL no requiere crear una nueva tarjeta por cada etapa ya planificada.
3. Registrar puertas de decisión y dependencias de código reales; verificar grafo sin ciclos. No borrar dependencias generales para fingir que están satisfechas.
4. Actualizar estado operativo solo mediante mecanismos soportados y leerlo de vuelta. Esta propuesta no cambia el Project por sí misma.
5. Estimar y reportar por hito/criterio, no por cantidad de PRs. Un PR puede completar varias tarjetas del mismo corte si cada aceptación queda demostrada.
6. Detenerse tras aceptación LOCAL-1. Las pruebas de GCP quedan fuera del backlog ejecutable local.
