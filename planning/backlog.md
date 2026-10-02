# Backlog técnico Kanban — registro fuente

Este archivo es el registro fuente de 102 tarjetas sincronizadas con el tablero Hermes Kanban local `espacigo`. Cada tarjeta es una unidad revisable; el estado se recalcula con dependencias y evidencia. Corte: 2026-10-01. No hay responsables humanos ni fechas asignados.

## Distribución

| Bloque | N.º tarjetas | ready | running | review | done | blocked | todo |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Transversal/fundación | 9 | 0 | 0 | 1 | 5 | 0 | 3 |
| M01 Identidad y cuenta | 10 | 0 | 0 | 0 | 0 | 0 | 10 |
| M02 Perfil y privacidad | 8 | 0 | 0 | 0 | 0 | 0 | 8 |
| M03 Verificación KYC/KYB | 8 | 0 | 0 | 0 | 0 | 0 | 8 |
| M04 Publicaciones y disponibilidad | 10 | 0 | 0 | 0 | 0 | 0 | 10 |
| M05 Búsqueda y cotización | 7 | 0 | 0 | 0 | 0 | 0 | 7 |
| M06 Reservas y pagos | 11 | 0 | 0 | 0 | 0 | 0 | 11 |
| M07 Contratos y firma | 8 | 0 | 0 | 0 | 0 | 0 | 8 |
| M08 Operación del arriendo | 7 | 0 | 0 | 0 | 0 | 0 | 7 |
| M09 Comunicación y reputación | 8 | 0 | 0 | 0 | 0 | 0 | 8 |
| M10 Disputas, liquidación y tributación | 8 | 0 | 0 | 0 | 0 | 0 | 8 |
| M11 Administración y auditoría | 8 | 0 | 0 | 0 | 0 | 0 | 8 |
| **Total** | **102** | **0** | **0** | **1** | **5** | **0** | **96** |

Estado al 2026-10-01: PR #2 (CORE-ARCH-01), #3 (CORE-DB-03), #4 (CORE-DB-01) y #5 (CORE-DB-02) están confirmados `MERGED`. CORE-DB-01 y CORE-DB-02 quedan `done` por sus PR aprobados; no se repitieron pruebas ambientales de #3/#4. La trazabilidad RQF-213–218 cubre 6/6, pero DB02-09 conserva abiertas las decisiones de preferencia de uso, historial de claves y notificación; ningún DDL dependiente se considera autorizado. Hermes Kanban `espacigo` registra 11 `done`, 1 `blocked` y 90 `todo`; CORE-DB-02 ya aparece `done` y coincide con el merge #5. CORE-BE-01 está en `review` en el backlog y PR #6 está abierto para revisión; Kanban conserva `done` por una ejecución local anterior. La operación admitida `request-review t_39a64c51` fue rechazada (`task is not in running/ready`); se añadió comentario con el PR mediante el CLI admitido, sin cambiar el estado ni editar directamente la base. El registro fuente suma 5 `done`, 1 `review`, 96 `todo`; la diferencia agregada con Kanban permanece documentada sin re-revisar las demás tarjetas. [PR #5](https://github.com/HernanEspinozaDev/espaciGo/pull/5) · [PR #6](https://github.com/HernanEspinozaDev/espaciGo/pull/6).

Los IDs académicos completos de motivación están en [visión y módulos](vision_y_modulos.md); la propuesta física está en [Base de Datos](base_de_datos.md). La fila de trazabilidad al abrir cada módulo aplica a todas sus tarjetas; cada objetivo especifica el flujo concreto que se dividirá en PRs. En cada tarjeta “Pruebas” son validaciones que Developer/Tester deberán automatizar al ejecutarla. “Desbloquea” enumera las tarjetas inmediatas siguientes del grafo.

## Fundación transversal

### PLAN-ARCH-01 — Revisar y fijar mapa global de dominio
- **Módulo/tipo/estado:** Transversal / ARCH / `done`.
- **Objetivo y motivación:** acordar límites de M01–M11, responsables lógicos de las 43 tablas y flujo de valor; ES1 A–E, ES2 Anexo B y prioridad actual.
- **Alcance:** revisar `vision_y_modulos.md`, resolver ownership de `ocupacion`, `documento`, `notificacion`, auditoría y outbox; dejar hallazgos aceptados o tickets de cambio antes de DDL.
- **Fuera de alcance:** DDL, cambios a informes ES1/ES2, decisiones de UI final o selección de proveedor.
- **Dependencias:** ninguna. **Desbloquea:** CORE-ARCH-01
- **Aceptación:** mapa aprobado por equipo con entidades/owners y discrepancias registradas con RQF/RNF/CU/HU afectados.
- **Pruebas esperadas:** revisión de cobertura de todas las 43 entidades y once módulos contra anexos autorizados.
- **Riesgo:** el catálogo documental puede ocultar cardinalidades o finalidades sin validar. **Estado:** done.
- **Evidencia de aprobación:** PR #1 aprobado y fusionado a `main` el 2026-10-01; el usuario ratificó el mapa y MAP-01–MAP-10. [PR #1](https://github.com/HernanEspinozaDev/espaciGo/pull/1). No acredita implementación ni autoriza DDL.

