# Fase 5 — Identidad y onboarding de EspaciGo

Fecha de verificación local: 2026-10-25
Rama: `frontend-next-hermes`
HEAD de origen antes del corte: `436a41d46894495c25eb2c3dfffe86964e42b208`
HEAD Go API de referencia: `7d62cea412085acb224ca783560826933941acad`
Base PostgreSQL nativa: `espacigo_dms_stage_20261020` (sin migraciones ni cambios de datos)
Estado: verificado y completado en frontend bajo `frontend-next-hermes`, con limitación de no enumeración backend documentada; sin PR, merge, `main` ni despliegue.

## Alcance ejecutado

Se implementó el ciclo completo de identidad y onboarding para EspaciGo dentro de la aplicación Next.js única bajo la agrupación de rutas `frontend/src/app/(auth)/`, consumiendo exclusivamente el contrato de Go a través del gateway same-origin y la capa tipada de adaptadores/fachadas:

- **Rutas implementadas:**
  - `/acceso`: formulario de login con manejo de estados de carga, bloqueo genérico ante credenciales inválidas (401), respeto de cabecera `Retry-After` ante rate-limiting (429) y retorno contextual seguro.
  - `/registro`: formulario de alta con validación de contraseña robusta, carga dinámica de términos y privacidad desde `/auth/terms`, aprobación explícita de identificadores de versión y bloqueo preventivo si los documentos legales no están disponibles o completos.
  - `/verificar-correo`: confirmación de correo electrónico mediante token de acción o reemisión de enlace opaque con tratamiento genérico.
  - `/recuperar-acceso`: solicitud de enlace de recuperación y restablecimiento de contraseña mediante credenciales de un solo uso.

- **Componentes y arquitectura de soporte:**
  - UI desacoplada y accesible: `AuthShell`, `AccessScreen`, `RegistrationScreen`, `ActionScreens`, `PasswordField`, `RateLimitNotice`.
  - Capa de aplicación e infraestructura: `onboarding.port.ts`, `onboarding-facade.ts` y `onboarding-openapi-adapter.ts`.
  - Hooks de ciclo de vida seguros:
    - `useActionCredential`: extracción, saneamiento y ciclo de vida de tokens pasados por fragmento URL (`#token=...&token_id=...`).
    - `useRetryRemaining`: cronómetro reactivo que computa el bloqueo exclusivamente desde la cabecera `Retry-After` de HTTP 429 entregada por Go.
    - `useOnboarding`: orquestador de estados de flujo, errores mapeados y llamadas idempotentes.
  - Dominio puro: validación de contraseñas, políticas de consentimientos legales y parser estricto de `return_to`.

## Gates de seguridad y políticas observadas

### 1. Tokens de acción seguros en hash fragment
- Los tokens de verificación y recuperación se leen **exclusivamente desde el hash fragment** (`#token=...&token_id=...`). No se admiten parámetros sensibles en la query string (`?token=...`) para evitar su filtración en logs de servidor, proxies, cabeceras `Referer` o historiales.
- Tras su lectura en cliente, el hook ejecuta de inmediato `window.history.replaceState` preservando el pathname limpio, removiendo el fragmento del historial del navegador y de la barra de direcciones.
- Si el usuario cancela la acción, reinicia el formulario o se dispara un evento `hashchange`, la credencial en memoria se purga de forma inmediata.
- El token se envía únicamente en el cuerpo JSON hacia los endpoints de Go (`POST /auth/verify-email`, `POST /auth/password-reset`) y **nunca se persiste** en `localStorage`, `sessionStorage`, cookies, variables globales ni salidas de consola.

### 2. Login confirmado por sesión
- La respuesta exitosa de `POST /auth/login` no navega ciegamente al destino.
- El frontend condiciona la navegación a la verificación previa y exitosa de `GET /auth/session` a través del gateway same-origin, confirmando que la sesión en memoria está plenamente inicializada con sus roles, capacidades y modo restringido antes de transicionar a vistas protegidas o al retorno contextual.

### 3. Mitigación de enumeración de cuentas y límites de backend
- **En frontend:** los flujos de recuperación de contraseña y reemisión de verificación implementan textos informativos y timing genéricos. Una solicitud de recuperación con correo inexistente o existente muestra la misma pantalla de confirmación opaca sin delatar la presencia de la cuenta.
- **Comportamiento y limitación observada en backend Go:**
  - En registro duplicado, la API Go responde con HTTP 409 y código `email_registered`. El frontend captura este error y ofrece de forma segura un enlace hacia la recuperación de acceso, sin inventar un comportamiento divergente.
  - En login con una cuenta cuyo correo no ha sido confirmado, la API Go emite HTTP 403 con código `email_unverified`. El frontend presenta el enlace para solicitar la reemisión del correo de verificación.
  - Estas respuestas son emitidas directamente por el backend de Go según el contrato vigente; quedan documentadas formalmente como una limitación parcial de no enumeración a nivel de servicio backend sin modificar el código de Go ni el OpenAPI.

