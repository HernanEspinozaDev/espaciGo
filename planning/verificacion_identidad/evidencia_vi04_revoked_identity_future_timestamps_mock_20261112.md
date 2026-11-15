# VI-04 — identidad revocada y vigencia futura en Mi cuenta

- **Fecha:** 2026-11-12.
- **Frontend:** `/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes`, rama `frontend-next-hermes`, HEAD `cf6e28a365d2495e78bc64c01686ed55efb17c8a`.
- **Plan:** `/home/nandev/Documentos/projects/espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD `274f37e483a078890e92382a159fa49fd23c2fe8`.
- Se añadió una prueba Playwright a `frontend/tests/phase-f6-document-verification.spec.ts`; no cambió código de producción. Se preservaron los worktrees compartidos y sus cambios no relacionados.

## Escenario y resultado

`revoked identity stays revoked despite future verification timestamps` entrega por fixtures un caso owner identity `revocada`, revisión 8, con `verified_at`, `verified_until` y vencimiento del documento en el futuro. En `/mi-cuenta/documentos`, la fila y el detalle conservan `Revocada`; no aparece `Aprobada` ni `Vigente hasta`. La prueba exige exactamente una lectura GET de la colección y una GET del detalle, sin POST de caso, carga o envío.

Las primeras corridas fallaron por defectos del propio test: el GET de detalle no incluía el ETag fuerte requerido por el adaptador y luego un locator esperaba dos nodos de texto separados cuando el estado forma parte del encabezado compuesto. Se corrigieron el fixture y la aserción. No se observó un fallo de producción ni se atribuye RED/GREEN.

## Gates locales

- Playwright Chromium, spec completo `tests/phase-f6-document-verification.spec.ts`: **4/4 PASS**; standalone local en `127.0.0.1:4203`.
- `pnpm typecheck`, `pnpm lint`, `pnpm test` (**103/103**), `pnpm format:check` y `git diff --check`: PASS.
- Solo cambió una prueba; no se reconstruyó el standalone. La suite de Node imprimió el warning preexistente `MODULE_TYPELESS_PACKAGE_JSON` para `src/lib/api/clp.ts`.

El caso, los timestamps, sesión y respuestas son sintéticos/interceptados. Esto verifica presentación con un estado terminal ya calculado por servidor; no demuestra que Go/DB derive la revocación, autorización real, BFF→Go, persistencia, ni que una aprobación vieja sea invalidada por un cambio material de declaración en almacenamiento. VI-04 sigue parcial. No ejecuté SQL/DDL, no accedí a bases persistentes ni Cloud; sin migraciones, datos reales, commit, push, integración o despliegue.
