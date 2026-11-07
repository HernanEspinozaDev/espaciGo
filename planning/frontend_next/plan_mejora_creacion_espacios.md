# Mejora del asistente de creación de espacios

## Encargo y base

El usuario autoriza el 2026-11-06 comenzar el plan adjunto de cinco fases mediante subagentes GPT-6 Luna con razonamiento medio; el padre coordina, revisa los cambios y verifica sus resultados. El plan no se declara implementado por existir este documento.

- Base comprobada: `origin/main` y checkout main-sync en `771134d165046b290b1f5ad3a1074a6357c2327f`.
- Rama de trabajo actual: `codex/frontend/fix/wizard-01-mejoras-creacion-espacios`.
- Worktree: `/home/nandev/Documentos/projects/espaciGo-wizard-improvements`.
- Se preservan los cambios sin commit de spaces-release y todos los demás worktrees. No se copian con rsync ni se descarta trabajo ajeno.
- Delegación verificada mediante `hermes config get delegation`: `gpt-6-luna`, `openai-codex`, `reasoning_effort: medium`. El padre no implementa código de producto; revisa, ejecuta gates y devuelve correcciones a las hojas.

## Revisión del diagnóstico propuesto

El serializer real `frontend/src/lib/api/exact-json.ts` acepta null y cadenas vacías: no son por sí solos la causa del error exact JSON. Sí rechaza undefined, números no finitos y objetos no JSON. La causa concreta y la corrección deben demostrarse con regresión ejecutable, no asumirse del mensaje.

- Limpiar ausencia en atributos opcionales antes de validación y envío, sin perder false/0, ocultar errores de tipo ni debilitar required, serialización, autorización o CAS/ETag.
- Etiquetas a partir del perfil de categoría vigente y versionado, no formularios hardcodeados. Los valores wire permanecen intactos.
- El catálogo geográfico debe conservar fuente/fecha comprobadas y códigos CUT con ceros iniciales. Un snapshot histórico no demuestra por sí solo vigencia administrativa en 2026.
- El calendario semanal describe horarios habituales. No acredita por sí mismo disponibilidad para una fecha ni ausencia de reservas/bloqueos. Reutilizar presentación pública solo con datos públicos autorizados; no exponer el endpoint privado ni inventar ocupaciones, cupos o contratación. La reserva/firma/cobro/garantía siguen fuera del corte.
- Conservar Mi Cuenta: lateral izquierdo y contenido a la derecha. No rediseñar el shell ni repetir cambios históricos de navegación por inferencia.

## Fases y gates

| Fase | Alcance | Criterio de salida | Estado inicial |
| --- | --- | --- | --- |
| 1 | JSON y resumen humano | Regresión RED/GREEN de atributos opcionales y serializer real; false/0 preservados; required y valores inválidos rechazados; resumen por esquema sin JSON, undefined ni claves técnicas; revisión del padre | Gate técnico local completado; nativo global en F5 |
| 2 | Región y comuna dependientes | Catálogo tipado con procedencia e integridad; región filtra comunas; cambio de región invalida comuna incompatible; nombres legibles y payload CUT correcto; pruebas de UI y payload | Gate técnico local completado; Antártica fuera de soporte backend |
| 3 | Navegación del wizard | Regreso directo a pasos visitados sin pérdida de datos; avance/confirmación revalida requisitos; cambios de categoría/esquema no permiten saltarse validación; teclado/foco/estados pendientes accesibles | Gate técnico local completado, recorrido padre1PASS |
| 4 | Slots/calendario | Componente compartido creación/editor y superficie pública soportada; edición sin perder horas/fracciones existentes; timezone, límites y no solapamiento según Go; modo público sin mutación ni falsa disponibilidad; escritorio/móvil/Axe | Gate técnico local completo: edición/recarga/CAS, HTTP público anónimo real, SQL/auditoría y exclusión del borrador día PASS; Axe y capturas escritorio/móvil revisados; reconciliación pública sin PUT PASS |
| 5 | Ensayo y entrega | Gates globales y build definitivo; Playwright contra artefacto nuevo; creación sintética con API/DB aisladas y readback independiente; revisión completa; publicación e integración normal solo tras cerrar criterios; despliegue trazable | Ensayo nativo, globales, build y browser definitivos PASS con skips explícitos; revisión independiente integral y cadena de publicación pendientes |

