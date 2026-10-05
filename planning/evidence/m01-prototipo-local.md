# Evidencia M01: prototipo local funcional

Fecha de ejecución: 2026-10-05 (America/Santiago). Rama `codex/auth-be-02-registration-session`, PR #124. Desarrollo y publicación: HernanMEC. Datos de prueba sintéticos; no se usaron cuentas/correos personales. La API escribe solo logs operativos genéricos.

## Alcance comprobado

- El stack reproducible `scripts/dev-env.sh up` levanta PostgreSQL/PostGIS del Compose, proceso de migración separado, API de backend y mock separado. Health de API y mock `healthy`; el proceso migrador completó migraciones y grants runtime. La API usa el rol runtime y atiende solo loopback.
- Contrato OpenAPI 3.1 completo: validado por `openapi-spec-validator 0.7.2`; YAML, referencias locales y JSON Schemas también comprobados por el flujo E2E.
- `scripts/dev-env.sh verify-m01` recorrió endpoints HTTP reales contra la base Compose y capturó SMTP en Mailpit: fixture de términos, campos desconocidos, política de contraseña, Bearer requerido, registro, duplicado canonicalizado, denegación de login antes de verificación, reemisión, token reemplazado/inválido/replay, activación, credencial incorrecta, login/rol, consulta de sesión, logout y replay revocado. Revisó CORS permitido/prohibido y disponibilidad de HTML, CSS y TypeScript compilado. Esquemas JSON validados en respuestas con cuerpo; la salida es solo `PASS`/status y no incluye valores de credenciales.
- El flujo comprobó que ningún token ni contraseña usados aparecen en logs de contenedores. Los valores del correo y de Bearer se mantuvieron en memoria durante la prueba.
- Prueba manual por interfaz: alta `browser-m01-20261005@ejemplo.invalid`, lectura en Mailpit, verificación en mock, login, consulta de sesión con rol únicamente `arrendatario`, logout. Estado visible del mock guardado en [m01-prototipo-sesion.png](m01-prototipo-sesion.png); se vaciaron los campos de contraseña y token.

## Suite y checks

- `go test -p 1 -race -count=1 -v ./...`: **72 pruebas principales y 11 subcasos PASS**, sin SKIP ni FAIL. Incluye todos los paquetes, casos PostgreSQL/PostGIS con migración por base temporal y pruebas de migrador. Transcript: [m01-go-suite-20261005.log](m01-go-suite-20261005.log). `-p 1` limita la presión del cluster desechable; los casos concurrentes internos mantienen su concurrencia.
- PostgreSQL 18/PostGIS del digest fijado del proyecto en contenedor desechable, cluster temporal y puerto loopback efímero; `DATABASE_URL` retirado. Contenedor cluster y bases de las pruebas eliminados por el harness. No se usó ni alteró el volumen persistente del prototipo.
- `go vet ./...`: PASS. `git diff --check`: PASS. `npm --prefix mock run build` (TypeScript 5.9.3): PASS. `openapi-spec-validator planning/openapi.yaml`: PASS.
- `scripts/dev-env.sh verify-m01`: PASS en salida completa E2E, guardada en `/tmp/espacigo-m01-e2e.log` durante la ejecución; transcript versionado en este archivo registra la cobertura observable. La comprobación de secretos ejecuta contra logs, sin copiarlos al transcript.

## Límites y trabajo restante

La instancia IP del correo es cuota atómica en memoria para un proceso local, 20 operaciones/h por IP por defecto; no resiste varias instancias ni un proxy sin adaptación explícita. Se ignora `X-Forwarded-For`. Mailpit y sus hasta 100 mensajes usan tmpfs. Los términos y privacidad son fixtures sintéticos ya versionados; no son textos jurídicos productivos. Correo SMTP no sale de la red local. HTTP, credenciales sintéticas y puertos loopback sirven solo para desarrollo: no hay TLS/proveedor productivo, cola durable, adaptador de cuota compartida, frontend definitivo, CI ni afirmación de cumplimiento legal. No usar datos reales.

La aceptación restante de #34/#35 —recuperación/cambio de credenciales (CU-05/PT-50) y dependencias #31/#33— se reserva para el siguiente corte. No se cierran Issues ni se declara terminado todo M01. DB02-09 sigue pendiente. La limpieza del servidor continúa en #123 OPEN y no se ejecutó desde este PC.
