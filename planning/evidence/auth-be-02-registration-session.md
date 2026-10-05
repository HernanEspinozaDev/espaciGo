# AUTH-BE-02 — registro, verificación y sesión

Issue operativa [#30](https://github.com/HernanEspinozaDev/espaciGo/issues/30). Base `f1da79a13e73274f8a0b9e987aa96705e05758ac` de `main`: PR #16 aprobado por HernanEspinozaDev y fusionado el 2026-10-05 01:09:47 America/Santiago. #29/#120 se cerraron como completed y se confirmaron Hecho en Projects por aceptación expresa; #31 quedó habilitada para su alcance independiente y no se implementó. No se reimportó la migración ni se modificaron estados en documentos históricos.

## Implementación y trazabilidad

El servicio `internal/identity/auth_service.go` ofrece `Register`, `ReissueVerification`, `VerifyEmail`, `Login`, `Authorize` y `Logout`, mediante puertos sin HTTP/GCP ni proveedores concretos. El adaptador PostgreSQL utiliza sqlc v1.31.1 y transacciones explícitas; no añade ni modifica DDL. El constructor requiere repositorio, hasher, correo, política IP, generador de credenciales y reloj; no incorpora bypass ni valores productivos por defecto.

| Criterio | Implementación / evidencia |
| --- | --- |
| RQF-001–007, CU-01 | Formato del correo antes de canonicalización M01 compartida; conserva original y Unicode casefold, puntos y +alias. Contraseña de 8 caracteres, mayúscula, número y especial. Duplicado explícito: iniciar sesión o recuperar, conforme CU-01 A1. |
| RQF-186–187, términos | Registro exige al menos una versión publicada de tipo términos, rechaza lista vacía/duplicada y versiones futuras; cuenta, arrendatario y aceptaciones versionadas se guardan atómicamente mediante el repositorio ya aceptado. Ningún parámetro permite conceder administrador. |
| RNF-013 | Adaptador bcrypt, costo 12, sal propia de bcrypt; sin contraseñas en claro. Dependencia fijada `golang.org/x/crypto v0.43.0`; resuelve `x/text v0.30.0`, con regresión de canonicalización. Límite técnico explícito de 72 bytes, sin truncar contraseñas. |
| RQF-008–010/188, CU-02 | Token aleatorio, solo SHA-256 persistido, TTL 24 h, reemisión invalida activos anteriores, consumo+activación atómicos, rechazo de expirados/reemplazados/reutilizados. ID público y secreto se entregan separados al puerto de correo; no se construyen URLs con secretos. |
| Límites ratificados M01 | 5 fallos por token; 3 emisiones/reemisiones por cuenta/propósito en ventana móvil de 1 h, incluyendo filas invalidadas/entregas fallidas. Bloqueo de cuenta serializa conteo+reemisión ante concurrencia. Política IP obligatoria tanto en reemisión como verificación; no modifica contador/bloqueo de login. |
| RQF-011–018, CU-03/04 | Credencial incorrecta/usuario inexistente devuelve error genérico. 5 fallos consecutivos bloquean 30 min y solicitan alerta por correo. El siguiente login al vencer el plazo libera y reinicia el contador; una cuenta no verificada sigue no verificada. Login exitoso reinicia fallos y crea sesión dentro de la misma transacción. |
| RNF-016, sesión/autorización | Reloj del backend, límites exclusivos idle 30 min y absoluto 8 h, rechazo antes de creación, roles/estado consultados en cada autorización. Solo `UserOperation` autorizada actualiza actividad. Health, polling, keepalive y renovación de tokens no la actualizan; tampoco una denegación de rol. |
| RQF-023, CU-06 | Logout revoca la sesión, es idempotente para la misma sesión y el replay queda rechazado. |
| Concurrencia/rollback | Lock de fila de cuenta PostgreSQL compartido por login, verificación, reemisión y autorización; un único ganador al verificar, sin pérdida de fallos de login ni más de tres emisiones/h. Errores de activación no consumen token; errores de inserción de sesión no reinician contador. |
| Secretos | `Secret` redacta formato/JSON de inputs y resultados; tokens/hashes no se incluyen en logs. Tokens de sesión/verificación concretos tienen 256 bits de azar criptográfico y IDs UUID v4. |

Registro y envío son operaciones distintas: después del commit de cuenta/términos se emite el token y se invoca el correo. Si emisión/límite/entrega falla, `Register` devuelve el ID de la cuenta pendiente y el error; el consumidor debe ofrecer reemisión, sin repetir el alta. Un fallo de entrega no se oculta ni revierte la cuenta. Verificar el correo durante un bloqueo de login conserva el contador y su vencimiento: la cuenta queda verificada pero bloqueada hasta los 30 minutos, sin habilitar acceso anticipado. La alerta de bloqueo se solicita después de commit; un fallo retorna `ErrDelivery` junto al error de bloqueo, que permanece persistido. No hay una promesa de cola durable ni reintento automático en memoria.

## Pruebas ejecutadas — PC, 2026-10-05

Transcript completo y hashes de sqlc: [auth-be-02-pc-20261005.log](auth-be-02-pc-20261005.log). Fixtures sintéticos; correo, hasher de los tests de flujo y política IP usan dobles explícitos. El bcrypt real se prueba por separado con costo 12; no se afirma que los dobles representen entrega real ni capacidad de un proveedor.

- `go test ./internal/adapters/postgres/identity -run '^TestAuth' -count=1 -v`: PASS, 11 pruebas principales y 11 subcasos, cero omitidos.
- `go test -race -count=1 -v ./...`, con `TEST_DATABASE_URL` del PostgreSQL desechable y `DATABASE_URL` retirado: PASS, 70 pruebas principales y 11 subcasos, cero SKIP/FAIL. Incluye las pruebas nuevas, adaptador, migrador y regresiones existentes.
- `go vet ./...`: PASS, exit 0.
- `git diff --check`: PASS, exit 0.
- Las consultas nuevas cambiaron el SQL: se regeneró con la versión fijada y se compararon los SHA-256 de los cuatro archivos generados antes/después de otra generación; `cmp` exit 0, idénticos.

Se usó el mismo perfil/digest PostgreSQL 18 previamente validado (`postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677`). No se repitieron probes de versiones/extensiones, aislamiento del mock ni migración de GitHub. Aplicar el esquema a cada base vacía forma parte del aislamiento necesario de los tests nuevos; no se alteró ni usó una base persistente. Datos en tmpfs, red none, sin puertos publicados y socket temporal con directorio padre 0700. Contenedor, imagen descargada y directorio/socket se retiraron; las consultas previas y comprobaciones de ausencia se conservan en el transcript. Dependencias de Go quedan en su caché normal.

## Pendientes y límites para revisión

- HTTP/OpenAPI, wiring del proceso, mock/UI, recuperación y cambio de clave están fuera de #30. No se ejecutaron navegador, build TypeScript, contrato OpenAPI ni proveedor real. No se acredita TLS, correo real ni CI con las pruebas locales.
- La política/umbral productivo por IP no fue ratificada: el puerto es obligatorio, está aplicado y probado con una cuota sintética. Antes de conectar el transporte real se debe aportar un adaptador con cuota atómica compartida y la política aprobada; no usar el fake de pruebas ni una cuota en memoria para afirmar protección entre instancias. El transporte debe aportar la IP confiable, clasificar actividad desde rutas controladas y transportar los secretos fuera de URLs conforme RNF-015.
- El adaptador real de correo y cualquier entrega/reintento durable corresponden a su integración: el puerto de #30 recibe solicitudes de verificación/alerta y hace visibles sus fallos, sin fabricar una entrega al proveedor. Una reemisión concurrente puede entregar un mensaje anterior después del más reciente; ese token anterior está invalidado por diseño.
- DB02-09 sigue abierto: sin preferencia RQF-213, historial RQF-217 ni evento/notificación RQF-218 añadidos por inferencia. No se implementó #31.
- Limpieza del servidor: seguimiento independiente [#123](https://github.com/HernanEspinozaDev/espaciGo/issues/123), OPEN, sin acceso remoto ni afirmación de limpieza. Los recursos retirados por esta entrega son exclusivamente del PC.
- La entrega de #30 queda para revisión y merge por HernanEspinozaDev. No se marca Hecho ni se habilita auto-merge antes de aceptación.
