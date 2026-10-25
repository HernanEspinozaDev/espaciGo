# Fase 7 — Panel de anfitrión y gestión de espacios de EspaciGo

Fecha de verificación local: 2026-10-25
Rama: `frontend-next-hermes`
HEAD de origen antes del corte: `2ace074a6755c31341c4b7942dc7c0fad340018e`
HEAD Go API de referencia: `7d62cea412085acb224ca783560826933941acad`
Base PostgreSQL nativa: `espacigo_dms_stage_20261020` (sin migraciones ni cambios de datos)
Estado: verificado y completado en frontend bajo `frontend-next-hermes`, con pruebas unitarias, de integración controlada y nativas Go aprobadas; sin PR, merge, `main` ni despliegue.

## Alcance ejecutado

Se implementó el módulo integral de panel de anfitrión y gestión de publicaciones de espacios de EspaciGo en la aplicación Next.js única bajo la agrupación de rutas `frontend/src/app/(workspace)/anfitrion/`, consumiendo el contrato de Go exclusivamente a través del gateway same-origin y la arquitectura de puertos, adaptadores y fachadas de dominio:

- **Rutas implementadas:**
  - `/anfitrion`: redirección canónica hacia `/anfitrion/espacios`.
  - `/anfitrion/espacios`: listado reactivo de espacios del anfitrión (`GET /spaces`), con filtrado por estado (`Todos`, `Borradores`, `Publicados`, `Archivados`), badge de estado semántico, paginación, métricas resumen y acceso directo a la creación de nuevos espacios y a la edición.
  - `/anfitrion/espacios/nuevo`: asistente estructurado en 5 pasos para el alta de nuevos espacios borrador con clave de idempotencia (`Idempotency-Key: UUIDv4`):
    - *Paso 1 (Tipo de espacio):* consulta y renderizado dinámico de categorías desde Go (`GET /spaces/categories`), selección de categoría y definición del título principal.
    - *Paso 2 (Ubicación y características):* dirección física y formulario de atributos generado dinámicamente según el esquema de la categoría (`GET /spaces/categories/{code}/attributes`), preservando estrictamente el valor de `attribute_schema_version`.
    - *Paso 3 (Tarifas):* configuración de precios base en CLP enteros positivos y moneda estricta.
    - *Paso 4 (Disponibilidad):* horarios de atención semanales con normalización de días y rangos horarios.
    - *Paso 5 (Confirmación):* resumen completo de datos ingresados y envío de creación vía `POST /spaces` incorporando cabecera `Idempotency-Key`, con redirección automática al editor (`/anfitrion/espacios/{id}/editar`).
  - `/anfitrion/espacios/[id]/editar`: editor de espacios integral estructurado en 5 pestañas temáticas, con soporte de control de concurrencia optimista (`If-Match: "{version}"`):
    - *Pestaña Información:* edición de datos básicos, dirección y atributos específicos de la categoría (`PUT /spaces/{id}/publication-content`) preservando el `attribute_schema_version` original.
    - *Pestaña Tarifas:* ajuste de tarifas base en CLP enteros (`PUT /spaces/{id}/tariff`) y visualización del historial cronológico de tarifas registradas (`GET /spaces/{id}/tariffs`).
    - *Pestaña Horarios:* actualización de ventanas de disponibilidad y horarios de apertura semanales (`PUT /spaces/{id}/availability`).
    - *Pestaña Fotos:* galería fotográfica con aviso visible permanente de entorno de ensayo sintético, carga de imágenes (`POST /spaces/{id}/photos`), selección de foto de portada (`PUT /spaces/{id}/photos/cover`) y reordenamiento (`PUT /spaces/{id}/photos/reorder`).
    - *Pestaña Publicación:* gestión del ciclo de vida del espacio (`PUT /spaces/{id}/publication`) para publicar formalmente borradores completos o archivar/retirar publicaciones existentes.

- **Componentes y arquitectura de soporte:**
  - UI desacoplada y accesible: `HostLayout`, `HostNavigation`, `HostListingsList`, `ListingWizard`, `ListingEditor`.
  - Capa de aplicación e infraestructura:
    - Dominio: `types.ts` (contratos de espacio, categorías, esquemas de atributos, tarifas, horarios, fotos y publicación) y `validation.ts` (validación de títulos, códigos postales, precios CLP enteros positivos, disponibilidad semanal y generación de UUIDv4 para idempotencia).
    - Puertos y adaptadores: `host-listings.port.ts`, `host-listings-facade.ts`, `host-listings-openapi-adapter.ts`, `host-listings-runtime.ts`.
  - Seguridad y metadatos:
    - Configuración preventiva `robots: { index: false, follow: false }` en todas las páginas de `/anfitrion/`.
    - Allowlist explícita en `return-target.ts` para autorizar `/anfitrion/espacios`, `/anfitrion/espacios/nuevo` y `/anfitrion/espacios/*` en flujos de autenticación con parámetro `return_to`.

## Gates de seguridad y políticas observadas

### 1. Sin lista manual autoritativa de categorías
- El frontend no almacena ni define un enum o catálogo estático de categorías de espacios.
- Todas las categorías disponibles se obtienen dinámicamente desde el endpoint Go `GET /spaces/categories`.
- Los campos de atributos específicos de cada categoría se construyen dinámicamente a partir del esquema provisto por `GET /spaces/categories/{code}/attributes`.

