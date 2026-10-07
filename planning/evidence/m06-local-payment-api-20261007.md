# Evidencia M06-LOCAL-API-01 — contrato HTTP y pruebas de pago fake

Subentrega de #76/#78 en #173. #74 fue aceptada y cerrada para el alcance fake/durable tras el merge de PR #172. Esta prueba no integra proveedor real ni sandbox.

## Comprobación posterior al merge de #172

- `git switch main && git pull --ff-only origin main` dejó `main` en el merge `d354ff495125aa7ac38d9e4e92f4d0709e5343d5`.
- `scripts/dev-env.sh config` y `scripts/dev-env.sh up -d` aplicaron V21 incrementalmente; el registro de esquema muestra V21 como versión actual. No se ejecutó `clean`, no se borró ni recreó el volumen y no se reemplazaron los secretos locales existentes. Se generó únicamente el secreto HMAC local que faltaba y el script del proyecto preserva credenciales existentes.
- Base, Backend, Mailpit y mock quedaron saludables. `/health/ready` devolvió `{"status":"ready"}` y el mock respondió HTTP 200.
- `bash scripts/test-m06-payment-inbox-postgres.sh` pasó en una base temporal desechable con `espacigo_runtime`; recuperó fake/inbox sin afectar `espacigo_pgdata`.

## Contrato HTTP integrado

La prueba de integración monta el handler real de reservas con el fake y repositorio PostgreSQL runtime:

- Sin sesión, el inicio responde 401 y no inicia el fake.
- Pago fake exitoso devuelve 200 y `ENSAYO LOCAL — SIN COBRO REAL`; repetir la misma clave/contenido no duplica resultado, evento, pago ni ocupación. Cambiar el contenido para la misma clave responde 409; reserva inexistente responde 404; resultado no permitido responde 422; JSON con campos desconocidos responde 400; media type incorrecto responde 415.
- `sin_respuesta` responde 504 con la reserva pendiente. El timeout fake persistido es distinguible de una intención nunca iniciada: repetir la clave vuelve a responder 504 sin iniciar el fake por segunda vez y sin crear un pago.
- El callback HMAC local rechaza firma inválida sin sesión y acepta firma válida sin sesión de usuario. Replay informa `reused`; conciliación materializa un solo evento aplicado y un solo pago fake.

## Alcance y pendientes

OpenAPI ahora declara límites de `Idempotency-Key` y respuestas observadas por el handler. La implementación y las pruebas usan exclusivamente el fake local, autenticación HMAC sintética y PostgreSQL desechable. Proveedor real/sandbox, contrato y firma, credenciales, cobros reales y cumplimiento de notificaciones quedan pendientes en #76/#78. #74 se acepta únicamente para sus criterios fake/durables; no se cierra M06. #79 continúa bloqueada por #77/#78.

## Validación

- `bash scripts/test-m06-payment-inbox-postgres.sh` — pasó, PostgreSQL temporal con rol `espacigo_runtime`.
- `go test ./...`, `go vet ./...`, parseo PyYAML de `planning/openapi.yaml`, `bash -n scripts/test-m06-payment-inbox-postgres.sh` y `git diff --check` — pasaron para este corte.
