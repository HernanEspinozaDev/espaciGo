# N08-D — webhook HTTP local y decisión autoritativa (2026-11-09)

## Baseline y alcance

- Checkout `espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD base `274f37e483a078890e92382a159fa49fd23c2fe8` más árbol dirty preservado.
- Probe probado `frontend/tooling/kyc-native-flow.mjs`, SHA-256 `ec45db4a0020edad307e416aaefca61271d2a534b66fa040c85155651d2197c4`; harness `cmd/api/identity_native_http_harness_test.go`, SHA-256 `e8242182db6bbc926f725655117a307d5d4e32a3ea1d509b6b96ac5305f5bf69`.
- Modo opt-in `KYC_NATIVE_FLOW=1 KYC_NATIVE_WEBHOOK=1`, más expectativa de DB `NATIVE_KYC_HTTP_EXPECT_WEBHOOK_EVENT=1` en el harness. No confundir esta entrada con una nueva capacidad de producción.
- API real en18090 y PostgreSQL nativo `espacigo_dms_stage_20261020`/OID16388/motor180006; UI/BFF HTTPS4219/Next4201 con el mismo build N07. Fake provider real de loopback19091 devuelve decisión `In Review`; no proveedor externo ni datos biométricos.
- Dos cuentas sintéticas existentes autorizadas; una declaración journaled y un intento. Sin altas/reset, worker general, main ni despliegue.

## Revisión y gates antes del ensayo

Entrega source-only `deleg_1adf7745`, dos archivos de prueba. Padre trazó mounts, verificación HMAC, envelope, consulta autoritativa y tablas reales. Corrigió antes de ejecutar la dependencia del probe de una variable secreta real: usa exclusivamente la cadena sintética fija exigida por el perfil local, sin importar credenciales de proveedor al entorno del navegador. Los fetch server-to-server tienen destino loopback constante, redirects rechazados y timeout10s.

La comprobación SQL de evento/intento se registra después del cleanup por journal y antes del cierre del servidor: por LIFO se detiene la API, se consulta READ ONLY el ámbito confiable y después se limpia. Un fallo de verificación no impide que el cleanup registrado se ejecute. No se añadieron DELETE ni permisos.

Preflight nativo PASS; Node syntax/Prettier/ESLint/TypeScript PASS; Go `-race -count=1`, vet y diff-check PASS, con el descubrimiento implícito de credenciales ya contenido y native lifecycle antiguos omitidos. No se ejecutaron esos lifecycle para este slice.

Backup fresco `.local/testing/kyc-native/pre-n08-webhook-20261109.dump`, 0600, 8,322,304bytes, SHA-256 `6b89f90d7458798bad930230f6d33f5149cb3567f8035c733c6a9b2dda25a864`; TOC y consumo completo con pg_restore PASS. No restore.

Prepare/finalize SQL versionados ejecutados por sudo directo parental. `run_kyc_n07_bff.py webhook` lanzó el harness con expectativa de evento y el probe con entorno allowlist. Firma raw HMAC-SHA256 de bytes JSON/timestamps sintéticos, sin cuerpo/ID/firma/keys/URLs tokenizadas en salida.

## Resultado nativo

| Caso HTTP al webhook Go loopback | Resultado |
| --- | --- |
| Firma incorrecta con timestamp actual | 401 |
| Firma correcta con timestamp de header y cuerpo vencido | 401 |
| Firma/timestamp correctos, sesión aleatoria desconocida | 404 |
| Evento válido, `status=Approved` solo en envelope | 200, processing_statusPROCESSED |
| Replay exacto de los mismos bytes/event_id/firma | 200, processing_statusPROCESSED |

Los POST de webhook se hicieron explícitamente server-to-server a `http://127.0.0.1:18090/api/v1/webhooks/didit`, sin Bearer ni cookies. No pasaron por el interceptor de rutas del navegador; sí por el mount y verificador HTTP Go reales. No se presenta el guard browser como prueba de esos fetch; su destino/redirect/timeout se fijan por separado.

GET propio por BFF después de válido/replay: status200, intento **IN_REVIEW**, badge noVERIFIED. La consulta autoritativa del fake provider `In Review` prevaleció sobre el envelope `Approved`; no se aprobó por el webhook. Retorno visible: «En revisión», query limpia y sin badge exacto «Identidad verificada» aun con `status=Approved` en URL.

Antes del cleanup, consulta READ ONLY bajo API/runtime y scope de cuenta/declaración capturados antes de escribir: **un evento**, processing_statusPROCESSED, digest32bytes; **un intento**, estadoIN_REVIEW. Esto prueba deduplicación durable del evento exacto y que los negativos no dejaron eventos adicionales en ese ámbito. No se guardó cuerpo crudo del webhook para estas aserciones.

También pasaron todas las aserciones N07/N08-B: guardar/readback/recarga, consentimiento separado, auth/cookies, A/B/A, intento ajeno404/retry404 sin PII y logouts204/sesiones401. Cero requests browser externos inesperados; proveedor externo interceptado localmente.

**probe_exit=0, harness_exit=0**, `TestNativeKYCAPIWithBFF` PASS11.55s (paquete12.634s con `-race`).

## Cleanup/readback y límites

Cleanup journaled PASS, finalizador directo COMMIT. Readback independiente posterior READ ONLY: ledger90, grants audit/outboxDELETE=false, auditSELECT=false, ownerCREATE=false; outbox50/pendientes0 estable. Preflight fixture posterior PASS; journal/markers ausentes, API18090 detenida, ambas cuentas conservadas. No se borraron las50filas ACK históricas del test de adaptador.

**N08-D completa para firma raw/frescura/correlación/replay y autoridad InReview descritas.** No acredita V2/rotación contra HTTP nativo, aprobación biométrica, media real, rechazo/expiración/FAILED ni SLA de retención; esas capas/casos conservan su evidencia de unidad o pendientes en la matriz. N08 global sigue parcial, no aceptación humana/sandbox externo.

Siguiente N08-E: observar únicamente contadores agregados autenticados del fake provider para comprobar cero creación al guardar/sin consentimiento y un solo create tras replay de la misma key. Fuente en implementación separada; se requiere revisión, build/restart del fake propio y ejecución parental. Sin modificar backend de producto ni ampliar bases/cuentas/colas.