### 2. Preservación estricta de `attribute_schema_version`
- Tanto en el asistente de creación como en la edición de contenido, el frontend conserva y retransmite de manera estricta la propiedad `attribute_schema_version` recibida desde el backend.
- Se garantiza que los payloads de `POST /spaces` y `PUT /spaces/{id}/publication-content` mantengan la correlación exacta con la versión del esquema de atributos requerida por la API Go.

### 3. Importes en CLP y horarios semanalmente normalizados
- Todos los importes monetarios se validan como números enteros positivos en pesos chilenos (CLP), rechazando valores negativos, decimales o monedas incompatibles antes de invocar la API.
- Las reglas de disponibilidad semanal se normalizan mediante estructuras canónicas por día y franjas horarias válidas, garantizando coherencia con las restricciones del modelo de dominio de Go.

### 4. Cobertura de conflictos e idempotencia
- La creación de nuevos espacios en `/anfitrion/espacios/nuevo` genera y envía una cabecera `Idempotency-Key: UUIDv4` única por operación, mitigando duplicados por reenvío accidental.
- Las actualizaciones en el editor (`/anfitrion/espacios/[id]/editar`) gestionan la concurrencia optimista utilizando cabeceras `If-Match: "{version}"`, controlando adecuadamente escenarios de colisión (HTTP 412 Precondition Failed / HTTP 409 Conflict).

### 5. Galería fotográfica con aviso sintético permanente
- El gestor de fotos incorpora un banner visible permanente indicando el carácter simulado del entorno de ensayo fotográfico.
- Se implementa soporte completo para designar la imagen de portada y reordenar las fotografías de la galería mediante llamadas dedicadas al contrato de la API.

### 6. Rutas de anfitrión protegidas y privadas
- Las rutas `/anfitrion/espacios`, `/anfitrion/espacios/nuevo` y `/anfitrion/espacios/[id]/editar` cuentan con directiva `robots: { index: false, follow: false }` en metadatos para evitar su indexación por motores de búsqueda.
- Se integraron a la lista de destinos permitidos de retorno post-login en `return-target.ts`, asegurando una navegación fluida y segura tras la autenticación.

## Verificación frontend y pruebas

Ejecutado en el entorno local (`PATH=/home/nandev/.nvm/versions/node/v24.21.0/bin:$PATH`):

- `pnpm lint` — 0 errores, 0 advertencias.
- `pnpm typecheck` — 0 errores en tipado TypeScript estricto.
- `pnpm test` — 39/39 pruebas unitarias y de dominio aprobadas (`tooling/scaffold.test.mjs`, `tooling/identity-domain.test.mjs`, `tooling/account-domain.test.mjs`, `tooling/host-listings-domain.test.mjs`).
- `pnpm format:check` — 0 discrepancias de formato Prettier.
- `pnpm test:e2e tests/phase7-host-listings.spec.ts` — 4/4 pruebas aprobadas en modo controlado:
  1. `/anfitrion/espacios`: listado de espacios, filtrado por estado (`Todos`, `Borradores`, `Publicados`, `Archivados`), badge de estado, paginación y renderizado sin desbordamiento.
  2. `/anfitrion/espacios/nuevo`: asistente en 5 pasos (Paso 1: selección dinámica de categorías desde `/spaces/categories`, título; Paso 2: dirección y atributos dinámicos con `attribute_schema_version`; Paso 3: tarifas CLP; Paso 4: disponibilidad semanal; Paso 5: confirmación y creación con `Idempotency-Key` en `POST /spaces` redirigiendo a `/anfitrion/espacios/{id}/editar`).
  3. `/anfitrion/espacios/[id]/editar`: navegación por pestañas (Información con `PUT /spaces/{id}/publication-content`, Tarifas e historial con `PUT /spaces/{id}/tariff` y `GET /spaces/{id}/tariffs`, Horarios con `PUT /spaces/{id}/availability`, Fotos con aviso visible permanente de entorno sintético y reordenamiento, y Publicación con `PUT /spaces/{id}/publication`).
  4. Auditoría de accesibilidad Axe sin violaciones en viewports desktop (`1365×900`) y mobile (`390×844`), y verificación estricta de no desbordamiento horizontal.
- `PHASE7_NATIVE_API_E2E=1 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 PLAYWRIGHT_PORT=4205 pnpm test:e2e tests/phase7-native.spec.ts` — 1/1 prueba aprobada contra loopback Go (`127.0.0.1:18082`) y base nativa `espacigo_dms_stage_20261020` (login con cuenta sintética autorizada, acceso a `/anfitrion/espacios`, consulta dinámica de `/spaces/categories`, y logout limpio sin fugas de credenciales en consola ni peticiones fuera de loopback).
- `pnpm test:e2e` (suite completa) — 52 pruebas aprobadas, 42 omitidas (por gates selectivos de fases posteriores), 0 fallos.
- **Aislamiento de red y seguridad:** todas las peticiones se mantuvieron estrictamente en `127.0.0.1`, sin almacenamiento de tokens Bearer en almacenamiento persistente ni registros sensibles en consola del navegador.

## Límites y cierre

- La implementación de Fase 7 en frontend está terminada y verificada contra especificación, pruebas controladas y runtime nativo de Go.
- Las categorías y atributos se consumen en tiempo real de la API Go sin definiciones estáticas locales.
- La galería fotográfica opera con avisos permanentes de entorno sintético.
- No se realizaron modificaciones en el backend de Go, archivos OpenAPI, esquemas SQL, migraciones, contenedores Docker ni infraestructura de base de datos.
- Sin PR, merge a `main` ni despliegue.
- Fase 8 (Reservas y operación) pendiente de inicio.
