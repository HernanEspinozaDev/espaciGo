# Evidencia VI-03 — revocación de preview al cambiar de expediente

- **Fecha:** 2026-11-12
- **Frontend:** worktree `/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes`, rama `frontend-next-hermes`, HEAD base `cf6e28a365d2495e78bc64c01686ed55efb17c8a`.
- **Registro del plan:** checkout `/home/nandev/Documentos/projects/espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Alcance:** test Playwright nuevo en `frontend/tests/reviewer-queue.spec.ts:601`; no se modificó código productivo.

## Caracterización

`revokes the private evidence preview when navigating to another case` inicia sesión sintética, abre A, descarga una evidencia PNG limpia mockeada y verifica su `blob:` URL. Al volver a la cola, el test espera que el URL exacto haya sido pasado a `URL.revokeObjectURL`; después abre B y verifica que solo la declaración de B está visible, que no aparece una preview y que no hay POST de decisión. La fixture de ambos casos contiene exactamente los tres roles únicos exigidos por el dominio.

El test pasó en la primera ejecución. Es una caracterización del ciclo de vida de la vista/Blob URL; no se afirma RED/GREEN ni cambio productivo.

## Verificación local

- Focal Playwright Chromium: 1/1 PASS.
- Suite `tests/reviewer-queue.spec.ts`: **14/14 PASS**, incluyendo la carrera de detalle tardío del subcorte anterior.
- `pnpm typecheck`, `pnpm lint`, `pnpm format:check`: PASS.
- `pnpm test`: 103/103 PASS, cero skips; warning existente `MODULE_TYPELESS_PACKAGE_JSON` en `src/lib/api/clp.ts`.
- Standalone local existente en `127.0.0.1:4203`; no se reconstruyó porque solo cambió el test.

## Límites

La ruta Go/BFF y el endpoint de descarga son interceptados; el archivo es sintético. No demuestra autorización/asignación efectiva, descifrado/custodia del storage, revocación fuera del navegador, integración BFF→Go, persistencia, auditoría o cambio real de cuenta. Sin SQL/DDL, acceso a bases persistentes o Cloud, commit, push, integración o deploy.
