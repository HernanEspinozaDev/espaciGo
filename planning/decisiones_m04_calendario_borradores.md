# Decisión de implementación: calendario privado para borradores propios

Estado: alcance autorizado para el corte local M04/M06-DRAFT-CALENDAR-01, Issue #144, subentrega de LIST-BE-02 (#56). No completa #56 ni M04/M06.

## Alcance

- Un titular configura y consulta una zona IANA explícita por borrador privado. No se asigna una zona por defecto: borradores históricos sin zona requieren configurarla antes de consultar disponibilidad o crear/listar bloqueos.
- El endpoint de disponibilidad recibe instantes RFC 3339 normalizados a UTC y comprueba el rango semiabierto `[inicio, fin)` contra todas las ocupaciones activas. El mock interpreta sus entradas `datetime-local` en la zona configurada y envía UTC.
- Bloqueos manuales requieren motivo y rango finito con inicio anterior al término. La exclusión GiST impide solapes activos concurrentes; la adyacencia se acepta. Eliminar un bloqueo es una desactivación lógica con marca temporal para conservar trazabilidad.
- Todas las lecturas y escrituras se filtran por titular, ID de espacio y estado borrador en PostgreSQL. Recursos ajenos e inexistentes responden 404.

## Ownership y evolución

`ocupacion` pertenece a M06 según MAP-01 y ES2. V9 crea la tabla M06 para este corte y el flujo M04 llama al servicio de calendario M06; no existe tabla administrativa de calendario en M04. Solo se insertan filas `bloqueo_manual` con `reserva_id` nulo. Cuando se habilite el agregado reserva, su migración añadirá la FK compuesta y las transiciones de reserva/retención según el contrato completo. La restricción de exclusión ya cubre cualquier fila activa futura.

La configuración `zona_horaria` se añade incrementalmente a `espacio` y admite NULL para datos existentes; el backend valida nombres con la base IANA instalada. La consulta no interpreta meses/días ni calcula tarifas; no hay horario semanal, recurrencia HU24, reservas, publicación o cobros.

## Dependencias y límites

Se reutilizan los contratos M01 de sesión y borradores M04 fusionados en #128. #43 queda abierta: implementa perfil/solicitudes M02 y no es necesaria para autorizar un borrador por su titular autenticado. #142 también queda abierta y separada: define documentos reales, proveedor y retención KYC; no se consulta KYC ni se otorgan permisos comerciales en este flujo privado. #52 y #56 conservan sus criterios y dependencias más amplias. #70/#71/#72 siguen pendientes para diseño/migración/operación completos de reservas, snapshots, expiración y concurrencia del ciclo de reserva; el slice de bloqueo manual no los completa ni los cierra. DB02-09 y #123 continúan independientes.

## Evidencia esperada

Pruebas de dominio para zona IANA e intervalos, PostgreSQL desechable para ownership/adyacencia/solape/desactivación, pruebas HTTP para autenticación/JSON estricto/404, OpenAPI actualizado y mock con `fetch`. El volumen de desarrollo y secretos no se usan como base de prueba ni se reinician.
