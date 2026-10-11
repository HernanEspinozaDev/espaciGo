# DOCKER-02 — instalación limpia y recorrido sintético

Fecha: 2026-10-11. PR #271 quedó fusionado en `main` como `e074d78`; #268 se aceptó y cerró por DOCKER-01. Esta evidencia corresponde a DOCKER-02/#269. El stack aislado sigue activo para revisión. LOCAL-1 permanece aceptado; DOCKER-03/#270 no se inició.

## Aislamiento e instalación

Se inició el Compose con el helper de DOCKER-01 usando proyecto `espacigo_docker02_20261011`, tags de imagen del mismo prefijo, puertos `18080` (API), `18081` (mock) y `18025` (Mailpit), volumen `espacigo_docker02_pgdata_20261011` y estado privado `.local/docker-isolated/docker02-20261011`. Antes de iniciar, ese volumen no existía. Los secretos fueron generados exclusivamente dentro de su subdirectorio privado con permisos restrictivos. No se copiaron credenciales del stack de desarrollo.

El servicio `migrate` inicializó el esquema desde vacío y terminó correctamente antes de arrancar el Backend. PostgreSQL reporta 54 migraciones aplicadas, versión máxima V54. El usuario administrador pertenece solo a la inicialización/migrador. La comprobación de privilegios efectiva para `espacigo_runtime` fue: lectura de la tabla de usuario permitida; inserción directa en el ledger de migraciones denegada; creación en el esquema público denegada. El Backend y mock usan tags de imagen propios del proyecto aislado.

## Recorrido HTTP y navegador

Con tres identidades sintéticas independientes, verificadas por correo local y almacenadas únicamente en la base aislada, se comprobaron registro/verificación, sesión, roles de anfitrión/arrendatario/administrador, aprobación sintética KYC y los avisos SMTP en Mailpit. Los roles del anfitrión y administrador se provisionaron como datos de fixture para el ensayo; la preferencia de registro no concedió roles.

En el mock, como arrendatario: iniciar sesión → buscar la oferta publicada → abrir detalle → consultar disponibilidad → seleccionar intervalo → cotizar → solicitar reserva → pagar con fake → autorizar la garantía fake → consultar bandeja e historial. Como anfitrión: consultar la solicitud → aprobar → generar contrato sintético. Ambos participantes firmaron; después el arrendatario registró check-in y check-out con evidencia PNG privada sintética. El historial y estado final `finalizada` se vieron en la bandeja. Se comprobó además la recepción del anfitrión, la consulta autorizada del PDF y la descarga/lectura ZIP en un flujo API/PostgreSQL complementario. No se registraron contactos, credenciales, tokens, direcciones ni contenido de mensajes en esta evidencia.

Mailpit aislado contenía 29 mensajes sintéticos al cierre de la comprobación. El conteo no constituye garantía de entrega SMTP exactamente una vez. `verify-http.py` pasó para salud/readiness, CORS y recursos HTML, JS y CSS del mock; su llamada al endpoint de readiness confirmó el acceso API desde el origen del mock.

## Validación ejecutada

- `go test ./cmd/api -run 'TestLoadConfig' -count=1` — pasó.
- `bash -n scripts/local-docker-stack.sh scripts/test-local-docker-01.sh scripts/dev-env.sh` — pasó.
- `bash scripts/test-local-docker-01.sh` — pasó: rechazó rutas de desarrollo, cuenta runtime incompatible y prefijo de imagen de desarrollo antes de crear archivos o invocar Docker; comprobó tags aislados distintos de los defaults de desarrollo y builds desacoplados API/mock.
- `API_ORIGIN=http://127.0.0.1:18080 MOCK_ORIGIN=http://127.0.0.1:18081 python3 scripts/verify-http.py` — pasó.
- `bash scripts/test-local-qa.sh` — pasó: `go test ./...`, `go vet ./...`, suite del mock, 89/89 integraciones PostgreSQL/PostGIS+Mailpit obligatorias (127 eventos Go), cero pruebas omitidas/SKIP y cinco repeticiones de check-in. El runner confirmó retirar sus propios recursos desechables; no retiró este stack.

La llamada de liquidación antes del plazo respondió el conflicto esperado: la política vigente exige que transcurran 24 horas desde el check-out persistido. No se adelantó el reloj ni se alteraron timestamps para el smoke. La liquidación, conciliación, timeout, reinicio e idempotencia están cubiertos por `TestLocalFakeSettlementBoundaryTimeoutRestartAndConcurrentReplay` con reloj sustituible e integración PostgreSQL obligatoria. Esta evidencia separa ese resultado determinista del estado real de la reserva del mock, que queda `finalizada` con liquidación pendiente hasta su vencimiento natural.

## Protección del entorno de desarrollo

Se verificaron por separado los contenedores de desarrollo y DOCKER-02. `espacigo_pgdata`, datos sintéticos y secretos locales de desarrollo no fueron reiniciados, borrados ni reemplazados. El volumen aislado, sus tres cuentas, sus reservas, archivos y secretos se conservan para la revisión. No se ejecutó `down`, limpieza de volumen, restauración, purga ni GCP. El archivo ZIP sintético inspeccionado está fuera del repositorio bajo el estado privado ignorado por Git.

La actualización del helper añade tags distintos por proyecto porque Docker comparte el almacén de imágenes entre proyectos Compose. Reusar `espacigo-local-*:dev` habría permitido que una construcción aislada cambiara el tag utilizado por desarrollo. La prueba ahora garantiza que los dos proyectos resuelvan tags diferentes.

## Repetición segura

La instalación verificada se mantiene en ejecución para revisión. Para reproducir desde vacío, usa valores nuevos y libres; no reutilices los identificadores ni elimines los datos actuales:

```sh
export LOCAL_DOCKER_PROJECT_NAME=espacigo_docker02_reemplazar
export LOCAL_DOCKER_IMAGE_PREFIX="$LOCAL_DOCKER_PROJECT_NAME"
export LOCAL_DOCKER_STATE_DIR="$PWD/.local/docker-isolated/$LOCAL_DOCKER_PROJECT_NAME"
export LOCAL_DOCKER_PGDATA_VOLUME="${LOCAL_DOCKER_PROJECT_NAME}_pgdata"
export LOCAL_DOCKER_API_PORT=18180
export LOCAL_DOCKER_MOCK_PORT=18181
export LOCAL_DOCKER_MAILPIT_PORT=18125
scripts/local-docker-stack.sh config
scripts/local-docker-stack.sh up
API_ORIGIN=http://127.0.0.1:18180 MOCK_ORIGIN=http://127.0.0.1:18181 python3 scripts/verify-http.py
```

Los puertos deben estar libres. `config`/`up` crean su propio estado y credenciales; `up` aplicado por segunda vez al mismo proyecto conserva la base, no repite una instalación limpia. `down` detiene ese proyecto, pero conserva su volumen y secretos.

## Estado

#269 queda presentada para revisión mediante el PR de DOCKER-02. Esta entrega no se acepta ni se cierra automáticamente. DOCKER-03/#270 sigue bloqueado hasta la aceptación de #269. No se hicieron pruebas ni despliegues GCP.
