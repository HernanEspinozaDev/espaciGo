# VI-03 — cobertura local BFF y decisiones reviewer (mocks)

- **Fecha:** 2026-11-12.
- **Frontend autorizado:** `/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes`.
- **Rama / HEAD base:** `frontend-next-hermes` / `cf6e28a365d2495e78bc64c01686ed55efb17c8a`.
- **Estado:** subcorte de pruebas locales terminado; no se modificó código productivo ni se creó commit. VI-03 continúa parcial.

## Alcance de este subcorte

Se completaron brechas de cobertura no bloqueantes identificadas en la revisión del formulario reviewer:

- `frontend/tooling/backend-gateway-credentials.test.mjs`: caracteriza el POST real del route handler Next `/api/backend/document-verification/reviewer/cases/{caseID}/review` con un body de aprobación completo conforme al contrato. Sustituye únicamente `fetch` upstream; afirma path/método, bytes JSON exactos, Bearer, ausencia de Cookie hacia upstream, `credentials: omit`, `cache: no-store` y cabeceras de privacidad. Un caso negativo entrega una Cookie sintética y verifica 400 antes de cualquier `fetch` upstream. No valida Go ni el contenido semántico de una decisión persistida.
- `frontend/tooling/reviewer-identity-decision.test.mjs`: comprueba que cada uno de los siete criterios checklist en falso bloquea la aprobación. Una prueba fija dos instantes alrededor del cambio de fecha civil en el retroceso DST de `America/Santiago` (2026-04-05T03:59:59Z y 2026-04-05T04:00:00Z): el 2026-04-04 observado deja de ser aprobable al comenzar el día civil siguiente.
- `frontend/tests/reviewer-queue.spec.ts`: el caso de rechazo `documento_vencido` prueba en Chromium que la fecha civil de hoy produce error neutral y cero POST; al cambiarla por el día civil anterior, envía una sola decisión con body exacto y observa el readback sintético de `rechazada`.
- La misma suite mantiene un POST en espera hasta que expira la sesión mockeada; comprueba que declaración/formulario se purgan, la respuesta retenida no se publica y no se crea un segundo POST automático. La ruta retenida no se completa en el navegador. Esto caracteriza la UI/fencing local bajo este mock, no el resultado remoto de una mutación real.

No se alteraron contratos OpenAPI ni producción en este subcorte. Las pruebas anteriores de aprobación y checklist completa permanecen activas.

## Verificación ejecutada

Desde `frontend/` en el worktree autorizado:

- `pnpm typecheck` — PASS.
- `pnpm lint` — PASS.
- `pnpm api:check` — PASS; inventario actual de 189 operaciones.
- `pnpm test` — PASS, 103/103, 0 fallos/omitidos.
- `pnpm format:check` y `git diff --check` — PASS.
- `PLAYWRIGHT_PORT=4203 pnpm exec playwright test tests/reviewer-queue.spec.ts --reporter=list --workers=2 --output=/home/nandev/.hermes/cache/scratch/reviewer-queue-next-coverage-20261112-final` — PASS, Chromium local 9/9; todas las APIs de identidad fueron interceptadas.
- Un intento previo de Playwright falló porque `getByRole("alert")` también seleccionó el route announcer de Next. El selector se limitó al formulario y la ejecución consolidada final pasó 9/9.
- No se repitió `pnpm build`: este subcorte solo modificó tests; el build de producción previo cubre el código UI sin cambios de este subcorte.

## Límites y gates pendientes

- La prueba BFF ejecuta el handler Next, pero `fetch` upstream es sintético. No demuestra forwarding a Go en red, autenticación real, grants/asignación, auto-revisión server-side, MFA, CAS/locks, transacción/auditoría durable ni persistencia.
- El test de sesión expirada no establece si una petición ya enviada llegó a un backend real ni si esa mutación se confirmó; solo prueba ausencia de publicación/replay en esta ejecución mock.
- Los tests de fecha caracterizan el constructor y la UI con reloj/fecha del entorno o fixtures; no sustituyen el cálculo Go ni prueban storage.
- No se ejecutó SQL/DDL ni se conectó a PostgreSQL persistente o Cloud. No se usaron documentos, cuentas ni datos reales. V95/V96 siguen sin aplicar.
- No hubo commit, push, integración a `main` ni despliegue. Los otros cambios dirty del worktree se preservaron.
- VI-03 permanece parcial; no habilita revisores reales. La autorización efectiva, MFA, integración BFF→Go y gates de persistencia/operación siguen fuera de la evidencia local.
