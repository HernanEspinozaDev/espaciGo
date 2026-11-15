# Evidencia — VI-03, refresco de asignaciones conserva detalle vigente

- **Fecha:** 2026-11-12.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y muy dirty; se preservaron los cambios preexistentes y ajenos.

## Cambio

La lista de reviewer se serializa mediante `reviewerDTO` y omite `identity_declaration`; el detalle autorizado la agrega en `loadDetail`. Al pulsar «Actualizar expedientes», `ReviewFacade.load` reemplazaba el detalle seleccionado por el DTO resumido de la lista, aunque conservara el mismo ID y revisión. La declaración desaparecía de la vista pese a que el expediente seguía asignado y la revisión no había cambiado.

Ahora el facade conserva únicamente la `identity_declaration` previa cuando la asignación actual contiene el mismo expediente de propósito `identity`, con la misma revisión, y el DTO resumido no trae declaración. Usa los demás campos del DTO actualizado. Si la asignación desaparece o cambia la revisión, no conserva la declaración anterior.

## Evidencia y verificación

- La regresión de `document-review-facade.test.mjs` primero falló: la declaración seleccionada quedó `undefined` tras refrescar una fila de lista sin `identity_declaration`. Tras el cambio pasó; también comprueba que una nueva revisión no hereda la declaración anterior.
- `node --test tooling/document-review-facade.test.mjs tooling/document-review.test.mjs`: 32/32 PASS.
- `node_modules/.bin/tsc --noEmit`: PASS.
- ESLint y Prettier en los archivos afectados: PASS.
- `node_modules/.bin/next build`: PASS.
- Playwright local, `tests/phase-document-review.spec.ts`: 15/15 PASS, incluido el recorrido visible de seleccionar detalle, refrescar asignaciones y seguir viendo la declaración. Servidor standalone únicamente en `127.0.0.1:4203`, iniciado con entorno mínimo y detenido al terminar. El test intercepta la API; no usa BFF→Go ni datos reales.
- `git diff --check`: PASS.

## Revisión independiente

`deleg_fc1fd66e` aprobó la lógica revisada en el checkout/rama/HEAD indicados, sin hallazgos de seguridad o errores lógicos. Confirmó que el detalle solo se conserva si el caso sigue asignado, sigue siendo `identity`, mantiene la misma revisión y la lista no trae declaración; los campos resumidos vienen de la fila actual. La revisión fue estática: no ejecutó pruebas, build, lint ni servicios.

Como sugerencia no bloqueante, propuso un caso donde el mismo ID deje de ser `identity`. No se agregó: la revisión no estableció que esa transición sea válida bajo el contrato; además, el test existente `authoritative empty assignment refresh purges the selected case` ya cubre la desaparición de la asignación y deja `selected` en `null`. La regresión nueva cubre la revisión cambiada. Estas coberturas son unitarias/mocks; no equivalen a integración.

## Límites

La prueba del facade y el recorrido Playwright usan mocks sintéticos. La inspección del handler confirma la forma actual del DTO, pero no se ejecutó HTTP real del backend, autorización integrada, PostgreSQL, persistencia, reinicio ni storage. No ejecuté SQL/DDL, no accedí a bases persistentes o Cloud y no hice commit, push, integración a `main` ni despliegue. VI-03 continúa parcial hasta sus gates de integración/transacción y validación humana/producto.
