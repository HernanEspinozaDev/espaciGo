# Fase 6 — Cuenta y verificaciones de EspaciGo

Fecha de verificación local: 2026-10-25
Rama: `frontend-next-hermes`
HEAD de origen antes del corte: `e383171bffd327fff2792f8465bf91ba694b5dcd`
HEAD Go API de referencia: `7d62cea412085acb224ca783560826933941acad`
Base PostgreSQL nativa: `espacigo_dms_stage_20261020` (sin migraciones ni cambios de datos)
Estado: verificado y completado en frontend bajo `frontend-next-hermes`, con pruebas unitarias, de integración controlada y nativas Go aprobadas; sin PR, merge, `main` ni despliegue.

## Alcance ejecutado

Se implementó el módulo integral de cuenta de usuario, privacidad, seguridad y verificaciones de EspaciGo en la aplicación Next.js única bajo la agrupación de rutas `frontend/src/app/(workspace)/mi-cuenta/`, consumiendo el contrato de Go exclusivamente a través del gateway same-origin y la arquitectura de puertos, adaptadores y fachadas de dominio:

- **Rutas implementadas:**
  - `/mi-cuenta`: gestión del perfil del usuario (`GET /profile`, `PUT /profile`), manejo transparente de HTTP 404 para cuentas nuevas permitiendo el alta mediante `PUT /profile`, y validación y normalización estricta de teléfono chileno a 9 dígitos. Incluye administración de foto de perfil sintética (`PUT /profile/photo`, `GET /profile/photo/content`, `DELETE /profile/photo`) con carga vía Blob URL autenticada y revocación estricta al desmontar o eliminar. Incorpora gestión de cuenta de cobro sintética (`PUT /payout-account`, `DELETE /payout-account`) con avisos de ensayo visibles permanentes y sin recolección de RUT ni datos bancarios reales.
  - `/mi-cuenta/privacidad`: derechos ARCO y privacidad de datos. Registro de solicitudes de derechos (`POST /rights-requests`), listado de solicitudes vigentes (`GET /rights-requests`), y descarga autenticada de exportaciones de datos en formatos JSON (`/privacy/export`) y archivo ZIP (`/privacy/export/archive`) mediante ciclo de vida Blob controlado con revocación inmediata tras completarse.
  - `/mi-cuenta/seguridad`: actualización de contraseña (`POST /auth/password/change`) con feedback accesible y validación de robustez, cierre de sesión seguro (`POST /auth/logout`) con limpieza completa de credenciales y revocación de recursos en memoria.
  - `/mi-cuenta/verificaciones`: visualización de elegibilidad KYC/KYB (`GET /verifications/eligibility`), listado de verificaciones con badges de estado (`GET /verifications`), detalle con historial de eventos (`GET /verifications/{id}`), reintento seguro de verificaciones rechazadas (`POST /verifications/{id}/retry`), y carga/inspección de evidencia sintética exclusivamente vía fixture `'synthetic-png-v1'` (`POST /verifications/{id}/documents`) con visualización privada y ciclo de vida Blob.

- **Componentes y arquitectura de soporte:**
  - UI desacoplada y accesible: `AccountLayout`, `AccountNavigation`, `ProfileForm`, `PhotoManager`, `PayoutAccountManager`, `PrivacySection`, `RightsRequestsList`, `SecurityForm`, `VerificationsSection`, `VerificationDetailModal`.
  - Capa de aplicación e infraestructura:
    - Dominio: `profile.ts` (normalización y validación de teléfono chileno, validación de nombre), `photo.ts` (política de avatares sintéticos), `payout-account.ts` (modelo de cobro simulado), `privacy.ts` (tipos ARCO, descargas privadas), `verification.ts` (estados de elegibilidad, tipos de verificación KYC/KYB, manejo de evidencia sintética).
    - Puertos y adaptadores: `account.port.ts`, `account-facade.ts`, `account-openapi-adapter.ts`, `verifications.port.ts`, `verifications-facade.ts`, `verifications-openapi-adapter.ts`.
  - Ciclo de vida y gestión segura de memoria:
    - Registro centralizado de URLs Blob (`privateBlobUrls` / hooks de cleanup): revocación estricta de URLs de objeto (`URL.revokeObjectURL`) al cambiar de vista, desmontar componentes, descargar archivos o cerrar sesión, previniendo memory leaks y retención de binarios sensibles.
    - Metadatos de indexación preventiva: `robots: { index: false, follow: false }` configurado en todas las páginas de la agrupación `/mi-cuenta`.

## Gates de seguridad y políticas observadas

### 1. Descargas privadas y Blob cleanup

- Los recursos binarios sensibles (foto de perfil, exportación JSON, archivo ZIP de exportación y evidencias documentales de verificación) se manejan mediante Blob URLs temporales creadas en memoria del cliente a partir de streams/buffers entregados por el gateway same-origin.
- Se implementa un registro estricto `privateBlobUrls` que garantiza la invocación de `URL.revokeObjectURL` al conmutar pestañas, desmontar componentes, actualizar/eliminar recursos o cerrar la sesión activa.
- En las descargas de privacidad (`/privacy/export` y `/privacy/export/archive`), la Blob URL temporal se asocia al elemento de descarga y se libera de inmediato tras disparar el evento nativo del navegador, sin retención prolongada en memoria.

### 2. Avisos sintéticos visibles permanentes

- Tanto la foto de perfil sintética como la cuenta de cobro fake disponen de advertencias y etiquetas permanentes visibles en la interfaz de usuario, indicando inequívocamente su carácter de ensayo y entorno de prueba.
- La cuenta de cobro maneja una referencia puramente sintética; bajo ninguna circunstancia se solicita, procesa ni almacena información financiera real (RUT bancario, cuentas corrientes/vista ni credenciales de pago reales).

### 3. Resiliencia ante 404 en perfil y validación telefónica estricta

- Para cuentas recién registradas donde el backend Go responde HTTP 404 (`profile_not_found`), el frontend interpreta el estado limpiamente como "perfil no inicializado", mostrando el formulario de alta en lugar de una pantalla de fallo genérico.
- La creación y actualización se realiza mediante `PUT /profile`. Los números de teléfono se someten a normalización estricta para teléfonos chilenos (prefijo `+56`, códigos de área y 9 dígitos exactos), garantizando consistencia antes de enviar la carga a Go.

### 4. Verificaciones KYC/KYB sintéticas y fixtures controlados

- Se consulta el estado de elegibilidad en `/verifications/eligibility` para determinar de forma reactiva si el usuario cumple los requisitos para publicar espacios o actuar como anfitrión.
- El módulo de verificaciones lista y detalla el historial de eventos (`pending`, `verified`, `rejected`), admitiendo el reintento formal de procesos rechazados vía `POST /verifications/{id}/retry`.
- La carga de documentos probatorios opera restringida al fixture `'synthetic-png-v1'`, garantizando que durante el desarrollo y las pruebas no se ingestan ni manipulan documentos de identidad reales.

