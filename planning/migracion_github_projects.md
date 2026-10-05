# Migración de Hermes Kanban a GitHub Issues y Projects

Corte de verificación: 2026-10-05. Este documento registra la migración y separa el historial de la operación vigente; no modifica requisitos ni decisiones académicas de ES1/ES2.

## Destino operativo

- Repositorio: `HernanEspinozaDev/espaciGo`.
- Project: [EspaciGo — Desarrollo](https://github.com/users/HernanEspinozaDev/projects/1), Project #1.
- Snapshot de correspondencia creado durante la migración, una fila por tarjeta: [github_issue_map.csv](github_issue_map.csv). Contiene Issue, estados, PR relacionado y racional; no contiene campos de prioridad ni aristas de dependencia, que deben consultarse en GitHub.
- Guía para continuar desde un PC local: [ejecucion_con_github_projects.md](ejecucion_con_github_projects.md).
- La lectura/escritura del Project durante la migración se verificó con `HernanEspinozaDev`. Para el flujo posterior, `HernanMEC` es la cuenta de desarrollo y de commits/pushes/PRs; `HernanEspinozaDev` queda para revisión, aprobación y merge. Los scopes son por cuenta y no deben darse por compartidos. No se guardaron tokens en el repositorio.

## Inventario y trazabilidad

Se importaron 104 tarjetas sin duplicados: las 103 tarjetas originales del backlog más la recuperación `t_112e6827` (`RECOVERY-t_112e6827`, Issue #120). El Project tiene exactamente 104 elementos activos, uno por Issue y uno por fila de correspondencia. Los títulos y cuerpos remotos se cotejaron contra las 104 cargas preparadas; las 104 coincidieron exactamente.

Las Issues mantienen los IDs Hermes, código, texto original, referencias, criterios, estado histórico y racional de reconciliación. Se preservaron en el cuerpo de Issues 27 comentarios históricos con autor/fecha de origen, 24 registros individuales de ejecución/pruebas y 4 resultados de tarea. El cuerpo marca esos datos como históricos y no como una nueva aprobación o prueba, ni los presenta como autoría del usuario que importó. El archivo portable conserva además los eventos de Kanban; estos eventos no se reatribuyen como Issues/comentarios.

Distribución histórica de Hermes en la captura verificada: 14 `done`, 2 `blocked`, 1 `review`, 87 `todo`. Distribución operativa reconciliada al corte, leída del Project: 13 `Hecho`, 2 `En revisión`, 89 `Bloqueado`; no hay elementos `Listo`. Se reexaminaron los 14 `done` históricos contra Issues, PRs remotos, estados y archivos publicados: 13 tienen PR aprobado y fusionado con cambios correspondientes a su alcance (incluidos runner/tests de CORE-DB-03 #3, Compose/tests/evidencia de CORE-ENV-02 #9 y migración/tests/evidencia de AUTH-DB-02 #15); uno, AUTH-BE-01 (#29), se corrigió a `En revisión` porque PR #16 sigue abierto. GitHub no reporta checks en los PRs revisados; esta revisión no afirma CI verde y conserva las limitaciones registradas en cada Issue. CORE-API-01 conserva su nota histórica: no se ejecutó un parser OpenAPI dedicado en aquella ejecución. AUTH-DB-02 (#28) está `Hecho` tras PR #15 `MERGED` el 2026-10-04. AUTH-BE-01 (#29) y su recuperación (#120) siguen `En revisión`; AUTH-BE-02 (#30) y AUTH-BE-03 (#31) siguen `Bloqueado`.

Campos del Project configurados/verificados: `Status` con `Triage`, `Por hacer`, `Listo`, `En curso`, `Bloqueado`, `En revisión`, `Hecho`; campos de texto `Módulo`, `Tipo`, `ID Hermes`, `Estado Hermes (histórico)`; selector `Prioridad` con `Base de Datos`, `Backend`, `APIs`, `Pruebas`, `Frontend mock`; y `Fecha planificada`. Las prioridades de ejecución no se asignaron a tarjetas `ARCH/ENV` por tipo.

## Dependencias y recuperación

Se verificaron 190/190 relaciones `blocked_by` nativas: el conjunto remoto coincide exactamente con el grafo preparado, sin aristas faltantes, extras ni duplicados. Las tres relaciones adicionales válidas asociadas a la recuperación se conservaron:

| Bloqueadora | Bloqueada | Motivo |
| --- | --- | --- |
| AUTH-BE-01 (#29) | Recuperación `t_112e6827` (#120) | La recuperación completa y evidencia el alcance AUTH-BE-01; no se acepta antes de su revisión y merge. |
| Recuperación `t_112e6827` (#120) | AUTH-BE-02 (#30) | La descripción de AUTH-BE-02 exige la revisión/aceptación del trabajo recuperado además de AUTH-BE-01. |
| Recuperación `t_112e6827` (#120) | AUTH-BE-03 (#31) | La descripción de AUTH-BE-03 exige ambas entregas y conserva sus propios límites de alcance. |

Se mantienen también las aristas originales AUTH-BE-01 → AUTH-BE-02 y AUTH-BE-01 → AUTH-BE-03. La cadena adicional representa gates de aceptación distintos, no relaciones redundantes; el grafo completo fue comprobado sin faltantes ni extras. PR #16 está `OPEN` y requiere revisión; `autoMergeRequest` es nulo. No se habilitó auto-merge ni se fusionó PR alguno.

## Tarjetas archivadas y dispatcher Hermes

Antes de archivar el board, la consulta oficial `hermes kanban list --status archived --json` devolvió cero tarjetas; `hermes kanban list --json` y `hermes kanban list --archived --json` tenían los mismos 104 IDs únicos. Por tanto, la cobertura de tarjetas archivadas queda acreditada para el board: no había tarjetas con estado `archived` omitidas del inventario.

El board `espacigo` se archivó con `hermes kanban boards rm espacigo` (sin `--delete`), conservándolo de forma recuperable y retirándolo de los boards activos. La base archivada se leyó después de la operación: integridad SQLite `ok`; 104 tareas, 190 relaciones, 27 comentarios, 381 eventos y 24 ejecuciones; estados intactos. El dispatcher del gateway enumera boards no archivados, por lo que no despachará este board. El gateway Hermes general permanece activo para otros boards; los cinco perfiles revisados no tienen trabajos cron programados ni hooks con referencias a EspaciGo/Kanban. Antes del archivado no había tareas `ready` ni `running`.

## Exportación portable

El respaldo portable saneado se llama `espacigo-kanban-portable-sanitized.tar.gz`. Ubicación en el host de origen: `~/backups/espaciGo-migration-20261005T023355Z-29b4a0/espacigo-kanban-portable-sanitized.tar.gz` (no está en Git ni es una ruta del PC). Se verificaron integridad del tar y SQLite, conteos, relaciones, comentarios y modos de archivo restringidos. No contiene logs de workers; se conservaron metadatos, board, manifest, tareas, comentarios, dependencias, eventos y ejecuciones históricas. Se retiraron rutas absolutas locales, claims/IDs de sesión, PIDs, heartbeats y referencias de ejecución activa; el escaneo de rutas, tokens, credenciales, emails y teléfonos dio cero hallazgos. El original Hermes no se modificó antes del archivado y el archivo portable no está versionado en Git.

Checksum SHA-256 del artefacto verificado: `78a8c37a57dd2e59127bfe1bdb75de282f4377889c3e0f586c3e18e7afa88fed`.

## Automatización de GitHub

GraphQL lista cinco workflows del Project: `Auto-add sub-issues to project`, `Auto-close issue`, `Item added to project`, `Pull request linked to issue` y `Pull request merged`. Estos workflows del Project no equivalen a auto-merge de PR. Comprueba el PR y el estado del Issue/Project después de un merge; no marques `Hecho` sin aceptación y evidencia. No habilites auto-merge automáticamente.

## Estado al entregar

Esta entrega contiene únicamente documentación y el CSV de correspondencia, en una rama separada. No incluye código de producto, respaldos internos ni credenciales; no altera requisitos académicos ni el checkout original.