### 4. Consentimiento legal y versiones de documentos
- `/auth/terms` se consulta para obtener las versiones activas de los documentos legales obligatorios (términos y política de privacidad).
- Si el backend responde HTTP 503 (`terms_unavailable`) o entrega metadatos sintéticos sin el contenido completo (`document`), el frontend **bloquea el formulario de registro** e informa al usuario de la indisponibilidad sin inventar cláusulas, versiones sintéticas ni consentimientos simulados.
- En caso de registrarse con documentos válidos, el payload envía explícitamente los `version_id` auditados que el usuario aceptó.

### 5. Retorno contextual seguro (`return_to`)
- El parámetro `return_to` se valida con una lista blanca estricta de rutas canónicas públicas del catálogo (`/explorar`, `/categorias/...`, `/espacios/...`, `/como-funciona`, `/terminos-y-condiciones`, `/politica-de-privacidad`).
- Se descartan entradas malformadas, URLs externas, dobles barras (`//`), caracteres de control o intentos de redirección abierta (open redirect), realizando un fallback determinista y seguro a `/`.

## Verificación frontend y pruebas

Ejecutado en el entorno local (`PATH=/home/nandev/.nvm/versions/node/v24.21.0/bin:$PATH`):

- `pnpm lint` — 0 errores, 0 advertencias.
- `pnpm typecheck` — 0 errores en tipado TypeScript estricto.
- `pnpm test` — 28/28 pruebas unitarias aprobadas (scaffold base, parser de retorno contextual, reglas de dominio de identidad y credenciales en fragmento).
- `pnpm format:check` — 0 discrepancias de formato Prettier.
- `pnpm test:e2e tests/phase5-identity.spec.ts` — 15/15 pruebas aprobadas en modo controlado (simulación de fallas, tokens en hash fragment, sanitización de historial, reintentos, 429 con cabecera `Retry-After`, flujos de cancelación y validación de accesibilidad con Axe sin violaciones en viewports `390×844` y `1365×900`).
- `PHASE5_NATIVE_API_E2E=1 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 PLAYWRIGHT_PORT=4205 pnpm test:e2e tests/phase5-native.spec.ts` — 6/6 pruebas aprobadas contra el binario Go en loopback (`127.0.0.1:18082`) y la base nativa `espacigo_dms_stage_20261020`:
  1. Bloqueo de registro ante `/auth/terms` incompleto o en 503.
  2. Recuperación de cuenta desconocida respetando el contrato 202 genérico.
  3. Reemisión de verificación de cuenta desconocida devolviendo 204 opaco.
  4. Manejo genérico de tokens ficticios de verificación y recuperación.
  5. Intento de login desconocido devolviendo 401 genérico sin gatillar bloqueo de IP.
  6. Login de cuenta sintética autorizada existente con confirmación de sesión, retorno contextual y logout 204 verificado.
- `pnpm test:e2e` (suite completa) — 41 pruebas aprobadas y 41 omitidas (por gates selectivos de otras fases), 0 fallos.
- **Aislamiento de red y seguridad:** las pruebas verificaron que ninguna solicitud de red salió de `127.0.0.1`, sin exposición de contraseñas o tokens en consola o almacenamiento persistente.

## Límites y cierre

- La implementación de Fase 5 en frontend está terminada y verificada contra especificación, pruebas controladas y runtime nativo de Go.
- Se documenta la limitación parcial de backend de no enumeración donde Go emite 409 `email_registered` y 403 `email_unverified`.
- El registro en runtime nativo permanece coherentemente bloqueado mientras `/auth/terms` devuelva 503 o datos sintéticos sin contenido documental en la base de datos de staging.
- No se introdujo persistencia en `localStorage`, `sessionStorage` o cookies para tokens de sesión ni tokens de acción.
- No se realizaron modificaciones en el backend de Go, archivos OpenAPI, esquemas SQL, migraciones, contenedores Docker ni infraestructura de base de datos.
- Sin PR, merge a `main` ni despliegue.
- Fase 6 (Cuenta y verificaciones) pendiente de inicio.
