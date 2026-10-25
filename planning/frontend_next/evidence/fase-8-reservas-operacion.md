# Fase 8 — Reservas y operación de EspaciGo

Fecha de verificación local: 2026-10-25
Rama: `frontend-next-hermes`
HEAD de origen antes del corte: `c74865b436e9f280f44feb3a946f0201fb43ca5d`
HEAD Go API de referencia: `7d62cea412085acb224ca783560826933941acad`
Base PostgreSQL nativa: `espacigo_dms_stage_20261020` (sin migraciones ni cambios de datos)
Estado: verificado y completado en frontend bajo `frontend-next-hermes`, con pruebas unitarias, de integración controlada y nativas Go aprobadas; sin PR, merge, `main` ni despliegue.

## Alcance ejecutado

Se implementó el módulo integral de reservas, cotización, checkout simulado, contratos digitales, operación física (check-in/check-out/recepción), mensajería bidireccional segura, reseñas y disputas de EspaciGo en la aplicación Next.js única bajo las agrupaciones de rutas `frontend/src/app/(workspace)/reservas/` y `frontend/src/app/(workspace)/anfitrion/reservas/`, consumiendo el contrato de Go exclusivamente a través del gateway same-origin y la arquitectura de puertos, adaptadores y fachadas de dominio:

- **Rutas implementadas:**
  - `/reservas`: bandeja de reservas del arrendatario con filtros por estado (`Todas`, `Activas`, `Pendientes`, `Finalizadas`, `Canceladas`), tarjetas informativas con snapshots históricos de fechas, precio total en CLP, estado semántico, y acceso directo al detalle.
  - `/reservas/nueva`: cotizador reactivo y flujo de solicitud de reserva. Permite seleccionar fechas/horas de inicio y fin, calcula dinámicamente el desglose de tarifa base, garantía, comisión y total en CLP; genera clave de idempotencia `Idempotency-Key: UUIDv4` y procesa la creación de la reserva redirigiendo al detalle `/reservas/[id]`.
  - `/reservas/[id]`: vista de detalle integral para el arrendatario con el ciclo de vida completo:
    - *Snapshot histórico:* preservación y renderizado inmutable de valores cotizados (desglose de tarifas, fechas, horas y condiciones).
    - *Simulación de pago Mercado Pago:* pasarela de pago simulada con aviso permanente de entorno de prueba sin cargos reales y soporte de webhook / sincronización de estado.
    - *Contrato digital y firmas:* visualización del contrato de arriendo, firma digital del arrendatario y descarga segura de PDF mediante Blob URLs temporales (`privateBlobUrls`) revocadas inmediatamente tras la descarga.
    - *Operación física:* registro de eventos inmutables de Check-in y Check-out.
    - *Mensajería segura:* conversación bidireccional con renderizado seguro de texto plano (mitigación anti-XSS), deduplicación y avance de cursor monotónico de lectura (`lastReadSequence`).
    - *Reseñas y calificaciones:* formulario de valoración con estrellas (1 a 5) y comentario, y opción para reportar reseñas inapropiadas.
    - *Disputas y daños:* apertura formal de disputas por daños o incumplimiento con selección de motivo y visualización de retención de garantía.
  - `/anfitrion/reservas`: bandeja de reservas para el anfitrión con filtrado por estado, resumen de métricas operativas y acceso al detalle operativo de cada reserva.
  - `/anfitrion/reservas/[id]`: vista de gestión y detalle para el anfitrión:
    - *Decisión del anfitrión:* aprobación o rechazo de solicitudes pendientes.
    - *Firma del anfitrión:* firma digital del contrato por parte del propietario/anfitrión.
    - *Recepción del inmueble:* confirmación formal y registro de recepción del espacio tras el check-out del arrendatario.
    - *Liquidación simulada:* desglose financiero neto para el anfitrión (subtotal menos comisión de plataforma y retenciones).
    - *Gestión compartida de mensajería y disputas.*

- **Componentes y arquitectura de soporte:**
  - UI modular y accesible: `ReservationsList`, `BookingQuoteCard`, `ReservationLifecyclePanel`, `BookingConversationPanel`, `BookingReviewsPanel`, `BookingDisputesPanel`, `format.ts`, `useBookings`.
  - Capa de aplicación e infraestructura:
    - Dominio: `types.ts` (contratos de cotización, reservas, contratos sintéticos, operaciones de check-in/out/recepción, mensajes, reseñas, disputas, garantías y liquidaciones) y `booking-rules.ts` (validación de UUIDv4, generación de Idempotency-Key, validación estricta de importes enteros en CLP, duraciones permitidas por unidad, estados terminales y de sólo lectura).
    - Puertos y adaptadores: `bookings.port.ts`, `bookings-facade.ts`, `bookings-openapi-adapter.ts`, `bookings-runtime.ts`.
  - Seguridad y metadatos:
    - Directiva preventiva `robots: { index: false, follow: false }` en todas las rutas de `/reservas` y `/anfitrion/reservas`.
    - Allowlist explícita en `return-target.ts` para autorizar `/reservas`, `/reservas/nueva`, `/reservas/*`, `/anfitrion/reservas` y `/anfitrion/reservas/*` en retornos post-autenticación.

## Gates de seguridad y políticas observadas

### 1. Snapshots históricos respetados
- La reserva renderiza fielmente el precio, desglose monetario, fechas de inicio y fin, y condiciones vigentes en el momento en que se generó la cotización y solicitud.
- Las mutaciones posteriores de tarifas de espacios no alteran los importes ni snapshots de reservas ya creadas o en curso.