### 5. Rutas privadas CSR y protección contra indexación

- Las rutas `/mi-cuenta`, `/mi-cuenta/privacidad`, `/mi-cuenta/seguridad` y `/mi-cuenta/verificaciones` definen en sus metadatos `robots: { index: false, follow: false }`.
- El acceso se ejecuta íntegramente en cliente (CSR) tras autenticar la sesión; ningún token Bearer ni información privada se incluye en payloads de SSR ni se expone a motores de búsqueda.

## Verificación frontend y pruebas

Ejecutado en el entorno local (`PATH=/home/nandev/.nvm/versions/node/v24.21.0/bin:$PATH`):

- `pnpm lint` — 0 errores, 0 advertencias.
- `pnpm typecheck` — 0 errores en tipado TypeScript estricto.
- `pnpm test` — 33/33 pruebas unitarias y de dominio aprobadas (`tooling/account-domain.test.mjs`, `tooling/scaffold.test.mjs`, validación y normalización telefónica, modelos de privacidad, verificaciones y ciclo de vida de blob URLs).
- `pnpm format:check` — 0 discrepancias de formato Prettier.
- `pnpm test:e2e tests/phase6-account.spec.ts` — 7/7 pruebas aprobadas en modo controlado:
  1. `/mi-cuenta`: 404 inicial muestra estado sin configurar y permite crear perfil con PUT; actualizar perfil envía PUT `/profile` con nombre y teléfono de 9 dígitos.
  2. Foto sintética: generar foto (PUT `/profile/photo`) carga contenido Blob autenticado (`/profile/photo/content`), muestra preview con aviso sintético visible y retirar (DELETE `/profile/photo`) limpia el recurso y revoca Blob URL.
  3. Cuenta de cobro fake: PUT `/payout-account` crea referencia sintética con aviso de ensayo visible permanente, DELETE `/payout-account` la revoca.
  4. `/mi-cuenta/privacidad`: registrar solicitud ARCO (POST `/rights-requests`), listar solicitudes vigentes (GET `/rights-requests`), y descargar exportaciones JSON (`/privacy/export`) y ZIP (`/privacy/export/archive`) verificando descarga Blob con cleanup.
  5. `/mi-cuenta/seguridad`: actualizar contraseña (POST `/auth/password/change`) con feedback accesible.
  6. `/mi-cuenta/verificaciones`: visualización de elegibilidad KYC/KYB, listado de verificaciones con badges de estado, detalle con historial, reintento de verificación rechazada (POST `/verifications/{id}/retry`), y carga/inspección de evidencia `'synthetic-png-v1'` con Blob URL privada.
  7. Suite de accesibilidad Axe sin violaciones en viewports desktop (`1365×900`) y mobile (`390×844`).
- `PHASE6_NATIVE_API_E2E=1 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 PLAYWRIGHT_PORT=4205 pnpm test:e2e tests/phase6-native.spec.ts` — 1/1 prueba aprobada contra el binario Go en loopback (`127.0.0.1:18082`) y la base nativa `espacigo_dms_stage_20261020` (login con cuenta sintética autorizada existente, consulta real de perfil en `/mi-cuenta`, consulta real de elegibilidad en `/mi-cuenta/verificaciones`, y logout limpio 204 sin fugas de credenciales en consola ni peticiones fuera de loopback).
- `pnpm test:e2e` (suite completa) — 48 pruebas aprobadas, 42 omitidas (por gates selectivos de fases posteriores), 0 fallos.
- **Aislamiento de red y seguridad:** todas las peticiones se mantuvieron estrictamente en `127.0.0.1`, sin almacenamiento de tokens Bearer en almacenamiento persistente ni registros en consola del navegador.

## Límites y cierre

- La implementación de Fase 6 en frontend está terminada y verificada contra especificación, pruebas controladas y runtime nativo de Go.
- La cuenta de cobro es estrictamente simulada y etiquetada como ensayo; no existe procesamiento de cuentas bancarias reales ni RUT de pago.
- La carga de evidencias KYC/KYB está restringida al fixture sintético `'synthetic-png-v1'`.
- No se realizaron modificaciones en el backend de Go, archivos OpenAPI, esquemas SQL, migraciones, contenedores Docker ni infraestructura de base de datos.
- Sin PR, merge a `main` ni despliegue.
- Fase 7 (Panel anfitrión) pendiente de inicio.

## Extensión local — verificación documental (2026-10-31)

Esta extensión no reescribe el cierre histórico del 2026-10-25 para perfil, privacidad y KYC/KYB; cubre el flujo documental F5/F6 solicitado posteriormente.

- Checkout frontend: `/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes`, rama `frontend-next-hermes`, HEAD `a2e4266b01deeed447b1a1ad045a7086de3a1999` (7 commits adelante de `origin/frontend-next-hermes`, con cambios locales conservados).
- Implementación comprobada en `/mi-cuenta/documentos`: listar expedientes, crear expediente, consultar caso propio, enviar con `If-Match`, cargar archivo con `If-Match` y descargar Blob privado autorizado. La fachada valida estado/propósito, conserva ETag fuerte y descarta respuestas al cambiar cuenta; el BFF aplica allowlist Bearer y `credentials: "omit"`; el registro revoca Blob URLs al cambiar expediente/desmontar.
- `PLAYWRIGHT_PORT=4206 npx --yes pnpm@12.15.0 check` — exit 0. `api:check` confirmó 185 operaciones generadas; lint, typecheck, 62 pruebas de tooling, Prettier y build pasaron. Playwright: 65 aprobadas, 45 omitidas (incluidas pruebas nativas condicionadas); el recorrido `tests/phase-f6-document-verification.spec.ts` completó las seis operaciones **contra mocks** y verificó Bearer, ETags, estado limpio y descarga PDF. Node emitió una advertencia no fatal `MODULE_TYPELESS_PACKAGE_JSON` en `src/lib/api/clp.ts`.
- Se comprobó por lectura del código Go que las rutas owner están registradas (`internal/documentverification/transport/http/owner_cases.go`, `owner_download.go`, `cmd/api/main.go`). La subida y descarga solo se conectan en `API_RUNTIME_PROFILE=local-document-scan-only`; ese perfil arranca scanner y retención, exige target F5/ClamAV/storage privada y no es worker-free. El perfil worker-free no provee esos servicios.
- No se inició API ni workers, no se reclamaron jobs, no se cambiaron cuentas ni se aplicaron V81/V82. El último readback de F5, posterior al ciclo de backup/restauración temporal, registró V1–V80 aplicadas y V81/V82 pendientes; falta satisfacer los gates de DDL/aislamiento y hacer preflight fresco de la cola antes de iniciar el perfil de escaneo. No se leyó la política privada de cuentas porque no se intentó login ni operación de cuenta.
- Puerto 4203 tenía un `next-server` preexistente; se dejó intacto. El ensayo Playwright usó el puerto libre 4206 y se detuvo al finalizar.

### Revalidación BFF de errores documentales — 2026-10-31

