# Continuar EspaciGo desde un PC con GitHub Projects

## Estrategia vigente autorizada — prototipo local M01, 2026-10-05

PR [#124](https://github.com/HernanEspinozaDev/espaciGo/pull/124) ya fue aceptado y fusionado. #30/#32 se cerraron por ese tramo. El corte #31/#33 ahora continúa en una sola rama/PR sobre main; API/mock se integran sin gates de aprobación por capa. #34/#35 siguen abiertas para aceptación integral, y RQF-217/218 continúan parcialmente pendientes por DB02-09. Mantener #123 como seguimiento separado. Esta estrategia y el flujo ejecutable están en [prototipo_local_m01.md](prototipo_local_m01.md); las instrucciones/snapshots históricos de las secciones posteriores no aplican cuando contradigan esta actualización.

GitHub Issues y Projects siguen siendo el registro operativo; este ajuste documenta el motivo y no replica estados del tablero. En la secuencia nativa se quitó #32 de los bloqueadores de #34 porque su API ya está integrada en esta rama, y se pasó #33 a seguimiento del próximo corte porque la recuperación queda fuera; #24 permanece como base de pruebas satisfecha. Se quitó #34 del bloqueo de #35 porque el flujo de pruebas está integrado y el mock se ejecutó en este mismo corte; #25, entorno local ya entregado, permanece como base satisfecha. Los textos originales de aceptación/dependencias y referencias permanecen en las Issues; los comentarios registran el cambio de etapa sin cerrar tarjetas. No reimportar tarjetas ni reactivar Hermes. Controles productivos por IP/correo pueden usar adaptadores locales explícitos para este prototipo, con evolución pendiente documentada. DB02-09 y la limpieza del servidor #123 quedan fuera. Comando, pasos y límites: [prototipo_local_m01.md](prototipo_local_m01.md).

Corte de referencia: 2026-10-05. Usa esta guía junto con [contexto_para_agentes.md](contexto_para_agentes.md), [README de planificación](README.md), [backlog histórico](backlog.md), [decisiones M01](decisiones_m01_identidad_sesion.md) y [la correspondencia Hermes–GitHub](github_issue_map.csv). Los requisitos y límites del producto siguen siendo los de ES1/ES2 versionados en `planning/referencias/`; esta migración no los modifica.

## 1. Preparar el PC sin perder trabajo

1. Desde el checkout del repositorio, inspecciona primero `git status --short --branch`. Si hay cambios locales, consérvalos: no uses `reset --hard`, `git clean`, sobrescrituras ni `push --force`. Si hace falta, trabaja en otro worktree o checkout.
2. Roles de cuenta: `HernanMEC` es la cuenta de desarrollo para commits, pushes y creación/actualización de PRs. `HernanEspinozaDev` queda exclusivamente para tu revisión, aprobación y merge. No publiques cambios de desarrollo con esta última cuenta. Activa la cuenta de desarrollo y configura la identidad local del repositorio:
   ```sh
   gh auth switch --hostname github.com --user HernanMEC
   gh auth setup-git
   gh api user --jq .login   # debe imprimir HernanMEC
   git config user.name "HernanMEC"
   NOREPLY=$(gh api user --jq '"\(.id)+\(.login)@users.noreply.github.com"')
   git config user.email "$NOREPLY"
   ```
   Comprueba `gh auth status --hostname github.com` sin copiar tokens. Para ver/escribir el Project desde CLI, `HernanMEC` también necesita acceso al Project y scope `project`; si falta, solicita el scope con `gh auth refresh --hostname github.com --scopes project` y verifica `gh project view 1 --owner HernanEspinozaDev` antes de depender de esa función. El acceso `repo` por sí solo no acredita acceso al Project.
3. Abre el Project `EspaciGo — Desarrollo` para filtrar Issues y comprobar campos. La lectura/escritura del Project debe verificarse con la cuenta que efectivamente tenga acceso; no supongas que el scope de una cuenta aplica a la otra.
4. Refresca la rama base de forma segura: `git fetch origin`; cambia a `main` solo si el checkout está limpio; luego `git pull --ff-only`.
5. Para inspeccionar el estado vigente, consulta las Issues #31/#33, el PR del corte actual y el Project v2. PR #124 ya está `MERGED`; no uses snapshots históricos de este archivo para deducir estados.

## 2. Seleccionar trabajo

- Ejecuta únicamente Issues autorizadas, con dependencias vigentes `blocked_by` satisfechas y criterios/referencias comprendidos. Una Issue `Listo` no se autoriza por sí sola.
- #30/#32 están aceptadas por PR #124 fusionado. #31/#33 continúan en el PR del segundo corte; #34/#35 siguen abiertas hasta aceptación integral. RQF-217/218 dependen de DB02-09; #123 continúa como limpieza de servidor independiente.
- AUTH-BE-03 conserva además los límites de RQF-217 y RQF-218; no agregues DDL ni decisiones implícitas. DB02-09 sigue abierto.
- Antes de empezar cualquier Issue, lee el cuerpo completo, los enlaces a requisitos, las pruebas/evidencias y el estado de sus PRs. La columna `Estado Hermes (histórico)` es solo evidencia del estado anterior, no aprobación actual.
- Mantén la secuencia del proyecto: Base de Datos → Backend → API → pruebas → mock visual temporal. Puedes completar una Issue autorizada cuando sus dependencias estén satisfechas, pero limita cada cambio a su alcance y criterios; no agregues funcionalidades fuera de esa autorización ni comiences frontend definitivo antes de su gate.

## 3. Retomar una Issue en revisión y su PR existente

Para AUTH-BE-01 (#29), su recuperación (#120) y PR #16, continúa el trabajo pendiente sobre la misma entrega; no crees otra Issue, rama ni PR. Primero revisa criterios, revisiones, comentarios, diff y checks actuales:

```sh
gh issue view 29 --repo HernanEspinozaDev/espaciGo
gh issue view 120 --repo HernanEspinozaDev/espaciGo
gh pr view 16 --repo HernanEspinozaDev/espaciGo --comments
gh pr diff 16 --repo HernanEspinozaDev/espaciGo
gh pr checks 16 --repo HernanEspinozaDev/espaciGo
```

Esta sección preserva el procedimiento histórico seguido para PR #16 y no es una solicitud vigente. Continúa la rama/PR actuales según la instrucción de usuario y la estrategia M01 resumida al inicio; nunca reconstruyas progreso a partir de ese snapshot antiguo.

## 4. Rama, cambios y PR

Para una Issue autorizada nueva, con dependencias satisfechas, checkout limpio y actualizado, crea una rama específica (por ejemplo, `feat/auth-be-02-registration-session`). Trabaja únicamente en su alcance; conserva las referencias RQF/CU/HU y agrega pruebas/evidencia sintética. Antes de publicar revisa `git diff --check`, pruebas aplicables, secretos/PII y el diff completo.

Con `HernanMEC`, abre el PR hacia `main`, enlaza la Issue y describe criterios verificados y pendientes; usa esa misma cuenta para actualizar PRs existentes. No habilites auto-merge ni fusiones automáticamente. Tras un merge real, vuelve a consultar el PR, los checks, la Issue y el Project antes de actualizar el estado; una automatización de Project no sustituye la aceptación. La revisión, aprobación y merge corresponden a `HernanEspinozaDev`.

Comandos de lectura útiles:

```sh
gh auth status --hostname github.com
gh project view 1 --owner HernanEspinozaDev
gh issue view 29 --repo HernanEspinozaDev/espaciGo
gh issue view 120 --repo HernanEspinozaDev/espaciGo
gh pr view 16 --repo HernanEspinozaDev/espaciGo
gh pr checks 16 --repo HernanEspinozaDev/espaciGo
```

## 5. Estados y trazabilidad

El Project ofrece `Triage`, `Por hacer`, `Listo`, `En curso`, `Bloqueado`, `En revisión` y `Hecho`. Usa `En revisión` con PR/evidencia pendiente; `Bloqueado` si queda un gate real; `Hecho` solo tras aceptación y evidencia verificable. No cambies a `Hecho` solo porque Hermes tenía `done`, se publicó una rama o existe un PR abierto. [github_issue_map.csv](github_issue_map.csv) es un snapshot de la migración: vincula código/ID Hermes, Issue, estados histórico y operativo, PR relacionado y racional. No contiene columnas de prioridad ni aristas de dependencia; consulta el Project y las dependencias nativas `blocked_by` en GitHub para esos datos vigentes.

El cuerpo de cada Issue contiene snapshot de criterios, referencias y contexto histórico. Los comentarios/runs antiguos se identifican como históricos con autor/fecha de origen cuando estaban disponibles; no los vuelvas a publicar atribuyéndolos a la cuenta ejecutora. Conserva además los resultados y evidencias relevantes al actualizar una Issue.

## 6. Hermes Kanban y respaldo

El board Hermes `espacigo` se archivó de forma recuperable para sacarlo del dispatcher, no se borró. No ejecutes `hermes kanban boards import` para continuar el trabajo habitual: crearía otro board y duplicaría el flujo. El archivo portable `espacigo-kanban-portable-sanitized.tar.gz` es solo respaldo histórico; no es necesario importarlo en el PC para trabajar con GitHub.

El archivado de Hermes no prohíbe continuar trabajo autorizado en GitHub: ejecuta Issues autorizadas cuando sus dependencias estén satisfechas, respetando alcance, criterios y gates. En este snapshot #29/#120 siguen en revisión y #30/#31 bloqueadas; retoma #29/#120 por el PR #16 y su rama existentes. No despaches trabajo en Hermes ni desarchives el board para operar: debe permanecer archivado y fuera del dispatcher. Para consultar historial, usa el respaldo o la copia archivada en modo de solo lectura; no importes ni dupliques tarjetas que ya están en GitHub.
