# Grafo de dependencias y Kanban

## Estado del tablero

> **Registro operativo al 2026-10-05:** GitHub Project `EspaciGo — Desarrollo` contiene 104 Issues sin duplicados y 190 dependencias nativas verificadas. La distribución actual es 13 `Hecho`, 2 `En revisión` y 89 `Bloqueado`; no hay tarjeta `Listo` habilitada. El tablero Hermes conserva la captura histórica, pero se archivó fuera del dispatcher. El párrafo siguiente y el diagrama Mermaid son snapshots/high-level de la planificación al 2026-10-04, no el inventario operativo completo. Ver [migracion_github_projects.md](migracion_github_projects.md) y [github_issue_map.csv](github_issue_map.csv) para el estado/relaciones vigentes.

Hermes Kanban local `espacigo` contiene 103 tarjetas. La captura CLI previa del 2026-10-04 registró 187 dependencias y coincidencia de enlaces con el backlog; esta actualización no modifica enlaces en Kanban. Se verificó puntualmente la cadena M01 (AUTH-DB-02 → AUTH-DB-01 → AUTH-ARCH-01/CORE-DB-03); no se repitió la auditoría global de aristas. Lectura de estados del 2026-10-04: 12 `done`, 1 `review`, 90 `todo`, 0 `blocked`, 0 `ready` y 0 `running`; el registro fuente suma los mismos estados. AUTH-ARCH-01 y AUTH-DB-01 están `done` tras PR #13/#14 aprobados y fusionados (SHAs `d495a03da6d76c1c795793376be6ca15670297fd` y `bd8144e611105306ca4163180ea3bb04530322fb`); AUTH-DB-02 está en `review`. CORE-ENV-01 está `done` en Kanban y backlog tras la fusión de PR #11 el 2026-10-04 (merge SHA `f57c932a10fe17ca371ffcc53b51154ade66650a`). PR #8 quedó `MERGED` el 2026-10-02 (CORE-TEST-01, merge SHA `45451871f53b384e8ee5e1a66bbe7cf82feba434`); su alcance fue la definición documental del harness, no un runner/DB de pruebas. CORE-ENV-01 ya no está bloqueada por CORE-TEST-01 y su contrato documental cubre topología sin ruta mock→DB y checklist para la implementación futura. CORE-ENV-02 quedó `done` tras aprobación y merge de PR #9 el 2026-10-04 (merge SHA `40124256ec7c5df81c7983656d55ec43aa3a19cd`); Chromium headless verificó carga/fetch readiness por CORS (HTTP 200 y status `ready`), y la prueba TCP desde la red real del mock a IP dinámica falló por timeout (`timeout=true`, límite 2 s), con control positivo `connect=success` desde `data`. DNS `temporary=true` sigue inconcluso. Evidencias preservadas en `planning/evidence/`. AUTH-DB-02 está en `review` en Kanban después de su desbloqueo mediante operación admitida; la rama `feat/auth-db-02-migrations` contiene la migración M01 y las pruebas PostgreSQL aprobadas, con evidencia en `planning/evidence/auth-db-02-postgresql.md`; DB02-09 mantiene abiertas RQF-213/-217/-218 y no se añadió DDL dependiente. El estado se contrastó con el CLI; no se editó directamente la base del tablero.

## Grafo global

Este diagrama resume el orden académico por módulo y no sustituye las 190 aristas operativas nativas. La recuperación AUTH-BE-01 conserva tres relaciones adicionales: AUTH-BE-01 → recuperación `t_112e6827`; recuperación → AUTH-BE-02; recuperación → AUTH-BE-03. Su justificación y verificación están en la [nota de migración](migracion_github_projects.md).

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

## Corte M05: simulación privada de tarifa en borrador propio

Issue #146 concentra diseño mínimo, migración, backend, API, pruebas enfocadas y mock en un PR. Sus dependencias reales ya están fusionadas: M04 borradores/catálogo (PR #134), atributos por categoría (PR #141), disponibilidad/zona horaria M06 (PR #145) y fundaciones CORE/M01 (PR #4–#9, #11, #13–#15). Reutiliza el servicio M06 de disponibilidad. No depende de búsqueda pública, KYC, reservas ni pagos. Los criterios globales de #54/#55 y las Issues originales M05/M06 permanecen abiertos; esta entrega puntual no los completa ni los cierra. Ver [decisión y reglas de cobro aprobadas](decisiones_m05_simulacion_precio_privada.md) y [mapa de Issues](github_issue_map.csv).

## Secuencia recomendada

