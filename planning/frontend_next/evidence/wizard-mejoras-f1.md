# Mejora del wizard — Fase 1, revisión local

## Alcance y fuente

Rama `codex/frontend/fix/mejoras-creacion-espacios`, worktree `/home/nandev/Documentos/projects/espaciGo-wizard-improvements`, base `771134d165046b290b1f5ad3a1074a6357c2327f`. Fuentes de esta fase todavía sin commit; no se adjudica el código nuevo al SHA base. Hojas GPT-6 Luna, razonamiento medio; padre revisa y repite gates.

La limpieza omite atributos ausentes, conserva strings no vacíos sin mutar enums wire y mantiene listas inválidas para que las rechace el validador. False y cero se conservan. La validación de categoría y la creación usan atributos limpiados. El resumen usa las etiquetas del perfil cargado y un fallback de texto legible, no imprime JSON/undefined/booleanos crudos. Los builders createDraft y updateDraft omiten opcionales ausentes en lugar de construir propiedades undefined. El serializador exacto no se debilita; se conserva If-Match y ETag.

## Pruebas repetidas por el padre

- `node --test frontend/tooling/host-listings-draft-payload.test.mjs`: 4 PASS tras adaptar el harness al helper real.
- Suite completa `pnpm test`: **460 tests; 459 PASS, 0 FAIL, 1 SKIP**. El omitido es la demostración histórica opt-in del adaptador; no se presenta como prueba ejecutada por el padre.
- `pnpm typecheck`: PASS.
- `pnpm lint`: 0 errores, 3 advertencias en archivos ajenos al corte (img en galería y dos variables de tests).
- `pnpm build`: PASS, Next16.4.3, artefacto standalone de estas fuentes.
- `pnpm exec playwright test tests/wizard-improvements.spec.ts --workers=1 --output=test-results-wizard-f1-parent`: **1 PASS**, Chromium, servidor propio loopback `127.0.0.1:4203` creado por Playwright; preflight de puertos4203/4204 libre. Recorrido login sintético controlado → perfil inmuebles versionado → atributo opcional duración tocado y borrado → confirmación humana → POST interceptado. Conserva amoblado=false y dormitorios=0; omite duración no informada. No se envió a Go ni a una base de datos.
- `git diff --check`: PASS.

La prueba navegador falló antes por descripción del fixture de86caracteres y selectores incorrectos; no era evidencia de otra regresión del producto. El padre leyó error-context y exigió corregir el fixture y ejecutar realmente el caso, no aceptar solo su descubrimiento.

## Límites y continuación

Gate técnico local F1 verificado; no aceptación humana ni integración nativa. La creación persistida con API/DB reales, el recorrido completo de edición y el despliegue pertenecen a F5 y no se acreditan con mocks. El redirect histórico a `/mi-cuenta/espacios/...` no tiene ruta real en el build inspeccionado; la prueba de payload solo observa navegación, no demuestra un editor funcional. Revisar y corregir en F3 usando las rutas privadas reales dentro del shell Mi Cuenta, sin inventar montajes.

El test reveló además feedback inline ausente para un enum requerido: debe mantenerse como hallazgo de accesibilidad/validación de F3, no suprimirse de la matriz por quitar una expectativa. F2 continúa con selectores de región/comuna y readback de CUT interno. Sin commit, push, integración a main ni despliegue de estas mejoras.
