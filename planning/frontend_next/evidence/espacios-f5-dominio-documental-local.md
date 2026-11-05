# F5 — primer corte ejecutable del dominio documental

Fecha: 2026-10-28. Base `main` / `3956d2f3459b`, cambios locales sin commit.

> **Corte histórico, no inventario vigente.** Este informe describe el primer dominio in-memory al 2026-10-28. Su afirmación de que no había handler de descarga, pruebas ClamAV ni reglas actuales de grants quedó ampliada/sustituida por `espacios-f5-politica-owner-aprobada.md`, `espacios-f5-documentos-flujo-arquitectura.md`, `espacios-f5-clamav-prueba-local-20261029.md` y `espacios-f5-delta-persistencia-integracion-20261029.md`. No hubo integración PostgreSQL/runtime; F5 continúa parcial.

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

## Revisión independiente posterior — 2026-10-29

La revisión read-only del diff F5 de `internal/documentverification` (`deleg_cc712df0`) no encontró defectos concretos de seguridad o lógica en asignación de reviewer ni descarga owner-only. Confirmó el vínculo actor autenticado→grant→capacidad/scope exactos, denegación de grants duales, validación del propietario actual, caso abierto y archivo limpio antes de descargar. **Límite de evidencia:** el adapter/repositorio que debe imponer autoridad/transacción y el store que debe aislar objetos quedan fuera de ese diff; estas pruebas usan fakes en memoria y no demuestran persistencia, wiring runtime ni autorización del storage. La revisión no cubrió el adapter ClamAV y no cierra F5.

## Revisión independiente ampliada — 2026-10-29

`deleg_facb116a` revisó read-only el diff F5 bajo HEAD `c50a2ef22dd2e3fb3134d0cb8bfe248fcedb7f6d`; **no encontró blocker concreto**. Confirmó en `service.go:262–278, 324–343, 364–385` que las acciones reviewer exigen actor autenticado igual al reviewer asignado, capability/scope exactos, deniegan dual-capability, self-review del owner actual y casos de espacio con owner histórico obsoleto. En descarga, `access_policy.go:8–30`, `download.go:40–71` y `transport/http/owner_download.go:38–95` requieren owner actual, caso abierto, archivo limpio, autenticación, IDs derivados del snapshot autoritativo y respuesta privada/no-store; el store no recibe object key del cliente.

**Límite confirmado:** los contratos del repositorio (`service.go:106–116`, `download.go:10–20`) aún no tienen implementación productiva en estos paths; la revisión no prueba persistencia, consistencia de autorización en PostgreSQL ni ACL del store. La prueba focal Go/vet se reejecutó aparte en el worktree y pasó; no se accedió a DB ni servicios externos.