- La inspección de las rutas Go owner encontró respuestas `http.Error` con `text/plain` en errores de autenticación y precondición/CAS. El BFF solo acepta el media type del contrato para respuestas normales; sin corrección traducía esos errores a 502, ocultando estados 401/409/428 y evitando que la fachada manejara una sesión vencida como 401.
- Se añadió una regresión al test `tooling/backend-gateway-credentials.test.mjs`: simuló cuerpos `text/plain` con detalles privados para 401, 409 y 428, esperando conservar el status y responder JSON genérico `upstream_error` sin filtrar el cuerpo. RED observado antes del cambio: `502 !== 401`; GREEN: 2/2 pruebas del archivo.
- El BFF ahora cancela el cuerpo upstream no permitido y, solo para status 4xx/5xx, conserva el status con su respuesta JSON genérica; no reenvía texto ni amplía la allowlist de media types. Respuestas no permitidas fuera de 4xx/5xx siguen siendo 502.
- Revalidación: `node --test tooling/backend-gateway-credentials.test.mjs` (2/2) y `PLAYWRIGHT_PORT=4206 npx --yes pnpm@12.15.0 check` — `api:check` 185 operaciones, lint, typecheck, tooling 63/63, Prettier, build y Playwright 65 aprobadas/45 omitidas; el recorrido documental F6 fue 1/1 contra mocks. Puerto 4206 quedó libre tras Playwright; el listener previo de 4205 no se tocó. Node emitió una advertencia no fatal `MODULE_TYPELESS_PACKAGE_JSON` en `src/lib/api/clp.ts`.
- Sin integración API/DB/storage/scanner real, login/cuentas, DDL, workers, claims, publicación o despliegue. F6 continúa parcial; la evidencia de mocks no acredita persistencia ni autorización real.

### Revalidación de checks frontend — 2026-10-31

- En el worktree `frontend-next-hermes` (`a2e4266b01deeed447b1a1ad045a7086de3a1999`) se ejecutaron `api:check`, `lint`, `typecheck`, `test` y `format:check` con `pnpm@12.15.0`: los cuatro primeros terminaron exit 0; `test` informó 61/61.
- El primer `format:check` informó `tooling/authenticated-operation.test.mjs`; ejecutar `prettier --write` sobre ese archivo indicó `(unchanged)`, y la repetición de `format:check` pasó. `node --test tooling/authenticated-operation.test.mjs` pasó 2/2. No se atribuye el resultado inicial a una modificación comprobada del archivo.
- `git diff --check` pasó. Esta revalidación no incluyó build, Playwright, API, DB, workers ni cuentas; las pruebas F6 siguen siendo mocks y no acreditan integración autenticada o persistencia.

### Revalidación completa local — 2026-10-31

- Tras comprobar libre el puerto local 4206, `PLAYWRIGHT_PORT=4206 npm exec --yes --package=pnpm@12.15.0 -- pnpm check` terminó exit 0: `api:check` 185 operaciones, lint y typecheck, tooling 63/63, Prettier, build Next.js con 30 páginas estáticas y Playwright 65 aprobadas/45 omitidas. `phase-f6-document-verification.spec.ts` pasó 1/1, pero opera contra mocks.
- El servidor Playwright usó `127.0.0.1:4206`; el socket quedó libre tras la corrida. Persiste la advertencia no fatal `MODULE_TYPELESS_PACKAGE_JSON` en `src/lib/api/clp.ts`.
- Esta corrida no inició ni consultó API/DB/storage/scanner Go, no usó cuentas o sesiones y no prueba integración real ni autorización/persistencia backend. F6 permanece parcial.

### Revalidación BFF de status/media y recorrido documental — 2026-10-31

- Se amplió `tooling/backend-gateway-credentials.test.mjs` para comprobar errores upstream `text/plain` en los límites 399/400/499/500/599/600, `Content-Type` ausente y malformado, cancelación del body (incluido rechazo de `cancel()`) y que ese fallo de cancelación no impide la respuesta sanitizada. Se añadió la comprobación de status 409 con error JSON del contrato allowlisted. `node --test tooling/backend-gateway-credentials.test.mjs`: 4/4.
- En `frontend/`, con Node 24.21.0 y pnpm 12.15.0: `api:check` (185 operaciones), lint, typecheck, tooling 65/65, Prettier, `next build` — todo exit 0. Se mantiene la advertencia no fatal `MODULE_TYPELESS_PACKAGE_JSON` en `src/lib/api/clp.ts`.
- Con el puerto 4206 previamente libre, `PLAYWRIGHT_PORT=4206 pnpm test:e2e tests/phase-f6-document-verification.spec.ts`: 1/1 contra mocks en standalone; prueba el recorrido de las seis operaciones, Bearer solo BFF, ETags, transición pendiente→limpio y descarga PDF autorizada. El spec desactiva trace/screenshot/video. Puerto 4206 quedó libre tras la prueba; el listener previo en 4205 permaneció intacto.
- `git diff --check` pasó en ambos worktrees. Esto no inició API Go, DB, storage ni workers y no acredita persistencia, CAS/concurrencia backend ni autorización real; F6 sigue parcial hasta el gate de integración local F5.

### Revalidación del recorrido documental local — 2026-11-01

- En el worktree existente `frontend-next-hermes`, HEAD `a2e4266b01deeed447b1a1ad045a7086de3a1999`, `PLAYWRIGHT_PORT=4206 npm exec --yes --package=pnpm@12.15.0 -- pnpm check` terminó exit 0: `api:check` (185 operaciones), lint, typecheck, tooling 65/65, Prettier, build (30 rutas estáticas) y Playwright 65 aprobadas/45 omitidas. `phase-f6-document-verification.spec.ts` pasó 1/1: seis operaciones, ETags, Bearer solo BFF y descarga PDF, todo **contra mocks**. Se mantuvo la advertencia no fatal `MODULE_TYPELESS_PACKAGE_JSON` en `src/lib/api/clp.ts`.
- Puerto 4206 quedó libre al terminar; el listener preexistente de 4205 no se tocó. `git diff --check` pasó. No se inició ni consultó API Go, PostgreSQL, storage o scanner; no se usaron cuentas ni sesiones.
- Se añadió manejo explícito de HTTP 409 en las mutaciones CAS: la UI conserva el archivo seleccionado, bloquea carga/envío con el ETag obsoleto y exige una actualización exitosa antes de volver a mutar. El E2E observa un envío stale con `If-Match: "2"`, el bloqueo, el GET de refresh y el reintento deliberado con `If-Match: "3"`. RED observado antes de la corrección: el botón de envío seguía habilitado; GREEN: el recorrido pasa tras refrescar. La respuesta y el refresh son mocks, no demuestran CAS de Go/PostgreSQL.
- Tras el cambio, la corrida completa `pnpm check` terminó exit 0 con API inventory 185, tooling 65/65, build de 30 rutas y Playwright 65 aprobadas/45 omitidas; el recorrido documental pasó 1/1. Se mantuvo la advertencia no fatal `MODULE_TYPELESS_PACKAGE_JSON` en `src/lib/api/clp.ts`.
- El E2E focalizado se reejecutó desde `frontend/` con `PLAYWRIGHT_PORT=4206 npm exec --yes --package=pnpm@12.15.0 -- pnpm test:e2e tests/phase-f6-document-verification.spec.ts`: 1 passed (2.4 s). Una invocación aparte desde la raíz del worktree produjo `ERR_PNPM_RECURSIVE_EXEC_FIRST_FAIL` porque allí no está el package script; no corresponde a una falla de aplicación. Puerto 4206 liberado al terminar; listener 4205 ajeno preservado.
- La UI mock y su build quedan verificados localmente, no la integración. F6 sigue parcial hasta ejecutar el recorrido autenticado BFF→Go→PostgreSQL/storage/scan en el perfil local autorizado y cerrar primero los gates F5; no presentar el mock como integración.

