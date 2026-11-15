# VI-03 — checklist y decisión reviewer en Next (mocks)

- **Fecha:** 2026-11-12.
- **Frontend autorizado:** `/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes`.
- **Rama / HEAD base:** `frontend-next-hermes` / `cf6e28a365d2495e78bc64c01686ed55efb17c8a`.
- **Plan:** `planning/verificacion_identidad/plan_verificacion_identidad_espacigo.md`.
- **Estado:** subcorte local implementado y verificado; worktree sin commit. VI-03 continúa parcial.

## Implementación

- Se incorporó a `planning/openapi.yaml` el POST reviewer `/document-verification/reviewer/cases/{caseID}/review`, contrastado con la ruta, decoder y campos existentes del handler Go. Se regeneraron los tipos/allowlist del cliente; el inventario quedó en 189 operaciones.
- La fachada exige una sesión autenticada no restringida y un UUID canónico; construye un body con revisión esperada, decisión y `review_identity`, y llama a la operación POST usando `runAuthenticatedOperation`/Bearer. Ese literal es un campo del body del contrato, no una capability añadida a `AdminCapability` ni a la sesión. El servidor conserva la autoridad sobre grant, asignación, autorización y auto-revisión.
- El constructor fail-closed valida decisión y razón allowlisted, los siete criterios observados, formato, revisión esperada y fecha civil `YYYY-MM-DD` en `America/Santiago`. Para un criterio aún no revisado la UI conserva `null` y no permite enviar hasta elegir Sí/No; la aprobación requiere todos positivos y fecha/formato admitidos. No se inventa una fecha para `needs_evidence`.
- El parser de readback acepta solo el mismo ID canónico, propósito identity, estado terminal esperado por la decisión y revisión exactamente incrementada. La UI presenta checklist y motivos allowlisted únicamente en estados `en_revision`/`requiere_antecedentes`, muestra un error neutro sin reintento automático y vuelve a leer el expediente tras un POST exitoso.
- No se añadió capacidad administrativa, asignación en cliente, cancelación, acceso al storage, ni otro flujo de autorización.

## Verificación local ejecutada

En el frontend autorizado:

- TDD: la prueba de contrato falló antes de incorporar el POST; la prueba de fachada falló antes de su método; y el recorrido Playwright falló antes de existir el formulario. Tras implementar, los tests focales correspondientes pasaron.
- `pnpm typecheck` — PASS.
- `pnpm lint` — PASS.
- `pnpm api:check` — PASS; tipos, allowlist e inventario actuales (189 operaciones).
- `pnpm test` — PASS, 100/100, 0 fallos/omitidos.
- `pnpm format:check` y `git diff --check` — PASS.
- `pnpm build` — PASS; Next.js 16.4.3 generó 31/31 páginas, incluida `/mi-cuenta/revisiones/[caseId]`.
- `PLAYWRIGHT_PORT=4203 pnpm exec playwright test tests/reviewer-queue.spec.ts --reporter=list --workers=2 --output=/home/nandev/.hermes/cache/scratch/reviewer-identity-decision-verified-20261112` — PASS, Chromium local 7/7. Incluye envío sintético de aprobación, body exacto con checklist y revisión esperada, lectura mockeada del estado/revisión actualizados, cola/detalle allowlisted y expiración de sesión. Toda llamada de API, incluido el POST, fue interceptada; no hubo BFF→Go.
- Scan estático sobre cambios candidatos: cero asignaciones de secretos/tokens, SQL formateado o comandos/eval inseguros. Un match fue `RegExp.exec` en `return-target.ts`, falso positivo de la búsqueda de `exec` y revisado como expresión regular.

## Límites y gates pendientes

- El servidor de test devuelve una respuesta sintética al POST. Las pruebas no demuestran grant/asignación efectiva, prohibición real de auto-revisión, MFA, transacción/auditoría durable, CAS/locks PostgreSQL, integración Next BFF→Go, persistencia/reinicio, storage, GCS, IAM ni Cloud. La revisión de código del handler Go y su servicio no sustituye una prueba integrada.
- V95/V96 no se aplicaron. No se ejecutó SQL/DDL, no se conectó a PostgreSQL ni a Cloud, y no se usaron datos, cuentas, documentos o proveedores reales.
- La fase VI-03 sigue abierta y no habilita revisores reales. La UI mockeada no equivale a aceptación de producto ni cierra los gates de autorización, MFA, privacidad/operación y persistencia.
- No hubo commit, push, integración a `main` ni despliegue. Se preservó el estado preexistente de ambos worktrees compartidos.

## Revisión independiente read-only

- **Resultado:** PASS; no se reportaron hallazgos concretos de seguridad ni errores lógicos en el subcorte listado. La revisión contrastó la ruta/body con el handler Go y mantuvo explícito que asignación, auto-revisión y autorización son autoridad del servidor.
- **Límites confirmados:** la respuesta POST y el recorrido browser son mockeados; siguen sin probar BFF→Go, grant real, MFA, auditoría atómica ni persistencia.
- **Sugerencias no bloqueantes para pruebas futuras:** cubrir en UI los valores/constraints de aprobación y rechazo y límites civiles/Santiago; probar respuesta POST ambigua y cambio de sesión/selección durante una decisión pendiente; añadir prueba sintética del route handler BFF para allowlist, body exacto y Bearer sin cookies. Se registran como brechas de cobertura, no como defectos encontrados ni como evidencia de integración.
