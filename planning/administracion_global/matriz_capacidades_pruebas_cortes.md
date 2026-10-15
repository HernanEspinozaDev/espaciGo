# Matriz GADM-00–04 — capacidades, pruebas y cortes

- Estado: GADM-00 se fusionó en PR #312 (`2589aa1139232f419cf1d1aebdd79a69c1f4007b`); GADM-01 se fusionó en PR #313 (`6dffcc92ae528d8a790956a8dd73960df1501802`); GADM-02 se aprobó y fusionó en PR #314 (HEAD revisado `e81cfbf9db67eef1d09801b55b98fce2336c64f6`, merge `6a721adc96ac5192e6bfcf8b672d052e272ed048`); GADM-03 se aprobó y fusionó en PR #315 (HEAD revisado `d8dad14214b4c9fcceb556531b1f7e28d59c5dc0`; código probado `436b52695043dd572e4bbb5981c43d3936ea145c`, merge `2fffd42fc0085084b045832ba15e16b1258592cd`); GADM-04 está activo en `codex/marketplace/chore/gadm-04-regression-master-delivery`.
- Base de GADM-02: `6dffcc92ae528d8a790956a8dd73960df1501802` (HEAD revisado de GADM-01: `0dcaa68e78d377d7ad5d0831366a73e5d72c3e3f`).
- La columna “actual” conserva el inventario anterior a GADM-01; la columna de pruebas del corte registra evidencia ejecutada, no intención futura.

## Evidencia GADM-02 (2026-10-15)

