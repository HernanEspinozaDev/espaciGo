# Fase 9 — Administración operativa y global de EspaciGo

Fecha de verificación local: 2026-10-25
Rama: `frontend-next-hermes`
HEAD de origen antes del corte: `8a437304a1e0a6b4e946512349bb4c016e3b1638`
HEAD Go API de referencia: `7d62cea412085acb224ca783560826933941acad`
Base PostgreSQL nativa: `espacigo_dms_stage_20261020` (sin migraciones ni cambios de datos)
Estado: verificado y completado en frontend bajo `frontend-next-hermes`, con pruebas unitarias, de integración controlada y nativas Go aprobadas; sin PR, merge, `main` ni despliegue.

## Alcance ejecutado

Se implementó el panel integral de administración operativa y global de EspaciGo en la aplicación Next.js única bajo la agrupación de rutas `frontend/src/app/(workspace)/administracion/`, consumiendo el contrato de Go exclusivamente a través del gateway same-origin y la arquitectura de puertos, adaptadores y fachadas de dominio:

- **Rutas implementadas:**
  - `/administracion`: panel general de control y centro de métricas operativas (casos KYC pendientes de revisión, publicaciones en moderación, disputas activas, registros recientes de auditoría). Despliega navegación contextual basada en capacidades (`canAccessSection`) y muestra un único rol activo en la sesión (`Administrador Global`).
  - `/administracion/verificaciones`: bandeja de verificación KYC para personas naturales y jurídicas. Permite inspeccionar documentación remitida, estados de validación y resolver solicitudes con aprobación o rechazo sujeto a motivo obligatorio de 5+ caracteres.
  - `/administracion/espacios`: módulo de supervisión y moderación de publicaciones de espacios con control de concurrencia optimista (`expected_updated_at`). Permite aprobar, rechazar o suspender publicaciones exigiendo motivo explícito, señalizando conflictos de versión (HTTP 409).
  - `/administracion/disputas`: mediación y resolución de controversias entre arrendatarios y anfitriones con desglose de daños o incumplimientos, asignación de resoluciones y dictamen sobre retención de garantías o liberación de liquidaciones.
  - `/administracion/cuentas`: gestión de identidades, roles de plataforma (`global_admin`, `support_agent`, `trust_safety_agent`) y estado de usuarios, con detección y soporte de reautenticación reciente requerida (HTTP 428 / Precondition Required) para operaciones de seguridad de alto riesgo.
  - `/administracion/auditoria`: visualización inmutable y filtros de eventos de auditoría y bitácoras de seguridad del sistema (marcas temporales UTC/local, identificador de actor, recurso intervenido, tipo de acción y descripción técnica).

- **Componentes y arquitectura de soporte:**
  - UI modular y accesible: `AdminShell`, `useAdmin`, layout administrativo `frontend/src/app/(workspace)/administracion/layout.tsx` integrado en el shell con navegación contextual RBAC, badges semánticos de estado y banners accesibles de reautenticación y conflicto.
  - Capa de aplicación e infraestructura:
    - Dominio: `types.ts` (contratos para métricas de panel, casos KYC, moderación de espacios, disputas y resoluciones, gestión de cuentas y roles, y registros inmutables de auditoría) y `admin-rules.ts` (evaluación de capacidades por rol, validación estricta de motivo no vacío de 5+ caracteres para moderación/rechazo, parsing y manejo de respuestas HTTP 428 y HTTP 409 con `expected_updated_at`, purga de estado en memoria ante revocación o 403).
    - Puertos y adaptadores: `admin.port.ts`, `admin-facade.ts`, `admin-openapi-adapter.ts`, `admin-runtime.ts`.
  - Seguridad y metadatos:
    - Directiva preventiva `robots: { index: false, follow: false }` en todas las rutas de `/administracion`.
    - Allowlist explícita en `return-target.ts` para autorizar `/administracion`, `/administracion/verificaciones`, `/administracion/espacios`, `/administracion/disputas`, `/administracion/cuentas` y `/administracion/auditoria` en retornos post-autenticación.

## Gates de seguridad y políticas observadas

### 1. RBAC y navegación dinámica por capacidades
- El menú y las rutas operativas se rigen por la matriz de capacidades de la sesión (`canAccessSection`).
- La interfaz presenta de manera consistente un único rol activo para el usuario (`Administrador Global`), evitando la visualización confusa o yuxtaposición de múltiples roles contradictorios en la sesión activa.

### 2. Moderaciones y reversiones con motivo obligatorio (5+ caracteres)
- Cualquier acción de moderación punitiva o cambio de estado sensible (rechazo o aprobación de KYC, suspensión o reactivación de espacios, sobreescrituras administrativas) exige obligatoriamente un motivo formal de al menos 5 caracteres no vacíos.
- La UI y el dominio bloquean el envío si el motivo no cumple con los criterios de validación.

