# LOCAL-PRIV-01A — clasificación y evaluación de bloqueadores

Fecha: 2026-10-08. Rama de continuación sobre `main` en el merge `380147e` (PR #187). V23 es aditiva; solo se aplicó a bases PostgreSQL desechables durante las pruebas. `espacigo_pgdata`, secretos y datos sintéticos persistentes no se usaron ni modificaron.

## Resultado

- Se clasificaron las relaciones de identidad, sesiones/tokens, perfil/aceptaciones, solicitudes, verificación/evidencia privada, borradores/fixtures, reservas/pagos, mensajes, historial de claves, outbox, auditoría y derivados de UI/cache en `planning/clasificacion_privacidad_local.md`.
- `GET /api/v1/privacy/suppression-requests` y `POST /api/v1/privacy/suppression-requests/{request_id}/review` son solo para rol administrador. La cola no incluye correo ni ID de cuenta.
- V23 amplía `evento_auditoria_local` con clave idempotente y códigos estructurados. Detecta reservas no terminales, operaciones de pago pendientes, eventos sin conciliar y devoluciones pendientes. Reintentar la misma clave devuelve la evaluación persistida y no duplica auditoría.
- La cuenta se bloquea antes de revisar; el reloj se consulta después de adquirir el bloqueo. Se reutiliza el registro de auditoría append-only; `espacigo_runtime` conserva solo `SELECT, INSERT`, sin permisos de modificación o borrado. La fila aplica cinco años desde el evento.
- Los registros terminales por sí solos no bloquean. Sin obligaciones activas, el resultado sigue `revision_incompleta` porque no existe modelo local de disputas y faltan fundamentos/plazos para históricos fuera de auditoría.
- La operación no cambia el estado de la solicitud, no bloquea otros derechos y no ejecuta borrado/desidentificación. No se llama anonimizada una cuenta ni se elimina archivo local.

## Pruebas ejecutadas

`GO_TEST_RUN='TestM02SuppressionReview|TestCredentialCoreRuntimeLeastPrivilege' bash scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/identity`

Pasaron en PostgreSQL desechable: rol administrador requerido; cola mínima sin correo; evaluación idempotente y única auditoría; aislamiento para titular no administrador; detección combinada de reserva activa y pago pendiente; reserva terminal histórica sin bloqueo; reloj actualizado mientras la revisión esperaba el bloqueo de cuenta; el titular mantiene consulta de solicitudes y exportación después de la revisión; y se rechazan campos libres en la carga estructurada de auditoría. `TestCredentialCoreRuntimeLeastPrivilege` confirmó que auditoría permite `SELECT/INSERT` y deniega `UPDATE/DELETE` al runtime.

La revisión administrativa del endpoint se ejecutó usando un `pgxpool` con `SET ROLE espacigo_runtime` aplicado en cada conexión. La autenticación y autorización HTTP se mantuvieron en el Backend de prueba. No se ejecutó una suite completa del repositorio.

## Pendientes explícitos

- #185 y #40 siguen abiertas; #39/#43 y exportación integral siguen pendientes. #186/#187 acepta solo exportación de identidad. La evidencia previa verificó JSON y enlace, pero no confirmó que el navegador guardara el archivo.
- No hay modelo de disputas M10 para certificar que no existan disputas abiertas.
- Falta ratificar fundamento/plazo por dato para aceptaciones, verificaciones, borradores, reservas, pagos, mensajes y otros hechos históricos. Las copias/backups no tienen inventario operativo local suficiente.
- No se ejecuta la baja; eliminar o minimizar cualquier registro persistente necesita completar la matriz y una operación transaccional con bloqueos reevaluados junto a los módulos que crean obligaciones.
- Archivos reales, retención productiva e inmutabilidad RNF-017 quedan fuera; #142 y el trabajo de GCP no cambian.

## Ajuste de presentación del PR #188

La pantalla separa el estado/contador de solicitudes (`#suppression-queue-status`) de la evaluación administrativa (`#suppression-review-output`). Refrescar la cola actualiza solo el contador; conserva visibles `outcome`, `obligations_detected`, `pending_checks` y la fecha devueltos por la API. El estado y resultado visibles se limpian junto con la cola al cambiar o perder la sesión. La evaluación sigue siendo de revisión: la supresión permanece deshabilitada.

Prueba enfocada añadida: `mock/test/suppression-review-state.test.mjs`, secuencia evaluar → refrescar cola → comprobar contador y persistencia del resultado/fecha → limpiar sesión. Validación ejecutada con `cd mock && npm run build && node --test test/suppression-review-state.test.mjs`; suites Backend y ambientales no se repitieron.