## Ejecución y propiedad

1. Hoja F1 posee wizard, helpers y pruebas exclusivos de F1.
2. Hoja catálogo F2 posee solo módulo geográfico y su prueba; no edita wizard.
3. El padre revisa diffs y resultados en este worktree antes de asignar la siguiente fase. Una hoja terminada no cierra la fase sin comprobar cada criterio.
4. Un solo responsable del build definitivo. No builds concurrentes ni Playwright sobre un standalone desactualizado.
5. Registrar por fase comandos realmente ejecutados, resultado, límites, archivos y revisión. Diferenciar pruebas unitarias, navegador con mocks, integración nativa y despliegue.

## Entorno y límites

Dependencias instaladas en este worktree con pnpm12.15.0 y `install --frozen-lockfile` (PASS). No se modifica el lockfile por inferencia. Binario disponible en `/home/nandev/.hermes/cache/scratch/espacigo-wizard-tools/node_modules/.bin/pnpm`.

Antes del ensayo nativo: consultar política privada local sin imprimir secretos, verificar target aislado, listeners y recursos propios. No reutilizar ni modificar bases persistentes, volúmenes retenidos, usuarios reales o Cloud SQL. No ejecutar migraciones como solución a una mejora UI.

La fase 5 solicitada incluye integración a main y rollout, pero no permite omitir gates ni publicar implementación parcial en main. Antes de integrar: revisar todo el diff, pruebas, hook e identidad HernanEspinozaDev y runbook actual. No force-push, PR ni reescritura de historia. Verificar cada ref remoto publicado. Para afirmar despliegue: SHA fuente → build SUCCESS → digest → revisión Ready → tráfico leído de vuelta. Las comprobaciones de interfaz online quedan reservadas al usuario; no navegador del usuario, CUA, credenciales reales ni navegación a la aplicación pública.

## Revisión previa F4 — contrato público

El padre contrastó el diagnóstico delegado con `frontend/src/app/(marketplace)/espacios/[id]/page.tsx:56–78`, `planning/openapi.yaml:4484–4514` y el validador/conversor de horarios. El DTO consumido por la ficha no aporta horario semanal. La operación existente `getOwnSpaceWeeklyHours` exige titular autenticado: no se utilizará desde el catálogo público. La parte pública de F4 queda pendiente de resolver el contrato público minimizado y su autorización, no de añadir datos de ejemplo ni quitar guards. Se puede continuar de forma independiente con F1–F3 y el calendario privado compartido. El calendario no afirmará disponibilidad por fechas a partir del mero horario semanal.

Contrato privado preservado: días ISO1–7, hasta dos tramos diarios, sin solapamientos/cruce de medianoche, cierre24:00 como límite final, días cerrados con arrays vacíos y If-Match fuerte. El componente visual conservará horarios fraccionarios existentes y dejará conversión/guardado en sus capas actuales. No se acredita todavía implementación ni prueba visual de F4.

## Gate técnico F1 — revisión del padre

Sobre las fuentes reunidas, el padre ejecutó `pnpm typecheck` PASS, `pnpm lint` con 0 errores/3 advertencias preexistentes y `pnpm build` PASS (Next16.4.3). Artefacto local construido; no desplegado. La regresión del adaptador actual pasó, pero su caso histórico falló por un path Git incorrecto; se devolvió como fallo de test, no como fallo resuelto del producto. El padre también rechazó la prueba navegador parcial: probaba solo required y usaba una descripción demasiado corta; se asignó completarla hasta confirmación y POST exacto. F1 continúa abierta hasta ejecutar ese recorrido sobre el artefacto nuevo y repetir la suite portable.

## Preflight de ensayo nativo F5 — solo lectura

