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
