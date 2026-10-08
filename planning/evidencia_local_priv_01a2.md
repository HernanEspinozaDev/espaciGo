# Evidencia LOCAL-PRIV-01A2 — baja sintética con retención residual

Fecha: 2026-10-08. Rama `feat/local-priv-01a2-suppression`, basada en `main` sincronizada después de #191. Issue #192 bajo #185 y relacionada con #40. La decisión vinculante de alcance es [`politica_privacidad_local_v1.md`](politica_privacidad_local_v1.md).

## Recorrido incluido

- Migración incremental V25; no modifica V1–V24 ni aplica cambios al volumen persistente durante las pruebas. Las pruebas de PostgreSQL crean una base por caso y ejecutan migraciones desde una base vacía.
- Evaluación recalculable y ejecución administrativa de solicitud propia. Dentro de la ejecución se bloquea la cuenta y se revisan de nuevo reserva activa, pago/devolución/evento pendiente y disputa abierta. Los escritores de reserva, ocupación, pago, devolución y disputa serializan sus cambios con las cuentas participantes.
- Los escritores de perfil, verificaciones, evidencias sintéticas y borradores (incluidas tarifas, horarios y bloqueos manuales) comparten ese bloqueo de cuenta y revalidan el estado `activo` dentro de su transacción. Una carga de evidencia que pierde la carrera borra el PNG que ya había generado. Verificaciones previamente resueltas conservan `resuelta_en`/`retirar_en`; `retirada_privacidad_en` registra el evento separado. Las verificaciones pendientes se cierran en la baja y su vencimiento de dos años parte de ese instante.
- Cuenta elegible: la transacción retira sesiones/tokens, hash vigente e historial, roles, perfil/preferencia, cotizaciones no convertidas, contenido libre y fixtures propios; no toca la cuenta de terceros ni elimina reservas, ocupaciones, snapshots o historiales convertidos.
- Vencimientos: aceptaciones y solicitud de derechos a cinco años calendario; verificación sintética terminal a dos años; outbox terminal a 30 días; reserva terminal registra `vinculos_retirar_en` a 24 meses desde el cierre correlacionado más reciente. Cotizaciones no convertidas vencen 90 días después de expirar y se eliminan al ejecutar la baja.
- Los blobs de evidencia se eliminan fuera de la transacción principal con trabajo durable por archivo. Se borra la referencia solo tras éxito. Un fallo deja `limpieza_pendiente`; el reintento del endpoint o el worker al reiniciar puede completarlo. El API comunica “baja con minimización y retención residual”, sin afirmar anonimización.
- El mock permite evaluar y ejecutar como administrador, muestra bloqueadores, decisión, retirado/conservado, archivos pendientes y fechas, con `textContent` y limpieza de estado al perder sesión.

## Pruebas ejecutadas

Con `TEST_DATABASE_URL` hacia dos instancias desechables PostgreSQL 18/PostGIS, ambas almacenadas en `tmpfs` y eliminadas al finalizar:

```text
go test -p 1 ./internal/adapters/postgres/identity -count=1
ok   github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identity  31.764s

go test -p 1 ./internal/adapters/postgres/booking -count=1
ok   github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/booking  2.947s
```

El paquete identity migró V1–V25 desde cero, aplicó grants runtime y ejecutó la baja a través de una conexión `SET ROLE espacigo_runtime`. Incluye:

- `TestM02SuppressionReviewListsOnlyLiveReservationAndPaymentObligations`: ejecución bloqueada por reserva activa sola, pago pendiente solo y disputa abierta sola; el caso terminal sin obligaciones no bloquea; reintento bloqueado devuelve el resultado persistido.
- `TestM02SuppressionExecutionRechecksDisputeAfterAccountLock`: una ejecución que espera el bloqueo ve una disputa concurrente ya confirmada y no minimiza la cuenta.
- `TestM02LocalSuppressionMinimizesSyntheticAccountAndRecoversFileCleanup`: rol no administrador denegado; minimización, revocación de sesión, retención por grupo, aislamiento de otra cuenta, mensajes/cursor terminales borrados sin tocar reserva/ocupación/snapshot; fallo de blob reintentado por el worker y replay idempotente con fecha persistida.
- `TestSuppressionSerializesPreviouslyAuthorizedProfileVerificationEvidenceAndDraftWrites`: bloquea cuenta, encola primero la baja y luego una escritura de perfil, creación de verificación, carga de evidencia y actualización de borrador previamente autorizadas; al liberarse, todas observan la cuenta retirada, no recrean datos y el PNG generado por la carga descartada queda eliminado.
- El test de ejecución de baja conserva una verificación aprobada siete meses antes con su fecha terminal y fecha de vencimiento originales, y comprueba que `retirada_privacidad_en` registra el instante de baja. Otra verificación pendiente queda cerrada en ese instante, con su vencimiento dos años después.
- La suite identity completa cubre además el reloj posterior al bloqueo y las rutas administrativas existentes.
- La suite booking completa confirma que la serialización account-first conserva conflictos de tarifa/horario, transiciones y ocupaciones.

