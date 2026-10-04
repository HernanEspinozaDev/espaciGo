# Grafo de dependencias y Kanban

## Estado del tablero

Hermes Kanban local `espacigo` contiene 103 tarjetas; 187 dependencias se verificaron por CLI el 2026-10-04 y coinciden exactamente con los 187 vínculos declarados en `backlog.md`. Lectura de estados del 2026-10-04: 12 `done`, 1 `blocked`, 90 `todo`. El registro fuente suma 10 `done`, 1 `blocked`, 92 `todo`; las dos discrepancias residuales son AUTH-ARCH-01 y AUTH-DB-01 (`done` en Kanban, `todo` en fuente) mientras sus artefactos requieren revisión. CORE-ENV-01 está `done` en Kanban y backlog tras la fusión de PR #11 el 2026-10-04 (merge SHA `f57c932a10fe17ca371ffcc53b51154ade66650a`). PR #8 quedó `MERGED` el 2026-10-02 (CORE-TEST-01, merge SHA `45451871f53b384e8ee5e1a66bbe7cf82feba434`); su alcance fue la definición documental del harness, no un runner/DB de pruebas. CORE-ENV-01 ya no está bloqueada por CORE-TEST-01 y su contrato documental cubre topología sin ruta mock→DB y checklist para la implementación futura. CORE-ENV-02 quedó `done` tras aprobación y merge de PR #9 el 2026-10-04 (merge SHA `40124256ec7c5df81c7983656d55ec43aa3a19cd`); Chromium headless verificó carga/fetch readiness por CORS (HTTP 200 y status `ready`), y la prueba TCP desde la red real del mock a IP dinámica falló por timeout (`timeout=true`, límite 2 s), con control positivo `connect=success` desde `data`. DNS `temporary=true` sigue inconcluso. Evidencias preservadas en `planning/evidence/`. AUTH-DB-02 es la única tarjeta actualmente `blocked` en `espacigo`: el usuario ratificó el 2026-10-04 las decisiones de correo, sesiones y tokens; falta registrarlas en `planning`, fusionar sus prerrequisitos AUTH-ARCH-01/AUTH-DB-01 y disponer de PostgreSQL descartable con permisos DDL. DB02-09 mantiene abiertas RQF-213/-217/-218; no implementar DDL dependiente. El estado se contrastó con el CLI; no se editó directamente la base del tablero.

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
  A0 --> E0[CORE-ENV-01: planificación local aislada (done, PR #11 merged)]
  T0 --> E0
  D3 --> E1[CORE-ENV-02: Compose aislado (done, PR #9 merged)]
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
4. CORE-API-01 quedó `done` por PR #7 fusionado el 2026-10-02 (SHA `100370e2e24dbf81a29d62014d6731081d4ff060`). CORE-TEST-01 también quedó `done` por PR #8 fusionado ese día (SHA `45451871f53b384e8ee5e1a66bbe7cf82feba434`), únicamente por la definición documental del harness. CORE-ENV-01 ya tiene satisfechas CORE-DB-03/CORE-API-01/CORE-TEST-01 y su documento `planning/entorno_local_aislado.md` cumple el entregable de diagrama/contrato sin ruta mock→DB más checklist; el estado `done` y su evidencia quedaron sincronizados mediante PR #11, fusionado el 2026-10-04 (merge SHA `f57c932a10fe17ca371ffcc53b51154ade66650a`). CORE-ENV-02, con dependencias CORE-DB-03/CORE-API-01/CORE-TEST-01 satisfechas, completó Chromium/Playwright para el mock y fetch readiness CORS, además del aislamiento TCP desde la red real del mock (`connect=failed timeout=true`, límite 2 s) y el control positivo en `data` (`connect=success`); DNS `temporary=true` sigue inconcluso. PR #9 fue aprobado y fusionado a `main` el 2026-10-04 (merge SHA `40124256ec7c5df81c7983656d55ec43aa3a19cd`); la tarjeta está `done` en Kanban y backlog.
5. AUTH-ARCH-01 también es elegible por CORE-API-01 y CORE-DB-02, pero el resultado local en `t_84cb0ac5` no está en un PR/main y contiene decisiones que requieren revisión; se conserva `todo` en el registro fuente. AUTH-DB-01 también permanece `todo` hasta que su artefacto local sea publicado y aceptado.
6. AUTH-DB-02 permanece `blocked`: el usuario ratificó el 2026-10-04 las decisiones de correo, expiración por inactividad y TTL/límites de tokens; falta registrarlas en `planning`, fusionar los prerrequisitos AUTH-ARCH-01/AUTH-DB-01 y disponer de PostgreSQL descartable con permisos DDL. AUTH-BE-01 depende de AUTH-DB-02 (`blocked`) y CORE-BE-01 (`done`). El audit CLI del 2026-10-04 encontró vacía la lista `ready`; todas las tarjetas DB/Backend en `todo` tienen al menos un padre no `done` (p. ej. PRIV-DB-01 espera PRIV-ARCH-01 en `todo`). No hay siguiente tarjeta DB/Backend habilitada: no se promovió ni inició ninguna. DB02-09 sigue como bloqueo separado para DDL/implementación dependiente.
7. Desarrollar M02 y M03 en paralelo si el equipo lo permite; M04 espera ambos.
8. M04 → M05 → M06, porque catálogo, tarifa, disponibilidad y cotización son prerrequisitos del flujo transaccional.
9. M07 y M08 parten desde M06; M08 además requiere contrato firmado según reglas aplicables. M09 requiere reserva y operación; M10 requiere reclamo/evidencia y cierre operativo.
10. M11 reportes operacionales al final; la infraestructura lógica mínima de auditoría/outbox ya se trabaja en fundación y se extiende en cada módulo.

## Conteo sincronizado al 2026-10-04

Conteo sincronizado por CLI el 2026-10-04: Kanban contiene 103 tarjetas y 187 aristas; las 187 relaciones coinciden exactamente con las declaradas en `backlog.md` (sin diferencias de aristas). Kanban: 12 `done`, 1 `blocked`, 90 `todo`; fuente: 10 `done`, 1 `blocked`, 92 `todo`. Las dos discrepancias de estado que quedan son AUTH-ARCH-01 y AUTH-DB-01 (`done` en Kanban, `todo` en fuente) hasta revisar sus decisiones/artefactos. CORE-TEST-01 y CORE-ENV-02 están `done` por PR #8 y PR #9 fusionados; AUTH-DB-02 sigue `blocked`. La lista `ready` está vacía y la auditoría de dependencias no encontró tarjeta DB/Backend `todo` con todos sus padres `done`; no se promovió ningún ticket. Evidencias CORE-ENV-02 preservadas en `planning/evidence/`. El estado se leyó/escribió mediante la CLI admitida; no se editó directamente la base del tablero.

El estado representa el avance y las dependencias del tablero local. No se asignaron responsables humanos ni fechas sin acuerdo del equipo.
