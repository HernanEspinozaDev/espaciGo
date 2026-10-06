# Evidencia — M09-LOCAL-02 unread por participante

Issue #158, trazable a #95–#98 y CU-37/CU-38/HU34. La subentrega no completa esas Issues generales.

## Validación técnica

- `bash scripts/test-m06-local-booking-postgres.sh`: integración PostgreSQL desechable pasó como `espacigo_runtime` con V17 y los permisos mínimos de cursor. Comprueba conteo independiente por participante, exclusión de mensajes propios, terminales con contador, permisos de outsider, marca repetida y concurrente con solicitudes fuera de orden, monotonicidad, cursor persistido al reconstruir el servicio y contadores actualizados.
- `npm --prefix mock run test:profile-races`: 13/13 pasó. Las pruebas de lectura cubren máximo de secuencia mostrado, orden display→mark, fallo de carga, selección obsoleta, mensajes que llegan después de la página y hilo vacío.
- `go test ./...`, `go vet ./...` y `git diff --check`: pasaron.
- `bash scripts/dev-env.sh up`: migración V17 aplicada incrementalmente; Backend y mock saludables.

## Recorrido con ambas cuentas

Sobre mensajes ya existentes de ensayo, la bandeja mostró 1 mensaje sin leer a la arrendataria y 2 al anfitrión: cada contador omitió los mensajes propios y usó su propio cursor. Abrir el hilo quitó el contador de esa persona sin presentar confirmación a la contraparte. El anfitrión volvió a iniciar sesión y el contador quedó en cero. No se enviaron mensajes nuevos durante esta comprobación.

## Persistencia local

La consulta de verificación después de V17 mostró versión máxima 17, 3 mensajes y 2 cursores de lectura. `espacigo_pgdata` conserva su fecha de creación (`2026-10-05T01:49:09-03:00`). No se borraron mensajes, volumen ni secretos ni se ejecutó el script de limpieza.
