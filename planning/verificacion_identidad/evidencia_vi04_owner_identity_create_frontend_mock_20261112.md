# Evidencia VI-04 — alta owner identity alineada al contrato local

- **Fecha:** 2026-11-12
- **Frontend:** `/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes`
- **Rama frontend:** `frontend-next-hermes`
- **HEAD frontend:** `cf6e28a365d2495e78bc64c01686ed55efb17c8a`
- **Checkout de plan/evidencia:** `/home/nandev/Documentos/projects/espaciGo-main-sync`
- **Rama/HEAD del plan:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`

## Cambio verificado

Se alineó el contrato frontend de `POST /document-verification/cases` para `purpose: identity` con la solicitud Go revisada: formato declarado (`legacy`, `new` o `unsure`), consentimiento afirmativo y versión del aviso local sintético. El alta de identidad ya no pide ni envía `document_expires_at`; el vencimiento opcional permanece en el formulario de documentos de espacio. La pantalla aclara que el uso permitido en este ensayo es solo sintético, exige marcar la casilla explícita y no presenta el formato declarado como prueba de autenticidad. Se regeneraron tipos, inventario y allowlist desde OpenAPI. También se ajustaron las pruebas de normalización, el contrato OpenAPI y el recorrido mock de alta; la caracterización de revocación ahora comprueba que la identidad revocada no muestra vigencia ni vencimiento.

La versión `identity-capture-local-v1` y la casilla son exclusivamente un contrato/aviso de ensayo local. No constituyen consentimiento legal, aprobación de captura real ni autorización para usar datos personales.

## Pruebas locales

Todas ejecutadas en el checkout frontend indicado, con datos sintéticos y APIs interceptadas cuando correspondía:

- `pnpm build` — PASS; artefacto standalone local actualizado.
- `PLAYWRIGHT_PORT=4203 pnpm exec playwright test tests/phase-f6-document-verification.spec.ts --output "$TMPDIR/vi-identity-form-playwright-verified"` — PASS, 4/4. El servidor standalone solo se accedió en loopback y las rutas API del spec fueron mockeadas.
- `pnpm test` — PASS, 103/103, 0 fallos, 0 omitidos. Node emitió el warning existente `MODULE_TYPELESS_PACKAGE_JSON` asociado a `src/lib/api/clp.ts`.
- `pnpm lint` — PASS.
- `pnpm typecheck` — PASS.
- `pnpm format:check` — PASS.
- `pnpm api:check` — PASS; tipos, allowlist e inventario generados vigentes, inventario de 189 operaciones.
- `node --test tooling/document-verification-openapi.test.mjs tooling/document-verification-domain.test.mjs` — PASS, 3/3.
- `git diff --check` — PASS.

En una primera corrida de la spec Playwright, 3/4 pasaron y el caso de revocación falló porque todavía esperaba el texto de vencimiento que esta UI identity dejó de mostrar; se actualizó la aserción para exigir el estado revocado y ausencia de vencimiento, y la spec completa pasó 4/4. La primera suite Node tras el cambio de contrato también encontró una expectativa OpenAPI antigua; se actualizó para localizar cada variante por `purpose.const` y comprobar los campos nuevos y vencimiento opcional solo para espacio; la repetición terminó 103/103.

## Límites y gates pendientes

Este resultado acredita únicamente contrato generado, lógica de formulario y comportamiento de presentación con API mockeada. No demuestra recorrido BFF→Go, recepción real del body, consentimiento legal aprobado, captura/carga de fotografías, autorización/asignación de revisor, estado durable, SQL/DDL, PostgreSQL, storage, GCS, Cloud ni uso con personas reales. No se ejecutó SQL/DDL ni se accedió a bases persistentes o Cloud. No hubo commit, push, integración a `main` ni despliegue. VI-04 y el MVP permanecen parciales; los gates de integración autorizada y captura real continúan cerrados.
