# Continuar EspaciGo desde un PC con GitHub Projects

Corte de referencia: 2026-10-05. Usa esta guía junto con [contexto_para_agentes.md](contexto_para_agentes.md), [README de planificación](README.md), [backlog histórico](backlog.md), [decisiones M01](decisiones_m01_identidad_sesion.md) y [la correspondencia Hermes–GitHub](github_issue_map.csv). Los requisitos y límites del producto siguen siendo los de ES1/ES2 versionados en `planning/referencias/`; esta migración no los modifica.

## 1. Preparar el PC sin perder trabajo

1. Desde el checkout del repositorio, inspecciona primero `git status --short --branch`. Si hay cambios locales, consérvalos: no uses `reset --hard`, `git clean`, sobrescrituras ni `push --force`. Si hace falta, trabaja en otro worktree o checkout.
2. Verifica la autenticación existente sin pegar tokens en el chat, archivos ni comandos: `gh auth status --hostname github.com`. La cuenta debe ser `HernanEspinozaDev`, con acceso al repositorio y scope `project`.
3. Verifica lectura del proyecto: `gh project view 1 --owner HernanEspinozaDev`. Abre el Project `EspaciGo — Desarrollo` en GitHub para filtrar/ordenar Issues y consultar sus campos.
4. Refresca la rama base de forma segura: `git fetch origin`; cambia a `main` solo si el checkout está limpio; luego `git pull --ff-only`.
5. Para inspeccionar la puerta actual: `gh pr view 16 --repo HernanEspinozaDev/espaciGo` y `gh issue view 29 --repo HernanEspinozaDev/espaciGo`. Verifica de nuevo el estado remoto; el snapshot de esta guía dice PR #16 `OPEN`, no fusionado.

## 2. Seleccionar trabajo

- Selecciona solo una Issue `Listo`, sin dependencias `blocked_by` abiertas y con criterios/referencias comprendidos. En el corte de referencia no había tarjetas `Listo`.
- AUTH-BE-01 (#29) y su recuperación (#120) siguen `En revisión` hasta que PR #16 cumpla revisión y aceptación. AUTH-BE-02 (#30) y AUTH-BE-03 (#31) siguen `Bloqueado` por ambas Issues; no las inicies ni despaches workers mientras esas dependencias no estén satisfechas.
- AUTH-BE-03 conserva además los límites de RQF-217 y RQF-218; no agregues DDL ni decisiones implícitas. DB02-09 sigue abierto.
- Antes de empezar cualquier Issue, lee el cuerpo completo, los enlaces a requisitos, las pruebas/evidencias y el estado de sus PRs. La columna `Estado Hermes (histórico)` es solo evidencia del estado anterior, no aprobación actual.
- Mantén la secuencia del proyecto: Base de Datos → Backend → API → pruebas → mock visual temporal. No comiences frontend definitivo ni agregues funcionalidades fuera de la Issue autorizada.

## 3. Rama, cambios y PR

Con el checkout limpio y actualizado, crea una rama específica para la Issue (por ejemplo, `feat/auth-be-02-registration-session`). Trabaja únicamente en su alcance; conserva las referencias RQF/CU/HU y agrega pruebas/evidencia sintética. Antes de publicar revisa `git diff --check`, pruebas aplicables, secretos/PII y el diff completo.

Abre un PR hacia `main`, enlaza la Issue y describe los criterios verificados y los que siguen pendientes. No habilites auto-merge ni fusiones automáticamente. Tras un merge real, vuelve a consultar el PR, los checks, la Issue y el Project antes de actualizar el estado; una automatización de Project no sustituye la aceptación.

Comandos de lectura útiles:

```sh
gh auth status --hostname github.com
gh project view 1 --owner HernanEspinozaDev
gh issue view 29 --repo HernanEspinozaDev/espaciGo
gh issue view 120 --repo HernanEspinozaDev/espaciGo
gh pr view 16 --repo HernanEspinozaDev/espaciGo
gh pr checks 16 --repo HernanEspinozaDev/espaciGo
```

## 4. Estados y trazabilidad

El Project ofrece `Triage`, `Por hacer`, `Listo`, `En curso`, `Bloqueado`, `En revisión` y `Hecho`. Usa `En revisión` con PR/evidencia pendiente; `Bloqueado` si queda un gate real; `Hecho` solo tras aceptación y evidencia verificable. No cambies a `Hecho` solo porque Hermes tenía `done`, se publicó una rama o existe un PR abierto. La matriz [github_issue_map.csv](github_issue_map.csv) vincula código/ID Hermes, Issue, estado histórico, estado actual, prioridades y relaciones.

El cuerpo de cada Issue contiene snapshot de criterios, referencias y contexto histórico. Los comentarios/runs antiguos se identifican como históricos con autor/fecha de origen cuando estaban disponibles; no los vuelvas a publicar atribuyéndolos a la cuenta ejecutora. Conserva además los resultados y evidencias relevantes al actualizar una Issue.

## 5. Hermes Kanban y respaldo

El board Hermes `espacigo` se archivó de forma recuperable para sacarlo del dispatcher, no se borró. No ejecutes `hermes kanban boards import` para continuar el trabajo habitual: crearía otro board y duplicaría el flujo. El archivo portable `espacigo-kanban-portable-sanitized.tar.gz` es solo respaldo histórico; no es necesario importarlo en el PC para trabajar con GitHub.

No inicies funcionalidades nuevas ni workers automáticos. Si alguna vez se requiere restaurar Hermes, primero evalúa explícitamente el alcance y evita duplicar tarjetas que ya están en GitHub.