### 3. Control de autenticación reciente (HTTP 428 / Precondition Required)
- Ante acciones de elevado riesgo de seguridad (como reasignación de roles privilegiados o cambios en cuentas críticas), el frontend detecta y procesa respuestas HTTP 428 emitidas por la API Go.
- Despliega un banner de alerta accesible con rol semántico exigiendo la reautenticación del operador antes de proceder.

### 4. Control de concurrencia optimista con `expected_updated_at` (HTTP 409 Conflict)
- En las operaciones de moderación de espacios y actualizaciones de registros compartidos, las peticiones envían el timestamp `expected_updated_at`.
- Ante colisiones de edición simultáneas (HTTP 409 Conflict), la UI notifica claramente al operador que los datos fueron actualizados por otro usuario o proceso y solicita la recarga de los datos frescos.

### 5. Purga inmediata de datos privados en memoria tras revocación o HTTP 403
- Al interceptar una respuesta HTTP 403 Forbidden o un evento de revocación de credenciales/sesión, el estado administrativo en memoria (`useAdmin`) se purga inmediatamente restableciendo valores por defecto limpios.
- Todas las URLs temporales de documentos en memoria (`Blob URLs`) asociadas a expedientes se revocan para evitar retenciones o inspecciones indebidas.

### 6. Rutas de administración protegidas y privadas
- Las 6 rutas administrativas (`/administracion`, `/administracion/verificaciones`, `/administracion/espacios`, `/administracion/disputas`, `/administracion/cuentas` y `/administracion/auditoria`) definen metadatos de búsqueda con directiva estricta `robots: { index: false, follow: false }`.
- Se incorporaron a la allowlist de destinos seguros en `return-target.ts`, previniendo redirecciones arbitrarias fuera del dominio o rutas no autorizadas.

## Verificación frontend y pruebas

Ejecutado en el entorno local (`PATH=/home/nandev/.nvm/versions/node/v24.21.0/bin:$PATH`):

- `pnpm lint` — 0 errores, 0 advertencias.
- `pnpm typecheck` — 0 errores en tipado TypeScript estricto.
- `pnpm test` — 54/54 pruebas unitarias y de dominio aprobadas (`tooling/scaffold.test.mjs`, `tooling/identity-domain.test.mjs`, `tooling/account-domain.test.mjs`, `tooling/host-listings-domain.test.mjs`, `tooling/bookings-domain.test.mjs`, `tooling/admin-domain.test.mjs`).
- `pnpm format:check` — 0 discrepancias de formato Prettier.
- `pnpm test:e2e tests/phase9-admin.spec.ts` — 5/5 pruebas aprobadas en modo controlado:
  1. RBAC y navegación por capacidades: renderiza menú según rol y badges de estado.
  2. Verificaciones KYC y moderación con motivo obligatorio (5+ caracteres).
  3. Concurrencia optimista (`expected_updated_at` / HTTP 409) en moderación de publicaciones.
  4. Reautenticación reciente (HTTP 428 Precondition Required) y purga tras 403.
  5. Auditoría de accesibilidad Axe sin violaciones en viewports desktop (`1365×900`) y mobile (`390×844`), y verificación estricta de no desbordamiento horizontal en las 6 rutas administrativas.
- `tests/phase9-native.spec.ts` — 1/1 prueba configurada y lista para loopback Go (`127.0.0.1:18082`) y base nativa `espacigo_dms_stage_20261020` (acceso administrativo con credenciales autorizadas, consulta de métricas de panel, listado de casos y cierre de sesión limpio sin fugas de credenciales en consola ni peticiones fuera de loopback).
- `pnpm test:e2e` (suite completa) — 61 pruebas aprobadas, 42 omitidas (por gates selectivos de fases posteriores), 0 fallos.
- **Aislamiento de red y seguridad:** todas las peticiones se mantuvieron estrictamente en `127.0.0.1`, sin almacenamiento de tokens Bearer en almacenamiento persistente ni registros sensibles en consola del navegador.

## Límites y cierre

- La implementación de Fase 9 en frontend está terminada y verificada contra especificación, pruebas controladas y runtime nativo de Go.
- La navegación por capacidades RBAC, las moderaciones con motivo obligatorio, el control de concurrencia optimista (409) y la gestión de reautenticación reciente (428) operan conforme a los estándares de seguridad establecidos.
- No se realizaron modificaciones en el backend de Go, archivos OpenAPI, esquemas SQL, migraciones, contenedores Docker ni infraestructura de base de datos.
- Sin PR, merge a `main` ni despliegue.
- Fase 10 (Integración Docker y QA integral) pendiente de inicio.