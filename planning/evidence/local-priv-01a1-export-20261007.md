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

## Comprobación adicional solicitada en PR #187

- `cd mock && npm run build && node --test test/privacy-export-state.test.mjs`: compilación y ambas pruebas enfocadas pasaron. La primera cubre una respuesta tardía después de cerrar sesión y volver a entrar con la misma cuenta (incluida la defensa ante generación distinta); la segunda cubre el descarte de un error tardío.
- Se reconstruyó únicamente `mock-frontend` con `scripts/dev-env.sh up -d --no-deps --build --wait mock-frontend`; quedó saludable sin reiniciar PostgreSQL.
- En el navegador se inició sesión con una cuenta sintética verificada, se preparó la exportación y se activó el enlace `espacigo-datos-propios.json`. El JSON presentado por el mock se pudo leer y parsear visualmente: el correo coincide con esa cuenta y el documento contiene solamente `account`, `roles`, `terms_acceptances`, `rights_requests` y `scope`; no incluye claves de credenciales, hashes ni tokens.
- Se mantuvo la URL Blob durante 60 segundos tras activar el enlace para que el navegador tenga tiempo de terminar la descarga; se revoca al cambiar de sesión o al expirar ese plazo.
- La automatización del navegador mostró el resultado de descarga y el documento JSON, pero su interfaz no devolvió una ruta de archivo ni notificó el evento Playwright `download` para el enlace Blob. Por ello la inspección cubre el JSON exacto entregado al enlace, pero no registra una ruta local del archivo guardado.
