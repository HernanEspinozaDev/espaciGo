# Contrato de ejecución — UI-AUTH fase 2

## Alcance

Implementar únicamente transporte y ciclo de sesión aprobado por ADR UI-AUTH-001. Mantener Tailwind, los colores y el diseño Next actual. No incorporar identidad de cabecera, rediseño de vistas ni otras fases. No modificar Go, SQL, IAM, Cloud Build, datos retenidos o `AGENTS.md`. Preservar los cambios locales de fase 1 sin descartar ni publicar por inferencia.

## Preflight y separación de evidencias

- PostgreSQL nativo previsto: `127.0.0.1:5432`, `espacigo_dms_stage_20261020`, rol runtime existente.
- El contenedor activo en `127.0.0.1:18081` usa **otra base**, `espacigo_local`. No sustituye el destino nativo autorizado.
- Material privado de cuenta sintética existente: política e inventario retenidos en `.local/testing/` del worktree anterior. No copiar valores a documentación, prompts ni logs; no crear cuentas o recuperaciones.
- No arrancar `cmd/api` indiscriminadamente: el código actual inicia workers de privacidad/retención/limpieza aunque `LOCAL_BOOKING_TRIAL=0`. La prueba nativa de identidad debe montar los handlers y servicios Go reales contra la misma base, en un harness aislado sin esos workers. Distinguir esa prueba de un arranque completo de la API.
- Pruebas controladas del BFF pueden usar un upstream sintético local; no se presentarán como Go/PostgreSQL real. El gate nativo se registra por separado y no se omite silenciosamente.

## Hallazgo contractual Go

`POST /auth/login` entrega `access_token`, `token_type`, `expires_at`, `account_id`, roles, capabilities y modo restringido. `GET /auth/session` confirma identidad/capacidades pero **no entrega expires_at**. Solo los handlers Go son autoridad para validar credencial y permisos.

La caducidad real no es una promesa de ocho horas continuas: existe un máximo absoluto de ocho horas **y un límite de inactividad de treinta minutos**, conforme a `identity.Session.ActiveAt`. El GET de sesión usa `AutomaticPolling` y no renueva actividad; no inventar refresh o heartbeat para extenderla.

## API del navegador y cookies

Conservar los paths same-origin existentes para limitar regresiones:

- `POST /api/backend/auth/login`
- `GET /api/backend/auth/session`
- `POST /api/backend/auth/logout`

El BFF intercepta esos paths de forma explícita; ninguna variante de catch-all debe poder devolver el login Go crudo. Los demás endpoints conservan inventario, allowlist, exact JSON, idempotencia, cancelación, media types y streaming existentes.

Login y sesión devuelven exclusivamente un DTO seguro:

```ts
interface BrowserSession {
  account_id: string;
  roles: SessionRoles;
  capabilities: SessionCapabilities;
  restricted_mode: boolean;
  expires_at: string;
}
```

Sin `access_token`, `token_type`, cookies crudas o valores secretos en JSON/HTML/props/URLs/logs. Login debe confirmar el token con `GET /auth/session` de Go, comprobar la identidad y emitir el DTO confirmado; no confiar solo en el primer HTTP 200.

Cookies propuestas para resolver la ausencia de expiry en el GET Go sin base de sesiones nueva:

1. `__Host-espacigo-session`: credencial opaca Go, exclusivamente servidor.
2. `__Host-espacigo-session-expiry`: fecha absoluta recibida de Go, como metadato de transporte, no autoridad de autorización.

Ambas: `HttpOnly`, `Secure`, `Path=/`, `SameSite=Lax`, sin `Domain`; `Expires` fijado al vencimiento Go y `Max-Age` redondeado hacia abajo. Nunca reemitirlas con una duración nueva al consultar sesión u operar. Sin JWT propio, Map de sesiones, ORM, signing key nueva o TTL backend distinto. Alterar el metadato no puede hacer que Go acepte una sesión inactiva, vencida o revocada. Rechazar formatos/duplicados anómalos y borrar ambas al invalidar el transporte.

## Seguridad y proxy

