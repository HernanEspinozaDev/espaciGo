# Fase 3 — contrato API, gateway y sesión

- Fecha: 2026-10-24
- Rama: `frontend-next-hermes`
- HEAD base antes de los commits de esta fase: `fdbcf5ee010dfbdeeb13f14175f3b33def68c9f8`
- Commit de implementación publicado: `74f166de60a1acded0d8cb8ed14b3cf07667ccf5`
- Estado: Fase 3 verificada y publicada directamente en `frontend-next-hermes`; Fase 4 no iniciada.

## Implementación

- OpenAPI tipado generado; gateway same-origin allowlisted; configuración de runtime; cliente de errores; sesión Bearer en memoria; serialización JSON exacta; intención/idempotencia; utilidades CLP, tiempo, cursor opaco y liberación de Blob URLs.
- El gateway rechaza operaciones, métodos, query keys y headers fuera de contrato. Se excluyeron los `x-forwarded-*` no confiables que Next agrega, en lugar de reenviarlos al API.
- La protección same-origin compara el `Origin` canónico con el `Host` entrante saneado, no con `request.url.origin`; rechaza host con whitespace, coma, userinfo, slash, backslash, query o fragment y Origin con userinfo/path/query/fragment. Una prueba confirma que el origen coincidente llega al API, un origen divergente recibe 403 `origin_not_allowed` y `x-forwarded-*` no cambia la decisión.
- El gateway normaliza streams de request de cero bytes como body ausente. Así permite el logout bodyless del contrato sin relajar límites para cuerpos reales; la prueba anónima recibe el 401 real de Go, no un 415 local.
- El wrapper del workspace pasó de `<section>` a `<div>` para que el header preserve el landmark banner; no se suprimieron reglas AXE.
- El login consume la fachada de autenticación; no se añadió almacenamiento persistente de credenciales o tokens. Se incorporó el código `unauthenticated` al mapeo seguro de errores y una prueba unitaria.
- `api:generate` produce 179 operaciones: 50 directas, 100 por familia/prefijo y 29 no verificadas; 0 clasificadas como sin montaje. “Familia/prefijo” no confirma el path y método exactos. No se presenta el total como 179 rutas confirmadas.
- La condición `authPrototypeEnabled` para las 179 operaciones es correcta: las registraciones del montaje revisado están dentro de `if prototypeEnabled` en `cmd/api/main.go`. La condición no altera el desglose anterior.

## Pruebas locales

- `pnpm install --frozen-lockfile`: pasó.
- `pnpm api:generate` y `pnpm api:check`: pasaron; la matriz se formatea con Prettier dentro del generador y el check confirma el resultado reproducible.
- `pnpm install --frozen-lockfile`, `pnpm api:generate`, `pnpm api:check`, `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm format:check`, `pnpm build` y `pnpm check`: pasaron. `pnpm check` ejecutó la suite E2E local por defecto; las pruebas nativas se habilitaron aparte.
- `PHASE3_NATIVE_API_E2E=1 PLAYWRIGHT_PORT=4205 PLAYWRIGHT_REUSE_SERVER=1 pnpm exec playwright test tests/phase3-gateway.spec.ts --workers=1 --reporter=list`: 8 pasaron, 1 omitida (`/auth/terms` continúa devolviendo 503 `terms_unavailable`).
- Axe anónimo y autenticado pasó en ambos viewports solicitados: 1280×900 y 390×844. Se verificaron roles, capabilities y `restricted_mode`; Bearer solo en memoria y ausente de cookies, local/session storage, IndexedDB, URL, HTML y logs. Logout final: 204; consulta posterior de sesión: 401 `unauthenticated`, sin Authorization.
- El escaneo del supervisor de API no encontró material sensible en logs. `phase3_private_runtime_check.py` pasó; el scrub final indicó que no había contextos Playwright sensibles. `gitleaks` no está instalado; el escaneo local de 37 archivos modificados/nuevos no encontró valores privados ni patrones de credenciales de alta confianza. `git diff --cached --check` pasó con el conjunto exacto previo a publicación; el árbol publicado quedó limpio.
- `go build -o .local/testing/phase3-native-runtime-20261024/api ./cmd/api`: pasó; el binario temporal quedó con modo 0700.