HEAD de código probado: `8ffeffa0704a7bfea989afd5674f55a49f9298f0` (branch `codex/marketplace/feature/gadm-02-account-publication-management`, PR #314). Entorno de integración PostGIS aislado y recorrido FE-14 desechable con nombres de proyecto, volumen y puertos exclusivos; sin `espacigo_pgdata`, credenciales eliminadas al salir y backend verificado sin correos/contraseñas sintéticos en logs.

| Validación | Entorno / alcance | Resultado |
| --- | --- | --- |
| Go unitario | `go test ./...` en el worktree | Correcto; los paquetes pasaron. Las integraciones que requieren `TEST_DATABASE_URL` se ejecutaron aparte en el runner PostGIS. |
| Mutaciones PostGIS | `GO_TEST_RUN='^TestGlobalBootstrapDirectoryAndLastMasterConcurrency$' bash scripts/test-local-comm-mailpit.sh ./internal/adminglobal` | Correcto: edición de perfil/atributos con versión, recursos de cuenta, galería, horario semanal, transiciones/retirada, visibilidad KYC, tarifas CLP, obligaciones y carrera del último maestro. Datos sintéticos recreables; contenedor temporal retirado. |
| Contrato y frontend | `npm run api:check`, `npm run lint`, `npm test -- --watch=false`, `npm run build` en `frontend/` | Correcto: OpenAPI/SDK generado, lint/arquitectura/SCSS, 296 pruebas en 39 archivos y build SSR/navegador con 3 rutas prerenderizadas. |
| E2E servido | `FE14_TEST_GREP='GADM-02 serves account publication management' bash scripts/test-frontend-e2e.sh` con FE-14 aislado | 1/1 recorrido pasó por autenticación del administrador sintético, inventario filtrado, galería y horario desde Angular→Go→PostGIS. El runner retiró stack/volumen/datos de ensayo. |
| Capturas / medidas | Artefactos de [FE-14 móvil](../frontend/evidencia/fe14_gadm_02_admin_mobile_1637077c57c1.jpg) y [escritorio](../frontend/evidencia/fe14_gadm_02_admin_desktop_1637077c57c1.jpg) | 390×844: documento 390 px; 1365×900: documento 1365 px. Capturas incluyen inventario, galería y horario; correos de ensayo redactados. JSON registra el HEAD de código probado. |
| Historial / whitespace | `git diff -- db/migrations` y `git diff --check` | Sin migraciones nuevas ni cambios históricos; V1–V58 intactas. No se necesitó V59. |

El recorrido y la verificación técnica están registrados. PR #314 fue aprobado por HernanEspinozaDev y fusionado en `6a721adc96ac5192e6bfcf8b672d052e272ed048`; HEAD revisado `e81cfbf9db67eef1d09801b55b98fce2336c64f6`, CI SUCCESS y siete hilos resueltos. Esta fusión técnica no equivale a aceptación de producto. No se añadió `DELETE` ni estado de dominio nuevo: V50 ya contiene `eliminada`, V52 conserva tarifas/snapshots/obligaciones. No se activaron cuenta real, soporte, GCP ni demo retenida.

## Sucesión GADM-02 → GADM-03 (2026-10-15)

Por instrucción explícita de este chat, GADM-03 está en desarrollo desde `origin/main` en `codex/marketplace/feature/gadm-03-kpi-account-audit`. El alcance es KPI conciliados con persistencia, auditoría global/por cuenta con actor y objetivo distintos y exportaciones privadas JSON/CSV; se conserva la lista de definiciones, límites, pruebas y exclusiones de `plan_admin_global_master.md`. GADM-03 quedó fusionado y GADM-04 está autorizado en su rama temporal; la aceptación del producto sigue siendo del propietario.

## Evidencia GADM-03 (2026-10-15)

Código probado: `436b52695043dd572e4bbb5981c43d3936ea145c` (rama `codex/marketplace/feature/gadm-03-kpi-account-audit`), partiendo de `6a721adc96ac5192e6bfcf8b672d052e272ed048`. La migración V59 añade columnas para objetivo e instante de registro e índices de auditoría; V1–V58 quedan intactas. El recorrido usa Compose FE-14 con proyecto, volumen y cuentas sintéticas efímeros; no referencia `espacigo_pgdata`; elimina la base/credenciales al salir.

| Validación | Entorno / alcance | Resultado |
| --- | --- | --- |
| Go / dominio | `go test ./internal/adminglobal/... ./internal/identity/...`; runner FE-14 con PostgreSQL 18 | Unitarias pasan. Integración obligatoria `TestGlobalBootstrapDirectoryAndLastMasterConcurrency` pasa sin `TEST_DATABASE_URL` omitido; incluye 404/auditoría de destino ausente, paginación con empates y filtros, export JSON/CSV con corte, conciliación KPI SQL con decisiones aprobadas/rechazadas, reclamo abierto/resuelto y CLP exacto. Migrador pasa desde base vacía, V56 y V58 con checksums previos. |
| Contrato y Angular | `npm run api:check`, `npm run lint`, `npm test -- --watch=false`, `npm run build` | OpenAPI/SDK y TypeScript pasan; lint/SCSS/arquitectura, build SSR/navegador pasan; 39 archivos y 306 pruebas Angular, incluidas alertas accesibles KPI/auditoría/export, errores 401/403, recuperación por reintento y descarte de respuestas obsoletas entre filtros/sesiones. |
| Recorrido conectado FE-14 | Playwright Chromium contra Angular y API Go, PostgreSQL/PostGIS de Compose aislado | 1/1 recorrido conectado pasa: panel KPI contra DB, filtros de actor/destino/recurso/motivo/correlación, paginación keyset sin repetición y snapshot fijo, evento tardío excluido de JSON/CSV, ambos formatos exportados con el mismo corte, lookup de destino inexistente 404 con rechazo auditado, descarga CSV real y rol no global rechazado. Logs sin correos ni contraseñas. |
| Rendimiento de directorio | Misma DB desechable; 1.000 cuentas sintéticas no autenticables; consulta por prefijo y página de 25 con `EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON)` | Plan en `evidencia/gadm_03/gadm03-account-query-plan.txt`: 1,265 ms; 1.043 bloques compartidos en caché, 0 físicos y 0 temporales en la raíz. Medida del contenedor de ensayo, no SLO de producción. |
| Capturas / medidas | `evidencia/gadm_03/fe14_gadm_03_admin_{mobile,desktop}.{jpg,json}` | Móvil 390×844, documento 390×5125; escritorio 1365×900, documento 1365×2879; sin desbordamiento horizontal. Sidecars registran HEAD `436b52695043dd572e4bbb5981c43d3936ea145c`; capturas sin credenciales de ensayo. |
| Límites | Contratos OpenAPI, datos de producto | Cobros/devoluciones y comisión son datos de ensayo; disponibilidad o cola no se transforma en cero cuando la fuente no existe. No se presentan DAU/visitas, pagos reales, aceptación de producto ni capacidad GCP. |

La matriz registra verificaciones técnicas, no aceptación de producto. La cuenta del propietario y aceptación de producto siguen pendientes.

## Evidencia GADM-04 (2026-10-15)

HEAD revisado en esta iteración `94aba71dff8e6abc7d03bdef0daf01e441381cf0`. QA backend: 146 eventos Go, 92 integraciones obligatorias, 0 skips, vet y cinco repeticiones de booking. Recorrido FE-14: GADM-02/03/04, 4/4 Playwright conectado; `globalAdmin` solo global, `admin` solo heredado y tercero ordinario. La activación local sobre `espacigo_local` fue sintética y auditada; migraciones V1–V59 intactas. Demo local de revisión retenida en `http://127.0.0.1:20942/`, RUN_ID `e83a3e47dcb3`; stop/restart y readiness comprobados. Capturas viewport 390×844/1365×900, sidecars y hashes están en [`evidencia/gadm_04/`](evidencia/gadm_04/resultado_gadm04.md). `LOCAL-CLOSE-02` corresponde a evidencia histórica previa, no a este recorrido. La cuenta real, soporte, GCP y aceptación de producto siguen pendientes. `operations.*` y `finance.*` permanecen propuestas; la cobertura prueba rutas heredadas existentes sin crear esos permisos.

## Evidencia GADM-01 (2026-10-14)

| Validación | Entorno / alcance | Resultado |
| --- | --- | --- |
| Runner backend local | `scripts/test-local-qa-backend.sh`; suite Go sin servicios persistentes + vet + integraciones PostgreSQL/PostGIS/Mailpit por paquete | Correcto; 133 eventos Go reportados, 91 integraciones requeridas aprobadas, 0 skips; el runner repite la integración de booking 5 veces como estabilidad. |
| Servicio/directorio y concurrencia | PostGIS 18-3.6, proyecto Compose aislado `espacigo_gadm01_20261014`; `go test ./internal/adminglobal -run TestGlobalBootstrapDirectoryAndLastMasterConcurrency -count=3 -v` | 3 ejecuciones correctas; bootstrap concurrente/replay, filtros, detalle, capabilities, revocación de permiso por cambio de rol, cuenta restringida y protección concurrente del último maestro. Datos solamente sintéticos. |
| Migración | Mismo runtime PostGIS aislado; `go test ./internal/migrator -run TestMPV211MigrationWorksFromEmptyAndV56WithoutChangingHistory -count=1 -v` | Correcta desde base vacía y desde V56; historial anterior preservado y V58 aplicado. |
| Angular | `npm test -- --watch=false`; `npm run api:check`; `npm run lint`; `npm run build` | 39 archivos / 289 pruebas; contrato generado y TypeScript; lint/estilos/arquitectura; build SSR y navegador, correcto. |
| Correcciones de revisión GADM-01 | IdentitySession real, flujo de paginación, CLI `recover`, `ExecuteSuppression` | 7 regresiones Angular: logout/selección/controles, pérdida de capabilities con el mismo Bearer, 403 vigente y revalidación sin logout, borrador frente a cursor aplicado, error obsoleto entre login/logout de la misma cuenta, búsqueda pendiente al abrir detalle y detalle pendiente al aplicar filtros. Go: ejecución CLI `recover`, auditoría éxito/rechazo y UUID distinto; transferencia del maestro de bootstrap obligatoria para supresión, lock compartido con bloqueo y carrera entre baja/bloqueo probada en PostgreSQL; runner 133/91, sin skips, y regresores dirigidos aprobados. |
| Recorrido real / capturas | API Go y Angular SSR locales, usuario maestro sintético creado únicamente en DB de ensayo; Playwright Chromium; viewport desktop 1365×900 y móvil 390×844 | Login real, navegación autorizada y lectura desde API real; directorio indica 31 cuentas, filtros y primera página de 25. Capturas: `evidencia/gadm-01/directorio-desktop-1365x900.png` y `evidencia/gadm-01/directorio-mobile-390x844.png`. |
| Protección del historial / diff | `git diff -- db/migrations/V000001__* db/migrations/V000057__*`; `git diff --check` | Sin cambios a migraciones V1–V57; sin errores de whitespace. |

La matriz completa de autorización/casos pendientes sigue siendo guía para cortes futuros; esta sección registra solo las comprobaciones ejecutadas en GADM-01. Los datos de DB, credenciales y configuración local permanecen excluidos en `.local/testing/` y no forman parte de las capturas versionadas.

## Inventario de autorización existente

| Área / operación actual | Gate literal encontrado | Alcance comprobado y hueco |
| --- | --- | --- |
| Rol, registro y sesión | `internal/identity/domain.go`; `RegistrationRoles()` solo da `arrendatario`; `AuthenticationService.Authorize` compara rol solicitado con igualdad y relee roles actuales. | No hay rol global ni herencia. V1 constraint solo admite tres roles. |
| Ficha individual y bloqueo de cuenta | `internal/adminlocal/transport/http/handler.go` exige `RoleAdministrator`; `internal/adminlocal/service.go` vuelve a validar administrador heredado o membresía global dentro de transacción. | Lookup por UUID y DTO minimizado; bloqueo/desbloqueo con motivo, revocación de sesiones/tokens e historial. No hay directorio global, edición de perfil ni asignación de roles. |
| Publicaciones administrativas | mismo handler; `internal/adminlocal/queries.go` consulta únicamente `estado='activa'`. | Listado sin cursor/paginación en esta fachada, ficha resumida; no ve borradores/ocultas y no edita/modera globalmente. |
| Reservas y auditoría local | `internal/booking/transport/http/handler.go` llama `Authorize(...RoleAdministrator...)` para endpoints `/admin/local/...`; capa de servicio registra auditoría y aplica filtros/cursor en operaciones documentadas. | Reportes y lectura administrativa acotados, no directorio/KPI global. |
| Reclamos de daño | `internal/damageclaim/transport/http/handler.go` requiere `RoleAdministrator`; `internal/adapters/postgres/damageclaim/repository.go` acepta administrador heredado o membresía global en la transacción de resolución. | Cola, detalle/evidencia sintética y resolución prevista; no es editor general financiero. |
| Disputas/finanzas | `internal/dispute/transport/http/handler.go` autentica con `RoleAdministrator`; endpoints documentados listan/resuelven disputas y operaciones financieras locales según reglas del dominio. | Dinero y ledger son de ensayo; operaciones no deben exponerse mediante SQL libre. |
| KYC/verificación | `internal/verification/transport/http/handler.go` enruta `/admin/verifications` por `authorize(admin=true)` y convierte ese flag a la cadena `administrador`. | Cola/evidencia fixture, revisión y revocación sintéticas. Revisar compatibilidad caso a caso. |
| Reportes de reseñas | `internal/reputation/transport/http/handler.go` usa `RoleAdministrator`; el repositorio Postgres acepta administrador heredado o membresía global. | Moderación de reportes existente, no catálogo completo de publicaciones. |
| Avisos locales | `internal/localnotice/transport/http/handler.go` exige `RoleAdministrator`; el repositorio acepta administrador heredado o membresía global. | Cola terminal y reapertura de aviso acotada. |
| Identidad/privacidad/credenciales | handlers de `internal/identity/transport/http/handler.go` pasan `RoleAdministrator` para colas/acciones administrativas. `internal/adapters/postgres/identity/retention.go` valida administrador heredado o membresía global para retención. | Mantener flujos, minimización y retención; el maestro no ve secretos ni omite obligaciones. |
| Avisos disparados por V36 | El trigger histórico busca recipients con `rol='administrador'`; no autoriza una petición. Membresía global no altera esa selección. | Historial V36 permanece byte a byte. La prueba de acceso a rutas operativas se registra por separado; distribución de avisos no forma parte de este corte. |
| Administración Angular | `app.routes.ts`, `app.html`, `identity-page.html`, `admin-facade.ts`, `identity.guard.ts` y `core/contracts/models.ts` comparan rol literal `administrador`. | Guard y menú solo UX; requieren modelo/capacidades confirmadas por Go, invalidación/limpieza de estado sensible y pruebas frente a respuestas obsoletas. |
| Contrato publicado | `planning/openapi.yaml` enumera `arrendatario`, `arrendador`, `administrador` en esquemas de identidad/cuenta (p. ej. líneas 2931, 4189, 4551 y 4579 en la base). | Cambiar tipos y SDK junto al contrato cuando GADM-01 agregue el rol. |
| Operación de ensayo/herramientas | scripts de grant/fixtures escriben o seleccionan `administrador`; son herramientas explícitas de ensayo y no bootstrap de propietario. | No reutilizar ni ampliar a cuenta global por efecto lateral; actualizar solo con alcance y pruebas del corte correspondiente. |

### Referencias directas a los checks literales

- **HTTP/Go:** `internal/adminlocal/transport/http/handler.go`; `internal/booking/transport/http/handler.go`; `internal/damageclaim/transport/http/handler.go`; `internal/dispute/transport/http/handler.go`; `internal/identity/transport/http/handler.go`; `internal/localnotice/transport/http/handler.go`; `internal/reputation/transport/http/handler.go`; `internal/verification/transport/http/handler.go` (usa `authorize(admin bool)` que convierte `true` a `identity.Role("administrador")`). Dominio: `internal/identity/domain.go` y `internal/identity/auth_service.go` (comparación exacta).
- **SQL/DB:** `internal/adminlocal/service.go` (`requireAdmin`); `internal/adapters/postgres/damageclaim/repository.go`; `internal/adapters/postgres/localnotice/repository.go`; `internal/adapters/postgres/reputation/repository.go`; `internal/adapters/postgres/identity/retention.go`; constraint de `db/migrations/V000001__m01_identity.sql`; trigger/función de `db/migrations/V000036__local_communication_reviews_and_notices.sql`. Las verificaciones fixture en `scripts/` no son autorización de runtime.
- **Angular:** `frontend/src/app/core/identity/identity.guard.ts` exact-match del `requiredRole`; `frontend/src/app/app.routes.ts` (ruta `/administracion`); `frontend/src/app/app.html` (menú); `frontend/src/app/features/identity/identity-page.html`; `frontend/src/app/features/admin/application/admin-facade.ts`; tres unions de sesión/cuenta en `frontend/src/app/core/contracts/models.ts`.
- **Contrato:** `planning/openapi.yaml` incluye enums cerrados con solo tres roles en los esquemas de login/sesión/cuenta. No se cambiaron en GADM-00.

## Mapeo propuesto de capacidad y pruebas

Los casos de prueba son criterios futuros por corte. “No aplica” significa fuera de GADM-00. Todas las lecturas privadas y mutaciones deberán verificar autorización en servidor; la UI no sustituye el gate.

| Capacidad propuesta | Operación objetivo | Administrador previo | Global | Otros/anónimo/bloqueado/restringido | Auditoría / protección | Corte y casos mínimos |
| --- | --- | --- | --- | --- | --- | --- |
| `accounts.list` | directorio paginado/filtros minimizados | denegado salvo decisión explícita y misma superficie heredada; no conceder global por compatibilidad | P con paginación keyset y límites | N | log consulta/filtros minimizados, sin export masivo accidental | GADM-01: P global; N admin previo, anfitrión, arrendatario, anónimo, bloqueado, modo restringido |
| `accounts.read` | ficha por cuenta/recursos | conserva solo lookup actual por UUID si capacidad heredada se asigna explícitamente | P minimizado; datos sensibles requieren permiso/motivo separado | N | actor≠objetivo; consulta inexistente auditada sin revelar existencia a no autorizado | GADM-01/02: P/N y minimización |
| `accounts.update` | allowlist de perfil | no recibe editor nuevo por defecto | P solo campos permitidos/concurrencia y motivo cuando aplique | N autopromoción/cambiar identidad validada | antes/después por nombres de campo minimizados; correo nuevo no validado automáticamente | GADM-02: P allowlist, N campo inesperado, replay/concurrente |
| `accounts.block` | bloqueo/desbloqueo, sesiones | mantiene bloqueo actual si mapeado a capacidad heredada | P con motivo y regla de estado | N autor/objetivo inválido, bloqueado o sin permiso | revocar sesión/token, preservar reservas/retención | GADM-02: P/N; token previo falla en siguiente petición |
| `accounts.roles` | conceder/revocar rol explícito | N elevación global; política heredada por determinar para roles ordinarios | P controlado a otro sujeto elegible; no autopromoción desde UI | N registro/edición pública, rol arbitrario | actor/motivo/capacidad y valores minimizados; bootstrap separado; último global protegido | GADM-01/02: bootstrap; pruebas último global y dos cambios concurrentes |
| `publications.list/read` | inventario completo y ficha con visibilidad efectiva | conserva listado actual de activas, no ampliar por defecto | P para todas las categorías/estados con página | N | separar `activa` de visible en catálogo y accesibilidad de fotos | GADM-01/02: P estados; N anónimo/no autorizado; no N+1 |
| `publications.update` | editar campos allowlist | no gana edición global | P validada por dominio/versión, preserva propietario y snapshots | N arbitrariedad/impersonación | auditar actor, recurso, motivo y campos modificados | GADM-02: P/N campos, conflicto de versión, reserva histórica intacta |
| `publications.moderate/retire` | publicar/ocultar/retirar | no extender las facultades antiguas sin compatibilidad probada | P motivo + reglas de ocupación/retención | N omitir obligaciones/alterar historial | acción atómica y auditada; fotos por soporte existente | GADM-02: P/N estado, concurrencia y obligaciones |
| `operations.read/manage` | panel de reservas/operación vigente | conserva rutas de lectura/acciones heredadas exactas | P consulta y solo comandos de dominio permitidos | N escritura libre de estado/contrato | acción actor≠propietario, motivo/recurso/request ID | GADM-02/03: positivos y matriz de estados/404-409 |
| `finance.read/operate` | reportes y operaciones financieras fake | conserva superficie heredada exacta | P según comando de dominio; CLP exacto | N ver/alterar credenciales, ejecutar transferencia real o saltarse disputa | ledger append-only; garantías no son ingresos | GADM-02/03: dedupe/idempotencia, importes exactos, sin movimientos reales |
| `audit.read/export` | auditoría por cuenta/global y CSV/JSON | actual lectura/export conserva exactamente filtros y límites | P con mismo filtro/snapshot/paginación y permisos explícitos | N export sin permiso o estado obsoleto | evento de consulta/export, neutralizar fórmulas CSV, límite/tamaño, sin secretos | GADM-03: P/N filtros, límite, concurrente, Blob cleanup |
| `kpi.read` | tarjetas/resúmenes desde DB | no añadir a admin anterior salvo decisión explícita | P definiciones, periodo, zona y `generated_at` | N datos no autorizados/indisponibles presentados como cero | agregados sin PII; separar stock/flujo, dinero ensayo y garantías | GADM-03: reconciliar con SQL fixture; no DAU/visitas inventadas |
| `admin_access.manage` | bootstrap, recuperación, concesión/revocación global | N; administradores existentes no escalan | P solo titular/maestro autorizado por política y flujo reciente | N endpoint público, UUID sin verificar, cuenta bloqueada/restringida | bootstrap auditable de un uso, procedimiento de recuperación y protección último global | GADM-01/04: P destino validado; N anónimo/replay/entorno equivocado; carrera |

### Compatibilidad de `administrador`

La política de compatibilidad se define por operación, no por etiqueta de rol: inventariar el gate actual y traducirlo a un conjunto cerrado de capacidades `legacy-admin` que reproduzca ese comportamiento. El catálogo global añade capacidades en otra asignación. No usar “si tiene `administrador` o `administrador_global`, permitir cualquier ruta”; tampoco convertir todos los checks SQL existentes a una condición `IN (...)` sin cobertura por endpoint. Para recursos compartidos, declarar la unión aceptada explícitamente y mantener pruebas separadas de administrador previo y global.

## Casos transversales trazados a cortes

| Caso | GADM-00 | GADM-01 | GADM-02 | GADM-03 | GADM-04 |
| --- | --- | --- | --- | --- | --- |
| Mapa endpoint→capacidad/servicio/SQL/DTO | inventario documental; confirmar lista completa | cerrar antes de cambiar gates | actualizar al añadir mutaciones | actualizar al añadir KPI/export | publicar inventario completo con SHA |
| Administrador previo retiene únicamente permisos existentes | diseño y criterio | P/N regresión de todas las rutas inventariadas | P/N por operación nueva | P/N lectura/export | regresión completa |
| Global no es concedido por registro/perfil ni por seed/dataset admin | criterio | N registro, update, migración/fixture; bootstrap solo en DB de ensayo propia | N en caminos de edición/rol | N export/consulta | regresión/documento |
| Revocación con Bearer anterior/bloqueo/modo restringido | especificar | N siguiente petición | N mutación en carrera y estado actual | N estado sensible en vuelo | regresión y recorrido |
| Último global + cambios concurrentes | especificar | P/N transaccional | N al revocar/bloquear el último | — | repetir bajo concurrencia |
| Minimización, auditoría, motivo, no secretos | ADR/matriz | P/N bootstrap/directorio | P/N datos/cambios | P/N filtros/export/CSV | revisión de evidencia y capturas |
| Upgrade de DB vacía y V1–V57 | marcar V1–V57 inmutables/V58 siguiente si se precisa | P base vacía + upgrade | preservar reservas/snapshots | reconciliar cifras | recreabilidad/guía |
| UI limpia estado/capacidades y descarta respuesta obsoleta | definir contrato UI | P/N roles y navegación | P/N mutación/cambio sesión | P/N filtros/blob | E2E desktop/móvil |

## Cortes autorizados y gates de revisión

| Corte | Entrega versionada | Terminado cuando | Gate siguiente |
| --- | --- | --- | --- |
| GADM-00 | plan, ADR, matriz, inventario y AGENTS; sin DB/código | fusionado en PR #312, merge `2589aa1139232f419cf1d1aebdd79a69c1f4007b` | GADM-01 autorizado tras fusión |
| GADM-01 | rol/capacidades, V58, bootstrap local/recuperación, sesión, OpenAPI y directorio inicial | Fusionado en PR #313, merge `6dffcc92ae528d8a790956a8dd73960df1501802`; HEAD revisado `0dcaa68e78d377d7ad5d0831366a73e5d72c3e3f`; aceptación de producto y cuenta real pendientes | GADM-02 inicia desde el merge de #313 |
| GADM-02 | gestión de cuentas/publicaciones y fichas/recursos | allowlists, motivo/autenticación reciente en servidor, concurrencia, auditoría atómica, snapshots, reglas de privacidad/ocupación y recorrido real comprobados. Correo validado queda fuera de edición directa; supresión requiere transferencia del UUID bootstrap. Historial V1–V58 inmutable. | Aprobado y fusionado #314; aceptación de producto pendiente |
| GADM-03 | KPI, auditoría cuenta/global, exportaciones JSON/CSV | definiciones y números conciliados con DB, paginación/ventana temporal, límites y seguridad; análisis de N+1 | Activo en rama temporal; iniciar GADM-04 solo tras fusión y nueva instrucción |
| GADM-04 | regresión y entrega/guía de cuenta maestra | recorrido/evidencia por SHA, credenciales aisladas, aceptación humana del propietario; sin soporte/GCP | cierre solo dentro del alcance aceptado |

No comenzar ningún sucesor antes de fusionar el corte anterior. La evidencia de GADM-01 se actualizará con HEAD, entorno y resultados comprobados antes de publicar.