- Origin estricto contra Host entrante saneado, sin usar Host interno canónico ni `x-forwarded-*` como autoridad. Rechazar Origin ausente en mutaciones, `null`, divergente o mal formado. Considerar Fetch Metadata como defensa adicional, no como autorización.
- Los ensayos reales de Secure se ejecutan sobre HTTPS local aislado; no emitir cookies inseguras ni relajar producción para ensayar HTTP.
- El navegador no envía Authorization. El gateway rechaza Bearer de navegador, obtiene la credencial de cookie solo para operaciones protegidas y nunca reenvía Cookie cruda, incluyendo cookies IAP.
- Mantener `X-Serverless-Authorization` IAM separado del Bearer del usuario. Para destinos loopback, no intentar consultar metadata cloud.
- Respuestas autenticadas/sesión: no caché compartida (`private, no-store`, sin serialización de secretos). Validar media types y redirecciones; no propagar `Set-Cookie` arbitrario del upstream.
- `401` de credencial inválida elimina cookie; `403` de una operación por permisos no demuestra sesión inválida. Purgar datos privados y revalidar con Go, sin convertir todo permiso denegado en logout. En `GET /auth/session`, un `403` tampoco autoriza borrar el transporte: puede corresponder a una denegación de servicio/IAM o a permisos, no a revocación comprobada. No publicar identidad autenticada sin confirmación `200`; conservar la cookie ante `403` permite revalidar posteriormente. Red/`503` no prueban vencimiento.
- Logout borra cookies incluso si Go devuelve error o no se puede contactar. Distinguir cierre de transporte local de revocación remota confirmada. Una petición abortada/generación anterior no debe restablecer UI ni credenciales.

## Contrato cliente y migración

- Eliminar `AccessToken`, `getAccessToken`, campos privados de credencial y parámetros Bearer de los puertos/facades/adaptadores del navegador.
- `runAuthenticated` recibe solo `(signal: AbortSignal) => Promise<{ response: Response; value: T }>` y conserva el aislamiento por generación/cancelación.
- `AuthPort.login(input, signal)` devuelve sesión segura; `currentSession(signal)` devuelve sesión segura; `logout(signal)` comunica por separado limpieza local y confirmación remota.
- Transporte con `credentials: same-origin`, no Authorization. Las operaciones binarias también usan cookie y mantienen revocación de blobs.
- Eliminar únicamente la clave legacy `espacigo_auth_session`; no leer/importar su token ni roles y no usar `sessionStorage.clear()`.
- Estado inicial comprobando, incluso tras SSR. Restauración real con GET de sesión; deduplicar comprobaciones concurrentes y descartar respuestas tardías.
- Revalidar al volver visible/foco/bfcache y sincronizar logout entre pestañas sin credenciales. Sin polling renovador.
- Un marcador booleano no secreto de **logout pendiente** puede persistirse para no restaurar automáticamente una sesión después de un fallo de red antes de alcanzar el BFF. No contiene token, ID, roles ni perfil. Se elimina solo al confirmar que el BFF limpió el transporte. Si el navegador no pudo contactar al BFF, no afirmar que la cookie HttpOnly desapareció ni que Go fue revocado.
- Cambio de cuenta: purgar datos privados antes de publicar otra identidad confirmada. No conservar roles/perfil restaurados sin comprobación Go.
- Cambiar únicamente el copy de persistencia que hoy afirma falsamente «solo en memoria»; preservar composición y clases de las vistas.

## Gates mínimos de fase 2

1. Unitarias TDD de parsing/flags/expiry, Origin/CSRF, whitelist de salida y carreras.
2. BFF real sobre HTTPS local, no interceptar sus endpoints con Playwright: login confirmado, cookie no accesible a JS, reload, pestaña nueva, reapertura con cookie vigente, expiry/revocación y logout.
3. Continuidad al servir desde dos procesos Next con idéntica configuración y sin almacenamiento de sesión local por instancia.
4. Sin token en JSON, HTML, JS bundle, almacenamiento web, logs o Authorization del navegador; preservar flags host-only/Secure/HttpOnly.
5. Errores 401/403/503 y fallo remoto de logout, logout pendiente, resultados tardíos y sincronización entre pestañas.
6. Regresión de todos los adaptadores protegidos, idempotencia, CSRF y binarios privados. Actualizar fixtures controlados al DTO BFF; no conservar contratos viejos con token para hacer pasar tests.
7. Evidencia nativa diferenciada con Go identity real y PostgreSQL nativo autorizado, sin workers ni bootstrap.
8. Tipos, lint, formato, unitarias, build, E2E/Axe; registrar omisiones, límites y limpieza. No iniciar fase 3 ni publicar sin gates y revisión del corte.
