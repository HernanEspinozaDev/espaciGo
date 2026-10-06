# Evidencia local de reserva M06 — 2026-10-06

Esta evidencia corresponde al PR #149 fusionado en `main` (`7718eb5fa758dda0274d787de2648f8af3398898`) y a la corrección de consulta explícita del historial que se publica en el PR de seguimiento. No es aceptación final de #148 ni de M05/M06.

## Recorrido comprobado

Entorno local con el fixture sintético autorizado de una sola pareja; el fixture habilita un borrador y dos cuentas explícitas, sin conceder permisos comerciales ni exponer otros borradores:

- Anfitrión: `espacigo-m06-host-20261006@example.test`.
- Arrendataria: `espacigo-m06-renter-20261006@example.test`.
- Versión de esquema observada: V14. Se preservó el volumen persistente `espacigo_pgdata`, sus datos y secretos; no se reinició ni borró.
- Se realizaron cotización, solicitud, pago fake exitoso con la cuenta arrendataria, aprobación del anfitrión y consulta autenticada de detalle/historial desde el mock.
- Reserva aprobada: `25b35be3-0cfb-4f01-b240-c320af661472`; secuencia exacta `pendiente_de_pago → pagada → aprobada_host`.
- Solicitud distinta con pago fake rechazado: `5e35675c-52f7-47ea-9714-c0b86bf7f68f`; `pendiente_de_pago → cancelada_por_pago`; la retención quedó liberada.
- Solicitud distinta cancelada por la arrendataria antes del pago: `cc690909-8f6a-4d9f-961d-fd565fec187d`; `pendiente_de_pago → cancelada_arrendatario`; la retención quedó liberada.
- Verificación de lectura en PostgreSQL confirmó las tres reservas y sus secuencias de historial; únicamente la reserva aprobada conserva ocupación activa.
- El mock ahora distingue la operación de listar reservas de la consulta por ID de detalle e historial. El GET de detalle usa la sesión autenticada y presenta las transiciones retornadas por API.

## Validación ejecutada

- `npm --prefix mock run build` — pasó.
- `npm --prefix mock run test:profile-races` — pasó, 2 pruebas.
- `go test ./...` — pasó. Los paquetes sin pruebas están identificados por Go; esta ejecución no sustituye la integración PostgreSQL aislada previa del PR #149.
- `go vet ./...` — pasó.
- `git diff --check` — pasó.
- Recorrido interactivo de mock/API ejecutado contra el entorno local persistente descrito arriba. La integración PostgreSQL con el rol runtime para transacciones fue ejecutada como parte de la entrega #149; no se repitió en este seguimiento, que solo añade el control mock de consulta por ID.

## Estado operativo

#148 se mantiene `En revisión` y abierta hasta integrar la consulta del historial desde el mock y recibir aceptación humana. Permanecen abiertas las Issues generales, DB02-09 y la limpieza del servidor #123. No se declara completa ninguna otra entrega.