1. `PLAN-ARCH-01` está cerrado con ratificación MAP-01–MAP-10 y PR #1. `CORE-ARCH-01` está cerrado tras aprobación y fusión de PR #2.
2. CORE-DB-01, CORE-DB-02 y CORE-DB-03 están cerradas por PRs #4, #5 y #3 fusionados. DB02-09 continúa abierto: cualquier DDL/implementación que dependa de preferencia de uso, historial de claves o evento de notificación necesita decisión y ticket trazable antes de ejecutarse.
3. CORE-BE-01 quedó `done` por PR #6 aprobado y fusionado el 2026-10-02. Kanban ya mostraba `done`; el merge quedó registrado mediante comentario admitido, sin transición ni edición directa.
4. CORE-API-01 quedó `done` por PR #7 fusionado el 2026-10-02 (SHA `100370e2e24dbf81a29d62014d6731081d4ff060`). CORE-TEST-01 también quedó `done` por PR #8 fusionado ese día (SHA `45451871f53b384e8ee5e1a66bbe7cf82feba434`), únicamente por la definición documental del harness. CORE-ENV-01 ya tiene satisfechas CORE-DB-03/CORE-API-01/CORE-TEST-01 y su documento `planning/entorno_local_aislado.md` cumple el entregable de diagrama/contrato sin ruta mock→DB más checklist; el estado `done` y su evidencia quedaron sincronizados mediante PR #11, fusionado el 2026-10-04 (merge SHA `f57c932a10fe17ca371ffcc53b51154ade66650a`). CORE-ENV-02, con dependencias CORE-DB-03/CORE-API-01/CORE-TEST-01 satisfechas, completó Chromium/Playwright para el mock y fetch readiness CORS, además del aislamiento TCP desde la red real del mock (`connect=failed timeout=true`, límite 2 s) y el control positivo en `data` (`connect=success`); DNS `temporary=true` sigue inconcluso. PR #9 fue aprobado y fusionado a `main` el 2026-10-04 (merge SHA `40124256ec7c5df81c7983656d55ec43aa3a19cd`); la tarjeta está `done` en Kanban y backlog.
5. AUTH-ARCH-01 quedó `done` tras PR #13 aprobado y fusionado el 2026-10-04 (SHA `d495a03da6d76c1c795793376be6ca15670297fd`); AUTH-DB-01 quedó `done` tras PR #14 aprobado y fusionado ese día (SHA `bd8144e611105306ca4163180ea3bb04530322fb`). Kanban y backlog coinciden; no se editó directamente la base del tablero.
6. AUTH-DB-02 quedó `done` tras revisión y merge del PR #15 (SHA `0911b9c85643c8893950b381aec89b6249511706`); evidencia PostgreSQL en `planning/evidence/auth-db-02-postgresql.md`. DB02-09 sigue abierta y sin DDL. AUTH-BE-01 depende de AUTH-DB-02 y CORE-BE-01 (`done`); esta entrega mantiene AUTH-BE-02/AUTH-BE-03 bloqueadas hasta la revisión y merge de su PR.
7. Desarrollar M02 y M03 en paralelo si el equipo lo permite; M04 espera ambos.
8. M04 → M05 → M06, porque catálogo, tarifa, disponibilidad y cotización son prerrequisitos del flujo transaccional.
9. M07 y M08 parten desde M06; M08 además requiere contrato firmado según reglas aplicables. M09 requiere reserva y operación; M10 requiere reclamo/evidencia y cierre operativo.
10. M11 reportes operacionales al final; la infraestructura lógica mínima de auditoría/outbox ya se trabaja en fundación y se extiende en cada módulo.

## Conteo sincronizado al 2026-10-04

Estado y cadena consultados el 2026-10-04: Kanban contiene 103 tarjetas; captura previa registró 187 aristas. AUTH-ARCH-01/AUTH-DB-01 y AUTH-DB-02 están `done` tras PRs #13/#14/#15; AUTH-DB-02 merge SHA `0911b9c85643c8893950b381aec89b6249511706`, evidencia en `planning/evidence/auth-db-02-postgresql.md`. AUTH-BE-01 se publica para revisión; AUTH-BE-02/AUTH-BE-03 dependen de revisión y merge de AUTH-BE-01 y permanecen bloqueadas. DB02-09 permanece abierto y no se añadió DDL. El grafo no cambia; el estado del tablero no se edita directamente desde este archivo.

El estado representa el avance y las dependencias del tablero local. No se asignaron responsables humanos ni fechas sin acuerdo del equipo.
