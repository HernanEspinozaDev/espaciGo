# Mejora del wizard — Fase 2, revisión local

Base y worktree iguales a `wizard-mejoras-f1.md`; fuentes sin commit sobre `771134d165046b290b1f5ad3a1074a6357c2327f`.

## Implementación y límites

Región antes de comuna, opciones con nombres y valores CUT internos; comuna deshabilitada sin región y filtrada por la seleccionada. Cambiar región limpia la comuna incompatible; el resumen muestra nombres. Validación contra el catálogo real, no solo prefijo numérico; se conserva el par opcional vacío del contrato. Se mantiene feedback accesible de comuna y se compone el ID de error con el aviso de límite, sin sobrescribirlo.

Catálogo estático completo de la fuente versionada más suplemento documentado en `wizard-catalogo-dpa.md`. Antártica12202 figura visible y deshabilitada: no está en el catálogo backend versionado ni satisface su FK. No se hizo migración ni se afirma persistencia de esa comuna. La cobertura UI operativa queda limitada por ese dato; no se presenta como soporte backend completo del catálogo oficial.

## Gates repetidos por el padre

- `node --test tooling/chile-dpa.test.mjs tooling/host-listings-wizard-location.test.mjs`: **7 PASS, 0 FAIL**.
- `pnpm typecheck`: PASS.
- `pnpm build`: PASS sobre las fuentes F1/F2 reunidas.
- `pnpm exec playwright test tests/wizard-improvements.spec.ts --workers=1 --output=<scratch>/espacigo-wizard-evidence/f2-parent`: **1 PASS** en Chromium loopback4203. Comprueba cambio Metropolitana→Valparaíso con comuna vacía, selección Cartagena y regreso a Santiago, luego conserva regresión JSON y POST CUT interno. Es transporte controlado, no Go/DB.
- `git diff --check`: PASS.

El padre detectó una prueba de ubicación rota al importar el nuevo catálogo y pérdida del feedback ARIA de comuna; se corrigieron antes de este build. Las pruebas históricas que antes escribían un código manual usan ahora selectOption y región explícita; las nativas no se ejecutaron con ese mero cambio.

## Continuación

Gate técnico local F2 aprobado en el alcance descrito. La integración nativa está en F5; soporte Antártica es una brecha de datos fuera de este corte sin autorización SQL nueva. F3 implementa regreso a pasos, revalidación y rutas privadas correctas. Ningún cambio publicado ni desplegado todavía.
