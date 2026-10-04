# Grafo de dependencias y Kanban

## Estado del tablero

Hermes Kanban local `espacigo` contiene 103 tarjetas; las 184 dependencias corresponden al último conteo verificado el 2026-10-02 y este cambio no modifica enlaces. Lectura de estados del 2026-10-04: 11 `done`, 1 `review`, 1 `blocked`, 90 `todo`. El registro fuente suma 8 `done`, 1 `review`, 1 `blocked`, 93 `todo`; la diferencia de tres `done` del tablero corresponde a CORE-ENV-01, AUTH-ARCH-01 y AUTH-DB-01, aún `todo` en fuente por falta de PR fusionado. PR #8 quedó `MERGED` el 2026-10-02 (CORE-TEST-01, merge SHA `45451871f53b384e8ee5e1a66bbe7cf82feba434`); su alcance fue la definición documental del harness, no un runner/DB de pruebas. CORE-ENV-01 ya no está bloqueada por CORE-TEST-01, pero sigue `todo` en la fuente. CORE-ENV-02 sigue `review` en Kanban: Chromium headless verificó carga/fetch readiness por CORS (HTTP 200 y status `ready`); la prueba TCP desde la red real del mock a IP dinámica falló por timeout (`timeout=true`, límite 2 s), con control positivo `connect=success` desde `data`. PR #9 está abierto; no se marca aceptada/done. AUTH-DB-02 es la única tarjeta actualmente `blocked` en `espacigo`: falta ratificar canonicalización/casefold y unicidad de correo, expiración por inactividad, TTL/límites de tokens por propósito y disponer de PostgreSQL descartable con permisos DDL. DB02-09 mantiene abiertas RQF-213/-217/-218; no implementar DDL dependiente. El estado se contrastó con el CLI; no se editó directamente la base del tablero.

## Grafo global

```mermaid
flowchart TD
  P[PLAN-ARCH-01: validar mapa global]
  P --> C[CORE-ARCH-01: límites y contratos]
  C --> D1[CORE-DB-01: convenciones PostgreSQL]
  D1 --> D2[CORE-DB-02: revisión de esquema/privacidad]
  D2 --> D3[CORE-DB-03: base de migraciones local]
  D3 --> B0[CORE-BE-01: límites del monolito Go]
  B0 --> A0[CORE-API-01: OpenAPI, auth y errores]
  D3 --> T0[CORE-TEST-01: harness DB/API (done, PR #8)]
  A0 --> E0[CORE-ENV-01: planificar entorno local (todo)]
  T0 --> E0
  D3 --> E1[CORE-ENV-02: Compose aislado (review, PR #9)]
  A0 --> E1
  T0 --> E1
  D3 --> M1[M01 Identidad]
  A0 --> M1
  M1 --> M2[M02 Perfil/privacidad]
  M1 --> M3[M03 Verificación]
  M2 --> M4[M04 Publicación/calendario]
  M3 --> M4
  M4 --> M5[M05 Búsqueda/cotización]
  M5 --> M6[M06 Reserva/pago]
  M6 --> M7[M07 Contratos]
  M6 --> M8[M08 Operación]
  M7 --> M8
  M6 --> M9[M09 Comunicación/reputación]
  M8 --> M9
  M8 --> M10[M10 Disputas/liquidación]
  M9 --> M10
  M10 --> M11[M11 Administración/auditoría]
  E1 -. habilita tarjetas MOCK .-> V[Una tarjeta MOCK al final del módulo]
```

Cada módulo tiene su propio subgrafo en `backlog.md`: diseño de módulo → modelado DB → migración → dominio/repositorio → casos de uso → API → pruebas → mock. M04 y M06 añaden cortes funcionales por tamaño. El orden global permite paralelizar M02 y M03 después de identidad; contratos y operaciones se separan tras reservas, con dependencias de sus estados.

## Secuencia recomendada

