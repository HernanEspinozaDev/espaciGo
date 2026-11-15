# N08-H — retorno con error técnico y reintento (2026-11-09)

## Alcance en preparación

Continuación de C07 tras N08-G, mismo checkout `espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD base `274f37e483a078890e92382a159fa49fd23c2fe8` más árbol dirty conservado. **Sin ensayo nativo de este gate todavía**. Dos cuentas existentes autorizadas, este caso utiliza solo A; sin nuevas cuentas, migraciones, proveedor externo, workers generales ni publicaciones.

Readback previo administrativo READ ONLY: `espacigo_dms_stage_20261020`, OID16388, PostgreSQL180006/ledger90; auditDELETE/outboxDELETE/ownerCREATE=false; outbox50/pendientes0. Listener5432 presente y puertos de ensayo libres. Snapshot frontend N08-G coincide byte a byte con fuentes relevantes; no cambios de UI de producto previstos.

## Contrato real inspeccionado

`DiditService.GetStatus` obtiene intento propio del repositorio. El handler `handleGetDiditIntentWithID` serializa su vista real y usa `writeDiditServiceError`:

- `ErrDiditUnavailable` produce **502/provider_unavailable**, no503.
- Intento `FAILED` devuelve **badge_result=NOT_VERIFIED**, noNONE. El adaptador normaliza y la página muestra fallo técnico sin rechazo.

El texto histórico503 de la matriz no autoriza cambiar el handler/gateway ni fabricar ese código para hacer pasar el gate. Este recorrido probará un **5xx real502** como representante de consulta no disponible;503 conserva cobertura histórica interceptada separada.

## Inyección explícita solo de ensayo

Nuevo decorador `_test.go` del repositorio de intentos, habilitado únicamente por modo explícito del harness:

1. Lee primero mediante el repositorio PostgreSQL real; dueño/intento/declaración deben coincidir con la cuenta autorizada y journal confiable. Desconocidos/ajenos no consumen inyección ni mutan.
2. Primera consulta válida del intento PENDING_USER_INPUT: error inyectado `ErrDiditUnavailable`, sin modificar DB. Handler/BFF reales producen502.
3. En el reintento deliberado, transición sintética acotada mediante el método real `MarkAttemptFailed`, seguida de nueva lectura real que debe devolverFAILED. No se fabrica respuesta de estado ni se escribe SQL directo desde el browser.
4. Lecturas posteriores pasan al repositorio sin nuevas transiciones. Contadores agregados prueban una inyección/una transición/recuperación; SQL READ ONLY antes del cleanup debe comprobar intentoFAILED propio, declaración/claim conservados y cero eventos/outbox propios.

Esto acredita UI/BFF/Go/DB con **fault injection controlada y estado sintético persistido**, no una caída real del proveedor ni la causa externa de FAILED. No añadir endpoints de control al producto.

## Criterios de salida previstos

- Probe dedicado con opt-in, sin argumentos; login HTTPS normal y cookie flags, navegación de login terminada antes de avanzar.
- Guardar una declaración201→GET200; consentimiento Didit separado; intento201 con una creación en fake y cero al guardar.
- Retorno con attempt propio y query falsificadaApproved/token: limpieza antes de GET,502 visible como fallo de consulta, sin aprobar/rechazar y sin retry automático.
- Click en retry real: mismo intento200/FAILED/NOT_VERIFIED, copy técnico explícitamente no rechazo, error anterior retirado. Nuevo ingreso por URL propia confirmaFAILED persistido; sin segunda creación del proveedor.
- Storage/URL sin campos seleccionados, cero exterior inesperado; logout204/sesión401/cookies ausentes y cierre de browser siempre.
- Unitarios conductuales del decorador, revisión parental de fuente congelada, regresión/race/vet, preflight autorizaciones, backup nuevo validado, prepare mínimo/readback, ejecución y finalización/readback independientes.
- Cleanup por journal y comparación baseline50/pendientes0. Detener solo recursos propios y conservar cuentas/fixtures autorizados. N08 global no se cierra por un gate C07.

## Estado inicial (superado por la ejecución siguiente)

Helper con tests y probe dedicados asignados en slices separados; launcher scratch preparado con modo `return-failures`. Falta recibir/revisar entregas, integrar helper al harness, ejecutar gates y registrar resultados reales. No se ha abierto ventana temporal de permisos ni escrito datos para N08-H.

## Revisión e integración parental

Entregas `deleg_e58e7190`: helper `cmd/api/identity_native_return_failure_test.go` y probe `frontend/tooling/kyc-native-return-failures.mjs`, originalmente source-only. Padre revisó ambos completos y conectó el decorador al harness exclusivamente en `NATIVE_KYC_HTTP_RETURN_FAILURES=1`; rechaza combinación con expectativas pair/webhook. El journal debe haber estado ausente al iniciar y ser el creado exclusivamente por el wrapper de este ensayo; se valida su metadata al leerlo para scope. Cleanup sigue comparándolo con el intent confiable en memoria.

Padre añadió RED conductual: un repositorio que devuelve un ID distinto al solicitado consumía la inyección, y un readbackFAILED con dueño cambiado se aceptaba como recuperado. Ambas pruebas fallaron antes del endurecimiento y pasaron después de exigir cuenta/ID/declaración exactos también tras la transición. Se corrigió además el test del error de readback para dispararlo en la lectura posterior a MarkFailed, no en la lectura inicial. No se reintenta automáticamente una mutación con resultado incierto.

Probe reforzado antes de ejecutar: body genuino del POST conservado en memoria en el relay exacto (evita perderlo tras navegación), recuento de POST realmente interceptados en vez de incrementar un contador tras guardar, contadorfake después de recuperación, exactamente tres lecturas de estado, cero exterior y etiqueta real `Verificación rechazada`. Sin respuestas de estado fabricadas ni cambios al producto/gateway.

Suite parental cmd/api/verificación/crypto/HTTP/adaptador/fake con race/vet PASS, preflight READ ONLY PASS; TypeScript/Node syntax/ESLint/Prettier PASS y contrato adaptador7/7PASS. Repetición final JSON parseada:518eventos de testPASS,10SKIP,0FAIL, incluidos subtests; ambos lifecycle nativos antiguos confirmadosSKIP, sin habilitar sus fixtures.

## Backup y ventanas de privilegios

Backups privados0600, TOC y consumo completo con pg_restore PASS, sin restore:

| Archivo | Bytes | SHA256 |
| --- | --- | --- |
| pre-n08-return-failures-20261109.dump | 8,323,565 | e163caf985d84bdc1d70a9e1f16e5f8d1dedd948e79278bafc51d5a924b16cbe |
| pre-n08-return-failures-retry-20261109.dump | 8,323,664 | d28ef35bda125f1682fec1a916340b95658ac9b98a96ed8a33f451efff3e6e34 |

Prepare/finalize versionados mediante sudo directo parental; lectura independiente de grants preparados: auditDELETE/outboxDELETE=true, ownerCREATE=false. DELETE es permiso temporal de tabla, no autorización SQL por fila; el cleanup revisado limita por journal. En ambas ejecuciones el finalizador terminóCOMMIT/exit0 y se leyó la revocación.

## Primer fallo: aserción demasiado amplia sobre alertas

Primera ejecución:502 esperado, retry200 conFAILED, SQL del harness PASS, pero probeexit1 en `failed-terminal-ui` por exigir ausencia de **todo** role=alert. No se declaró PASS conjunto: harnessexit0 no sustituye al navegador.

Diagnóstico local sin credenciales ni mutaciones: navegación a retorno sin attempt_id y evaluación de las alertas reales mostró la alerta P del error y otra DIV dentro del shadow host **NEXT-ROUTE-ANNOUNCER**. Ese announcer es independiente del error de consulta. Se limitó la aserción de desaparición al role=alert que contiene `No pudimos consultar el estado`, conservando checks de no aprobación/rechazo/query limpia. No se suprimió accesibilidad ni se modificó UI de producto.

Cleanup del primer ensayo confirmado por preflight/readback antes de empezar otro; journals ausentes, permisos revocados y outbox50/pendientes0 estable. Marker stop propio retirado validando regular0600/UID/tamaño0. Reintento como ensayo nuevo con backup fresco, no replay de guardado posiblemente confirmado.

## Ejecución nativa corregida: PASS

`run_kyc_n07_bff.py return-failures`, runtime worker-free, API18090, Next4201/HTTPS4219, fake19091. Snapshot frontend N08-G reutilizado después de comparación exacta de las fuentes relevantes; cambios solo de prueba no exigieron rebuild de producto. Playwright aislado y navegación de proveedor interceptada sin tráfico externo.

| Criterio | Resultado real |
| --- | --- |
| Login/cookies | Login200; cookies Secure/HttpOnly/SameSiteLax/path raíz comprobadas |
| Guardar/releer | 201/200 y resumen; cero creates en proveedor |
| Consentimiento Didit separado/inicio | POST201; deltafake1 |
| Primera consulta propia en retorno | HTTP502 genuino del handler por fault injection; query falsaApproved/token retirada antes de dispatch |
| Estado del fallo | Alerta recuperable, sin aprobación/rechazo; ninguna lectura adicional durante ventana2.5s |
| Retry explícito | HTTP200 para mismo path de intento; bodyFAILED/NOT_VERIFIED; copy técnico no rechazo; alerta de consulta retirada |
| Nuevo ingreso al retorno propio | HTTP200/FAILED/NOT_VERIFIED, copy técnico visible; exactamente tres GET de estado en total |
| No duplicación del proveedor | Deltafake permanece1 después de retry/nuevo ingreso; un solo POST de declaración y uno de inicio |
| Privacidad/red | Sin campos seleccionados en localStorage/sessionStorage/URL; cero peticiones externas inesperadas |
| Cierre de sesión | Logout204, sesión401 y cookies propias retiradas |

**probe_exit=0, harness_exit=0, native_run_exit=0, finalize_exit=0.** `TestNativeKYCAPIWithBFF` PASS7.78s; paquete8.848s con race.

Antes del cleanup, callback READ ONLY y Snapshot exigieron: una falla inyectada/una transición/recuperación; exactamente un intentoFAILED propio con declaración/claim conservados, cero eventos y cero outbox propio. Eso acredita persistencia del estado sintético usando el adaptador real, no su causa externa.

## Readback y retirada

Readback independiente posterior: destino/OID/motor correctos, ledger90, auditDELETE/outboxDELETE/ownerCREATE/auditSELECT=false; outbox50/pendientes0 estable. Preflight posterior PASS para política/fixtures/ausencia de artefactos propios, ambos journals ausentes y markers retirados.

API18090 cerrada por harness. Next/proxy/fake propios retirados con SIGTERM solo después de verificar UID/cwd/exe; `ss` confirma4201/4219/18090/19091sin listeners. PostgreSQL, cuentas autorizadas, backups privados y datos ajenos conservados. Sin migraciones, reset de cuentas, publicación o despliegue.

| Fuente probada | SHA256 |
| --- | --- |
| cmd/api/identity_native_return_failure_test.go | 26fbc6b77ce0dfeb9ecda125fdb5ef0cc295e4c050bdab2551ea5d1f058f4b7f |
| cmd/api/identity_native_http_harness_test.go | 153a32dab636305d14178fdc965754eb54bb54eca0e9f1684aba1cd39232fa5f |
| frontend/tooling/kyc-native-return-failures.mjs | 849ebfefc7975bb885fbe677bfda1e7f10ba9205fc144a4a20bc81b13b904e9f |

**N08-H completa para recuperación 5xx502→retryFAILED y readback descritos.** No se probó503exacto contraHTTP nativo, caída real de Didit, creación fallida por red, todos los estados C07/C08 ni SLA de retención. N08global sigue parcial, sin sandbox externo, biometría ni aceptación humana.