### Descarga binaria comprobada por contenido — 2026-11-01

- `frontend/tests/phase-f6-document-verification.spec.ts` ahora abre el archivo descargado y compara los bytes completos con el fixture PDF sintético (`%PDF-1.4 synthetic private document`), además del nombre de archivo. El caso verifica el recorrido de las seis operaciones; sus respuestas siguen siendo mocks.
- Con `PLAYWRIGHT_PORT=4203 pnpm check` desde `frontend/`: `api:check` 185 operaciones, ESLint, TypeScript, tooling 66/66, Prettier y build Next.js (30/30 páginas estáticas) pasaron. Playwright ejecutó 110 pruebas: 65 pasaron y 45 se omitieron; `phase-f6-document-verification.spec.ts` pasó 1/1. Salida global exit 0. Permanece la advertencia no fatal `MODULE_TYPELESS_PACKAGE_JSON` en `src/lib/api/clp.ts`.
- El servidor del ensayo usó `127.0.0.1:4203`; el puerto quedó libre. No se consultó ni inició Go API, PostgreSQL, storage o scanner, y no se usaron cuentas/sesiones. El cuerpo binario está comprobado únicamente en el E2E con mocks: F6 continúa parcial, sin integración real autenticada ni persistencia.

### Revalidación local coordinada — 2026-11-01

- En el worktree `frontend-next-hermes`, rama `frontend-next-hermes`, HEAD `a2e4266b01deeed447b1a1ad045a7086de3a1999`, `PLAYWRIGHT_PORT=4206 npm exec --yes --package=pnpm@12.15.0 -- pnpm check` terminó exit 0: API inventory 185, lint, typecheck, tooling 67/67, Prettier, build Next.js (30 rutas) y Playwright 65 aprobadas/45 omitidas. El recorrido `phase-f6-document-verification.spec.ts` aprobó 1/1; sus seis operaciones y descarga por bytes completos siguen siendo mock UI, no integración Go.
- El E2E usó `127.0.0.1:4206`; el puerto quedó libre. El listener preexistente en `127.0.0.1:4205` se conservó. Permanece la advertencia no fatal `MODULE_TYPELESS_PACKAGE_JSON` en `frontend/src/lib/api/clp.ts`.
- En Go, con `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL` unset, `go test -count=1 ./internal/documentverification/inspection ./internal/adapters/postgres/documentverification ./cmd/api` terminó exit 0; estas pruebas no ejecutaron integración PostgreSQL. La suite local del fork `third_party/pdfcpu` (`./pkg/filter`, `./pkg/pdfcpu/types`, `./pkg/pdfcpu/model`, `./pkg/pdfcpu`) terminó exit 0.
- Preflight PostgreSQL de solo lectura en el target sintético F5 autorizado `127.0.0.1:15434/espacigo_f5_gate_20261101`: PostgreSQL 18.6, `max(schema_migrations.version)=80`, 80 filas en el ledger y cero jobs activos de scan/retención. Se usó la identidad administrativa solo para la consulta read-only; API/workers no se iniciaron. V81/V82 siguen sin aplicar y no se ejecutó DDL.
- F6 permanece parcial: no se inició el perfil de scan, no se hicieron claims ni operaciones de storage, y no se probó el ciclo autenticado BFF→Go→PostgreSQL→storage/scanner. La integración real sigue condicionada a cerrar los gates de migración, privilegios, aislamiento/cola y revisión independiente de F5. Sin cambios de cuentas, publicación, Cloud, push, PR, merge o despliegue.

### Revalidación integral local — 2026-11-01

- En `frontend/`, rama `frontend-next-hermes`, HEAD `a2e4266b01deeed447b1a1ad045a7086de3a1999`, con Node 24.21.0 y pnpm 12.15.0, `PLAYWRIGHT_PORT=4206 pnpm check` terminó exit 0: API inventory 185; lint, typecheck, tooling 67/67, Prettier y build Next.js completados (30 páginas estáticas); Playwright 65 aprobadas y 45 omitidas de 110. `phase-f6-document-verification.spec.ts` aprobó 1/1, cubriendo seis operaciones y descarga binaria **contra mocks**.
- El puerto 4206 estaba libre antes y quedó libre tras Playwright; `git diff --check` pasó. Permanece una advertencia no fatal `MODULE_TYPELESS_PACKAGE_JSON` en `src/lib/api/clp.ts`.
- No se inició ni consultó Go, PostgreSQL, storage o scanner, ni se usaron cuentas o sesiones. F6 continúa parcial; esta corrida no acredita integración autenticada ni persistencia.

### Alcance autorizado para la integración documental — 2026-11-01

- El owner autorizó las seis operaciones F6 contra el target F5 solo después de cerrar sus gates y extender privadamente la política de cuentas F5. No se autoriza reutilizar en F5 las cuentas/sesiones de F6 ni iniciar ahora API, workers o DDL.
- Hasta entonces, queda verificada únicamente la UI/E2E mock. La integración real requiere aún revisión/privilegios y concurrencia V81, gates V82, backup aislado restaurado/verificado, aplicación secuencial autorizada y preparación privada de cuentas F5; F6 permanece parcial.

### Revalidación integral del worktree F6 — 2026-11-01

- En la rama `frontend-next-hermes`, HEAD `a2e4266b01deeed447b1a1ad045a7086de3a1999`, `PLAYWRIGHT_PORT=4206 npm exec --yes --package=pnpm@12.15.0 -- pnpm check` terminó exit 0: inventario OpenAPI 185, lint, typecheck, tooling 67/67, Prettier, build con 30 rutas estáticas y Playwright 65 aprobadas/45 omitidas. El E2E documental aprobó 1/1 y verifica seis operaciones, CAS/ETag y bytes descargados, pero **solo contra mocks**. Se mantiene la advertencia no fatal `MODULE_TYPELESS_PACKAGE_JSON` en `src/lib/api/clp.ts`.
- El puerto 4206 estaba libre antes y después. `git diff --check` pasó; se conservaron todos los cambios preexistentes. No se usó navegador del usuario, credenciales/cuentas, API Go, PostgreSQL, storage ni scanner. F6 sigue parcial hasta completar la integración autenticada permitida después de cerrar F5 y extender la política privada F5.

