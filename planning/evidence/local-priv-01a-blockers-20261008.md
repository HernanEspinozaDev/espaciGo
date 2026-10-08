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

## Aceptación posterior al merge

PR #188 se integró en `main` el 2026-10-08 (merge `18f2d65`). El checkout avanzó por fast-forward; `bash scripts/dev-env.sh up -d` aplicó V23 incrementalmente y dejó database, migrate, backend, Mailpit y mock saludables. La consulta de `schema_migrations` confirmó la versión 23. No se borraron ni reiniciaron `espacigo_pgdata`, secretos o datos locales. Este merge acepta la revisión/presentación administrativa local únicamente; la supresión sigue deshabilitada. #185 y #40 permanecen abiertas.

## Dependencia M10 en curso

La evaluación de V23 puede detectar pagos y reservas activos, pero la tabla de pendientes admitía `fuente_disputas_no_modelada`. La subentrega de seguimiento se acota a persistir y consultar el ciclo mínimo de disputas requerido por privacidad. No completa DIS-ARCH-01 ni M10: evidencia fotográfica, ventanas de operación, descargos, decisión sobre garantía/fondos y avisos siguen fuera de esta subentrega y conservan sus dependencias originales.

## LOCAL-PRIV-01A2 / #189 — incidencia sintética como bloqueador

Alcance ratificado el 2026-10-08: el anfitrión de una reserva sintética persistida puede abrir una incidencia `abierta` con motivo codificado `ensayo_privacidad`, incluso si la reserva es terminal. No se aplica aquí la ventana comercial de 24 horas. Reintentos con la misma clave recuperan el mismo registro; una segunda incidencia abierta en esa reserva produce conflicto. Ambas partes consultan estado/historial y los terceros reciben 404. Solo administrador cierra con `ensayo_finalizado`, `registro_erroneo` o `duplicada`. La incidencia cerrada no se reabre y el cierre no resuelve asuntos económicos.

La migración incremental V24 incorpora `disputa_ensayo_local` y `disputa_ensayo_historial`, con claves foráneas restrictivas, índice único parcial por reserva abierta, huella/idempotencia y secuencia estable. `espacigo_runtime` puede leer/insertar/actualizar el registro de ensayo e insertar, pero no actualizar ni borrar, historial. La revisión de supresión comprueba disputas abiertas para anfitrión y arrendatario bajo el mismo bloqueo de cuenta usado por apertura/cierre; terminales históricos no cuentan como disputa abierta y no alteran las demás obligaciones.

### Evidencia automatizada

- `TestLocalDisputeBlocksBothParticipantsUntilAdministratorCloses`: API con autenticación y `espacigo_runtime`; apertura en una reserva terminal, replay antes y después del cierre, conflicto por segunda apertura, motivo inválido, anfitrión de otro espacio denegado, lectura por participantes/404 de tercero, admin-only close, historial secuencial append-only, `disputa_abierta` para ambas cuentas y eliminación solo de ese bloqueador tras cierre. También confirma que reserva y ocupación no cambian, y que runtime no puede modificar el actor ni modificar/borrar historial.
- `TestLocalDisputeOpenAndCloseSerializeWithSuppressionReview`: PostgreSQL mantiene el bloqueo de fila mientras apertura y evaluación quedan esperando; al liberar, la revisión ve la disputa confirmada. Luego hace cola de cierre antes de otra evaluación y confirma que ve el estado cerrado. Comprueba que no hay historial parcial ni duplicado.
- Ejecución aprobada: `GO_TEST_RUN='TestLocalDispute' bash scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/identity` pasó ambos tests en PostgreSQL 18 desechable, con migraciones V1–V24. El harness crea `espacigo_runtime` antes de aplicar las migraciones. `go vet ./internal/dispute/... ./internal/adapters/postgres/dispute ./internal/adapters/postgres/identity ./cmd/api` pasó. `cd mock && npm run build && node --test test/suppression-review-state.test.mjs` pasó; OpenAPI YAML parseó y `git diff --check` no reportó errores. No se tocó el volumen persistente ni se ejecutaron pruebas GCP.

El mock muestra el estado al seleccionar/actualizar una reserva; el anfitrión puede abrir el motivo sintético; ambas partes consultan, y un panel administrativo intenta cierre estructurado. Las respuestas se insertan como texto DOM y los resultados tardíos se descartan tras cambiar reserva/sesión. No se implementan evidencia, operación comercial, mensajes libres, avisos ni resolución financiera.

La tabla de recomendación por fundamento/plazo se entrega separadamente en `planning/propuesta_fundamentos_plazos_privacidad_local.md`. Es una base para ratificación; no se codifica en V24. La matriz pendiente de retención mantiene `revision_incompleta` aun después de cerrar la disputa, por lo que la supresión continúa deshabilitada. #185/#40 y los requisitos generales M10 permanecen abiertos; este corte no desbloquea DIS-ARCH ni trabajo de GCP.
