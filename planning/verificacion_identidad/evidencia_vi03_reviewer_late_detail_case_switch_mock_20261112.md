# Evidencia VI-03 — detalle tardío al cambiar de expediente

- **Fecha:** 2026-11-12
- **Frontend:** worktree `/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes`, rama `frontend-next-hermes`, HEAD base `cf6e28a365d2495e78bc64c01686ed55efb17c8a`.
- **Registro del plan:** checkout `/home/nandev/Documentos/projects/espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Alcance del subcorte:** solo test Playwright; no se modificó código productivo.

## Caracterización

`frontend/tests/reviewer-queue.spec.ts:452` agrega `does not publish a late identity detail after navigating to another case`. El test retiene el GET de detalle sintético de A, vuelve a la cola, abre B y confirma que B muestra su declaración. Luego libera el GET 200 de A, espera que termine y cede dos frames del navegador; verifica que A no se publica, B permanece visible y no hubo POST de decisión. Las respuestas de A y B usan DTOs sintéticos completos con exactamente tres roles únicos, conforme al contrato tipado del feature.

El primer intento falló por fixture inválido: detalle sin los tres roles requeridos; no fue un RED de producción. Tras completar los DTOs, la caracterización pasó. No se afirma RED/GREEN ni cambio productivo.

## Verificación local

- Playwright Chromium local `tests/reviewer-queue.spec.ts`: **13/13 PASS**, incluida la nueva carrera de detalle tardío.
- `pnpm typecheck`: PASS.
- `pnpm lint`: PASS.
- `pnpm test`: **103/103 PASS**; Node emitió el warning existente `MODULE_TYPELESS_PACKAGE_JSON` para `src/lib/api/clp.ts`.
- `pnpm format:check`: PASS.
- Se usó el standalone local existente en `127.0.0.1:4203`; al ser un cambio test-only no se reconstruyó. No hubo solicitudes a servicios externos.

## Límites

La prueba usa Playwright y respuestas API interceptadas; cubre navegación entre expedientes con la misma sesión, no un cambio real de cuenta, autenticación/autorización, asignación server-side, BFF→Go, PostgreSQL, persistencia, almacenamiento privado ni auditoría. No se ejecutó SQL/DDL ni se accedió a bases persistentes o Cloud. Sin commit, push, integración a `main` o deploy.