### Revalidación integral posterior del worktree F6 — 2026-11-01

- En el worktree `/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes`, rama `frontend-next-hermes`, HEAD `a2e4266b01deeed447b1a1ad045a7086de3a1999`, `PLAYWRIGHT_PORT=4203 pnpm check` terminó exit 0. `api:check` confirmó 185 operaciones; lint, typecheck, tooling (66/66), Prettier y build (30 rutas estáticas) pasaron. Playwright ejecutó 110 pruebas: 65 pasaron y 45 se omitieron; `phase-f6-document-verification.spec.ts` pasó 1/1, con las seis operaciones y descarga binaria **contra mocks**.
- El preflight confirmó el puerto 4203 libre antes del arranque y después de la suite; `git diff --check` pasó. Se conservó todo el trabajo local existente. Persistió la advertencia no fatal `MODULE_TYPELESS_PACKAGE_JSON` en `frontend/src/lib/api/clp.ts`.
- No se inició ni consultó Go, PostgreSQL, storage o scanner, ni se usaron cuentas/sesiones o el navegador del usuario. La corrida no acredita integración autenticada ni persistencia; F6 permanece parcial hasta superar los gates F5.

### Revalidación integral Node 24 — 2026-11-01

- En el worktree `frontend-next-hermes`, rama `frontend-next-hermes`, HEAD `a2e4266b01deeed447b1a1ad045a7086de3a1999`, con Node 24.21.0, npm 12.1.0 y pnpm 12.15.0, `PLAYWRIGHT_PORT=4206 npm exec --yes --package=pnpm@12.15.0 -- pnpm check` terminó exit 0: inventario OpenAPI 185, lint, typecheck, tooling 67/67, Prettier y build Next.js (30 rutas estáticas); Playwright ejecutó 110 pruebas, 65 pasaron y 45 se omitieron. El spec documental pasó 1/1: seis operaciones, CAS/ETag y bytes del PDF comprobados **contra mocks**.
- El servidor Playwright usó `127.0.0.1:4206`; el puerto quedó libre después. El listener preexistente en `127.0.0.1:4205` se conservó. Persiste la advertencia no fatal `MODULE_TYPELESS_PACKAGE_JSON` en `frontend/src/lib/api/clp.ts`.
- No se inició ni consultó Go, PostgreSQL, storage o scanner y no se usaron cuentas/sesiones. Esta revalidación confirma UI/build/E2E controlado, no integración ni persistencia; F6 sigue parcial hasta cerrar los gates F5.

### Cobertura UI del propósito identidad — 2026-11-01

- Se agregó un segundo recorrido Playwright al spec documental: crea un caso `identity` con `document_expires_at` obligatorio (el payload no incluye `space_id`), lo lista/consulta, carga un archivo sintético con el ETag vigente, lo actualiza a limpio, lo envía con el ETag nuevo y confirma que la UI no ofrece ni solicita la descarga de una evidencia de identidad.
- El recorrido del expediente de espacio existente sigue cubriendo las seis operaciones documentales, incluida la descarga de bytes PDF. Ambos recorridos son interceptados con respuestas sintéticas; no acreditan BFF→Go, autorización de titular, persistencia, scanner ni almacenamiento real.
- En `frontend/`, `PLAYWRIGHT_PORT=4206 npm exec --yes --package=pnpm@12.15.0 -- pnpm check` terminó exit 0: inventario OpenAPI 185, lint, typecheck, tooling 67/67, Prettier, build Next.js con 30 rutas y Playwright 66 aprobadas/45 omitidas de 111. Los specs documentales aprobaron 2/2. Permanece la advertencia no fatal `MODULE_TYPELESS_PACKAGE_JSON` en `src/lib/api/clp.ts`.
- El ensayo usó `127.0.0.1:4206`, libre antes y después. Se preservó el listener preexistente de `127.0.0.1:4205`. `git diff --check` pasó. No se inició ni consultó Go, PostgreSQL, storage o scanner; no se usaron cuentas/sesiones ni el navegador del usuario. F6 sigue parcial y la integración real continúa gated por F5 y la política privada F5.

### Adjudicación del review del CAS stale — 2026-11-01

- La review source-only `deleg_130b80d8` creyó que el test espera ocultar el botón mientras la vista lo conserva. Revisión parent de la ruta completa corrige ese dictamen: `DocumentVerificationFacade.loadCases()` limpia `snapshot.selected` cuando el expediente no aparece en la lista del titular (`frontend/src/features/document-verification/application/document-verification-facade.ts:197-205`); por eso la tarjeta y el botón desaparecen y la expectativa Playwright (`phase-f6-document-verification.spec.ts:338-345`) sí corresponde al comportamiento. Mientras un expediente sí permanece seleccionado pero necesita refresh, carga y envío quedan deshabilitados (`document-verification-view.tsx:598,614`).
- Confirmación en el worktree padre `frontend-next-hermes`, HEAD `a2e4266b01deeed447b1a1ad045a7086de3a1999`: `PLAYWRIGHT_PORT=4206 npm exec --yes --package=pnpm@12.15.0 -- pnpm test:e2e tests/phase-f6-document-verification.spec.ts` pasó 2/2 pruebas en standalone. Puerto 4206 libre antes y después; `git diff --check` pasó.
- Cobertura exclusivamente mock UI; no prueba integración real ni persistencia/authorization Go o PostgreSQL. No hubo cambios de implementación frontend en esta adjudicación.

### Reconciliación del review fresco de refresh/409 — 2026-11-01

- La review source-only `deleg_27ed21e1` vuelve a reportar que el caso no se recarga al reaparecer, pero el call path actual contradice el finding: `handleRefresh` siempre llama `loadOwnCase(selectedCaseId)` cuando el ID está en la lista recibida (`document-verification-view.tsx:238-251`). El E2E ya verifica que, tras desaparecer de una respuesta y reaparecer en la siguiente, el envío vuelve a habilitarse (`phase-f6-document-verification.spec.ts:362-376`).
- También se verificó que el spec sí ejercita un HTTP 409 de upload: el mock avanza el ETag del servidor a `"2"` mientras la vista conserva `"1"`; el handler retorna 409 (`phase-f6-document-verification.spec.ts:162-169`), y el spec comprueba error, botón deshabilitado, mismo archivo nativo retenido, solo dos POST observados antes del retry explícito y éxito tras refresh (`:291-321`). La afirmación contraria del reviewer no coincide con el source vivo.
- Brecha confirmada: fallaba el GET de detalle después de que la lista confirmaba el caso, y no había aserción de que upload/submit permanecieran bloqueados. Se añadió al mock un 503 único de detalle y aserciones de bloqueo, ausencia de POST submit automático y habilitación solo tras el siguiente refresh con GET de detalle exitoso. No se cambió producción.
- Reejecución parent desde `frontend/`: el spec focal terminó **2/2 PASS**; luego `PLAYWRIGHT_PORT=4206 npm exec --yes --package=pnpm@12.15.0 -- pnpm check` terminó exit 0: inventario OpenAPI 185, ESLint, TypeScript, tooling 67/67, Prettier, build de 30 rutas y Playwright 66 aprobadas/45 omitidas de 111. Puerto 4206 libre antes y después; `git diff --check` pasó. Se mantiene la advertencia no fatal `MODULE_TYPELESS_PACKAGE_JSON` en `src/lib/api/clp.ts`. El escenario sigue siendo mock; no valida integración API/DB.

