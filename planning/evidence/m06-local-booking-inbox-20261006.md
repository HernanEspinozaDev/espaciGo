# Evidencia M06-LOCAL-02 — bandeja local de reservas propias

Issue #153, PR de la entrega vertical. Se conservan los contratos actuales: `GET /reservations`, `GET /reservations/{id}`, `POST /payment`, `POST /decision` y `POST /cancel`. El listado y detalle ya filtran por participante, las transiciones se validan en PostgreSQL y los plazos se procesan por el mecanismo existente; no se agregó migración ni endpoint/filtro nuevo.

## Implementación

- El mock muestra listas separadas por rol usando el `account_id` de la sesión. Cada tarjeta incluye espacio, precio, intervalo/zona, estado y plazo relevante; seleccionar abre detalle e historial sin copiar IDs.
- Arrendatario: pago fake y cancelación se habilitan solo en `pendiente_de_pago` dentro del plazo actual.
- Anfitrión: las reservas `pagada` se ordenan primero y se presentan como pendientes de decisión; aprobar/rechazar solo se habilita dentro del plazo actual.
- Luego de cada acción se vuelve a consultar lista y detalle. La API informa 409/expiración y el mock mantiene visible el mensaje mientras refresca el estado actual.

## Pruebas

- `bash scripts/test-m06-local-booking-postgres.sh`: pasó en PostgreSQL desechable con `espacigo_runtime`; cubre listado para arrendatario/anfitrión/tercero, lectura por participante, pago/cancelación/decisión por actor, estado permitido y conflicto fuera de estado.
- `npm --prefix mock run test:profile-races`: pasó (8 pruebas), incluidos permisos del estado de bandeja por actor/estado/deadline y estados terminales.
- Recorrido mock con ambas cuentas sintéticas: arrendatario listó, consultó historial, pagó de forma simulada y canceló otra solicitud pendiente; anfitrión encontró la reserva `pagada` como pendiente de decisión, abrió detalle/historial y aprobó. Cada operación actualizó lista y detalle. Se comprobó que el arrendatario no ve reservas bajo rol de anfitrión y el anfitrión no ve reservas bajo rol de arrendatario; un tercero recibe lista vacía en integración.
- Validaciones generales del corte: `go test ./...`, `go vet ./...`, `git diff --check`.

Todas las reservas usadas son datos sintéticos. La reserva aprobada de recorrido permanece como ocupación local conforme a los estados vigentes; la cancelación solo se ejecutó en una solicitud pendiente. El volumen y secretos se conservaron. DB02-09 y #69–#79 siguen abiertos con los criterios generales pendientes.