La hoja de diagnóstico encontró el harness `playwright.space-gallery.config.ts` y `phase-space-gallery-native.spec.ts` acoplados al target F5 previo (`127.0.0.1:15434/espacigo_f5_gate_20261101`, API18082, Next HTTPS4203) y a inventario/política privados. No había listeners en15434/18082/4203/4204. Sí se observaron PostgreSQL5432 y API18081 de otro stack persistente: no se usaron como sustituto. No se arrancaron servicios, no se autenticó ni se leyeron filas.

La política privada existe en el checkout principal con permisos0700/0600, no en este worktree; su contenido no fue leído por esa hoja y su existencia no acredita autorización de reutilización. La ausencia de entorno activo bloquea **esa ejecución nativa actual**, no implementación frontend, mocks ni una preparación aislada futura debidamente autorizada. Antes de F5 debe resolverse el target y la política exacta sin copiar credenciales, tocar volúmenes retenidos ni ejecutar el harness mutante por inferencia. No se declara todavía aprobado el gate nativo.

## Autorizaciones posteriores explícitas

El usuario respondió afirmativamente en las dos preguntas de alcance: (1) implementar y probar el contrato público mínimo de horarios en Go/OpenAPI/adaptador, sin datos privados, reservas ni cambios SQL; (2) preparar un ensayo local nuevo y aislado, aplicar allí únicamente migraciones existentes sin alterar sus bytes y crear como máximo dos cuentas sintéticas por registro/verificación normales, con política y credenciales privadas, preservando bases y demos retenidas. Estas decisiones sustituyen solo los bloqueos de autorización correspondientes: no acreditan por sí mismas implementación ni integración.

Target propuesto para la nueva preparación: proyecto/DB `espacigo_wizard_20261106`, PostgreSQL loopback15436, API18086, NextHTTPS4206 y correo de ensayo en18087/11036. Deben verificarse disponibilidad, aislamiento, imagen y guards antes de arrancar. La primera hoja prepara perfil worker-free y procedimiento; el padre revisa antes de habilitar recursos o DDL. No se reutiliza el entorno F5 retenido.

## Autorización posterior del ensayo público local

Ante la pregunta concreta sobre habilitar el catálogo sólo para el perfil/destino `espacigo_wizard_20261106` y activar el borrador sintético horario por el flujo normal Go/UI con CAS, auditoría y readback, el usuario eligió **«Autorizar ese ensayo público local por el flujo normal»**. Esto permite preparar el opt-in con guard exacto, clave de cursor propia del ensayo y una publicación local normal del recurso existente; no autoriza SQL para forzar estados, nuevos usuarios, otro destino, GCP ni pruebas online. La operación aún debe ejecutarse y verificarse antes de cerrar su gate. No se cambiarán las condiciones de elegibilidad de publicación ni se confundirá identidad contractual con publicación.

## Estado verificable

F1–F3 tienen sus gates técnicos locales revisados en las evidencias respectivas. El catálogo incorpora el suplemento documentado; Antártica permanece deshabilitada por falta de soporte backend, sin ampliar migraciones.

F4 comparte calendario entre wizard, editor y ficha pública. El ensayo aislado tiene V1–V83 y ACL verificadas, una cuenta sintética y tres espacios distintos creados como borradores con recibos verificados. Hora fue creada y reconciliada tras un fallo posterior al alta; día y mes pasaron como dos casos combinados. No se repitieron altas. La edición del lunes a 09:00–17:30 quedó persistida; la reconciliación separada pasó con recarga, rechazo CAS 412 del ETag anterior y readback intacto. Después de la autorización pública posterior, la hora se publicó mediante un único PUT normal de UI con CAS: caso nativo PASS, lectura anónima del BFF/SSR real PASS y recibo independiente verificado. La prueba PostgreSQL read-only confirmó hora activa, exactamente una auditoría de borrador a activa atribuida al propietario y DTO/proyección coincidentes con los tramos. El borrador día conserva `ErrNotFound`. No se habilitó elegibilidad mediante SQL ni se falsificó KYC. Evidencias: `evidence/wizard-native-editor-cas.md`, `evidence/wizard-native-public-sql.md` y `evidence/wizard-native-public-positive.md`.