Mock:

```text
npm run build
node --test test/suppression-review-state.test.mjs
  tests 2, pass 2, fail 0
npm run test:profile-races
  tests 22, pass 22, fail 0
```

También pasaron `go test ./...`, `go vet` en privacy, transporte identity, repositorios identity/booking/dispute, bootstrap y API, `git diff --check`, y `python -m openapi_spec_validator planning/openapi.yaml` (`OK`). `sqlc v1.31.1 generate` ejecutado dos veces dejó idénticos los tres outputs generados afectados. No se ejecutó GCP ni la suite de una pasarela real.

## Retención residual y límites abiertos

- `vinculos_retirar_en` agenda la fecha, pero el proceso que purga/desvincula referencias transaccionales una vez vencida sigue pendiente de los criterios integrales #185/#40. No se alteraron reservas históricas ni sus hechos financieros.
- No se enumeraron snapshots/backups externos. Se conservaron el volumen `espacigo_pgdata`, secretos locales, datos sintéticos y el almacenamiento privado existente. Ninguna copia se declaró borrada. La política exige reaplicar bajas registradas tras una restauración local; automatizar/verificar ese replay queda pendiente.
- Los términos de una baja sintética no definen retención legal ni productiva. Documentos reales, operaciones reales, proveedor real, GCP, supresión integral y anonimización quedan fuera.
- Fallos de archivos quedan recuperables y visibles como pendientes. No se marca la solicitud resuelta hasta confirmar el borrado coordinado de archivo y metadata.

## Corrección solicitada en #193 — escrituras concurrentes y fecha terminal

La corrección usa `internal/adapters/postgres/accountlock.LockActive` como punto de serialización común. Perfil, creación/reintento/revisión de verificación, alta de evidencia, borradores, tarifa, simulación privada, zona horaria, horario semanal y bloqueos manuales adquieren el bloqueo de `usuario` antes de las filas propias y revalidan `estado='activo'` dentro de la transacción. Si una carga de evidencia generó el archivo antes de perder el bloqueo, `EvidenceService.Upload` elimina ese archivo tras el rechazo; la prueba comprueba que el directorio temporal queda vacío.

`verificacion.retirada_privacidad_en` conserva la fecha de retirada independiente de `resuelta_en`. La prueba incluye una verificación aprobada siete meses antes de la baja: conserva su fecha terminal y `retirar_en` original (dos años desde aquella resolución). También incluye otra verificación pendiente, que se cierra en la fecha de baja y cuyo vencimiento parte de ese cierre.

Comprobaciones posteriores en instancias separadas PostgreSQL 18/PostGIS descartables (el script `scripts/test-m04-attributes-postgres.sh` usa red aislada y `tmpfs`; no conectó al volumen persistente):

```text
TestM02ProfileAndRightsQueriesRemainOwnerScoped — PASS
TestM02LocalSuppressionMinimizesSyntheticAccountAndRecoversFileCleanup — PASS
TestSuppressionSerializesPreviouslyAuthorizedProfileVerificationEvidenceAndDraftWrites — PASS
internal/adapters/postgres/verification — PASS
internal/adapters/postgres/spaces — PASS
internal/adapters/postgres/occupancy — PASS
internal/adapters/postgres/pricing — PASS
internal/adapters/postgres/booking — PASS
go test ./... — PASS
go vet en accountlock, identity, verification, spaces, pricing, occupancy, booking, dbbootstrap y privacy — PASS
git diff --check — PASS
```

La primera ejecución agrupada de varios paquetes que crean `espacigo_runtime` compartió una instancia y encontró colisión de ese rol global; además, una expectativa antigua de fixture trataba una cuenta pendiente de verificación como activa. El fixture ahora establece estado activo explícitamente y los paquetes con rol global se reejecutaron en instancias separadas; todas las comprobaciones listadas arriba pasaron. Los contenedores de prueba fueron retirados por el trap del script.

## Cómo probar en el mock

1. Levantar los servicios locales con `bash scripts/dev-env.sh up` (no usar `clean` ni `down --volumes`; el volumen y secretos se conservan).
2. Iniciar sesión con una cuenta sintética con rol administrador y consultar **Baja local de solicitudes de supresión**.
3. Evaluar una solicitud; abrir una incidencia/disputa o mantener una reserva/pago pendiente para ver el bloqueo y código concreto.
4. Para la cuenta elegible, ejecutar la baja y revisar el resultado “baja local con minimización y retención residual”. Si el almacenamiento falla, la fila queda en limpieza pendiente; la misma clave permite reintentar y el worker también recupera trabajo tras reinicio.

El comando de preparación de cuentas sintéticas previamente aceptado en #191 continúa disponible; esta entrega no cambia sus credenciales ni elimina sus fixtures.
