# Evidencia local — Wizard-02 horario y galería

- Fecha: 2026-11-07.
- Rama: `codex/frontend/feature/wizard-02-horario-galeria`.
- HEAD base: `8f0e06e2d9e5f16b429c69f190aa9dcd44148fc6`.
- Commit: `dafa890cb2f571e707d206dc42e81391fa86978a`, verificado en local y en `origin/codex/frontend/feature/wizard-02-horario-galeria`.
- Artefacto validado: build Next.js local de esta rama, servido por Playwright en loopback `127.0.0.1:4205`.
- Datos/servicios: solo E2E controlados y mocks de frontend; sin API/DB de usuario, sin cuentas nuevas y sin servicio externo.

## Cambios observados

- Nuevo dominio `frontend/src/features/host-listings/domain/chile-time-zones.ts`: cuatro valores IANA de Chile con etiquetas humanas, y conserva un valor histórico no listado al editar.
- `listing-wizard.tsx` y `listing-editor.tsx`: selector en lugar de campo de texto; copy distingue horario semanal habitual de disponibilidad por fecha y mantiene la regla de día/mes.
- `(workspace)/layout.tsx`: enlace del logo a `/` y nombre accesible actualizado.
- `listing-gallery.tsx`: elimina el aviso de ensayo permanente y presenta tipos aceptados/validación sin quitar las barreras de incertidumbre/recarga.
- Pruebas controladas adaptadas a `selectOption`, las cuatro opciones, paso 4, editor, logo, galería y tarifas día/mes.

## TDD y ejecución final

- RED reproducido en Playwright local al exigir etiqueta humana «Región de Magallanes»: el artefacto previo todavía mostraba el identificador IANA en el texto. No hubo mutaciones; la falla ocurrió en la aserción del selector.
- Tras cambiar la etiqueta visible y reconstruir, las corridas completas previas dieron **145 passed, 63 skipped, 0 failed**. Se reforzó el E2E controlado para elegir `America/Coyhaique`, comprobar el cuerpo enviado a `PUT /availability` y verificar que, después de recargar, se consulta `GET /weekly-hours` y se rehidrata esa zona desde su respuesta. El mock de detalle se mantiene intencionalmente en `America/Santiago` para que no pueda satisfacer por sí solo la aserción. El mismo recorrido ahora hace clic en el logo, comprueba navegación real a `/` y vuelve al editor; el test enfocado pasó (**1/1**).
- El calendario manual por fecha pasó su E2E focal (**1/1**) en el build local: conflicto `409`, creación, listado y lectura tras recarga, eliminación, zona horaria ausente sin consulta y enlace a configuración. Axe y overflow horizontal pasaron en `1280×800` y `390×844`. No usa API/DB ni identidad real.
- Revisión independiente encontró que borrar «Hasta» lanzaba antes de la validación. Se agregó la regresión: RED comprobó que no aparecía el error de rango y el botón quedaba en estado de carga; se cambió el cálculo para validar el valor crudo y tratar el extremo inclusivo sin conversión que arroje. La prueba focal volvió a pasar (**1/1**) y verificó mensaje de rango inválido, botón habilitado y ningún GET adicional.
- Corrida Playwright completa final tras corregir ese defecto: **146 passed, 63 skipped, 0 failed**. Los casos opt-in permanecieron omitidos intencionalmente; no habilité variables de ensayo/API/galería nativa.
- Gates finales observados: `node tooling/api-check.mjs` PASS (197 operaciones); `tsc --noEmit` PASS; `node --test tooling/*.test.mjs` PASS (480 passed, 1 skipped); ESLint completo PASS (0 errores, 4 advertencias; ESLint dirigido sobre los E2E modificados PASS); `prettier --check .` PASS; `pnpm build` PASS (Next.js 16.4.3); `git diff --check` PASS.
- No se prueba upload a almacenamiento real, persistencia GCS/Postgres, permisos de Cloud Run, catálogo anónimo, integración API/DB del calendario manual ni disponibilidad derivada de reservas. La aserción de carga habilitada confirma solamente la UI controlada en estado de galería confirmado.

## Estado y límites

- No se leyó ni modificó el espacio existente informado por el propietario; no se creó otro espacio.
- Sin interacción con la aplicación online ni login/cuenta real; sí hubo push solo a la rama de tarea y consulta de solo lectura del trigger Cloud Build. El delta de tarea no cambia backend/SQL/migraciones/Cloud/IAM; en el candidato main se regeneraron artefactos frontend para sincronizarlos con el OpenAPI que ya estaba en main.
- El port/servidor de Playwright fue gestionado por el runner local. El resultado E2E no constituye aceptación del producto ni prueba de upload real.
- Código publicado en la rama de tarea e integrado localmente en un candidato `main`. `origin/main` no se actualizó y no hubo despliegue. La validación funcional online queda para el propietario.
- La revisión independiente inicial no encontró riesgos de seguridad en zona horaria ni en el resto del delta; en calendario encontró el fallo de fecha final vacía. La corrección y su regresión fueron re-revisadas y aprobadas; el reviewer confirmó preservación de los límites de rango y protección frente a respuestas obsoletas. El valor por defecto corresponde a espacios nuevos; el editor preserva valores IANA históricos. La rehidratación se comprueba mediante `GET /weekly-hours` tras recarga y un mock de detalle que conserva Santiago. La prueba UI de IANA inválida se retiró porque el `<select>` limita opciones alcanzables; dominio continúa probando el rechazo. El cambio de galería solo modifica el aviso y la brecha de upload real/online sigue abierta.

## Revisión e integración contra main

- `origin/main` verificado en `274f37e483a078890e92382a159fa49fd23c2fe8`; rama de tarea en `1c49c042814695f5257feb0262ebac9291622b5f`. La historia tenía 10 commits exclusivos de main y 2 de tarea desde `8f0e06e2d9e5f16b429c69f190aa9dcd44148fc6`. Se revisaron ambos lados; las diferencias de main incluyen Didit/KYC y otras áreas, sin cambios concurrentes en los archivos de calendario/zona horaria. `git merge-tree` y el merge normal no encontraron conflictos.
- Integración local normal: merge `141d348aa6e4bc7002020741830da899715c277f` (padres `274f37e...` y `1c49c04...`). El primer `api:check` reveló que OpenAPI de main ya incluía tres operaciones Didit ausentes del cliente generado; se regeneraron tipos, allowlist e inventario (200 operaciones) en commit local `b89fe0f196eb5ff2c3b155bafd4b60e3ea074618`. El API check posterior pasó.
- Gates del candidato: typecheck, lint (0 errores/4 advertencias), unitarias (480 aprobadas/1 omitida), Prettier, build y diff check pasaron. Playwright completo: 144 aprobadas, 63 omitidas, 2 fallidas. Ambas se reprodujeron sobre `origin/main` solo: test KYC espera etiquetas previas a Didit; test de accesibilidad detecta dos `h1` en Verificaciones. Los archivos afectados no forman parte del diff de tarea; se dejan pendientes fuera del corte.
- El trigger `espacigo-main-rollout` está activo para push en `^main$`, configurado por `cloudbuild.yaml`. El delta fuente se clasifica como `web`; `rollout_web.py` construye y promueve los servicios `espacigo-test-public` y `espacigo-test-private`. No se inició el rollout. El candidato local `main` no está publicado; el push queda pendiente de autorización expresa para el despliegue automático.