Sobre las fuentes finales, el padre ejecutó `go test -count=1 ./...` y `go vet ./...` PASS; frontend 477 unitarias PASS/1 SKIP/0 FAIL, API check actualizado, TypeScript, lint sin errores y formato global PASS. Build de producción `3vVei_2oiWumIiG9a51RW` PASS; browser conjunto 145 PASS/61 SKIP y calendario público SSR controlado 2 PASS. Los skips no acreditan ejecución nativa de esos casos.

La revisión visual detectó la ambigüedad de una celda 17:00–18:00 cubierta sólo hasta 17:30. Se preservaron capturas BEFORE; el calendario ahora distingue cobertura parcial con texto visible y estado accesible `mixed`, sin cambiar los minutos. La corrección incluye los límites 09:15–10:00 y 23:30–24:00, que permanecen parciales. Después de reconstruir, el padre repitió sobre el artefacto nuevo el caso visual nativo (Axe completo y contención en escritorio/móvil) y la reconciliación CAS: ambos PASS. Inspeccionó las imágenes AFTER y confirmó “Parcial” y cierre exacto 17:30 legibles, sin recortes en la región del calendario. Esto no es aceptación humana.

La revisión independiente de las correcciones operativas y del opt-in público concluyó sin bloqueadores concretos. La reconciliación pública sin PUT pasó y la captura móvil reproducida se inspeccionó completa. Los checks frontend de cierre pasaron, y el browser conjunto actualizado reportó 145 PASS/63 SKIP. La suite Python de cuentas pasó 14 casos; scripts combinados, 47; legacy CLI/import, dos ejecutados directamente. La prueba de Mailpit quedó aislada con mocks sin detener ni ocupar su puerto real. Las capturas finales se retienen privadamente fuera de la caché expirable, con hashes verificados.

El gate local previo a publicación de código queda cerrado con estos resultados y límites; F5 de entrega sigue abierto. Pendientes: reconciliar `origin/main` avanzado antes de integrar; repetir gates afectados y ejecutar la entrega según el flujo vigente sin bastión. El fetch verificó `origin/main=214c75dc612374c868cc4209ecef521b497d0c19`, 18 commits posteriores a la base y sin cambios a migraciones. Sólo hay tres paths superpuestos: dos de formato frontend y el CLI `cmd/dbmigrate/main.go`, cuyos guards deben conservar tanto los modos nuevos de release como el modo administrativo local limitado. No sobrescribir esos avances ni reconstruir el procedimiento cloud histórico. Se leyó la skill mantenida en main `.agents/skills/espacigo-release-flow/SKILL.md` y `planning/flujo_entornos/operacion_main_sin_bastion.md`.

La API del ensayo conserva su identidad limitada, destino y opt-in público exactos; DB, cuentas, volumen y credenciales se preservan. La hora se retiene activa sólo en el ensayo. El código probado quedó en `0659dda11e9bc60576e33a6bd60c79d5b7088a06` y se publicó en la rama de tarea, con lectura remota y autor/committer verificados. El primer push fue rechazado por el nombre sin identificador numérico; el usuario autorizó renombrar la rama al destino actual, conservando el mismo commit e historia, sin modificar ni desactivar el hook. `main` permanece sin integrar y no hay despliegue de este conjunto. Evidencias: `evidence/wizard-final-local-gates.md` y `evidence/wizard-branch-publication.md`.

## Conciliación con main avanzado — 2026-11-07

Se concilia main `214c75dc612374c868cc4209ecef521b497d0c19` mediante merge normal dentro de la rama de tarea. El frontend completo coincide con la tarea publicada y probada; no cambian migraciones ni se sobrescribe el nuevo pipeline. Se adaptaron los guards de modos del CLI y se corrigió un bypass por argumentos posicionales detectado en revisión independiente y reproducido sin credenciales. Go global/vet posteriores PASS; API combinada recompilada con perfil limitado; dos pruebas SQL reales y dos reconciliaciones browser PASS, sin repetir mutaciones. La segunda revisión independiente aprobó el CLI sin bloqueadores; el gate local de integración queda cerrado. Detalles, límites y preflight del trigger en `evidence/wizard-integration-main.md`. Pendientes publicación del merge, integración normal a main y entrega verificable; este gate no acredita por sí solo despliegue ni aceptación online.