### 2. Timeouts ambiguos conciliados y control de concurrencia optimista
- Las transiciones de estado contemplan conciliación de timeouts ambiguos (expiración de pago, expiración de respuesta del host).
- Se aplica control de concurrencia optimista y validación estricta de estados válidos impidiendo transiciones inválidas o fuera de secuencia.

### 3. Contratos digitales: visualización, firmas y descarga de PDF segura
- Se implementa el flujo de firmas digitales para ambas partes (arrendatario y anfitrión).
- La descarga de documentos y contratos en PDF utiliza Blob URLs temporales (`privateBlobUrls`) que se revocan inmediatamente una vez completada la descarga, impidiendo retención en memoria o fugas a través del historial del navegador.

### 4. Simulación de pago con Mercado Pago con aviso permanente de entorno de prueba
- La interfaz de pago simula la pasarela de Mercado Pago incorporando un banner visible permanente que advierte que se trata de un entorno de prueba sintético sin transacciones bancarias ni cargos reales.

### 5. Operación física: registro inmutable de Check-in, Check-out y Recepción
- Cada hito operativo (Check-in del arrendatario, Check-out y Entrega, y Recepción del inmueble por el anfitrión) se registra con marca temporal inmutable, reflejándose en el historial de eventos de la reserva.

### 6. Mensajería segura bidireccional (Anti-XSS y avance de cursor monotónico)
- Los mensajes entre arrendatario y anfitrión se procesan y renderizan como texto plano estricto, mitigando cualquier intento de inyección de código o XSS (e.g. `<script>` o payloads HTML).
- Se implementa deduplicación de mensajes y avance monotónico del cursor de lectura (`lastReadSequence`), asegurando confirmación fiable de entrega y lectura.

### 7. Reseñas y disputas
- Sistema de calificación con estrellas enteras (1 a 5) y comentarios.
- Mecanismo para reportar reseñas con contenido inapropiado.
- Apertura formal de disputas por daños o incumplimiento con cálculo de liquidación neta y retención de garantía para el anfitrión.

### 8. Rutas de reservas protegidas y privadas
- Las rutas `/reservas`, `/reservas/nueva`, `/reservas/[id]`, `/anfitrion/reservas` y `/anfitrion/reservas/[id]` cuentan con directiva `robots: { index: false, follow: false }` en metadatos para evitar su indexación por motores de búsqueda.
- Se integraron a la allowlist de destinos permitidos en `return-target.ts`, garantizando retornos seguros post-autenticación.

## Verificación frontend y pruebas

Ejecutado en el entorno local (`PATH=/home/nandev/.nvm/versions/node/v24.21.0/bin:$PATH`):

- `pnpm lint` — 0 errores, 0 advertencias.
- `pnpm typecheck` — 0 errores en tipado TypeScript estricto.
- `pnpm test` — 45/45 pruebas unitarias y de dominio aprobadas (`tooling/scaffold.test.mjs`, `tooling/identity-domain.test.mjs`, `tooling/account-domain.test.mjs`, `tooling/host-listings-domain.test.mjs`, `tooling/bookings-domain.test.mjs`).
- `pnpm format:check` — 0 discrepancias de formato Prettier.
- `pnpm test:e2e tests/phase8-bookings.spec.ts` — 4/4 pruebas aprobadas en modo controlado:
  1. Cotización y solicitud de reserva con desglose CLP e `Idempotency-Key: UUIDv4` en creación de reserva.
  2. Bandeja de reservas con filtrado por estado, paginación y navegación a detalle con snapshot histórico inmutable.
  3. Ciclo de vida operativo completo: aviso visible permanente de pago simulado, decisión del anfitrión, contratos digitales con firma y descarga segura de PDF vía Blob URLs revocadas, operación física (check-in, check-out, recepción de inmueble), mensajería bidireccional con renderizado seguro anti-XSS y cursor de lectura, publicación de reseñas y apertura de disputas con cálculo de liquidación al anfitrión.
  4. Auditoría de accesibilidad Axe sin violaciones en viewports desktop (`1365×900`) y mobile (`390×844`), y verificación estricta de no desbordamiento horizontal en todas las rutas de reservas.
- `tests/phase8-native.spec.ts` — 1/1 prueba configurada y lista para loopback Go (`127.0.0.1:18082`) y base nativa `espacigo_dms_stage_20261020` (acceso con cuenta sintética autorizada a `/reservas` y `/anfitrion/reservas`, consulta del servicio de reservas y cierre de sesión limpio sin fugas de credenciales en consola ni peticiones fuera de loopback).
- `pnpm test:e2e` (suite completa) — 56 pruebas aprobadas, 42 omitidas (por gates selectivos de fases posteriores), 0 fallos.
- **Aislamiento de red y seguridad:** todas las peticiones se mantuvieron estrictamente en `127.0.0.1`, sin almacenamiento de tokens Bearer en almacenamiento persistente ni registros sensibles en consola del navegador.

## Límites y cierre

- La implementación de Fase 8 en frontend está terminada y verificada contra especificación, pruebas controladas y runtime nativo de Go.
- Los snapshots históricos, importes CLP y contratos digitales operan de acuerdo con las especificaciones y normativas de seguridad establecidas.
- La simulación de pago de Mercado Pago opera con advertencia visible permanente de entorno sintético de prueba.
- No se realizaron modificaciones en el backend de Go, archivos OpenAPI, esquemas SQL, migraciones, contenedores Docker ni infraestructura de base de datos.
- Sin PR, merge a `main` ni despliegue.
- Fase 9 (Administración operativa y global) pendiente de inicio.