### CORE-ARCH-01 — Fijar contratos entre módulos y reglas transversales
- **Módulo/tipo/estado:** Transversal / ARCH / `done`.
- **Objetivo y motivación:** definir interfaces internas, transacciones, errores, authz, idempotencia, auditoría/outbox y dependencias conforme RNF-013–043 y ES2 3.3.
- **Alcance:** registrar dueño de datos por módulo, reglas de importación, formato de error, correlación, límites de transacción y prácticas sin PII en logs.
- **Fuera de alcance:** elegir microservicios, implementar Go o fijar UX.
- **Dependencias:** PLAN-ARCH-01 (`done`, PR #1). **Desbloquea:** ADMIN-ARCH-01, BOOK-ARCH-01, CORE-BE-01, CORE-DB-01.
- **Aceptación:** documento breve con límites, invariantes, estrategia de error/idempotencia y decisión explícita de monolito modular.
- **Pruebas esperadas:** revisión estática de dependencias propuestas contra M01–M11 y RNF-033.
- **Riesgo:** interfaces vagas llevan a consultas/escrituras cruzadas y acoplamiento. **Estado:** done.
- **Evidencia:** `planning/contratos_entre_modulos.md`; verificación estática 43/43 tablas únicas, 11/11 módulos, RNF-013–043 (31/31), referencias presentes y `git diff --check` aprobado. PR #2 aprobado y fusionado a `main` el 2026-10-01 (merge commit `fa6c1237f75e63ec8c27fab02edadc0b7db8158a`). No implica código, DDL, migraciones ni pruebas de aplicación.

### CORE-DB-01 — Acordar perfil PostgreSQL y convenciones de persistencia
- **Módulo/tipo/estado:** Transversal / ARCH / `done`.
- **Objetivo y motivación:** fijar PostgreSQL 18/PostGIS/`btree_gist` del Anexo B y validar disponibilidad local/CI antes del primer esquema.
- **Alcance:** decidir imagen/versiones verificadas, extensiones, nombres, UUID, `timestamptz`, rangos, moneda decimal exacta, esquemas y estrategia de índices.
- **Fuera de alcance:** crear tablas, imagen productiva o elegir proveedor cloud alternativo.
- **Dependencias:** CORE-ARCH-01. **Desbloquea:** CORE-DB-02
- **Aceptación:** decisión versionada con verificación local reproducible de motor/extensiones y compatibilidad objetivo.
- **Pruebas esperadas:** levantar motor de prueba y consultar versiones/extensiones previstas.
- **Riesgo:** PostGIS/`btree_gist` o versión no disponible en una plataforma objetivo. **Estado:** done; perfil versionado en `planning/base_de_datos.md`. Evidencia ambiental existente reutilizada sin repetir el smoke; no se eligió plataforma productiva ni se creó esquema/DDL. PR #4 aprobado y fusionado por el usuario el 2026-10-01. [PR #4](https://github.com/HernanEspinozaDev/espaciGo/pull/4).

### CORE-DB-02 — Revisar diccionario, ownership y tratamiento de datos
- **Módulo/tipo/estado:** Transversal / ARCH / `review`.
- **Objetivo y motivación:** validar Anexo B v2 como contrato inicial y criterio Ley 21.719 desde incremento uno.
- **Alcance:** revisar 43 tablas, cardinalidades, estados, reglas de borrado, finalidad/acceso/retención, módulos dueños y tablas diferidas.
- **Fuera de alcance:** ampliar alcance para incluir columnas nuevas sin evidencia ni crear tablas.
- **Dependencias:** CORE-DB-01. **Desbloquea:** AUTH-ARCH-01, CORE-DB-03, KYC-ARCH-01, LIST-ARCH-01, PRIV-ARCH-01
- **Aceptación:** lista de decisiones de modelo resueltas y cambios futuros trazados a requisito, sin declarar cumplimiento legal.
- **Pruebas esperadas:** conciliación 43 tablas ↔ módulo ↔ RQF/RNF/CU; revisión de nulabilidad/relaciones en diccionario.
- **Riesgo:** hay capacidades promocionales/NPS todavía no aprobadas y conservarlas puede ampliar datos innecesarios. **Estado:** `done`; PR #5 aprobado y fusionado por el usuario (merge commit `a8fc2392a416ef4a5683d02bbc3ee221fe1eafec`). Se verificó 6/6 RQF-213–218 y Kanban ya muestra `done`. DB02-09 y sus tres brechas permanecen abiertas; resolverlas exige decisión/tickets trazables antes de cualquier DDL dependiente. No se editaron datos del tablero. [PR #5](https://github.com/HernanEspinozaDev/espaciGo/pull/5).

### CORE-DB-03 — Definir migrador y base local reproducible
- **Módulo/tipo/estado:** Transversal / DB / `done`.
- **Objetivo y motivación:** entregar un migrador ejecutable para aplicar migraciones PostgreSQL y verificar su comportamiento desde una base vacía.
- **Alcance:** runner versionado, naming/orden/checksum, advisory lock, transacción por archivo, detección de deriva, forward-fix, extensiones autorizadas, roles mínimos y pruebas automatizadas.
- **Fuera de alcance:** migraciones funcionales o datos reales.
- **Dependencias:** CORE-DB-02. **Desbloquea:** ADMIN-DB-01, AUTH-DB-01, BOOK-DB-01, COMM-DB-01, CONT-DB-01, CORE-BE-01, CORE-ENV-01, CORE-TEST-01, DIS-DB-01, DISC-DB-01, KYC-DB-01, LIST-DB-01, OPS-DB-01, PRIV-DB-01
- **Aceptación:** runner ejecutable en PostgreSQL 18; aplica desde vacío, segunda ejecución idempotente, detecta drift y gaps antes de DDL, hace rollback ante fallo y serializa ejecuciones concurrentes.
- **Verificación:** seis pruebas de integración PostgreSQL pasaron el 2026-10-01 con la imagen por digest registrada en `base_de_datos.md`; PostgreSQL 18.6, PostGIS 3.6.4, `btree_gist` 1.8. El contenedor tmpfs fue eliminado y verificado. PR #3 fue aprobado y fusionado a `main` el 2026-10-01 (merge commit `7dd963aadcb3cc8a003ca9b246a7869c02eed842`).
- **Riesgo/estado:** verificaciones y revisión del usuario completadas; `done`. No inicia AUTH-DB-02, que conserva sus dependencias propias.

### CORE-BE-01 — Fijar estructura lógica del monolito Go
- **Módulo/tipo/estado:** Transversal / ARCH / `review`. **Dependencias reevaluadas:** CORE-ARCH-01 (#2) y CORE-DB-03 (#3) están fusionadas; CORE-DB-02 (#5) también está cerrada. Siguiente tarjeta habilitada del bloque Backend; la revisión del PR es el gate actual.
- **Objetivo y motivación:** trasladar la propuesta ES2 a límites de paquetes, casos de uso y adaptadores, RNF-033.
- **Alcance:** acordar interfaz dominio/aplicación/persistencia/HTTP, uso propuesto de pgx/pgxpool y sqlc, configuración, salud y workers durables.
- **Fuera de alcance:** crear código, elegir framework web sin evaluación o separar servicios.
- **Dependencias:** CORE-ARCH-01, CORE-DB-03. **Desbloquea:** ADMIN-BE-01, AUTH-BE-01, BOOK-BE-01, BOOK-BE-03, COMM-BE-01, CONT-BE-01, CORE-API-01, DIS-BE-01, DISC-BE-01, KYC-BE-01, LIST-BE-01, OPS-BE-01, PRIV-BE-01
- **Aceptación:** mapa de paquetes y reglas de dependencias verificables, con acceso SQL encapsulado por dueño de datos.
- **Pruebas esperadas:** revisión de una dependencia ejemplo y límites de transacción externa.
- **Riesgo:** SDK cloud en dominio o workers sin estado durable. **Estado:** `review`; especificación documental y ejemplo M05→M06 en `planning/estructura_backend_go.md`; no hay código ni DDL. PR #6 está abierto en `docs/core-be-01-review`. Kanban conserva `done`; `request-review t_39a64c51` falló porque la tarea no está en running/ready (o no coincide el run). Se añadió comentario por la operación CLI admitida con el enlace del PR; no se cambió el estado ni se editó directamente la base. No marcar `done` hasta la revisión requerida.

### CORE-API-01 — Fijar contrato HTTP, OpenAPI y autorización
- **Módulo/tipo/estado:** Transversal / API / `todo`.
- **Objetivo y motivación:** sostener RNF-032 y validar clientes exclusivamente por HTTP/JSON.
- **Alcance:** definir versionado, schema de error, autenticación, autorización por recurso, paginación, money/time format, headers de correlación/idempotencia y errores de conflicto.
- **Fuera de alcance:** rutas de todos los módulos, HTML/HTMX o necesidades exclusivas del mock.
- **Dependencias:** CORE-BE-01. **Desbloquea:** ADMIN-API-01, AUTH-API-01, AUTH-API-02, AUTH-ARCH-01, BOOK-API-01, BOOK-API-02, COMM-API-01, CONT-API-01, CORE-ENV-01, CORE-TEST-01, DIS-API-01, DISC-API-01, KYC-API-01, LIST-API-01, LIST-API-02, OPS-API-01, PRIV-API-01
- **Aceptación:** OpenAPI base de error/auth/versionado validable y guía para definir rutas específicas en tickets de módulo.
- **Pruebas esperadas:** lint/parse OpenAPI y ejemplos de 2xx/4xx/5xx sin secretos.
- **Riesgo:** diseñar contrato sin flujos de CU o exponer datos vinculables. **Estado:** todo.

### CORE-TEST-01 — Definir harness automatizado de DB y API
- **Módulo/tipo/estado:** Transversal / TEST / `todo`.
- **Objetivo y motivación:** habilitar pruebas deterministas contra PostgreSQL real de test y contratos HTTP.
- **Alcance:** decidir fixtures sintéticos, aislamiento/reset, ejecución unitaria/integración, comprobación migración desde vacío y artefactos de evidencia.
- **Fuera de alcance:** pruebas de producción, carga medida o proveedores sin sandbox.
- **Dependencias:** CORE-DB-03, CORE-API-01. **Desbloquea:** ADMIN-TEST-01, AUTH-TEST-01, BOOK-TEST-01, BOOK-TEST-02, COMM-TEST-01, CONT-TEST-01, CORE-ENV-01, DIS-TEST-01, DISC-TEST-01, KYC-TEST-01, LIST-TEST-01, LIST-TEST-02, OPS-TEST-01, PRIV-TEST-01
- **Aceptación:** procedimiento que separa unit, DB integration y API contract; ningún secreto/PII real.
- **Pruebas esperadas:** ejecutar smoke de conexión, limpieza de fixtures y fallo visible del test runner.
- **Riesgo:** tests usando SQLite no ejercitarían rangos/exclusión PostgreSQL. **Estado:** todo.

### CORE-ENV-01 — Planificar entorno local aislado database/backend/mock
- **Módulo/tipo/estado:** Transversal / ARCH / `todo`.
- **Objetivo y motivación:** preparar el objetivo `docker compose up` y RNF-035 más requisito actual del mock aislado.
- **Alcance:** especificar servicios, redes, readiness, variables de entorno sin secretos, puertos, DB de test, CORS y dependencia mock→API.
- **Fuera de alcance:** editar Docker, crear compose o levantar contenedores durante esta ejecución de planificación.
- **Dependencias:** CORE-DB-03, CORE-API-01, CORE-TEST-01. **Desbloquea:** ADMIN-MOCK-01, AUTH-MOCK-01, BOOK-MOCK-01, COMM-MOCK-01, CONT-MOCK-01, DIS-MOCK-01, DISC-MOCK-01, KYC-MOCK-01, LIST-MOCK-01, OPS-MOCK-01, PRIV-MOCK-01
- **Aceptación:** diagrama/contrato local sin ruta mock→DB y checklist para que un futuro ticket lo implemente.
- **Pruebas esperadas:** en la futura implementación: health checks, red sin acceso DB desde mock, `docker compose up` desde limpio.
- **Riesgo:** publicar DB al host/red del mock o filtrar secretos por env. **Estado:** todo.

## M01 — Identidad y gestión de cuenta

**Trazabilidad:** RQF-001–023, 186–188, 213–218; CU-01–06 y CU-50; HU01–03. Entidades: usuario, rol_usuario, sesion, token_accion, version_terminos, aceptacion_terminos.

### AUTH-ARCH-01 — Acordar invariantes de cuenta y sesión
- **Tipo/estado:** ARCH / todo. **Objetivo:** concretar registro, aceptación versionada de términos, login, revocación y recuperación.
- **Motivación:** M01 y requisitos de credenciales/roles. **Alcance:** flujos, estados, límites de intento, hashing y tokens de un solo uso.
- **Fuera de alcance:** SSO o proveedor externo no acordado. **Dep:** CORE-API-01, CORE-DB-02. **Desbloquea:** AUTH-DB-01
- **Aceptación:** casos de estado y amenazas acordados, sin guardar contraseña/token en claro. **Pruebas:** matriz de flujos éxito/fallo. **Riesgo:** políticas contradigan seguridad ES1. Estado `todo`.

### AUTH-DB-01 — Diseñar persistencia de cuenta, términos y sesión
- **Tipo/estado:** DB / todo. **Objetivo/traza:** RQF-001–023, 186–188, 213–218; CU-01–06/50.
- **Alcance:** especificar PK/UK/casefold, estados, revocación, expiración, índice de tokens y FK restrictivas para las seis tablas M01.
- **Fuera de alcance:** almacenar contraseña/token legibles o introducir perfiles/KYC. **Dep:** AUTH-ARCH-01, CORE-DB-03. **Desbloquea:** AUTH-DB-02
- **Aceptación:** esquema revisado contra Anexo B y reglas de privacidad. **Pruebas:** revisar duplicado de correo, expiración y restricciones. **Riesgo:** enumeración de cuentas. Estado `todo`.

### AUTH-DB-02 — Crear migraciones iniciales de M01
- **Tipo/estado:** DB / todo. **Objetivo:** persistir usuario/roles/sesión/tokens/términos.
- **Alcance:** DDL versionado, constraints/índices y catálogo mínimo de términos de prueba sintético.
- **Fuera:** datos de personas reales, lógica de autenticación. **Dep:** AUTH-DB-01. **Desbloquea:** AUTH-BE-01
- **Aceptación:** aplica desde vacío, repetición controlada no duplica y constraints frenan duplicados/estados inválidos. **Pruebas:** migración up/forward-fix e integridad. **Riesgo:** no aplicar hasta que el runner de CORE-DB-03 pueda ejecutar y verificar migraciones contra PostgreSQL. Estado `todo`.

### AUTH-BE-01 — Implementar tipos de dominio y repositorio de cuenta
- **Tipo/estado:** BE / todo. **Objetivo/traza:** estados de cuenta/sesión de RQF-001–023/213–218.
- **Alcance:** tipos, puertos, repositorios sqlc/pgx y transacciones explícitas para cuenta/sesión/términos.
- **Fuera:** handlers y UI. **Dep:** AUTH-DB-02, CORE-BE-01. **Desbloquea:** AUTH-BE-02, AUTH-BE-03
- **Aceptación:** dominio no importa HTTP/GCP; secretos se redactan; repositorio respeta unicidad y expiración. **Pruebas:** unitarias y DB integration. **Riesgo:** asignar rol admin por registro. Estado `todo`.

### AUTH-BE-02 — Implementar registro, verificación y sesión
- **Tipo/estado:** BE / todo. **Objetivo:** CU-01–04/06, HU01–02.
- **Alcance:** casos de uso de alta, aceptación de versión, verificación de correo según flujo, login/logout y revocación; errores sin revelar si correo existe.
- **Fuera:** restablecimiento de contraseña. **Dep:** AUTH-BE-01. **Desbloquea:** AUTH-API-01
- **Aceptación:** sesión revocable; término aceptado por versión; acceso respeta estado/rol. **Pruebas:** duplicado, credencial mala, bloqueo, logout/replay. **Riesgo:** fuerza bruta y enumeración. Estado `todo`.

### AUTH-BE-03 — Implementar cambio y recuperación de credenciales
- **Tipo/estado:** BE / todo. **Objetivo:** CU-05/50, HU03, RQF-019–022/214–218.
- **Alcance:** emisión/consumo de token de un solo uso, cambio contraseña, revocación de sesiones que corresponda y expiración.
- **Fuera:** email real sin adaptador. **Dep:** AUTH-BE-01. **Desbloquea:** AUTH-API-02
- **Aceptación:** token expirado/reutilizado falla y nunca aparece en logs. **Pruebas:** expiración, repetición, contraseña actual/nueva y sesiones. **Riesgo:** secuestro/reuso de token. Estado `todo`.

### AUTH-API-01 — Exponer endpoints de registro y sesión
- **Tipo/estado:** API / todo. **Objetivo/traza:** CU-01–04/06.
- **Alcance:** rutas OpenAPI para registro/aceptación/verify/login/logout, schemas, auth y códigos de error.
- **Fuera:** HTML, persistencia directa desde handler. **Dep:** AUTH-BE-02, CORE-API-01. **Desbloquea:** AUTH-TEST-01
- **Aceptación:** OpenAPI refleja response/error; logout y recursos privados aplican auth. **Pruebas:** contrato 2xx/4xx/401, validación de payload. **Riesgo:** cookies/tokens deben respetar política definida. Estado `todo`.

### AUTH-API-02 — Exponer endpoints de cambio/recuperación
- **Tipo/estado:** API / todo. **Objetivo/traza:** CU-05/50.
- **Alcance:** request de recuperación, consumo del token y cambio de contraseña autenticado, con throttling definido en diseño.
- **Fuera:** servicio externo de email. **Dep:** AUTH-BE-03, CORE-API-01. **Desbloquea:** AUTH-TEST-01
- **Aceptación:** respuesta no permite enumeración; token no se retorna en entornos reales. **Pruebas:** contrato expiración/reuso/rate. **Riesgo:** abuso de endpoint. Estado `todo`.

### AUTH-TEST-01 — Probar ciclo de identidad de extremo a extremo
- **Tipo/estado:** TEST / todo. **Objetivo:** evidenciar CU-01–06/50.
- **Alcance:** unitarias + DB/API integration desde alta a cierre/recuperación con fixtures sintéticos.
- **Fuera:** proveedor de correo/producto real. **Dep:** AUTH-API-01, AUTH-API-02, CORE-TEST-01. **Desbloquea:** AUTH-MOCK-01
- **Aceptación:** caminos felices y fallidos reproducibles; suite verifica permisos y datos persistidos. **Pruebas:** test automatizado por flujos. **Riesgo:** falsos positivos si no ejecuta contra PostgreSQL. Estado `todo`.

### AUTH-MOCK-01 — Validar visualmente cuenta y sesión
- **Tipo/estado:** otro (MOCK) / todo. **Objetivo:** permitir validación temprana de M01.
- **Alcance:** formularios mínimos de registro/login/logout/recuperación y salida legible de responses/errores.
- **Fuera:** diseño/UX definitivo y acceso DB. **Dep:** AUTH-TEST-01, CORE-ENV-01. **Desbloquea:** KYC-ARCH-01, PRIV-ARCH-01
- **Aceptación:** solo HTML/CSS/TS/DOM/fetch; contenedor separado; respeta auth/tokens; funciona en entorno local previsto. **Pruebas:** ejecutar operaciones HTTP, 401/422 y error de red. **Riesgo:** no persistir token inseguramente; aplicar reglas de auth decididas. Estado `todo`.

## M02 — Perfil y privacidad del usuario

**Trazabilidad:** RQF-024–037, 189–191; CU-07–09; HU31–32. Entidades: perfil_usuario, cuenta_cobro, solicitud_titular.

### PRIV-ARCH-01 — Definir contrato de perfil, cobro y derechos
- **Tipo/estado:** ARCH / todo. **Objetivo:** acordar actualización de perfil, acceso a datos de cobro y ciclo de derecho del titular.
- **Alcance:** actor/finalidad, campos mínimos, acceso a cuenta bancaria, roles, retención y estados solicitud. **Fuera:** borrar en cascada y decidir base jurídica por inferencia.
- **Dep:** AUTH-MOCK-01, CORE-DB-02. **Desbloquea:** PRIV-DB-01
- **Aceptación:** matriz de campos/accesos/plazos con pendientes legales explicitados. **Pruebas:** revisión de CU-07–09/RQF. **Riesgo:** eliminación incompatible con obligaciones históricas. Estado `todo`.

### PRIV-DB-01 — Diseñar perfil, cuenta de cobro y solicitud de titular
- **Tipo/estado:** DB / todo. **Objetivo/traza:** tablas M02 contra RQF-024–037/189–191.
- **Alcance:** nulabilidad, owner, cifrado/huella si aplica, FK restrictiva, estados y vínculos de solicitud.
- **Fuera:** guardar credenciales bancarias/token proveedor en claro. **Dep:** PRIV-ARCH-01, CORE-DB-03. **Desbloquea:** PRIV-DB-02
- **Aceptación:** campos/finalidad/acceso trazables al Anexo B. **Pruebas:** constraint/FK y matriz de acceso revisadas. **Riesgo:** PII vinculable/retención indeterminada. Estado `todo`.

### PRIV-DB-02 — Migrar persistencia M02
- **Tipo/estado:** DB / todo. **Objetivo:** aplicar perfil, cuenta de cobro y solicitud de titular.
- **Alcance:** DDL e índices iniciales, sin secretos externos ni seed personal. **Fuera:** automatizar borrado legal.
- **Dep:** PRIV-DB-01. **Desbloquea:** PRIV-BE-01. **Aceptación:** migración reproducible y no cascada sobre hechos. **Pruebas:** desde cero, FK y clasificación de datos. **Riesgo:** políticas de retención abiertas. Estado `todo`.

### PRIV-BE-01 — Implementar dominio y persistencia de perfil/cobro
- **Tipo/estado:** BE / todo. **Objetivo/traza:** CU-07/08, HU31.
- **Alcance:** leer/editar perfil y administrar alta/cambio/revocación de cuenta de cobro bajo actor propietario.
- **Fuera:** efectuar payout. **Dep:** PRIV-DB-02, CORE-BE-01. **Desbloquea:** PRIV-BE-02
- **Aceptación:** autorización por dueño en consulta/comando; datos financieros redactados. **Pruebas:** dueño/ajeno, actualización y revocación. **Riesgo:** ID del cliente no es autoridad. Estado `todo`.

### PRIV-BE-02 — Implementar solicitud y gestión de derechos
- **Tipo/estado:** BE / todo. **Objetivo:** CU-09/HU32, PT-16, RNF-018/026/029.
- **Alcance:** registrar y tramitar solicitud de acceso/eliminación según procedimiento acordado, evaluar dependencias de retención y desidentificación.
- **Fuera:** supresión automática irreversible antes de validación de fundamento/plazos. **Dep:** PRIV-BE-01. **Desbloquea:** PRIV-API-01
- **Aceptación:** solicitud auditable, estado/motivo y revisión de copias/derivados; decisión fundada trazable. **Pruebas:** acceso ajeno, estado pendiente y casos retenidos. **Riesgo:** anonimización no elimina vínculo si UUID/derivados persisten. Estado `todo`.

### PRIV-API-01 — Exponer API de perfil y privacidad
- **Tipo/estado:** API / todo. **Objetivo/traza:** CU-07–09.
- **Alcance:** endpoints mínimos OpenAPI de lectura/actualización de perfil, cuenta de cobro y solicitud/consulta de derechos.
- **Fuera:** datos bancarios completos, HTML. **Dep:** PRIV-BE-02, CORE-API-01. **Desbloquea:** PRIV-TEST-01
- **Aceptación:** autorización por recurso y campos sensibles no se devuelven sin finalidad/rol. **Pruebas:** schemas, 401/403/404 y validaciones. **Riesgo:** fuga por serializer. Estado `todo`.

### PRIV-TEST-01 — Probar perfil y ejercicio de derechos
- **Tipo/estado:** TEST / todo. **Objetivo:** demostrar CU-07–09/PT-16 a nivel disponible.
- **Alcance:** unitarias y DB/API de propietario/ajeno, modificación, baja con hechos retenidos y auditabilidad.
- **Fuera:** declarar cumplimiento legal. **Dep:** PRIV-API-01, CORE-TEST-01. **Desbloquea:** PRIV-MOCK-01
- **Aceptación:** datos de prueba sintéticos; casos de retención sin borrado destructivo. **Pruebas:** matriz permisos/estados. **Riesgo:** criterio jurídico requiere confirmación. Estado `todo`.

### PRIV-MOCK-01 — Validar perfil y solicitud de derechos
- **Tipo/estado:** MOCK / todo. **Objetivo:** validar visualmente API M02.
- **Alcance:** visualizar/actualizar perfil, datos de cobro permitidos, enviar/consultar solicitud.
- **Fuera:** mostrar secretos financieros o diseñar UX final. **Dep:** PRIV-TEST-01, CORE-ENV-01. **Desbloquea:** LIST-ARCH-01
- **Aceptación:** criterios comunes de frontend_mock; no accede a DB. **Pruebas:** 200/401/403/422 y respuesta redacted. **Riesgo:** no guardar token sensible en almacenamiento del browser. Estado `todo`.

## M03 — Verificación de identidad KYC/KYB

**Trazabilidad:** RQF-038–059, 192–194, 219–220; CU-10–14; HU04. Entidades: verificacion, documento y referencia/vínculo de proveedor.

### KYC-ARCH-01 — Definir flujos de verificación y revisión
- **Tipo/estado:** ARCH / todo. **Objetivo:** separar KYC persona, KYB empresa, revisión manual y reintento.
- **Alcance:** estados, evidencia mínima, acceso, vendor ports, retención, consentimiento y alternativa manual. **Fuera:** afirmar acceso API Registro Civil/SII.
- **Dep:** AUTH-MOCK-01, CORE-DB-02. **Desbloquea:** KYC-DB-01
- **Aceptación:** flujo CU-10–14 con capacidades confirmadas vs pendientes identificadas. **Pruebas:** análisis de casos/estados. **Riesgo:** documentos de identidad de alta sensibilidad. Estado `todo`.

### KYC-DB-01 — Diseñar verificaciones y referencias de evidencia
- **Tipo/estado:** DB / todo. **Objetivo/traza:** RQF-038–059/192–194/219–220.
- **Alcance:** solicitante, tipo, proveedor, estado, resolución/motivo, fechas, referencia a documento privado y retención/finalidad.
- **Fuera:** imagen/documento binario en PostgreSQL. **Dep:** KYC-ARCH-01, CORE-DB-03. **Desbloquea:** KYC-DB-02
- **Aceptación:** reconciliado con tabla verificacion y clases P/R del Anexo B. **Pruebas:** FK, estados, actor y acceso revisados. **Riesgo:** conservar más datos KYC de los necesarios. Estado `todo`.

### KYC-DB-02 — Migrar persistencia de verificación
- **Tipo/estado:** DB / todo. **Objetivo:** materializar modelo mínimo KYC/KYB.
- **Alcance:** migración de tablas/índices y metadatos de evidencia sin PII real. **Fuera:** integración/proveedor.
- **Dep:** KYC-DB-01. **Desbloquea:** KYC-BE-01. **Aceptación:** migración nueva reproducible; acceso a referencias controlado. **Pruebas:** desde vacío, FK y estado final exige resolución. **Riesgo:** exportar PII en logs. Estado `todo`.

### KYC-BE-01 — Implementar dominio y repositorio de verificación
- **Tipo/estado:** BE / todo. **Objetivo:** estados KYC/KYB y storage privado detrás de puerto.
- **Alcance:** entidades, comandos, repositorio y adaptadores fake; revisión manual con autorización.
- **Fuera:** SDK/credenciales Registro Civil/SII. **Dep:** KYC-DB-02, CORE-BE-01. **Desbloquea:** KYC-BE-02
- **Aceptación:** adapters reemplazables; evidencia no se ofrece públicamente. **Pruebas:** estados/transiciones y ownership. **Riesgo:** confundir validación simulada con identidad probada. Estado `todo`.

### KYC-BE-02 — Implementar solicitud, revisión y reintento KYC/KYB
- **Tipo/estado:** BE / todo. **Objetivo/traza:** CU-11–14/HU04.
- **Alcance:** crear caso, recibir resultado adaptador, revisión manual, motivo de rechazo y reintento idempotente.
- **Fuera:** determinar elegibilidad comercial/legal fuera de especificación. **Dep:** KYC-BE-01. **Desbloquea:** KYC-API-01
- **Aceptación:** decisión trazable y permiso de publicar/reservar consume estado de verificación aprobado. **Pruebas:** aprobado/rechazado/reintento/timeout fake. **Riesgo:** acceso de operador demasiado amplio. Estado `todo`.

### KYC-API-01 — Exponer endpoints de verificación
- **Tipo/estado:** API / todo. **Objetivo:** iniciar/consultar/reintentar y revisión admin según CU.
- **Alcance:** contrato OpenAPI, metadata de carga autorizada y endpoint de estado; carga binaria sigue política de documento/URL acordada.
- **Fuera:** aceptar payload externo sin autenticación ni revelar documento a otro usuario. **Dep:** KYC-BE-02, CORE-API-01. **Desbloquea:** KYC-TEST-01
- **Aceptación:** cada endpoint tiene authn/authz, límite y errores; no devuelve dato KYC excesivo. **Pruebas:** 401/403/404, limit, estados. **Riesgo:** APIs públicas exponen PII. Estado `todo`.

### KYC-TEST-01 — Probar flujos KYC/KYB con adaptadores fake
- **Tipo/estado:** TEST / todo. **Objetivo:** probar CU-10–14 sin acceso de proveedor.
- **Alcance:** suite dominio/DB/API, evidencia sintética, timeout, duplicado, review y permisos.
- **Fuera:** afirmar integración real. **Dep:** KYC-API-01, CORE-TEST-01. **Desbloquea:** KYC-MOCK-01
- **Aceptación:** cualquier resultado simulado identificado como fixture y no como verificación real. **Pruebas:** matriz de fallos/provider replay. **Riesgo:** fixture filtrada al entorno real. Estado `todo`.

### KYC-MOCK-01 — Validar visualmente verificación
- **Tipo/estado:** MOCK / todo. **Objetivo:** probar API M03.
- **Alcance:** iniciar solicitud sintética, mostrar estado/motivo y validar permiso de revisión.
- **Fuera:** cargar identidad real o producir UX final. **Dep:** KYC-TEST-01, CORE-ENV-01. **Desbloquea:** LIST-ARCH-01
- **Aceptación:** mock etiqueta sandbox/datos sintéticos y respeta roles; criterios comunes. **Pruebas:** 201/403/rechazo/reintento. **Riesgo:** exposición de documentos sensibles. Estado `todo`.

## M04 — Publicaciones, archivos y disponibilidad

**Trazabilidad:** RQF-060–093, 195–198, 221–226; CU-15–18 y coordinación CU-17; HU05–09/HU20–23. HU24 queda fuera de ES1, sin RQF/CU, y requiere decisión/ticket nuevo antes de incluir recurrencia. Entidades: categoria_espacio, espacio, regla_tarifa, politica_cancelacion, tramo_cancelacion, documento; calendario `ocupacion` coordinado con M06.

### LIST-ARCH-01 — Fijar contrato de publicación, tarifa y calendario
- **Tipo/estado:** ARCH / todo. **Objetivo:** delimitar editor del arrendador, ciclo de publicación, reglas de precio y disponibilidad.
- **Alcance:** categorías, unidad exclusiva, zonas horarias, reglas versionadas, bloqueos, galería y ownership del calendario. **Fuera:** reservar desde este módulo.
- **Dep:** PRIV-MOCK-01, KYC-MOCK-01, CORE-DB-02. **Desbloquea:** LIST-DB-01
- **Aceptación:** todos los tipos de ES1 caben en catálogo de datos y calendario tiene owner único con M06. **Pruebas:** revisión CU/RQF y casos modal/intervalo. **Riesgo:** introducir categorías rígidas en esquema. Estado `todo`.

### LIST-DB-01 — Diseñar persistencia de catálogo y publicación
- **Tipo/estado:** DB / todo. **Objetivo/traza:** RQF-060–093/195–198/221–224.
- **Alcance:** categoría, espacio, reglas tarifas/comisión, políticas/tramos cancelación, atributos comunes, datos geoespaciales e índices por consulta.
- **Fuera:** tabla por categoría y telemetría por impresión. **Dep:** LIST-ARCH-01, CORE-DB-03. **Desbloquea:** LIST-DB-02
- **Aceptación:** todas las categorías como datos; price/policy versionadas y snapshots posibles. **Pruebas:** cardinalidad/nulabilidad/consultas justificadas. **Riesgo:** modelo polimórfico sin constraints. Estado `todo`.

### LIST-DB-02 — Migrar catálogo y publicación
- **Tipo/estado:** DB / todo. **Objetivo:** crear catálogo base y tablas de publicación/tarifa/política.
- **Alcance:** DDL, índices iniciales y seed controlado de categorías verificadas del alcance.
- **Fuera:** datos ficticios que pretendan oferta real, ocupación/reserva transaccional. **Dep:** LIST-DB-01. **Desbloquea:** LIST-BE-01, LIST-BE-02
- **Aceptación:** migración reproducible; categoría referenciada; FK restrictivas y unicidad validada. **Pruebas:** migración/seed/integridad. **Riesgo:** lista de categorías requiere verificación ES1. Estado `todo`.

### LIST-BE-01 — Implementar dominio y persistencia de publicación
- **Tipo/estado:** BE / todo. **Objetivo:** alta/edición/publicación/pausa con permiso arrendador verificado.
- **Alcance:** repositorio/casos CRUD; validación de atributos comunes, precios y política versionada.
- **Fuera:** búsqueda pública y generación frontend final. **Dep:** LIST-DB-02, CORE-BE-01. **Desbloquea:** LIST-API-01, LIST-BE-02
- **Aceptación:** actor solo modifica espacio propio; borradores no aparecen activos. **Pruebas:** unitarias/DB owner y versionado. **Riesgo:** cambio de regla no debe alterar cotización existente. Estado `todo`.

### LIST-BE-02 — Implementar archivos y calendario del arrendador
- **Tipo/estado:** BE / todo. **Objetivo/traza:** CU-16/17, HU20–23; HU24 requiere decisión y requisitos nuevos antes de incluir recurrencia.
- **Alcance:** metadata de documento, flujo upload privado con validación, calendario disponible y bloqueos manuales; adaptador storage detrás de puerto.
- **Fuera:** aceptar doble reserva, imágenes públicas por URL permanente. **Dep:** LIST-BE-01, LIST-DB-02. **Desbloquea:** LIST-API-02
- **Aceptación:** permiso y objeto específico, MIME real/tamaño/hash verificados; calendar rules no duplican ocupación M06. **Pruebas:** fake storage y límites de archivo/intervalo. **Riesgo:** URL firmada no equivale a autorización continua. Estado `todo`.

### LIST-API-01 — Exponer endpoints de publicación y reglas
- **Tipo/estado:** API / todo. **Objetivo:** CU-15/18 y CRUD arrendador.
- **Alcance:** OpenAPI para crear/leer/editar/activar/pausar publicación, categoría, tarifas y políticas.
- **Fuera:** endpoint de reserva y acceso a archivos privados ajenos. **Dep:** LIST-BE-01, CORE-API-01. **Desbloquea:** LIST-TEST-01, LIST-TEST-02
- **Aceptación:** validaciones explícitas, ownership, versiones de tarifa/política en response. **Pruebas:** contrato, 403 owner, valores límite. **Riesgo:** APIs de edición publican datos prematuramente. Estado `todo`.

### LIST-API-02 — Exponer endpoints de galería y calendario
- **Tipo/estado:** API / todo. **Objetivo/traza:** CU-16/17.
- **Alcance:** contrato de metadata, autorización/URL de subida temporal y consulta/configuración de disponibilidad/bloqueos según diseño.
- **Fuera:** acceso directo a bucket desde mock, confirmar reservas. **Dep:** LIST-BE-02, CORE-API-01. **Desbloquea:** LIST-TEST-01, LIST-TEST-02
- **Aceptación:** respuestas no exponen URL persistente ni objetos no autorizados. **Pruebas:** límites, rango horario, ACL y firma fake. **Riesgo:** endpoint de calendario compita con `ocupacion` de M06. Estado `todo`.

### LIST-TEST-01 — Probar publicaciones, archivos y disponibilidad
- **Tipo/estado:** TEST / todo. **Objetivo:** validar CU-15–18/17 y RNF-004/041.
- **Alcance:** DB/API integration, permisos, versionado, carga privada y bloqueos, con storage fake.
- **Fuera:** ensayo con oferta/archivo real. **Dep:** LIST-API-01, LIST-API-02, CORE-TEST-01. **Desbloquea:** LIST-MOCK-01
- **Aceptación:** límites de publicación y permisos reproducibles; archivos no servidos antes de validación. **Pruebas:** concurrencia calendario se completa con BOOK-TEST-01. **Riesgo:** expectativa incorrecta de disponibilidad frente a reserva. Estado `todo`.

### LIST-TEST-02 — Probar límites de archivos y bloqueos de calendario
- **Tipo/estado:** TEST / todo. **Objetivo:** validar reglas de carga, rangos configurables y bloqueos manuales de CU-16/17.
- **Alcance:** pruebas de tamaño/tipo/MIME/hash, intervalo vacío/adicional, zona horaria y propietario del bloqueo.
- **Motivación/traza:** RQF-074–085/225–226; RNF-004/041. **Fuera de alcance:** carrera de reserva, cubierta por BOOK-TEST-01.
- **Dependencias:** LIST-API-01, LIST-API-02, CORE-TEST-01. **Desbloquea:** LIST-MOCK-01
- **Aceptación:** límites declarados se verifican y no se marca disponible objeto no validado. **Pruebas esperadas:** unitarias/fake storage y DB/API integración de bloqueos. **Riesgo:** MIME declarado por cliente no es tipo real. **Estado:** todo.

### LIST-MOCK-01 — Validar visualmente publicación
- **Tipo/estado:** MOCK / todo. **Objetivo:** probar creación/edición y consulta de disponibilidad via API.
- **Alcance:** formulario simple, listado propio, detalle, archivos sintéticos y bloqueos manuales.
- **Fuera:** CRUD que API no exponga; UX de producción. **Dep:** LIST-TEST-01, LIST-TEST-02, CORE-ENV-01. **Desbloquea:** DISC-ARCH-01
- **Aceptación:** criterios comunes, auth por rol y ninguna DB/storage interna accesible. **Pruebas:** éxito y 400/401/403/409/413. **Riesgo:** URLs de objeto/token no persistidas por el navegador. Estado `todo`.

## M05 — Búsqueda y cotización

**Trazabilidad:** RQF-094–107; CU-19–21; HU10–14. Entidades: cotizacion y lecturas autorizadas de espacio/categoría/tarifas/políticas/ocupación.

### DISC-ARCH-01 — Definir filtros, búsqueda geográfica y cotización
- **Tipo/estado:** ARCH / todo. **Objetivo:** fijar criterios de consulta y cálculo previo sin retener inventario.
- **Alcance:** filtros allowlist, distancia/ubicación, paginación, zona horaria, versión de precio y vencimiento. **Fuera:** ranking patrocinado sin regla aprobada.
- **Dep:** LIST-MOCK-01. **Desbloquea:** DISC-DB-01
- **Aceptación:** formato de request/response y límites de búsqueda definidos contra CU-19–21. **Pruebas:** casos sin disponibilidad/precio/fecha y precisión monetaria. **Riesgo:** inyección SQL en filtros dinámicos. Estado `todo`.

### DISC-DB-01 — Diseñar persistencia de cotización e índices de lectura
- **Tipo/estado:** DB / todo. **Objetivo/traza:** RQF-094–107.
- **Alcance:** snapshot cotización, vencimiento, monto exacto, relación a tarifa/política y consultas geoespaciales justificadas.
- **Fuera:** persistir búsquedas anónimas indefinidamente. **Dep:** DISC-ARCH-01, CORE-DB-03. **Desbloquea:** DISC-DB-02
- **Aceptación:** cotización no bloquea ocupación y puede reproducir cálculo. **Pruebas:** review de cardinalidad/índices. **Riesgo:** historial puede contener ubicación/interés personal. Estado `todo`.

### DISC-DB-02 — Migrar cotizaciones e índices iniciales
- **Tipo/estado:** DB / todo. **Objetivo:** materializar snapshot y requerimientos de lectura.
- **Alcance:** DDL/índices justificados y consultas EXPLAIN con fixtures sintéticos. **Fuera:** prometer rendimiento RNF sin benchmark.
- **Dep:** DISC-DB-01. **Desbloquea:** DISC-BE-01. **Aceptación:** migración reproducible y query plan registrado. **Pruebas:** desde vacío, precision/expiry. **Riesgo:** índices prematuros aumentan costo de escritura. Estado `todo`.

### DISC-BE-01 — Implementar búsqueda, detalle y cálculo de cotización
- **Tipo/estado:** BE / todo. **Objetivo:** CU-19–21.
- **Alcance:** filtros allowlist, lectura de espacio activo, disponibilidad, tarifas y cálculo exacto con versión vigente.
- **Fuera:** retener espacio o confirmar reserva. **Dep:** DISC-DB-02, CORE-BE-01. **Desbloquea:** BOOK-BE-02, DISC-API-01
- **Aceptación:** búsqueda no incluye recursos no publicados; cotización registra desglose y vencimiento, sin modificar ocupación. **Pruebas:** filtros, zona IANA, precio versionado. **Riesgo:** race entre cotización y reserva se resuelve al reservar. Estado `todo`.

### DISC-API-01 — Exponer endpoints de búsqueda y cotización
- **Tipo/estado:** API / todo. **Objetivo/traza:** RQF-094–107.
- **Alcance:** OpenAPI para listado/filtros, detalle, disponibilidad consultiva y cotización.
- **Fuera:** contrato de UI/mapeas visuales. **Dep:** DISC-BE-01, CORE-API-01. **Desbloquea:** DISC-TEST-01
- **Aceptación:** filtros permitidos y paginación documentada, request JSON, errores claros; visitante puede consultar lo público. **Pruebas:** schemas, entradas malformadas, auth cuando el caso lo requiere. **Riesgo:** geolocalización precisa puede ser restringida. Estado `todo`.

### DISC-TEST-01 — Probar búsqueda y cotización
- **Tipo/estado:** TEST / todo. **Objetivo:** probar CU-19–21/RNF-001 (funcionalidad y medición después).
- **Alcance:** unitarias/DB/API con zonas y fixtures; asserts monetarios y expiración.
- **Fuera:** declarar SLA/rendimiento sin carga comparable. **Dep:** DISC-API-01, CORE-TEST-01. **Desbloquea:** DISC-MOCK-01
- **Aceptación:** precio reproducible con datos sintéticos y no reserva inventario. **Pruebas:** filtros, sin resultados, límites y expiración. **Riesgo:** RNF-001 requiere benchmark separado. Estado `todo`.

### DISC-MOCK-01 — Validar visualmente búsqueda y cotización
- **Tipo/estado:** MOCK / todo. **Objetivo:** probar API pública M05.
- **Alcance:** búsqueda con formulario simple, resultados, detalle, selección de rango y desglose de cotización.
- **Fuera:** mapa/UX final o crear reserva. **Dep:** DISC-TEST-01, CORE-ENV-01. **Desbloquea:** BOOK-ARCH-01
- **Aceptación:** criterios comunes; refleja exactamente response y errores. **Pruebas:** no result, quote válida, 422 y timeout. **Riesgo:** no mostrar cotización expirada como reservable. Estado `todo`.

## M06 — Reservas y pagos

**Trazabilidad:** RQF-108–129, 199–201, 227–231; CU-22–28,47,51; HU15–19. Entidades: reserva, ocupacion, reserva_transicion, pago, evento_proveedor, movimiento_financiero, garantia, liquidacion y snapshots.

### BOOK-ARCH-01 — Acordar máquina de estados e invariantes transaccionales
- **Tipo/estado:** ARCH / todo. **Objetivo:** determinar quién puede transicionar reserva, ocupación y pago; integrar el proveedor sin asumir Escrow.
- **Alcance:** matriz transición/actor/condición, timeout, idempotency key, conflicto de rango, outbox/inbox y conciliación.
- **Fuera:** fijar capacidad no verificada de Mercado Pago o capturar fondos reales. **Dep:** DISC-MOCK-01, CORE-ARCH-01. **Desbloquea:** BOOK-DB-01
- **Aceptación:** estados de reserva/pago/garantía diferenciados; falla remota queda conciliable. **Pruebas:** secuencia concurrente y webhook tardío simulados en diseño. **Riesgo:** operación proveedor no es reversible por DB rollback. Estado `todo`.

### BOOK-DB-01 — Diseñar calendario de reserva y snapshots
- **Tipo/estado:** DB / todo. **Objetivo/traza:** RQF-108–113/120/227–231, RNF-001/030.
- **Alcance:** reserva/ocupación/transición, constraint EXCLUDE parcial GiST, FK compuesta, ranges finitos `[inicio,fin)`, índices, expiry y snapshot.
- **Fuera:** relajar la restricción por rapidez o duplicar calendario. **Dep:** BOOK-ARCH-01, CORE-DB-03. **Desbloquea:** BOOK-DB-02
- **Aceptación:** mismo espacio no acepta ocupaciones activas solapadas; adyacentes permitidas; cadena transición atomic. **Pruebas:** inspección del DDL previsto/concurrencia en plan. **Riesgo:** extensión `btree_gist` y error SQLSTATE `23P01`. Estado `todo`.

### BOOK-DB-02 — Migrar calendario y reserva
- **Tipo/estado:** DB / todo. **Objetivo:** persistir reserva, ocupación y transición antes de habilitar operaciones.
- **Alcance:** migración con EXCLUDE/FK/index, estado inicial y datos sintéticos.
- **Fuera:** tabla de pagos/contrato si no está en migración independiente. **Dep:** BOOK-DB-01. **Desbloquea:** BOOK-BE-01, BOOK-BE-03
- **Aceptación:** migración limpia y restricción activa antes del endpoint de reservar. **Pruebas:** adyacencia, solape, transacción y carrera de dos conexiones. **Riesgo:** no traducir conflicto de constraint como éxito/fallo opaco. Estado `todo`.

### BOOK-BE-01 — Implementar dominio y repositorio de reserva/calendario
- **Tipo/estado:** BE / todo. **Objetivo:** mantener disponibilidad bajo concurrencia y estados válidos.
- **Alcance:** repositorio transaccional, versión de reserva, lock/conflicto, `reserva_transicion`, expiración y release atómico.
- **Fuera:** comunicación de red dentro de transacción. **Dep:** BOOK-DB-02, CORE-BE-01. **Desbloquea:** BOOK-BE-02, BOOK-BE-03
- **Aceptación:** rango incompatible nunca confirma; transitions/ocupación se escriben en mismo commit. **Pruebas:** test paralelo en PG real. **Riesgo:** workers no son protección única para expiración. Estado `todo`.

### BOOK-BE-02 — Implementar casos de uso de solicitud y ciclo de reserva
- **Tipo/estado:** BE / todo. **Objetivo/traza:** CU-22/23/26/27/28/47/51.
- **Alcance:** solicitar con cotización vigente, aprobación/rechazo arrendador, consulta/historial y cancelaciones válidas.
- **Fuera:** payment provider. **Dep:** BOOK-BE-01, DISC-BE-01. **Desbloquea:** BOOK-API-01
- **Aceptación:** valida titularidad, términos snapshot y disponibilidad revalidada; cada transición auditada. **Pruebas:** estado inválido, vencimiento y rollback. **Riesgo:** cotización stale/no asumir espacio bloqueado durante cotización. Estado `todo`.

### BOOK-BE-03 — Implementar pagos idempotentes, inbox y conciliación
- **Tipo/estado:** BE / todo. **Objetivo:** modelar intentos y hechos externos sin inventar garantía/custodia.
- **Alcance:** puerto de payment, pago/inbox webhook, dedup, estados por conciliar, reconciler durable y fake provider; sandbox adapter separado tras evidencia.
- **Fuera:** manejar PAN/tarjetas o liberar dinero por evento no autenticado. **Dep:** BOOK-DB-02, BOOK-BE-01, CORE-BE-01. **Desbloquea:** BOOK-API-02, DIS-BE-02
- **Aceptación:** idempotency key estable; webhook autenticado/deduplicado; timeout no dispara nuevo cobro. **Pruebas:** duplicado, replay, timeout y saldo insuficiente fake. **Riesgo:** Split/Bricks/medios y tarifas siguen sujetos a confirmación externa. Estado `todo`.

### BOOK-API-01 — Exponer APIs de disponibilidad y reserva
- **Tipo/estado:** API / todo. **Objetivo/traza:** CU-22/23/26/27/28/47/51.
- **Alcance:** endpoints OpenAPI para crear/consultar/aprobar/rechazar/cancelar reserva y consultar mis reservas; conflicto 409 documentado.
- **Fuera:** endpoint de pago/captura de fondos. **Dep:** BOOK-BE-02, CORE-API-01. **Desbloquea:** BOOK-TEST-01
- **Aceptación:** authz por participante y respuesta incluye estado/intervalo confirmado. **Pruebas:** 401/403/404/409/422 y contrato. **Riesgo:** no filtrar reserva ajena. Estado `todo`.

### BOOK-API-02 — Exponer APIs de pago y recepción de eventos
- **Tipo/estado:** API / todo. **Objetivo/traza:** CU-24/25 y RQF-114–120.
- **Alcance:** iniciar/consultar intento de pago y webhook proveedor según firma/idempotencia acordadas.
- **Fuera:** aceptar webhook anónimo, enviar datos de tarjeta o prometer Escrow. **Dep:** BOOK-BE-03, CORE-API-01. **Desbloquea:** BOOK-TEST-02
- **Aceptación:** schema y manejo de duplicado/tardío/por conciliar documentados. **Pruebas:** firma inválida, replay y respuesta timeout. **Riesgo:** endpoint depende de contrato sandbox aún no cerrado. Estado `todo`.

### BOOK-TEST-01 — Probar reserva y concurrencia PostgreSQL
- **Tipo/estado:** TEST / todo. **Objetivo:** validar no-overbooking y lifecycle CU-22/23/26–28/47/51.
- **Alcance:** múltiples conexiones, límites semiabiertos, transición, cancelación, expiración y actor.
- **Fuera:** carga 500 usuarios sin entorno comparable. **Dep:** BOOK-API-01, CORE-TEST-01. **Desbloquea:** BOOK-MOCK-01
- **Aceptación:** exactamente una solicitud concurrente incompatible confirma; evidencia de SQLSTATE/traces sintéticas. **Pruebas:** MD-01–MD-13 aplicables, especialmente rangos y atomicidad. **Riesgo:** tests seriales no reproducen carrera. Estado `todo`.

### BOOK-TEST-02 — Probar pago, webhook y conciliación
- **Tipo/estado:** TEST / todo. **Objetivo:** validar RQF pago con provider fake y sandbox si está habilitado.
- **Alcance:** repetición idempotente, evento duplicado/tardío, timeout y conciliación/compensación.
- **Fuera:** declarar liquidación real a partir de simulator. **Dep:** BOOK-API-02, CORE-TEST-01. **Desbloquea:** BOOK-MOCK-01
- **Aceptación:** resultado remoto y local distinguidos; sandbox reportado aparte de fake. **Pruebas:** matriz idempotencia, firma y por conciliar. **Riesgo:** pagos requieren respuesta escrita/proveedor/test accounts. Estado `todo`.

### BOOK-MOCK-01 — Validar visualmente reserva y pago
- **Tipo/estado:** MOCK / todo. **Objetivo:** probar los endpoints M06 con contratos existentes.
- **Alcance:** elegir disponibilidad, crear/consultar/cancelar reserva, iniciar pago de prueba y mostrar estado/response/error.
- **Fuera:** frontend producto, cobro real, bypass de auth. **Dep:** BOOK-TEST-01, BOOK-TEST-02, CORE-ENV-01. **Desbloquea:** COMM-ARCH-01, CONT-ARCH-01, OPS-ARCH-01
- **Aceptación:** criterios comunes; pago solo sandbox/fake claramente etiquetado; no DB. **Pruebas:** 409 concurrente, estados y webhook simulado/autorizado. **Riesgo:** no confundir simulación con confirmación comercial. Estado `todo`.

## M07 — Contratos y firma

**Trazabilidad:** RQF-130–142/202; CU-29–32; HU33. Entidades: contrato, firma_contrato, documento.

### CONT-ARCH-01 — Definir ciclo de contrato y proveedor neutral
- **Tipo/estado:** ARCH / todo. **Objetivo:** snapshots legales, versiones, firmas parciales/rechazadas y resguardo.
- **Alcance:** datos desde reserva, template versionado, proveedor interface, correlación, retención y estados. **Fuera:** interpretar validez legal automáticamente.
- **Dep:** BOOK-MOCK-01. **Desbloquea:** CONT-DB-01
- **Aceptación:** relación con reserva/partes y qué constituye documento final definida; proveedor pendiente señalado. **Pruebas:** revisión CU-29–32. **Riesgo:** firma avanzada/API/costo no confirmado. Estado `todo`.

### CONT-DB-01 — Diseñar contrato, firmas y evidencias documentales
- **Tipo/estado:** DB / todo. **Objetivo/traza:** RQF-130–142/202.
- **Alcance:** relaciones a reserva/parte/documento, versión de plantilla y estado por firmante, hash/referencia externa.
- **Fuera:** PDF binario o URL firmada en tabla. **Dep:** CONT-ARCH-01, CORE-DB-03. **Desbloquea:** CONT-DB-02
- **Aceptación:** historia de firmas legible y hechos previos no se sobrescriben. **Pruebas:** cardinalidades y retención revisadas. **Riesgo:** distinguir evidencia técnica de validez jurídica. Estado `todo`.

### CONT-DB-02 — Migrar persistencia de contrato/firma
- **Tipo/estado:** DB / todo. **Objetivo:** crear tablas y referencias de contrato.
- **Alcance:** DDL/migración reproducible y constraints. **Fuera:** integrar firma. **Dep:** CONT-DB-01. **Desbloquea:** CONT-BE-01
- **Aceptación:** FK a reserva y documento, versión preservada, sin borrado cascada. **Pruebas:** migración desde cero, estados/fk. **Riesgo:** retención mínima requiere revisar tratamiento. Estado `todo`.

### CONT-BE-01 — Implementar dominio, repositorio y generación de snapshot
- **Tipo/estado:** BE / todo. **Objetivo:** CU-29 y consistencia de contrato con reserva.
- **Alcance:** armar snapshot con valores aceptados y puerto de generación/document storage; template versionado.
- **Fuera:** editor web o microservicio PDF definido por README antiguo. **Dep:** CONT-DB-02, CORE-BE-01. **Desbloquea:** CONT-BE-02
- **Aceptación:** cambios futuros en espacio/precio no alteran contrato ya solicitado; acceso autorizado. **Pruebas:** snapshot y hash fake. **Riesgo:** PDF proveedor/renderer y contenido jurídico requieren decisión. Estado `todo`.

### CONT-BE-02 — Implementar orquestación de firma y descarga
- **Tipo/estado:** BE / todo. **Objetivo/traza:** CU-30–32/HU33.
- **Alcance:** solicitud, callbacks/webhooks idempotentes, rechazo/reintento, estado por firmante y acceso al firmado.
- **Fuera:** credenciales/proveedor real no habilitado. **Dep:** CONT-BE-01. **Desbloquea:** CONT-API-01
- **Aceptación:** firma parcial/final reconciliable y documentos solo para participantes autorizados. **Pruebas:** callbacks duplicados/tardíos/firmas falsas. **Riesgo:** validez y API requieren tercero. Estado `todo`.

### CONT-API-01 — Exponer contrato y firma
- **Tipo/estado:** API / todo. **Objetivo:** CU-29–32.
- **Alcance:** endpoints para generar/consultar/enviar firma y descargar resultado según permisos, callback proveedor separado.
- **Fuera:** exponer URL permanente. **Dep:** CONT-BE-02, CORE-API-01. **Desbloquea:** CONT-TEST-01
- **Aceptación:** operaciones idempotentes donde proceda, acceso participante, OpenAPI vigente. **Pruebas:** 401/403/404/409 y response callback. **Riesgo:** URLs firmadas y retención. Estado `todo`.

### CONT-TEST-01 — Probar ciclo contractual con proveedor fake
- **Tipo/estado:** TEST / todo. **Objetivo:** validar generación/firma sin afirmar efecto jurídico.
- **Alcance:** snapshots, partial/reject/final, auth, documentos y repetición callback.
- **Fuera:** integración productiva. **Dep:** CONT-API-01, CORE-TEST-01. **Desbloquea:** CONT-MOCK-01
- **Aceptación:** cada caso registra fake vs sandbox y conserva evidencia sintética. **Pruebas:** contrato y privacidad/retención. **Riesgo:** no sustituye revisión profesional. Estado `todo`.

### CONT-MOCK-01 — Validar visualmente contrato y firma
- **Tipo/estado:** MOCK / todo. **Objetivo:** inspeccionar estados y recursos del API M07.
- **Alcance:** solicitar documento de reserva de prueba, mostrar estado de firmas y descargar artefacto sintético permitido.
- **Fuera:** editor/UX final ni envío de firma real. **Dep:** CONT-TEST-01, CORE-ENV-01. **Desbloquea:** OPS-ARCH-01
- **Aceptación:** criterios comunes y control por participante; sin archivo sensible. **Pruebas:** firma parcial/rechazo/403/download. **Riesgo:** mock no debe guardar contrato real. Estado `todo`.

## M08 — Operación del arriendo

**Trazabilidad:** RQF-143–152/203–206; CU-33/34/48; HU35. Entidades: operacion_arriendo, documento, reserva_transicion.

### OPS-ARCH-01 — Definir check-in/out y confirmación de recepción
- **Tipo/estado:** ARCH / todo. **Objetivo:** fijar actor, ventana, evidencia, ubicación y excepciones de operación.
- **Alcance:** transiciones vinculadas a reserva/contrato, autorización, entrega/retorno y confirmación/objeción. **Fuera:** avanzar estado automáticamente solo por reloj.
- **Dep:** CONT-MOCK-01, BOOK-MOCK-01. **Desbloquea:** OPS-DB-01
- **Aceptación:** ruta normal y excepción trazadas; evidencia y finalidad/retención precisadas. **Pruebas:** revisión CU. **Riesgo:** geolocalización/fotos sensibles. Estado `todo`.

### OPS-DB-01 — Diseñar operación y evidencia
- **Tipo/estado:** DB / todo. **Objetivo/traza:** RQF-143–152/203–206.
- **Alcance:** relación de eventos check-in/out y documento, actor, instante, ubicación mínima, estado y duplicidad.
- **Fuera:** almacenar binario en PostgreSQL. **Dep:** OPS-ARCH-01, CORE-DB-03. **Desbloquea:** OPS-DB-02
- **Aceptación:** hechos históricos no se sobrescriben; participante/evidencia relación clara. **Pruebas:** ownership/retención. **Riesgo:** ubicación exacta por defecto no minimizaría datos. Estado `todo`.

### OPS-DB-02 — Migrar operación del arriendo
- **Tipo/estado:** DB / todo. **Objetivo:** crear persistencia de registro check-in/out.
- **Alcance:** DDL versionado y constraints con FK a reserva/documento. **Fuera:** evidencias reales.
- **Dep:** OPS-DB-01. **Desbloquea:** OPS-BE-01. **Aceptación:** sin borrado cascada y migración limpia. **Pruebas:** FK, estado duplicado, migración. **Riesgo:** conservar dato de ubicación fuera del plazo acordado. Estado `todo`.

### OPS-BE-01 — Implementar registro de entrega/recepción y devolución
- **Tipo/estado:** BE / todo. **Objetivo:** CU-33/34/48.
- **Alcance:** autorización participante, evidencia vía storage port, sello temporal y comandos de confirmar/objetar.
- **Fuera:** adjudicar disputa o capturar garantía. **Dep:** OPS-DB-02, CORE-BE-01. **Desbloquea:** OPS-API-01
- **Aceptación:** cambio de reserva solo por transición permitida y evento auditado. **Pruebas:** actor incorrecto, evidencia ausente, retry. **Riesgo:** reloj/metadata cliente no son prueba incontrovertible. Estado `todo`.

### OPS-API-01 — Exponer endpoints check-in/out
- **Tipo/estado:** API / todo. **Objetivo/traza:** CU-33/34/48.
- **Alcance:** registrar evidencia, consultar estado, confirmar recepción o reportar excepción.
- **Fuera:** endpoints de acceso público a binarios. **Dep:** OPS-BE-01, CORE-API-01. **Desbloquea:** OPS-TEST-01
- **Aceptación:** participante puede operar solo en etapa permitida; errors mappeados. **Pruebas:** 401/403/409/413 y contrato. **Riesgo:** carga de archivo requiere límite/tipo real. Estado `todo`.

### OPS-TEST-01 — Probar ciclo de uso y evidencia
- **Tipo/estado:** TEST / todo. **Objetivo:** validar casos CU-33/34/48 con documentos sintéticos.
- **Alcance:** workflow, permisos, reintento y vínculo a reserva/objeto.
- **Fuera:** demostrar calidad probatoria legal. **Dep:** OPS-API-01, CORE-TEST-01. **Desbloquea:** OPS-MOCK-01
- **Aceptación:** test rechaza avance no autorizado/incompleto. **Pruebas:** evidencia faltante, actor ajeno, doble envío. **Riesgo:** privacidad/falsificación. Estado `todo`.

### OPS-MOCK-01 — Validar visualmente check-in/out
- **Tipo/estado:** MOCK / todo. **Objetivo:** inspeccionar operación por API.
- **Alcance:** seleccionar reserva de prueba, registrar evidencia sintética, consultar y confirmar/objetar.
- **Fuera:** captura de fotos reales y UX final. **Dep:** OPS-TEST-01, CORE-ENV-01. **Desbloquea:** DIS-ARCH-01
- **Aceptación:** criterios comunes; respeta estado/roles. **Pruebas:** éxito y 403/409/413. **Riesgo:** no almacenar evidencia local fuera de API. Estado `todo`.

## M09 — Comunicación y reputación

**Trazabilidad:** RQF-153–158, 207, 232–235; CU-35–38/49; HU25/34 y notificaciones relacionadas con HU30. Entidades: mensaje_reserva, resena, reporte_resena, notificacion, entrega_notificacion.

### COMM-ARCH-01 — Definir reglas de chat, reseñas y avisos
- **Tipo/estado:** ARCH / todo. **Objetivo:** delimitar participación, elegibilidad de reseña, moderación y entrega de notificación.
- **Alcance:** visibilidad/retirada, rate limits, acceso en reserva, canales, retry/idempotencia, privacidad/retención. **Fuera:** proveedor de mensajería no seleccionado.
- **Dep:** BOOK-MOCK-01. **Desbloquea:** COMM-DB-01
- **Aceptación:** contrato y estados definidos para CU-35–38/49. **Pruebas:** revisión de permisos y lifecycle. **Riesgo:** texto libre contiene PII. Estado `todo`.

### COMM-DB-01 — Diseñar mensajes, reseñas y entregas
- **Tipo/estado:** DB / todo. **Objetivo/traza:** entidades M09 y RQF del módulo.
- **Alcance:** FK a reserva/participante, reseña elegible, reportes, notificación/outbox delivery attempts, índices/paginación.
- **Fuera:** replicar eventos por impresión. **Dep:** COMM-ARCH-01, CORE-DB-03. **Desbloquea:** COMM-DB-02
- **Aceptación:** mensajes de participantes; reseña una vez por elegibilidad definida; notificación separa intención y envío. **Pruebas:** cardinalidad y retención. **Riesgo:** contenido libre y volumen de reintentos. Estado `todo`.

### COMM-DB-02 — Migrar persistencia de comunicación
- **Tipo/estado:** DB / todo. **Objetivo:** crear tablas/índices M09.
- **Alcance:** DDL, estados y fixtures sintéticos. **Fuera:** datos personales reales.
- **Dep:** COMM-DB-01. **Desbloquea:** COMM-BE-01. **Aceptación:** migración limpia y FK owner. **Pruebas:** unique/elegibilidad/fk. **Riesgo:** TTL/retención pendiente por finalidad. Estado `todo`.

### COMM-BE-01 — Implementar chat de reserva y consulta de mensajes
- **Tipo/estado:** BE / todo. **Objetivo:** CU-37/38/HU34.
- **Alcance:** crear/listar mensajes con autorización de participantes, cursor pagination, retirada/reporting según reglas.
- **Fuera:** chat externo en tiempo real. **Dep:** COMM-DB-02, CORE-BE-01. **Desbloquea:** COMM-BE-02
- **Aceptación:** no participante no lee ni escribe; logs omiten body sensible. **Pruebas:** owner/outsider, orden/página, XSS payload como dato escapable. **Riesgo:** abuso/retención. Estado `todo`.

### COMM-BE-02 — Implementar reseñas, moderación/reportes y avisos
- **Tipo/estado:** BE / todo. **Objetivo/traza:** CU-35/36/49, notificaciones HU30.
- **Alcance:** elegibilidad y publicación de reseña, reporte/moderación por rol; registrar intención y reintentos por adaptador de notificación.
- **Fuera:** enviar correo/SMS real sin integración elegida. **Dep:** COMM-BE-01. **Desbloquea:** COMM-API-01
- **Aceptación:** una reseña elegible; avisos idempotentes y delivery state separado. **Pruebas:** duplicado, moderador, retry fake. **Riesgo:** sesgo/abuso y canales externos. Estado `todo`.

### COMM-API-01 — Exponer API de comunicación/reputación
- **Tipo/estado:** API / todo. **Objetivo:** CRUD permitido de mensajes/reseñas/reportes y estado de avisos.
- **Alcance:** OpenAPI, paginación, roles, errores, límites de payload. **Fuera:** endpoint admin genérico sin authz.
- **Dep:** COMM-BE-02, CORE-API-01. **Desbloquea:** COMM-TEST-01
- **Aceptación:** cada recurso filtra por reserva/participante y documentación de límites. **Pruebas:** 401/403/404/422, paginación. **Riesgo:** scraping o contenido inapropiado. Estado `todo`.

### COMM-TEST-01 — Probar chat, reseña y notificaciones
- **Tipo/estado:** TEST / todo. **Objetivo:** verificar CU-35–38/49.
- **Alcance:** DB/API permissions, elegibilidad, moderación, outbox/retries con fake. **Fuera:** entrega externa productiva.
- **Dep:** COMM-API-01, CORE-TEST-01. **Desbloquea:** COMM-MOCK-01
- **Aceptación:** pruebas cubren participante/ajeno y duplicados; sin cuerpo sensible en logs. **Pruebas:** unitarias/contract/integration. **Riesgo:** políticas de contenido/retención sin acuerdo. Estado `todo`.

### COMM-MOCK-01 — Validar visualmente comunicación y reputación
- **Tipo/estado:** MOCK / todo. **Objetivo:** comprobar API M09.
- **Alcance:** listar/enviar mensaje de reserva de prueba, enviar reseña/reportar y ver estado simulado de aviso.
- **Fuera:** realtime, correo real, diseño final. **Dep:** COMM-TEST-01, CORE-ENV-01. **Desbloquea:** DIS-ARCH-01
- **Aceptación:** criterios comunes y muestra response; auth role correcta. **Pruebas:** 200/403/409/422, red. **Riesgo:** no usar texto real sensible. Estado `todo`.

## M10 — Disputas, liquidación y tributación

**Trazabilidad:** RQF-159–177, 208–211; CU-39–42; HU28. HU29 (centro de ayuda) no tiene RQF/CU y queda fuera del core planificado. Entidades: disputa, garantia, liquidacion, movimiento_financiero, documento_tributario y documento/notificacion.

### DIS-ARCH-01 — Definir reclamo, decisión y cierre financiero
- **Tipo/estado:** ARCH / todo. **Objetivo:** separar evidencia, decisión humana, garantía prevista/observada, liquidación y boleta.
- **Alcance:** estados, roles, descargos, audit trail, compensación, datos tributarios. **Fuera:** declarar tarifa/IVA/flujo de fondos resueltos.
- **Dep:** OPS-MOCK-01, COMM-MOCK-01. **Desbloquea:** DIS-DB-01
- **Aceptación:** workflows por separado y proveedor/contador pendientes explícitos. **Pruebas:** tabla de transiciones y fallos. **Riesgo:** modelo académico previo asume Escrow; ES2 condiciona capacidad. Estado `todo`.

### DIS-DB-01 — Diseñar disputa, garantía, liquidación y documentos económicos
- **Tipo/estado:** DB / todo. **Objetivo/traza:** RQF-159–177/208–211.
- **Alcance:** persistir reclamo/descargos/resolución, obligación, valor observado proveedor, movimientos y documento tributario separado.
- **Fuera:** registrar fondos de terceros como ingreso de plataforma. **Dep:** DIS-ARCH-01, CORE-DB-03. **Desbloquea:** DIS-DB-02
- **Aceptación:** dinero exacto/moneda, estados diferenciados, correlación externa y no cascada. **Pruebas:** integridad/asientos y referencias. **Riesgo:** regla contable/fiscal requiere asesoría/evidencia. Estado `todo`.

### DIS-DB-02 — Migrar persistencia de disputas y cierre
- **Tipo/estado:** DB / todo. **Objetivo:** esquema versionado M10.
- **Alcance:** DDL con estados/índices y fixtures sintéticos. **Fuera:** emitir documentos tributarios reales.
- **Dep:** DIS-DB-01. **Desbloquea:** DIS-BE-01. **Aceptación:** FK a reserva/evidencia, hechos financieros no sobrescribibles. **Pruebas:** migración/state/moneda. **Riesgo:** estados denotan hecho confirmado vs previsto. Estado `todo`.

### DIS-BE-01 — Implementar reclamo, descargos y resolución
- **Tipo/estado:** BE / todo. **Objetivo:** CU-39–41/HU28.
- **Alcance:** autorización por parte, adjuntos privados, decisión admin motivada, transición reserva y notificaciones/outbox.
- **Fuera:** movimiento de dinero real desde la resolución local. **Dep:** DIS-DB-02, CORE-BE-01. **Desbloquea:** DIS-BE-02
- **Aceptación:** resolución queda auditada, disputa cerrada no reabre y decisión no cambia saldo sin provider confirmado. **Pruebas:** role/transition/evidence. **Riesgo:** proceso requiere política y revisión legal. Estado `todo`.

### DIS-BE-02 — Implementar conciliación de liquidación/garantía/documento tributario
- **Tipo/estado:** BE / todo. **Objetivo/traza:** CU-42/RQF-172–177/208–211.
- **Alcance:** adapters neutrales para consulta/reversa, movimientos observados, estado por conciliar y emisión documental tras reglas aprobadas.
- **Fuera:** fingir que split retiene/libera garantía. **Dep:** DIS-BE-01, BOOK-BE-03. **Desbloquea:** DIS-API-01
- **Aceptación:** tercero confirmado genera hecho correlacionado; timeout/fallo permanece conciliable. **Pruebas:** provider fake, reverso y falta de saldo. **Riesgo:** tarifa, IVA, cuenta y contrato siguen abiertos. Estado `todo`.

### DIS-API-01 — Exponer API de reclamo y cierre económico
- **Tipo/estado:** API / todo. **Objetivo:** CU-39–42.
- **Alcance:** creación de reclamo/descargos, acceso admin a resolver, consulta de liquidación/documento propio.
- **Fuera:** acción admin sin MFA/permiso acordado; cobrar desde el cliente. **Dep:** DIS-BE-02, CORE-API-01. **Desbloquea:** DIS-TEST-01
- **Aceptación:** authz roles/participants y datos por finalidad; OpenAPI. **Pruebas:** 401/403/409, transitions y montos. **Riesgo:** exposición de datos financieros. Estado `todo`.

### DIS-TEST-01 — Probar disputas y resultados financieros observados
- **Tipo/estado:** TEST / todo. **Objetivo:** validar workflows separados.
- **Alcance:** fake provider, descargos, decisión, compensación, falta saldo y conciliación.
- **Fuera:** declarar conciliación con PSP real sin sandbox. **Dep:** DIS-API-01, CORE-TEST-01. **Desbloquea:** DIS-MOCK-01
- **Aceptación:** audit trail y valores observados diferenciados de proyección. **Pruebas:** duplicados, estados terminales, reintento. **Riesgo:** no prueba validez tributaria/legal. Estado `todo`.

### DIS-MOCK-01 — Validar visualmente disputa y liquidación
- **Tipo/estado:** MOCK / todo. **Objetivo:** inspeccionar API para partes/admin.
- **Alcance:** abrir reclamo sintético, presentar descargo, resolver como rol prueba, mostrar liquidación/documento de fixture.
- **Fuera:** payout/cobro real o acceso sin autorización. **Dep:** DIS-TEST-01, CORE-ENV-01. **Desbloquea:** ADMIN-ARCH-01
- **Aceptación:** criterios comunes, roles separados y respuesta real del API. **Pruebas:** 403, cierre, estado por conciliar. **Riesgo:** no presentar fixture como ingreso real. Estado `todo`.

## M11 — Administración y auditoría

**Trazabilidad:** RQF-178–185/212/236; CU-43–46/52; HU26–27. Entidades: evento_auditoria, outbox_evento y accesos limitados a entidades de producto. Promociones, métricas premium y NPS quedan diferidas.

### ADMIN-ARCH-01 — Definir controles de administración y auditoría
- **Tipo/estado:** ARCH / todo. **Objetivo:** precisar roles, motivos, consulta/export, moderación y eventos auditables.
- **Alcance:** permisos mínimos, finalidad, retención, filtros, correlación y límites de datos exportados.
- **Fuera:** afirmar inmutabilidad con BigQuery; política bloqueada requiere plazo y ensayo separado. **Dep:** DIS-MOCK-01, CORE-ARCH-01. **Desbloquea:** ADMIN-DB-01
- **Aceptación:** acciones críticas y permisos definidos; RNF-017 tratado aparte de analítica. **Pruebas:** matriz de acceso/retención. **Riesgo:** bloqueo de retención irreversible y datos personales. Estado `todo`.

### ADMIN-DB-01 — Diseñar auditoría/outbox y catálogo diferido
- **Tipo/estado:** DB / todo. **Objetivo/traza:** RQF-178–185/212/236, RNF-017/024/028/043.
- **Alcance:** `evento_auditoria`, `outbox_evento`, payload mínimo, claves/lease/dedup/orden, índices y separar promociones/NPS como diferidas.
- **Fuera:** usar BigQuery como fuente transaccional o insertar evento de impresión por fila. **Dep:** ADMIN-ARCH-01, CORE-DB-03. **Desbloquea:** ADMIN-DB-02
- **Aceptación:** payload sin PII libre, transacción outbox declarada y retención ligada a finalidad. **Pruebas:** review de duplicado, retención y ownership. **Riesgo:** no usar outbox para simular inmutabilidad. Estado `todo`.

### ADMIN-DB-02 — Migrar auditoría y outbox mínimos
- **Tipo/estado:** DB / todo. **Objetivo:** persistir la infraestructura común de eventos/auditoría.
- **Alcance:** DDL append-oriented, índices y migrations para leases/dedup definidos.
- **Fuera:** bucket lock, Pub/Sub o dataset productivo. **Dep:** ADMIN-DB-01. **Desbloquea:** ADMIN-BE-01
- **Aceptación:** tabla permite reclamar/reintentar outbox sin duplicar evento lógico; historial no se borra en cascada. **Pruebas:** atomicidad con hecho de dominio en integración. **Riesgo:** retención y privacidades aún deben aprobarse. Estado `todo`.

### ADMIN-BE-01 — Implementar registro de acciones críticas y publicador outbox
- **Tipo/estado:** BE / todo. **Objetivo:** auditar negocio y publicar eventos eventual/idempotentemente.
- **Alcance:** API interna de auditoría, inserción atómica con dominio, worker durable, lease/checkpoint/retry y consumer dedupe contract.
- **Fuera:** despliegue Pub/Sub/BigQuery o afirmar exactly-once. **Dep:** ADMIN-DB-02, CORE-BE-01. **Desbloquea:** ADMIN-BE-02
- **Aceptación:** evento se guarda en misma transacción; retry no duplica efecto; logs redactados. **Pruebas:** crash/restart, lease expirado, duplicado. **Riesgo:** cada módulo debe integrar outbox. Estado `todo`.

### ADMIN-BE-02 — Implementar gobierno, moderación y consultas de auditoría
- **Tipo/estado:** BE / todo. **Objetivo/traza:** CU-43–46/52.
- **Alcance:** bloquear cuenta, moderar reporte, reportes acotados, consulta/export con permisos y motivo auditado.
- **Fuera:** acceso admin irrestricto o métricas premium sin entitlement aprobado. **Dep:** ADMIN-BE-01. **Desbloquea:** ADMIN-API-01
- **Aceptación:** cada operación privilegiada autoriza, justifica y deja evento; export mínimo. **Pruebas:** roles/filters/export/blocked users. **Riesgo:** privilegio excesivo y exportación PII. Estado `todo`.

### ADMIN-API-01 — Exponer API de administración/auditoría
- **Tipo/estado:** API / todo. **Objetivo:** casos de uso admin bajo contrato HTTP.
- **Alcance:** endpoints versionados para bloqueo/moderación/reportes/export filtrado y lectura de auditoría.
- **Fuera:** dashboard frontend final o acceso directo BigQuery. **Dep:** ADMIN-BE-02, CORE-API-01. **Desbloquea:** ADMIN-TEST-01
- **Aceptación:** authz admin y scope/periodo obligatorio; export trazable. **Pruebas:** 401/403, paginación y redacción. **Riesgo:** retención/inmutabilidad pendiente. Estado `todo`.

### ADMIN-TEST-01 — Probar permisos, auditoría y outbox
- **Tipo/estado:** TEST / todo. **Objetivo:** validar CU-43–46/52, RNF-017/043 y robustez outbox.
- **Alcance:** atomicidad, duplicados/retry, consulta/export con rol, retención configurada en ambiente de prueba.
- **Fuera:** ensayo Cloud Storage lock productivo. **Dep:** ADMIN-API-01, CORE-TEST-01. **Desbloquea:** ADMIN-MOCK-01
- **Aceptación:** evidencia sintética de evento y denegación de alteración solo del mecanismo elegido en sandbox. **Pruebas:** PT-16 cruzada, crash/retry, export. **Riesgo:** BigQuery no prueba inmutabilidad. Estado `todo`.

### ADMIN-MOCK-01 — Validar visualmente gobierno y auditoría
- **Tipo/estado:** MOCK / todo. **Objetivo:** probar API administrativa con roles de prueba.
- **Alcance:** formulario simple de bloqueo/moderación y consulta/export de eventos sintéticos.
- **Fuera:** dashboard productivo, métricas comerciales y acceso no autorizado. **Dep:** ADMIN-TEST-01, CORE-ENV-01. **Desbloquea:** ninguna tarjeta
- **Aceptación:** criterios comunes, auth admin, información redactada, servicio aparte y retirabilidad. **Pruebas:** 403 user normal, admin autorizado, error/response. **Riesgo:** mocks no constituyen operación administrativa real. Estado `todo`.

## Cierre del backlog

Antes de implementar cada módulo, revisar sus criterios contra las fichas exactas del Anexo E/D/B y refinar rutas OpenAPI. Si se descubre una carencia, crear ticket nuevo y actualizar `grafo_dependencias.md`; no ampliar una tarjeta silenciosamente ni marcar completada por haber redactado el plan.