1. `PLAN-ARCH-01` está cerrado con ratificación MAP-01–MAP-10 y PR #1. `CORE-ARCH-01` está cerrado tras aprobación y fusión de PR #2.
2. CORE-DB-01, CORE-DB-02 y CORE-DB-03 están cerradas por PRs #4, #5 y #3 fusionados. DB02-09 continúa abierto: cualquier DDL/implementación que dependa de preferencia de uso, historial de claves o evento de notificación necesita decisión y ticket trazable antes de ejecutarse.
3. CORE-BE-01 quedó `done` por PR #6 aprobado y fusionado el 2026-10-02. Kanban ya mostraba `done`; el merge quedó registrado mediante comentario admitido, sin transición ni edición directa.
4. CORE-API-01 quedó `done` por PR #7 fusionado el 2026-10-02 (SHA `100370e2e24dbf81a29d62014d6731081d4ff060`). CORE-TEST-01 también quedó `done` por PR #8 fusionado ese día (SHA `45451871f53b384e8ee5e1a66bbe7cf82feba434`), únicamente por la definición documental del harness. CORE-ENV-01 ya tiene esa dependencia satisfecha y permanece `todo` en la fuente hasta su propio entregable. CORE-ENV-02, con dependencias CORE-DB-03/CORE-API-01/CORE-TEST-01 satisfechas, completó la comprobación Playwright/Chromium de mock y fetch readiness por CORS, así como la prueba TCP negativa desde la red real del mock con `timeout=true` y el control positivo desde `data`; DNS `temporary=true` sigue inconcluso. La rama `feat/core-env-02-compose` se publicó en el PR #9 abierto para revisión; no se ha fusionado ni marcado la tarjeta aceptada/done.
5. AUTH-ARCH-01 también es elegible por CORE-API-01 y CORE-DB-02, pero el resultado local en `t_84cb0ac5` no está en un PR/main y contiene decisiones que requieren revisión; se conserva `todo` en el registro fuente. AUTH-DB-01 también permanece `todo` hasta que su artefacto local sea publicado y aceptado.
6. AUTH-DB-02 es el próximo candidato de implementación DB por orden de M01, pero no está habilitado: Kanban lo marca `blocked`, no se pudo probar DDL en una instancia desechable y siguen pendientes decisiones sobre canonicalización de correo, semántica de expiración inactiva y TTL/límites de tokens. DB02-09 continúa abierto aparte; no crear DDL que lo presuponga. AUTH-BE-01 permanece dependiente de la migración aceptada.
7. Desarrollar M02 y M03 en paralelo si el equipo lo permite; M04 espera ambos.
8. M04 → M05 → M06, porque catálogo, tarifa, disponibilidad y cotización son prerrequisitos del flujo transaccional.
9. M07 y M08 parten desde M06; M08 además requiere contrato firmado según reglas aplicables. M09 requiere reserva y operación; M10 requiere reclamo/evidencia y cierre operativo.
10. M11 reportes operacionales al final; la infraestructura lógica mínima de auditoría/outbox ya se trabaja en fundación y se extiende en cada módulo.

## Conteo sincronizado al 2026-10-04

Conteo verificado por CLI el 2026-10-04: Kanban contiene 103 tarjetas; el último conteo disponible de dependencias es 184 (2026-10-02), sin cambios de enlaces en esta actualización. El tablero muestra 11 `done`, 1 `review`, 1 `blocked`, 90 `todo`. El registro fuente suma 8 `done`, 1 `review`, 1 `blocked`, 93 `todo`: CORE-TEST-01 pasó a `done` por PR #8; CORE-ENV-02 está `review` con PR #9 abierto. Chromium confirmó visualmente el mock y el fetch readiness por CORS; el probe TCP desde la red real del mock a la IP dinámica terminó `connect=failed timeout=true` dentro del límite de 2 s, y el control positivo autorizado desde `data` fue `connect=success`. La consulta DNS `temporary=true` sigue inconclusa y no es evidencia aprobatoria. Las evidencias están en `planning/evidence/`. Las tres tarjetas que Kanban marca `done` pero siguen `todo` en la fuente son CORE-ENV-01/AUTH-ARCH-01/AUTH-DB-01 por falta de PR fusionado. AUTH-DB-02 figura `blocked` en ambos registros. No se editó el almacenamiento del tablero ni se avanzó CORE-ENV-02 a aceptada/done.

El estado representa el avance y las dependencias del tablero local. No se asignaron responsables humanos ni fechas sin acuerdo del equipo.