### Revisión independiente F6 y ajuste de cabeceras privadas — 2026-11-01

- La revisión source-only `deleg_6afd1a97` confirmó allowlist BFF, Bearer explícito con `credentials: "omit"`, ETag fuerte/`If-Match`, aislamiento de sesión en auth/fachada y revocación de Blob URL. Confirmó que autorización de descarga debe probarse en backend y que las pruebas actuales son mock/fake, no integración.
- Se revalidó en el árbol vivo que `handleRefresh` siempre vuelve a cargar el detalle cuando el ID reaparece (`document-verification-view.tsx:238-251`), por lo que ese finding también es falso positivo. El `key` de sesión remonta el formulario (`:73-84`), limpiando su `File` local al cambiar el fingerprint; la fachada descarta resultados obsoletos por generación.
- Se confirmó un ajuste contractual real: la ruta BFF reemplazaba `Cache-Control: private, no-store` de OpenAPI/backend por `no-store`. Ahora devuelve `private, no-store` tanto en respuestas proxy como errores sanitizados; se agregó aserción a la prueba gateway.
- Verificación parent en `frontend/`: `npm exec --yes --package=pnpm@12.15.0 -- pnpm --dir frontend exec node --test tooling/backend-gateway-credentials.test.mjs` pasó **4/4**; `git diff --check` pasó. No se reconstruyó ni ejecutó E2E porque el cambio es únicamente de cabecera/test.
- Límite pendiente señalado por review: tras upload exitoso la fachada incorpora el archivo devuelto al caso previo, no recarga el DTO completo. Revisar si la respuesta contractual permite estado autoritativo suficiente o si conviene refrescar detalle; no inferir integración real. Ninguna prueba de este bloque acredita backend/DB/storage real.

### Carrera de resultados dentro de una misma sesión — 2026-11-01

- La revisión source-only `deleg_3a5ba817` encontró que el fencing anterior cubría cambios de fingerprint/sesión, pero no impedía que una petición anterior de selección/listado/espacios sobrescribiera una respuesta posterior de la misma sesión.
- Se agregó control de generación por canal: resultados antiguos de casos (lista, detalle y mutaciones) y espacios se descartan como `AbortError` antes de modificar el snapshot; las generaciones de casos y espacios son independientes.
- TDD en `tooling/document-verification-facade.test.mjs`: la regresión de selección primero falló porque el resultado antiguo se aceptaba; tras la corrección, las regresiones de selección, lista y espacios comprueban que gana el request más reciente. Suite focal final: 7/7 PASS.
- `node --test tooling/*.test.mjs`: 69/69 PASS; `npm exec --no -- tsc --noEmit`, ESLint del facade y Prettier de ambos archivos pasaron; `git diff --check` limpio. Node reportó una advertencia preexistente no fatal `MODULE_TYPELESS_PACKAGE_JSON` en `src/lib/api/clp.ts`.
- En la actualización inicial aún no se había reconstruido ni ejecutado Playwright tras el cambio de fachada; esa limitación quedó resuelta por el gate posterior documentado abajo. No se usó navegador del usuario, API Go, DB, storage, scanner ni cuentas; las pruebas siguen siendo mock-only y F6 permanece parcial hasta la integración autorizada tras F5.

### Build y E2E focal posteriores al fencing de requests — 2026-11-01

- Tras el cambio de fachada se ejecutó `npm exec --yes --package=pnpm@12.15.0 -- pnpm build` seguido de `PLAYWRIGHT_PORT=4206 npm exec --yes --package=pnpm@12.15.0 -- pnpm test:e2e tests/phase-f6-document-verification.spec.ts`: build y E2E finalizaron exit 0; el spec pasó **2/2** (casos de espacio e identidad) contra mocks.
- El puerto 4206 estaba libre antes y quedó libre después; el listener ajeno 4205 se conservó. La suite de tooling 69/69, TypeScript, ESLint focal, Prettier y `git diff --check` también pasaron. No se inició API Go, DB, storage/scanner ni se usaron cuentas o el navegador del usuario.
- Este gate verifica build y comportamiento UI controlado, no integración persistida. F6 sigue parcial hasta el recorrido real autorizado después de cerrar F5.

### Manejo UI de resultados obsoletos — 2026-11-01

- La fachada ya descartaba requests más antiguos dentro de la misma sesión con `AbortError`; se añadió manejo explícito en la vista para que cancelaciones por supersession no aparezcan como fallos de carga/acción, no marquen CAS como stale ni alteren la selección nueva.
- Regresión Playwright con dos casos sintéticos y GET de detalle diferido: la selección nueva resuelve primero y habilita el formulario; al resolver tarde la petición anterior, la UI conserva el caso más nuevo y no muestra “No se pudo cargar el expediente seleccionado”. RED observado antes del manejo UI: el aviso espurio aparecía; GREEN tras el cambio: el escenario pasa.
- `pnpm build` pasó; `PLAYWRIGHT_PORT=4206 pnpm test:e2e tests/phase-f6-document-verification.spec.ts` pasó 3/3: flujo espacio, propósito identidad y selección obsoleta. El E2E es mock-only; no acredita Go, PostgreSQL, storage ni scanner.
- `PLAYWRIGHT_PORT=4206 npm exec --yes --package=pnpm@12.15.0 -- pnpm check` terminó exit 0: API inventory 185, ESLint, TypeScript, tooling 69/69, Prettier, Next build con 30 páginas estáticas y Playwright 67 aprobadas/45 omitidas de 112. Los 3 escenarios documentales focales pasaron dentro de la corrida; son mock-only. Se mantiene la advertencia no fatal `MODULE_TYPELESS_PACKAGE_JSON` en `src/lib/api/clp.ts`.
- Puerto 4206 quedó libre después; listener ajeno 4205 preservado. `git diff --check` se revalidó después. No se usó el navegador del usuario, cuentas, API Go, PostgreSQL, storage ni scanner. F6 permanece parcial y gated por F5.

### Adjudicación de revisión independiente F5/F6 — 2026-11-01

