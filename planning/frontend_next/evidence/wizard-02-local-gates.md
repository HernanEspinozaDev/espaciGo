# Evidencia local — Wizard-02 horario y galería

- Fecha: 2026-11-07.
- Rama: `codex/frontend/feature/wizard-02-horario-galeria`.
- HEAD base: `8f0e06e2d9e5f16b429c69f190aa9dcd44148fc6`.
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
- Sin operaciones online, sin login/cuenta real, sin cambios de backend/API/OpenAPI/SQL, sin migraciones y sin cambios Cloud/IAM.
- El port/servidor de Playwright fue gestionado por el runner local. El resultado E2E no constituye aceptación del producto ni prueba de upload real.
- Cambios aún no comprometidos ni publicados. No hubo integración a `main` ni despliegue. La validación funcional online queda para el propietario.
- La revisión independiente inicial no encontró riesgos de seguridad en zona horaria ni en el resto del delta; en calendario encontró el fallo de fecha final vacía. La corrección y su regresión fueron re-revisadas y aprobadas; el reviewer confirmó preservación de los límites de rango y protección frente a respuestas obsoletas. El valor por defecto corresponde a espacios nuevos; el editor preserva valores IANA históricos. La rehidratación se comprueba mediante `GET /weekly-hours` tras recarga y un mock de detalle que conserva Santiago. La prueba UI de IANA inválida se retiró porque el `<select>` limita opciones alcanzables; dominio continúa probando el rechazo. El cambio de galería solo modifica el aviso y la brecha de upload real/online sigue abierta.