## API nativa y gateway

- API Go de este worktree, proceso host temporal en `127.0.0.1:18082`, configurado contra PostgreSQL nativo `127.0.0.1:5432`, base `espacigo_dms_stage_20261020`; `LOCAL_BOOKING_TRIAL` permaneció desactivado. No se usó la API/DB de Docker anterior.
- El endpoint de salud/readiness respondió HTTP 200. Sesión anónima directa y a través del gateway: HTTP 401, código `unauthenticated`; origen coincidente llegó a Go y origen divergente fue rechazado por el gateway con 403.
- `/api/v1/auth/terms`: HTTP 503, código `terms_unavailable`. El endpoint no entregó versiones utilizables; la respuesta no permite distinguir si faltan registros publicables o si otra condición interna impide servirlos. No se registró otra cuenta ni se volvió a consultar para crearla.
- La prueba E2E nativa verificó runtime-config, allowlist, headers de origen, logout bodyless, sesión y logout autenticados; la solicitud a términos llegó al API y recibió el 503 documentado. La respuesta de términos sigue siendo una capacidad no disponible.
- El proceso también inició workers normales del runtime. El worker de limpieza de medios privados registró repetidamente el mensaje genérico `private media cleanup pass failed`; no se registraron identificadores en esos logs y no se hizo diagnóstico adicional. El estado de todos los workers no queda certificado por las comprobaciones de identidad.

## Cuenta sintética y flujo autenticado

- El login previo a la recuperación falló porque el hash guardado no correspondía a la contraseña privada de política; no fue un defecto demostrado del frontend. Tras autorización humana explícita, se recuperó exactamente una cuenta sintética existente mediante solicitud/completado normal de Go: 202 y 204. No se creó una cuenta y la recuperación no se repitió.
- En la primera ejecución posterior a esa recuperación, el gateway rechazó el `Origin` de `127.0.0.1:4205` al compararlo con el origen interno de Next. Corregida la comparación para usar `Host` saneado, el login y la consulta de sesión autenticada pasaron; Axe también pasó. Esa primera ejecución de logout devolvió 415 `request_body_not_allowed`: Next entregaba un stream vacío y el gateway lo confundía con un cuerpo. La corrección y regresión bodyless quedaron verificadas.
- La corrida E2E final completó login, sesión, claims/privacidad, Axe, logout 204 y sesión posterior 401. En la primera corrida que alcanzó login, el frontend limpió su token de memoria pero el gateway rechazó el logout antes de que Go pudiera revocarlo. Ese bearer no se persistió ni se conservó; no se puede confirmar su revocación y la sesión asociada puede seguir válida hasta su expiración normal (máximo 8 horas desde su emisión). La segunda corrida revocó su propio token por el flujo normal. No se hizo SQL ni se intentó una revocación alternativa.
- No se incluye correo, ID, contraseña, token ni valor de `Authorization` en evidencia, capturas, logs, commits o reportes.

## Alcance, limpieza y límites

- No se creó cuenta, base, contenedor o volumen; no se ejecutó migrador, migración, grant ni SQL directo. Se usó únicamente el PostgreSQL nativo autorizado; no se modificaron fuentes backend, SQL, `planning/openapi.yaml`, Compose ni infraestructura. Sí se generaron tipos de frontend desde el contrato existente.
- Política e inventario privados permanecieron bajo `.local/testing/` con permisos 0600 e ignorados por Git; no se guardó token temporal en disco. No se copiaron datos privados a documentación, capturas o commits.
- API y Next efímeros se detuvieron. La comprobación final confirmó libres `18082`, `4205` y `4203`; PostgreSQL nativo y Mailpit preexistentes continuaron disponibles. No se inició Fase 4.
- La publicación utilizó GitHub API tras confirmar `gh` como `HernanEspinozaDev`; se verificaron autor y árbol remoto de los commits. El commit de implementación es `74f166de60a1acded0d8cb8ed14b3cf07667ccf5`; la rama quedó sincronizada local/remoto y limpia.