- `deleg_4c112421` revisó source-only el test de interleavings F5 y el F6 frontend en sus worktrees indicados; no editó ni ejecutó pruebas/DB/browser. En F5 atribuyó un posible deadlock a que `VerifyScanClaim` lee las mismas filas que la transferencia actualiza. Parent cotejó el código: `VerifyScanClaim` usa `READ ONLY` y un `SELECT EXISTS` ordinario sin `FOR UPDATE` (`scan_jobs.go:50-80`); PostgreSQL no bloquea ese snapshot de lectura por los row updates bajo MVCC. Además, el test espera transferencia mientras el poll está pausado antes del commit; no hay evidencia fuente de la inversión/lock del reviewer. El test sí verifica el caso opuesto con outcome manteniendo locks y transferencia esperando por `pg_blocking_pids`, y llama `VerifyScanClaim` para el segundo claim antes de registrar outcome (`scan_ownership_fence_controlled_integration_test.go:163-204`). Esta adjudicación es source-only, no prueba PostgreSQL.
- El review F6 reportó un posible body multipart mal tipado y revocación Blob al cambiar sesión. Parent verificó `openapi-fetch/src/index.js:81-115`: ejecuta el serializer proporcionado y usa exactamente su retorno como request body; el adapter retorna `FormData`, así que el cast del objeto-placeholder solo satisface el tipo generado y no es el body enviado. La UI deriva `key` de status/account/roles/capabilities/restrictedMode/expiry (`document-verification-view.tsx:77-87`); al cambiar fingerprint React desmonta el contenido, cuyo cleanup revoca el Blob URL (`:132-134`). El reporte no demuestra un defecto concreto. Sigue siendo útil añadir prueba directa del adapter para multipart, aparte de la cobertura gateway ya existente.
- Revalidación parent de fuentes actuales: `node --test tooling/document-verification-facade.test.mjs tooling/document-verification-domain.test.mjs tooling/authenticated-operation.test.mjs tooling/backend-gateway-credentials.test.mjs` — 16/16 PASS; `./node_modules/.bin/tsc --noEmit`, `node tooling/api-check.mjs` (185 operaciones), ESLint focal y Prettier focal PASS. Posteriormente se completó `PLAYWRIGHT_PORT=4206 npm exec --yes --package=pnpm@12.15.0 -- pnpm check`: el puerto 4206 estaba libre antes/después; `api:check`, lint, typecheck, tooling, Prettier y build pasaron; Playwright reportó 66 aprobadas y 45 omitidas. El recorrido F6 es UI/mock, no integración backend. `git diff --check` PASS. No se inició API/DB/storage/scanner ni se usaron cuentas/browser.
- F6 sigue parcial: todas estas pruebas son tooling/unitarias o mocks; no acreditan BFF→Go→PostgreSQL/storage/scanner. El test F5 controlado también sigue sin ejecución PostgreSQL hasta cerrar los gates autorizados de V81/V82.

### Segunda revisión independiente F6 — 2026-11-01

- `deleg_1355f387` task 2 revisó source-only frontend y handlers Go; no editó ni ejecutó pruebas/build/browser/DB. Sus supuestos faltantes de `400` para precondiciones malformadas no se reproducen en el árbol actual: OpenAPI declara `400` tanto en submit como upload y `ownerIfMatch` implementa 400 malformado/428 ausente. La autorización de descarga se mantiene en el backend; la UI no se considera frontera de seguridad.
- El reviewer señaló la revocación de Blob a los 2 segundos como heurística susceptible a descargas lentas; no aportó una reproducción que demuestre falla. Se mantiene como punto de observación, no defecto confirmado: el Blob URL también se revoca al cambiar sesión/capacidades/recurso y al desmontar. No se alteró el timeout sin un contrato de ciclo de descarga que permita probarlo.
- Se reforzó `phase-f6-document-verification.spec.ts` para comprobar en la solicitud real del navegador que el body multipart serializado contiene el campo `file`, el nombre sintético y los bytes del fixture, además del `Content-Type` y `If-Match`. `PLAYWRIGHT_PORT=4206 npm exec --yes --package=pnpm@12.15.0 -- pnpm test:e2e tests/phase-f6-document-verification.spec.ts` pasó 3/3; el puerto estaba libre antes y quedó libre después. Es E2E local con BFF/Go mockeados, no integración real.
- La integración real browser→BFF→Go→PostgreSQL/storage/scanner sigue sin verificarse; F6 subordinada a gates F5 y política privada de cuentas del target exacto.

### Adjudicación de re-revisión F6 — respuestas y vencimiento — 2026-11-01

- `deleg_284d03ec` task 1 señaló que submit de un caso con archivos no limpios podía devolver 400 en Go y que el vencimiento futuro no estaba documentado. Relectura del árbol encontró que OpenAPI ya declaraba `400` para create/submit; la afirmación de ausencia de esos status era incorrecta. Sí había una brecha semántica de contrato: las dos variantes `document_expires_at` no indicaban que el instante debe ser futuro. Se añadieron esas descripciones, junto con un test de contrato que verifica 400 y futuro estricto; RED fue por la descripción ausente y GREEN después del cambio.
- La revisión sí descubrió una divergencia real en Go: `Submit` devuelve `ErrFilesNotClean`, pero `ownerError` lo convertía en `503`, mientras OpenAPI y el cliente esperan error de entrada/estado `400`. Se añadió `TestOwnerCasesSubmitRejectsUncleanFilesAsBadRequest`; RED confirmó 503 antes del cambio. El mapeo ahora responde 400; el test focal y suite HTTP con `-race`, `go vet` pasaron. La UI marca actualización necesaria para fallos no-409 y evita reenvío automático.
- `PLAYWRIGHT_PORT=4206 ... pnpm check` posterior pasó: api-check (185 operaciones), ESLint, TypeScript, tooling 71/71, Prettier, build y Playwright 67 PASS/45 SKIP de 112; puerto 4206 libre antes/después. Los E2E siguen con Go/DB/storage/scanner simulados y no acreditan integración real. Continúan los límites F5/V81 y política privada de cuentas.

### Revisión del escenario de refresh fallido y CAS — 2026-11-01

- `deleg_085327a1` no evaluó la revisión del working tree porque la prueba y fuentes F6 están sin seguimiento en el checkout y ausentes del HEAD indicado `a2e4266b01deeed447b1a1ad045a7086de3a1999`. Esto limita atribución a commit, pero no invalida inspección del contenido local; no se publicó ni se cambió HEAD.
- Parent inspeccionó test/facade/vista/adapter/OpenAPI actuales. El escenario solicita submit con ETag obsoleto (un único POST 409), luego lista exitosa y detalle 503; mantiene upload/submit deshabilitados, no reintenta POST, el refresh posterior exitoso los habilita, la desaparición del caso limpia el detalle visible y su reaparición lo vuelve a cargar. También verifica upload multipart, 409 con If-Match obsoleto, preservación del `File` nativo y conteo/orden de requests. Son assertions contra respuestas sintéticas, no integración real.
- Reejecución en parent: `PLAYWRIGHT_PORT=4206 npm exec --yes --package=pnpm@12.15.0 -- pnpm test:e2e -- tests/phase-f6-document-verification.spec.ts` lanzó toda la suite por el forwarding de argumentos: 67 PASS/45 SKIP de 112; los 3 escenarios F6 pasaron. Puerto 4206 libre antes/después. F6 sigue parcial y el HEAD actual no incluye el trabajo local sin seguimiento.
- Resultado posterior del proceso local reportó `pnpm build` exit 0 y 2/2 escenarios F6; el árbol actual contiene además el test de selección tardía. Parent repitió el comando enfocado sin `--` adicional contra el standalone recién construido: 3/3 escenarios pasaron y el puerto 4206 quedó libre. Son recorridos mock; no acreditan integración con Go/DB/storage/scanner.

