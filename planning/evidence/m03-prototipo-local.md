# Evidencia — recorrido local M03 sintético

## Alcance que se puede revisar

La entrega incluye migraciones incrementales V3/V4, contrato de flujo KYC/KYB sintético, repositorio PostgreSQL generado con sqlc, API HTTP/JSON, permisos de titular/administrador, clave idempotente, resolución con motivo obligatorio al rechazar, reintento y mock HTML/CSS/TypeScript. El caso fixture no acredita identidad ni concede permisos comerciales. No se reciben RUT o documentos reales; no hay consulta a Registro Civil/SII ni correo de resultado.

## Validación ejecutada

- `go generate ./internal/adapters/postgres/verification` con sqlc v1.31.1; hashes antes/después de la segunda generación idénticos para ambos paquetes `dbgen`.
- `go test ./... -count=1` con `TEST_DATABASE_URL` conectado a PostgreSQL/PostGIS 18 desechable. Ejecutó migraciones desde vacío, pruebas de persistencia KYC (propiedad, idempotencia, cola, revisión, rechazo, reintento) e integración previa. El test no apuntó a `espacigo_pgdata`.
- E2E en un Compose temporal aislado: registro y confirmación por Mailpit, sesión real, alta/listado/consulta de caso, idempotencia/replay/conflicto, 403 de revisión para usuario, rechazo sin motivo (422), rechazo con motivo, reintento del titular, aprobación por administrador y 404 de lectura ajena. El mock sirvió HTML, JavaScript compilado y CSS; contraseña y tokens de prueba no aparecieron en los logs.
- `go vet ./...`, `git diff --check`, `npm run build`, validación YAML/OpenAPI, referencias locales y esquemas JSON pasaron.
- Se retiraron solo los contenedores, redes, override y volumen desechable `espacigo-m03-e2e_pgdata` después del E2E. `espacigo_pgdata` y los hashes de los secretos locales se conservaron.
- En el entorno persistente ya están aplicadas V1–V4. V3 conserva el mismo checksum con que se aplicó; la base y servicios saludables siguen preservados.

## Pendientes por decisión o integración

Verificación oficial de cédula/RUT y SII; carga segura, MIME/tamaño, almacenamiento privado, selfie/biometría, notificación de resultado, retención/borrado de evidencia y consumo del estado aprobado para habilitar publicación/reserva. Permanecen fuera del corte los pendientes M02 #37–43, DB02-09 y la limpieza remota #123. No declarar completo M02 ni M03 por esta evidencia.
