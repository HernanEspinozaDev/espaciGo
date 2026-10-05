# Prototipo local M01 — registro, verificación y sesión

Entrega en PR [#124](https://github.com/HernanEspinozaDev/espaciGo/pull/124), misma rama `codex/auth-be-02-registration-session`. Conserva AUTH-BE-02 y añade integración AUTH-API-01 (#32), pruebas del corte (#34) y mock del corte (#35). Hito [M01](https://github.com/HernanEspinozaDev/espaciGo/milestone/1). La secuencia nativa de #34 conserva #24 como base satisfecha y retira #32 (implementada en la misma rama) y #33 (recuperación del siguiente corte); #35 conserva #25 como base satisfecha y deja de bloquearse por #34, cuyo flujo parcial ya está integrado y probado. Las referencias/dependencias y aceptación originales siguen visibles en cada Issue. No completa recuperación/cambio de credenciales ni toda la aceptación de M01.

## Levantar y recorrer

Requisitos: Docker con Compose, Python 3 para generar los secretos locales y puertos 8080, 8081 y 8025 disponibles. Desde la raíz del checkout del PR:

```sh
scripts/dev-env.sh up
```

El comando construye backend y TypeScript, inicia PostgreSQL, aplica las migraciones existentes con un proceso administrador separado y espera salud del backend/mock. La API utiliza exclusivamente el rol runtime; no modifica DDL. Las credenciales sintéticas se generan en `.local/secrets` (ignoradas, permisos 0600). No copiar esos archivos ni mensajes del buzón a evidencias.

1. Abre [mock](http://127.0.0.1:8081). Debe mostrar «API y PostgreSQL listos».
2. Registra un correo sintético único, por ejemplo `prueba-001@ejemplo.invalid`, y una contraseña como `Synthetic#123`. Acepta únicamente el fixture de términos señalado como prueba. El registro concede solo `arrendatario`.
3. Puedes intentar login antes de verificar: HTTP 403. Abre el [buzón local](http://127.0.0.1:8025), mensaje de verificación. Copia `Token ID` y `Token` en el formulario y verifica. No hay proveedor ni entrega a Internet. Reenvío invalida el token anterior; usa el mensaje más reciente.
4. Inicia sesión con el mismo correo/contraseña. Pulsa «Consultar sesión»: muestra tu ID y roles desde la API. El token está solo en memoria, no en localStorage/cookies ni salida visual; recargar pierde la credencial local.
5. Pulsa «Cerrar sesión». La API revoca la sesión. Otra consulta solicita iniciar sesión; la prueba automatizada también comprueba el rechazo HTTP 401 al reutilizar la credencial revocada.

API: `http://127.0.0.1:8080/api/v1/auth/{terms,register,verification,verification/reissue,login,session,logout}`. Contrato servido en [OpenAPI](http://127.0.0.1:8080/openapi.yaml), fuente `planning/openapi.yaml`. POST usa JSON; sesión/logout llevan `Authorization: Bearer …`, nunca tokens en URL. El mock tiene contenedor separado y usa exclusivamente DOM/fetch.

## Validación reproducible del corte

```sh
python3 -m venv .local/m01-tests
.local/m01-tests/bin/pip install -r scripts/requirements-m01.txt
M01_PYTHON=.local/m01-tests/bin/python scripts/dev-env.sh verify-m01
.local/m01-tests/bin/python -m openapi_spec_validator planning/openapi.yaml
```

`verify-m01` crea una cuenta sintética nueva, valida respuestas contra OpenAPI, comprueba correo SMTP real en Mailpit, reemisión, tokens inválidos/replay, credenciales/roles, CORS y revocación. No imprime credenciales y compara los tokens reales con logs de contenedores en memoria. Evita correrlo repetidamente en una hora: el límite local por IP es 20 operaciones de verificación/hora; se configura antes de levantar con `LOCAL_VERIFICATION_IP_LIMIT` (1–1000). Las cuotas persistentes por cuenta son 3 emisiones/h y 5 fallos por token. La suite Go requiere `TEST_DATABASE_URL` apuntando a PostgreSQL 18/PostGIS desechable; sin esa variable omite integración. Evidencias ejecutadas: [m01-prototipo-local.md](evidence/m01-prototipo-local.md).

## Límites y siguiente entrega

Bcrypt costo 12, tokens criptográficos de 256 bits almacenados como SHA-256, transacciones, bloqueo de login, verificación de estado/roles en cada autorización, idle 30 min y absoluto 8 h permanecen activos. Consultar sesión es polling y no prolonga la actividad. Orígenes CORS explícitos, JSON limitado a 16 KiB, campos desconocidos rechazados, errores sin secretos. La IP se toma de la conexión directa; se ignoran cabeceras reenviadas. Docker puede agrupar clientes bajo una IP: este mecanismo atómico en memoria es solo para una instancia local, se reinicia con el proceso. La cuota compartida y la política productiva, proxy/TLS, correo con entrega/reintento durable, CI y frontend definitivo quedan pendientes; no se acredita infraestructura productiva.

Los términos son fixtures sintéticos publicados existentes; no son un texto contractual productivo. Mailpit conserva hasta 100 mensajes en tmpfs (pierde el buzón al recrearse). PostgreSQL conserva datos en volumen local. Se usa HTTP exclusivamente en puertos loopback. No añadir datos personales reales.

Recuperación/cambio de credenciales (#31/#33, CU-05/PT-50 y aceptación restante de #34/#35) es el siguiente corte funcional. Preferencia de uso/historial/eventos de DB02-09 siguen pendientes. Limpieza del servidor: [#123](https://github.com/HernanEspinozaDev/espaciGo/issues/123), seguimiento independiente, sin acceso ni limpieza remota en esta entrega.

```sh
scripts/dev-env.sh down   # detiene; conserva datos y secretos locales
scripts/dev-env.sh up     # vuelve a levantar; migraciones aplicadas no se repiten
# Solo cuando quieras destruir TODOS los datos sintéticos del prototipo local:
scripts/dev-env.sh clean
```

Para la revisión se deja el prototipo del PC funcionando. El PostgreSQL desechable y los recursos temporales de validación se eliminan por separado; esa limpieza no corresponde al servidor pendiente.
