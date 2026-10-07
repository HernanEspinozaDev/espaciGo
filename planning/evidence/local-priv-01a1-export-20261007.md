# LOCAL-PRIV-01A1 — Evidencia de exportación de identidad

Fecha: 2026-10-07. Subentrega bajo #185/#40; #39/#40/#43 permanecen abiertas.

## Comprobaciones ejecutadas

- `bash scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/identity`: suite de integración del paquete en PostgreSQL efímero PostGIS desechable. Pasaron todas las pruebas de integración reportadas por `go test`; no se repitió la suite completa del repositorio. La prueba `TestM02ProfileAndRightsQueriesRemainOwnerScoped` cubre dos cuentas, la exportación del titular, aislamiento de perfil/solicitudes, cuenta inexistente y ausencia de hash/campos de credenciales.
- La misma ejecución incluyó `TestM02IdentityExportUsesAuthenticatedAccountAndExcludesCredentials`: dos cuentas autenticadas reales en el harness PostgreSQL del Backend reciben cada una su correo propio; la respuesta HTTP `GET /api/v1/privacy/export` no incluye identificadores de credenciales.
- `cd mock && npm run build`: compilación TypeScript exitosa.
- Parseo de `planning/openapi.yaml` con PyYAML: correcto; `/privacy/export` presente.
- `go test ./internal/privacy ./internal/identity/transport/http`: pasó (validación enfocada de servicio/handler). Las integraciones que usan `TEST_DATABASE_URL` se comprobaron en el comando desechable anterior, no contra `espacigo_pgdata`.

## Límite aceptado

La exportación cubre exclusivamente identidad/perfil/roles/aceptaciones/solicitudes modelados. No afirma exportación integral del producto ni resuelve supresión. No modifica solicitudes existentes, volumen ni secretos. No se probó un recorrido visual manual de navegador; el control mock se validó por compilación y el contrato/endpoint por integración HTTP.
