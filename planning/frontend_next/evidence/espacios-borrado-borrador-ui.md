# Evidencia E2E: borrado de borrador

**Fecha:** 2026-10-27  
**Base:** `main` en `3956d2f3459b0bf4b15511f4e1a74ffd06a29975`  
**Alcance:** recorrido frontend local con Playwright y `mockGateway`; no hubo llamadas al Go real ni escrituras en PostgreSQL.

## Comportamiento verificado

- En `/anfitrion/espacios`, desde la pestaña «Borradores», se solicita confirmación para «Oficina Providencia A».
- Al confirmar, la UI envía `DELETE /api/backend/spaces/00000000-0000-4000-8000-000000000011` con UUID sintético válido.
- El mock devuelve HTTP 200 con el recibo Go de eliminación (`space_id`, `state: eliminada`, `occurred_at`, `removed`, `preserved`, `files_pending_cleanup`, `reused`).
- La UI presenta «Borrador eliminado correctamente.» y quita la tarjeta de la lista.

## Verificación

- `pnpm exec playwright test tests/phase7-host-listings.spec.ts` — **6/6 PASS**.
- `pnpm format:check` — PASS.
- `pnpm lint` y `pnpm typecheck` — PASS.
- `pnpm test` — **259/259 PASS**.
- `pnpm build` — PASS después del cambio de etiqueta de la tarjeta Mi Cuenta; las modificaciones posteriores fueron solo de tests/documentación.
- `tests/phase6-account.spec.ts` — **7/7 PASS** después del cambio de etiqueta «Archivado».

## Límites

Esta evidencia verifica el contrato de frontend, la confirmación y el estado visible usando un mock local. No demuestra borrado persistente ni integración real Go/PostgreSQL. No se modificaron backend, OpenAPI, SQL, cuentas ni datos persistentes. No se creó commit ni se hizo push.
