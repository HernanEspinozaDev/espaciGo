# Evidencia — VI-03, fallo al refrescar asignaciones del reviewer

- **Fecha:** 2026-11-12.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y muy dirty; se conservaron sus demás cambios.

## Alcance

La lógica ya existente de `ReviewFacade.load` fallaba cerrada ante error de la lista, pero faltaba una regresión que cubriera estado privado de revisor completo y la recuperación visible. Se amplió la caracterización unitaria existente para iniciar con expediente identity seleccionado, declaración sintética y preview `Blob`; tras un HTTP 500 del puerto de lista afirma limpieza de filas, selección y preview, revocación de la URL, error genérico sin texto crudo y bloqueo de re-selección/decisión hasta que una actualización autoritativa tenga éxito. La recuperación requiere volver a seleccionar el expediente. Revisión independiente `deleg_696bcb1e` aprobó sin hallazgos y observó que la cobertura no afirma explícitamente el bloqueo de cada lectura posible; el test sí cubre la re-selección/decisión por UI y cero writes. Se conserva ese límite en la conclusión.

Se añadió un recorrido Playwright con API stub local: un GET de lista devuelve 503 tras abrir el detalle; la UI quita filas, declaración, estado y controles de archivo, muestra el aviso genérico y no envía decisiones. El refresh exitoso repuebla la lista sin auto-seleccionar ni restaurar el detalle; la re-selección explícita vuelve a cargarlo. El contador sintético confirma cero writes.

No cambió código de producción: el comportamiento caracterizado ya estaba presente. La primera ejecución unitaria pasó, por lo que este subcorte está identificado como caracterización, no como RED/GREEN. Una ejecución E2E inicial solo falló por un locator ambiguo entre la alerta y el announcer de Next; se afinó el locator y se repitieron las pruebas.

## Verificación local

- `node --test tooling/document-review-facade.test.mjs tooling/document-review.test.mjs`: 32/32 PASS.
- `node_modules/.bin/tsc --noEmit`: PASS.
- ESLint y Prettier en ambos tests: PASS.
- Playwright local, `tests/phase-document-review.spec.ts`: 16/16 PASS en `127.0.0.1:4203`, usando el standalone previamente construido y respuestas API interceptadas. No hubo BFF→Go. Servidor aislado, con entorno mínimo, detenido al terminar.
- `git diff --check`: PASS.
- Revisión independiente del subcorte: pendiente.

## Límites

Fixtures, puerto falso y API interceptada; no valida HTTP/backend/auth integrado, DB, transacción, persistencia, reinicio, ni almacenamiento remoto. No ejecuté SQL/DDL, no accedí a bases persistentes o Cloud, no usé datos reales y no hice commit, push, integración a `main` ni despliegue. VI-03 sigue parcial por sus gates de integración y producto.
