# M01: evidencia del segundo corte funcional — 2026-10-05

Base: `main` en `be5ac934` (PR #124 fusionado); rama `codex/auth-be-03-recovery-credentials`. Validación sobre cambios integrados en la misma rama, sin declarar las Issues completadas.

## Resultado ejecutado

- `go test -p 1 -race -count=1 -v ./...` con `TEST_DATABASE_URL` hacia PostgreSQL 18/PostGIS desechable: **pasa**. Incluye `TestAuthRecoveryAndPasswordChangeRevokeSessions` (hash/TTL, reemisión, cuota de 3, nuevo login y revocación de sesiones) y `TestAuthRecoveryTokenStopsAfterFiveFailuresWithoutBlockingLogin`, además de las integraciones existentes. Transcript: [m01-credenciales-go-suite-20261005.log](m01-credenciales-go-suite-20261005.log).
- `go vet ./...`: **pasa**.
- `git diff --check`: **pasa**.
- `npm run build --prefix mock`: **pasa** con TypeScript 5.9.3 instalado desde la versión declarada en `mock/package.json`.
- `python -m openapi_spec_validator planning/openapi.yaml`: **pasa** con openapi-spec-validator 0.7.2.
- `M01_PYTHON=/tmp/espacigo-m01-contract-venv/bin/python scripts/dev-env.sh verify-m01`: **pasa** contra el stack Docker local. Resultado: YAML/referencias/esquemas OpenAPI; registro, correo de verificación capturado, reemisión y rechazo de replay; recuperación igual para correo existente/desconocido, token incorrecto/replay y nueva clave; cambio autenticado con rechazo de clave actual errónea/idéntica, revocación de sesión y login posterior; CORS, mock HTML/JS/CSS y ausencia de credenciales/token en logs.
- `scripts/dev-env.sh verify-http`: **pasa** en liveness/readiness, CORS/preflight y entrega de HTML/configuración/JS compilado/CSS por el mock.
- `scripts/dev-env.sh up`: **pasa**; backend y mock construidos desde esta rama, migrador aplicado sin cambios de esquema. PostgreSQL del prototipo conserva su volumen local y servicios permanecen levantados para revisión.
- Inspección accesible del mock servido en `http://127.0.0.1:8081`: estado «API y PostgreSQL listos»; formularios visibles para solicitar/consumir recuperación y cambiar contraseña.

La validación desechable de Go se ejecutó con `bash /tmp/espacigo-final-tests.sh`, que crea `espacigo-m01-validation` usando la imagen fijada PostGIS, exporta su puerto solo a loopback, ejecuta tests race + vet + diff-check y tiene trap de eliminación. El resultado final incluyó `PASS disposable PC PostgreSQL container and socket removed`; este es únicamente el recurso temporal de este PC. No afirma limpiar ningún recurso del servidor: ese seguimiento sigue abierto en #123.

## Límites registrados

- RQF-217: no se guarda historial de contraseñas; DB02-09 aún debe decidir finalidad/retención/minimización y autorizar el modelo.
- RQF-218: aviso por cambio/reset se envía tras commit a Mailpit en entorno local, sin outbox ni entrega durable. Una falla de SMTP puede ocurrir después de actualizar la clave; el endpoint la informa y el mock limpia el token de sesión.
- Cuota por IP es memoria local/volátil; no acredita límite distribuido de producción. Sin TLS/correo productivo ni frontend definitivo.
- Issue #123 (limpieza del servidor) permanece separada y pendiente.