### Revalidación integral posterior de F6 — 2026-11-01

- En `frontend/`, worktree `frontend-next-hermes`, rama `frontend-next-hermes`, HEAD `a2e4266b01deeed447b1a1ad045a7086de3a1999`, con `PLAYWRIGHT_PORT=4206 npm exec --yes --package=pnpm@12.15.0 -- pnpm check` y puerto libre verificado antes/después: exit 0. `api:check` confirmó 185 operaciones; lint, typecheck, tooling 71/71, Prettier y build Next.js (30 páginas estáticas) pasaron. Playwright ejecutó 112 pruebas: 67 aprobadas y 45 omitidas. Los tres escenarios `phase-f6-document-verification.spec.ts` aprobaron: recorrido espacio, propósito identidad y selección tardía supersedida.
- Las pruebas F6 son E2E de UI con upstream Go simulado; no acreditan integración autenticada ni persistencia en PostgreSQL/storage/scanner. Node emitió la advertencia no fatal preexistente `MODULE_TYPELESS_PACKAGE_JSON` en `src/lib/api/clp.ts`. No se inició API, DB, storage, scanner ni se usaron cuentas/sesiones o navegador del usuario. F6 permanece parcial y gated por F5.

### Revisión independiente del fencing de requests F6 — 2026-11-01

- `deleg_63dfc949` revisó source-only el facade, la vista, el adapter y el spec Playwright del worktree en HEAD `a2e4266b01deeed447b1a1ad045a7086de3a1999`; verdict PASS, sin riesgos de seguridad ni errores lógicos concretos. Confirmó que los resultados supersedidos no mutan el estado y que el `AbortError` de cancelación no se presenta como error visible.
- El caso diferido selecciona un segundo expediente, luego resuelve tarde el GET anterior y comprueba que no aparece error espurio y se conserva la selección nueva. Cobertura de cambio de cuenta/sesión y otros canales se apoya en las guardas del facade, no en esta regresión UI específica. El flujo completo mock mantiene las assertions de seis operaciones, Bearer, `If-Match` y bytes de descarga; no demuestra integración backend/storage.
- La revisión no ejecutó pruebas ni cambió archivos. La revalidación integral parent inmediatamente anterior está registrada arriba: tooling 71/71 y Playwright 67/45 de 112, exit 0; los tres escenarios F6 pasaron con Go upstream simulado.

### Corrección de clasificación de AbortError F6 — 2026-11-01

- Se reprodujo el hallazgo `deleg_948047dc`: un `Error` normal con `name="AbortError"` dejaba su mensaje en el snapshot del facade (`facade.getSnapshot().error`). El test nuevo falló primero con `actual: "superseded request"`, esperado `null`.
- Se añadió `isAbortError` estructural y compartido en `frontend/src/lib/api/errors.ts`, basado en `name`, y se reutilizó en el facade, la vista y el catch de carga de consentimientos de registro. Así, abortos representados como `Error`/objeto no-DOMException no publican alertas falsas ni quedan almacenados como error.
- Sobre la segunda observación de `deleg_948047dc`: no se confirma que el auth facade pierda el signal. `runAuthenticatedOperation` invoca el callback como `(accessToken, signal)` y `#advanceGeneration` aborta el controller de sesión (`frontend/src/features/auth/application/auth-facade.ts:254–275,288–297`); la fachada documental propaga el signal recibido al port. Se agregó una aserción al test real de `AuthFacade`: logout aborta el signal previo antes de que se resuelva la respuesta tardía. Las requests supersedidas dentro de una misma sesión se cercan por generación, pero no se cancelan en red; la respuesta tardía se descarta y esto no es una falta de fencing de estado.
- TDD/validación local: la regresión de facade fue RED antes del fix; después pasaron los tres grupos `node --test tooling/authenticated-operation.test.mjs tooling/document-verification-facade.test.mjs tooling/api-primitives.test.mjs` (18/18), `pnpm format:check`, y `git diff --check`. La revalidación de producción `PLAYWRIGHT_PORT=4206 ... pnpm check` terminó exit 0 con 67 Playwright aprobadas y 45 omitidas. El cambio de código es local; las pruebas de interfaz continúan siendo mock-only, no se conectó Go/DB/storage/scanner ni se publicó.
- Review independiente source-only `deleg_770df549` confirmó `isAbortError` basado en `error.name`, su uso en facade/vista/registro, y el trayecto `AuthFacade`→fachada→ports→adapter/API client; no encontró errores lógicos. La review llamó brecha a la propagación exacta del signal a cada operación y a su aborto por cambio de generación. Adjudicación parent: el aborto en cambio de sesión ya estaba probado en `tooling/authenticated-operation.test.mjs` (logout aborta el signal capturado y rechaza el resultado tardío); sí faltaba comprobar la identidad del mismo signal en cada operación de documento.
- Se añadió prueba de fachada que captura el argumento final en list, espacios, create, get-case, upload, submit y download; el segundo get-case cubre la recarga necesaria para descarga. RED antes de ajustar el auth fake: los ports recibían signals distintos al signal de sesión esperado. GREEN después: las 8 invocaciones recibieron exactamente el mismo `AbortSignal`. Las pruebas del servicio/adapter conservan límites distintos: es cobertura de fachada y no observación de red real. El review también señaló que supersession dentro de la misma sesión descarta resultados por generación pero no cancela activamente el transporte; se conserva como limitación no bloqueante.
- Revalidación parent: `node --test tooling/*.test.mjs` pasó **74/74**; `pnpm format:check` pasó; `PLAYWRIGHT_PORT=4206 ... pnpm check` terminó exit 0: API inventory 185, lint/typecheck, Prettier, build Next.js (30 rutas) y Playwright 67 aprobadas/45 omitidas de 112. El spec documental continúa contra mocks; permanece la advertencia no fatal `MODULE_TYPELESS_PACKAGE_JSON` en `src/lib/api/clp.ts`. Puerto 4206 libre antes y después.
- Un proceso asíncrono anterior informó fallo de Prettier en `tooling/document-verification-openapi.test.mjs`; la revalidación parent con `prettier --write` informó `unchanged`, `pnpm format:check` pasó y el `pnpm check` repetido terminó exit 0, por lo que el fallo no se reprodujo en el árbol verificado.
- No se inició ni consultó Go, PostgreSQL, storage o scanner; no se usaron cuentas/sesiones ni navegador del usuario. Las pruebas siguen siendo unitarias/UI mock, F6 permanece parcial y gated por F5; no hubo publicación.
