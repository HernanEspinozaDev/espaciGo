# Continuar EspaciGo desde un PC con GitHub Projects

## Estrategia vigente — prototipo local, 2026-10-06

Aceptación: #148 y #151 fueron aceptadas tras PRs #149–#152; #153 se aceptó tras PR #154; #156 se aceptó y cerró tras PR #157 y prueba con ambas cuentas; #158 se aceptó y cerró tras PR #159 y comprobación de contadores/cursor con ambas cuentas. Entrega autorizada en curso: #160, M05-LOCAL-02, filtros consultivos de catálogo local por atributos tipados del perfil/version guardado y rango inclusivo del total estimado para el intervalo. Se combinan por AND con categoría y disponibilidad; ordenar por total ascendente y UUID. Búsqueda no persiste cotizaciones ni crea ocupaciones; cotizar y reservar revalidan tarifa y disponibilidad. No completa #62–#68 ni los criterios generales M05/M06. Véanse [decisión de filtros M05](decisiones_m05_busqueda_filtros_locales.md) y [lectura M09](decisiones_m09_lectura_local.md). Las Issues generales #95–#98, DB02-09 y #123 conservan sus criterios y pendientes. Mantener `espacigo_pgdata` y secretos locales.

PRs #124/#125 (M01), #126 (tramo M02), #127 (M03 sintético), #143 (evidencia sintética local), #134 (borradores M04) y #141 (catálogo extensible M04) están fusionados. M03 y M04 siguen parciales; #142 conserva requisitos KYC productivos. El nuevo corte #144 separa explícitamente disponibilidad y bloqueos manuales privados de borradores como subentrega de LIST-BE-02 (#56). No espera #43 (perfil M02) ni #142 (documentos KYC productivos): la sesión M01 identifica al titular y el recorrido no publica ni transa. #43/#142 y los criterios completos de #56 permanecen abiertos.

El calendario persiste en `ocupacion`, propiedad única de M06; no se crea una tabla paralela en M04. El corte #144 usa solamente bloqueos manuales, zona IANA explícita e intervalos finitos `[inicio, fin)`. #70/#71/#72 siguen abiertos para diseño, migración y flujo completo de reserva; este slice no los cierra. M04/DRAFT-01 #128 y M04/ATTR-01 #135–#141 ya se fusionaron; no repetir esas implementaciones. DB02-09 y limpieza remota #123 siguen separados y abiertos. La base local conserva `espacigo_pgdata`, secretos y datos; PostgreSQL desechable exclusivamente para pruebas. No auto-merge ni marcar Issues como Hecho antes de aceptación.

La base de desarrollo mantiene `pgdata`, datos sintéticos y credenciales entre ejecuciones; no usar `clean`, `down --volumes` en ese proyecto ni reiniciar desde cero. Suite PostgreSQL exclusivamente en ambiente de pruebas separado. Reutilizar imágenes/cachés. DB02-09, historial de claves, avisos durables y limpieza de servidor siguen pendientes. GitHub Issues y Projects son el registro operativo; no reimportar tarjetas ni reactivar Hermes. Los snapshots históricos que siguen abajo no aplican cuando contradigan este estado vigente.

La autorización temporal #47 permite únicamente el fixture `synthetic-png-v1` generado por la API en almacenamiento privado local fuera del repo/web pública. Documentos reales, proveedor productivo, consentimiento y retención siguen abiertos en #142. Sigue [estas instrucciones de M03](evidence/m03-evidencia-sintetica-local.md); no borres el directorio de fixtures ni el volumen de desarrollo como limpieza rutinaria.

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
5. Para inspeccionar el estado vigente, consulta #135–#140, el PR del corte actual y el Project v2. Los PRs #124–#127 están fusionados; no uses snapshots históricos de este archivo para deducir estados.

## 2. Seleccionar trabajo

- Ejecuta únicamente Issues autorizadas, con dependencias vigentes `blocked_by` satisfechas y criterios/referencias comprendidos. Una Issue `Listo` no se autoriza por sí sola.
- Las entregas M01 #30–#35 tuvieron cortes parciales en PR #124/#125; #34/#35 y M02 #37–#43 siguen abiertas donde criterios permanecen pendientes. #153 se aceptó como bandeja local; #156 agrega conversación persistente y #158 agrega lectura/contador personal como subentregas. Ninguna cierra #95–#98 ni otras partes generales de M09. RQF-217/218 dependen de DB02-09; #123 continúa como limpieza de servidor independiente.
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
