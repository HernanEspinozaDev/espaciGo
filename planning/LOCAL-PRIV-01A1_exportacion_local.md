# LOCAL-PRIV-01A1 — Exportación local de identidad

Este corte agrega `GET /api/v1/privacy/export`, autorizado por la sesión y limitado a los datos de identidad actualmente modelados: correo que entregó el titular, estado y preferencia de cuenta, perfil, roles, aceptaciones de términos y solicitudes de derechos propias. PostgreSQL lee una instantánea consistente. La respuesta excluye el correo normalizado, hashes, sesiones, tokens de acción, secretos y datos de otros participantes.

El mock descarga JSON y deja visible el alcance aplicado. Esto no es la exportación integral exigida por M02: espacios, reservas, conversaciones, archivos, pagos y derivados quedan fuera hasta que su inventario/propósito y el ensamblado propietario se integren. El endpoint no cambia el estado de solicitudes.

La decisión local D-PRIV ratificada permite avanzar acceso y recepción de solicitudes, pero la supresión ejecutable permanece pendiente de una matriz por dato para referencias históricas, archivos, copias y derivados, y de integrar todos los bloqueadores de reservas, pagos y disputas que existan. La mera baja de correo/nombre no se presentará como anonimización. No se añade expiración ni borrado automático.

Validación enfocada prevista: `bash scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/identity`, `cd mock && npm run build`, verificación del contrato OpenAPI y prueba HTTP con dos sesiones sintéticas.
