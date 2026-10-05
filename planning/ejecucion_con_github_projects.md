# Continuar EspaciGo desde un PC con GitHub Projects

## Estrategia vigente — prototipo local, 2026-10-05

PRs #124/#125 (M01), #126 (tramo M02) y #127 (M03 sintético) están fusionados; la aceptación de M03 es parcial. Las Issues M02 #37–43 siguen abiertas por cobro/KYC, foto, resolución de supresión y retención. #52 y #43 no se cierran ni se desbloquean artificialmente. Por autorización de producto del 2026-10-05, el corte local vigente es M04-DRAFT-01 (#128–#133), sobre `codex/m04-space-drafts`: crear, listar, consultar y editar borradores propios, sin publicación comercial. La rama de implementación crea la migración incremental, backend/API/OpenAPI, pruebas y mock en un PR integrado. El recorrido se prueba en [prototipo_local_m04.md](prototipo_local_m04.md); sus límites técnicos están en [decisiones_m04_borradores.md](decisiones_m04_borradores.md). #123 y DB02-09 permanecen separados. La base local conserva `espacigo_pgdata`, secretos y datos; PostgreSQL desechable exclusivamente para pruebas. No auto-merge ni marcar Issues como Hecho antes de aceptación.

La base de desarrollo mantiene `pgdata`, datos sintéticos y credenciales entre ejecuciones; no usar `clean`, `down --volumes` en ese proyecto ni reiniciar desde cero. Aplicar solo migraciones pendientes. Suite PostgreSQL exclusivamente en ambiente de pruebas separado. Reutilizar imágenes/cachés. DB02-09, historial de claves, avisos durables y limpieza de servidor siguen pendientes. GitHub Issues y Projects son el registro operativo; no reimportar tarjetas ni reactivar Hermes. Los snapshots históricos que siguen abajo no aplican cuando contradigan este estado vigente.

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
5. Para inspeccionar el estado vigente, consulta #128–#133, el PR del corte actual y el Project v2. Los PRs #124–#127 están fusionados; no uses snapshots históricos de este archivo para deducir estados.

## 2. Seleccionar trabajo

- Ejecuta únicamente Issues autorizadas, con dependencias vigentes `blocked_by` satisfechas y criterios/referencias comprendidos. Una Issue `Listo` no se autoriza por sí sola.
- Las entregas M01 #30–#35 tuvieron cortes parciales en PR #124/#125; #34/#35 y M02 #37–#43 siguen abiertas donde criterios permanecen pendientes. El trabajo vigente es M04-DRAFT-01 #128–#133. RQF-217/218 dependen de DB02-09; #123 continúa como limpieza de servidor independiente.
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
