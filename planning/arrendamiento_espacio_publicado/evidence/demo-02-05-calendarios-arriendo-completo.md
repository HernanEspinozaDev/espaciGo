# DEMO-02 a DEMO-05: Calendarios mensuales interactivos y arriendo simulado completo

- **Fecha de verificación:** 2026-11-14
- **Entorno:** Candidato local aislado `/home/nandev/Documentos/projects/espaciGo-rental-demo-reconciled`
- **Base Git:** `f3671834727417733a11132aef783fa9941ba012` (`origin/main`)
- **Worktree original preservado:** `/home/nandev/Documentos/projects/espaciGo-rent-published-space` intacto con 116 archivos respaldados bajo manifiesto SHA-256 en scratch.
- **Modo de ejecución:** En memoria sintético, loopback local `127.0.0.1`, sin base de datos real, sin KYC real, sin contrato jurídico vinculante y sin pasarela de pago real.

---

## 1. Resumen de Fases Completadas

### DEMO-02: Mes interactivo del propietario
- **Componente:** `MonthCalendar` compartido (`frontend/src/features/rental-demo/ui/month-calendar.tsx`), sin dependencias de librerías externas de calendario.
- **Vista:** `RentalDemoView` en modo anfitrión (`mode=host`), accesible en `/demo/arriendo?mode=host`.
- **Funcionalidades verificadas:**
  - Navegación mensual civil completa (mes anterior/siguiente/hoy, 31 días en enero, manejo de bisiestos y zonas horarias sin drift del navegador).
  - Presentación por día: estados `available`, `partial`, `unavailable` y bloques propios.
  - Creación de bloqueos con validación CAS mediante `expected_revision` contra el estado Go.
  - Eliminación de bloqueos propios vía `If-Match` con readback inmediato.

### DEMO-03: Mes público y continuación
- **Vista:** `RentalDemoView` en modo público (`mode=public`), accesible en `/demo/arriendo?mode=public`.
- **Funcionalidades verificadas:**
  - Misma fuente autoritativa en memoria que el propietario: si el propietario bloquea un tramo, la vista pública lo refleja inmediatamente sin exponer campos privados (`blocks` y `revision` se omiten en la proyección pública DTO).
  - Navegación de mes y selección de intervalos sin escrituras ni retenciones.
  - Solicitud de cotización (`POST /quotes`): genera snapshot inmutable con vigencia (10 min), tarifa y desglose CLP sin reservar ni bloquear el slot para otros concurrentes.

### DEMO-04: Recorrido completo simulado
- **Exclusiones autorizadas por el usuario aplicadas:**
  - **KYC omitido:** Indicador visible «KYC omitido en esta demo» sin consultar proveedor ni bloquear la prueba.
  - **Casilla de aprobación contractual:** Checkbox interactivo «Aprobar contrato» con leyenda explicativa persistente «Confirmación simulada: no se genera ni firma un contrato». La confirmación se bloquea estrictamente (400 Bad Request) si la casilla no está marcada.
  - **Pago simulado:** Botón «Pagar (simulado)» que invoca `POST /quotes/{id}/confirm` enviando `approved_contract: true` y UUID de intención estable.
  - **Resultado:** Estado `confirmed_demo` y pago `paid_simulated` con snapshot inmutable serializado.
  - **Concurrencia e idempotencia:** La re-confirmación con el mismo `intent_id` es idempotente (200 OK y mismo resultado). Una confirmación con un `intent_id` diferente sobre una cotización ya consumida es rechazada con `409 Conflict`.
  - **Sincronización:** El intervalo confirmado queda registrado como ocupado en memoria y desaparece inmediatamente tanto del calendario público como del calendario del propietario.

### DEMO-05: Revisión y aceptación local combinada
- Pruebas automatizadas ejecutadas y aprobadas:
  1. **Go Backend:**
     `go test -race ./cmd/rentaldemo ./internal/rentaldemo ./internal/calendar ./internal/pricing -count=1` -> PASS.
     Cubre: Store concurrente con exclusión mutua, persistencia en memoria, cotizaciones sin hold, confirmación CAS, validaciones civiles bisiesto/DST y rechazo de IPs/hosts no loopback.
  2. **Frontend Typecheck:**
     `pnpm typecheck` -> PASS (0 errores).
  3. **Frontend Suite Completa:**
     `pnpm test` -> 526 passed, 0 failed, 1 skipped.
     Incluye los tests dedicados de rental demo:
     - `rental-demo-adapter.test.mjs` (transporte sin auth de producto, adaptadores tipados OpenAPI)
     - `rental-demo-calendar.test.mjs` (cuadrícula civil lunes-domingo, formato español, independencia de zona horaria del host)
     - `rental-demo-gateway.test.mjs` (seguridad BFF loopback, aislamiento de cookies, rechazo CSRF y no-store)
     - `rental-demo-openapi.test.mjs` (separación estricta entre operaciones demo `/local/rental-demo` y producción)
     - `rental-demo-e2e.test.mjs` (recorrido completo extremo a extremo Go + BFF + Adaptador).
  4. **Higiene de código:**
     `git diff --check` -> PASS (limpio, sin espacios en blanco residuales ni marcadores de conflicto).

---

## 2. Límites y Estado Final
- El flujo comercial de producción (KYC real, contratos notariales/legales vinculantes, pasarela de pago real y persistencia PostgreSQL en `cmd/api`) permanece diferido y sin tocar según lo instruido por el usuario.
- Las fases DEMO-00 a DEMO-05 quedan plenamente implementadas, verificadas y respaldadas en el candidato aislado.
