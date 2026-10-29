# F5 — primer corte ejecutable del dominio documental

Fecha: 2026-10-28. Base `main` / `3956d2f3459b`, cambios locales sin commit.

## Código y límites

Nuevo paquete `internal/documentverification` con servicio, snapshots separados identidad/espacio y puerto de creación/transición atómica. No está conectado a `cmd/api`, booking, PostgreSQL, scanner ni UI. No hay carga operativa, expediente persistido, distintivo público ni job de retención. F5 sigue parcial.

El servicio crea únicamente casos vacíos con sujeto derivado de autorización, exige coincidencia actor/sujeto y propietario autoritativo para espacio. No acepta archivos ya limpios ni campos de verificación en CreateInput. Verifica cuotas 3/10, formato JPEG/PNG/PDF, bytes positivos hasta10MiB y PDF5/20 páginas sobre metadata que el adaptador debe obtener de inspección confiable. Eso no demuestra decode/sanitización ni scanner real.

Submit valida sujeto, estado y archivos limpios y cambia borrador→enviada. Review requiere capacidad separada, anti-autoaprobación, snapshot scoped y archivo limpio; usa reloj del servidor y vencimiento documental del snapshot, no fecha de aprobación/vencimiento suministrada por cliente. Verificación de espacio requiere identidad vigente en la autorización transaccional. Revoke exige capacidad/propósito, impide self-review y registra revocación inmediata con reloj servidor.

El puerto exige que creación y transición obtengan autorización y bloqueen/revaliden identidad/caso dentro de la misma transacción, comparen revisión y avancen revisión al persistir. Esa es una obligación del futuro adaptador, NO una garantía PostgreSQL ya demostrada.

## Revisión y pruebas

- Primera revisión `deleg_8800ea3a` falló por Create sin ownership/cuarentena, fechas del caller y scope; se reasignó la corrección.
- El agente entregó tests verdes pero reemplazó la suite anterior y no registró RED por separado. No se acredita TDD de esa entrega.
- El padre inspeccionó y añadió `authorization_parent_test.go`: observó RED real porque Submit no persistía estado y Create aceptaba sujeto ajeno/SpaceID en identidad. Se corrigieron esos defectos; también se rechaza metadata de archivo vacío y se retiró condición muerta.
- `go test -count=1 ./internal/documentverification` y `go vet ./internal/documentverification` ejecutados por el padre: PASS. Tests adicionales cubren archivos no limpios sin avance, falta de capacidad, self-review, identidad no vigente, vigencia máxima24/12 meses y revocación con reloj controlado.
- Revisión independiente posterior asignada en `deleg_0e486608`; resultado aún pendiente al escribir este corte.

## Pendientes antes de sistema documental operativo

Persistencia/DDL revisados y target autorizado; autoridad de sesión/capacidades y locks reales; intake/metadata inspeccionados y scanner fail-closed real; archivos y estados técnicos; reenvío de antecedentes/ciclo completo, auditoría/reason durable, unicidad de solicitud abierta, vencimiento/retención/eliminación; API/OpenAPI/BFF/UI; identidad persistente y revalidación atómica booking; pruebas integradas. No se aplicó DDL ni se inició servicio/DB/scanner/cloud.
