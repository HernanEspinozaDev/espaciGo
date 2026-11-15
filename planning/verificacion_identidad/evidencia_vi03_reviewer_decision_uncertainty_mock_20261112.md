# VI-03 — cuarentena y readback explícito tras decisión incierta

- **Fecha:** 2026-11-12.
- **Frontend checkout:** `/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes`, rama `frontend-next-hermes`, HEAD `cf6e28a365d2495e78bc64c01686ed55efb17c8a`.
- **Plan/evidencia checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD `274f37e483a078890e92382a159fa49fd23c2fe8`.
- Worktrees compartidos y dirty; los cambios ajenos/preexistentes fueron preservados. Sin commit.

## Cambio

En la interfaz de decisión reviewer, un error después de intentar el POST se trata como resultado incierto: borra el snapshot privado de detalle, desmonta el visor (revocando Blob URLs), oculta declaración/evidencias y retira el formulario. No se reenvía la decisión. Solo un GET explícito vuelve a habilitar el detalle; si el GET falla, la cuarentena permanece y solo se ofrece otro GET. Si el GET confirma el estado terminal, se presenta ese estado autoritativo.

La validación local del draft se separa del fallo posterior al envío para no clasificar una fecha inválida como mutación incierta; tal error no hace POST ni oculta el expediente. Al perder sesión se conserva el flujo existente de purga/no disponibilidad, sin panel que sugiera una sesión usable.

## Prueba sintética

`tests/reviewer-queue.spec.ts` simula un commit y pérdida de respuesta (`route.abort`), luego una lectura explícita 503 y recuperación GET 200. Comprueba que PII/formulario desaparecen, el Blob URL se revoca, el fallo de refresco no desbloquea acciones, la recuperación presenta `aprobada` y existe exactamente un POST. El test observó RED antes de la implementación: faltaba el estado «Resultado no confirmado». La corrida completa posterior encontró dos regresiones adicionales (validación local confundida con incertidumbre y expiración de sesión); se separó prevalidación y se mantuvo el camino de purga por sesión.

## Gates finales

- `pnpm typecheck` — PASS.
- `pnpm lint` — PASS.
- `pnpm test` — PASS, 103/103.
- `pnpm format:check` y `git diff --check` — PASS.
- `pnpm build` — PASS; Next.js 16.4.3 generó 31/31 páginas.
- `PLAYWRIGHT_PORT=4203 pnpm exec playwright test tests/reviewer-queue.spec.ts --reporter=list --workers=2` — PASS, 11/11 Chromium local sobre el standalone reconstruido.

No se cambió OpenAPI; `api:check` no se repitió para este subcorte. La suite Node emitió un warning de Node preexistente `MODULE_TYPELESS_PACKAGE_JSON` en `src/lib/api/clp.ts`; salida 0 y 103 tests PASS.

## Límites

La simulación no demuestra cuándo un servidor real confirmó el POST, autorización/asignación, CAS, auditoría, BFF→Go, transacción ni persistencia. La cuarentena UI no revierte una mutación remota; el GET solo reconcilia el estado visible. No ejecuté SQL/DDL ni accedí a bases persistentes o Cloud; V95/V96 no se aplicaron. Sin datos reales, commit, push, integración a `main` o despliegue. VI-03 continúa parcial.
