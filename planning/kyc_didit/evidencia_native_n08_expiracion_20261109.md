# N08-I — expiración del intento en límite TTL (2026-11-09)

## Preparación y alcance, aún sin ejecución nativa

Continuación C08 en `espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD base `274f37e483a078890e92382a159fa49fd23c2fe8` más árbol dirty conservado. Un solo guardado de la cuenta sintética A autorizada, journal existente create-only; sin nuevas cuentas, migraciones, proveedor externo ni workers generales.

Readback administrativo previo READ ONLY: destino `espacigo_dms_stage_20261020`, OID16388, PostgreSQL180006/ledger90; auditDELETE/outboxDELETE/ownerCREATE=false, outbox50/pendientes0. Puertos de ensayo libres,5432presente. Ambos journals y markers ausentes. Fuentes relevantes frontend coinciden con build aislado N08-G, sin cambios de producto.

## Cadena real y prueba propuesta

`identitydeclaration.Repository.GetAttemptForAccount` lee primero por cuenta/ID. Antes de TTL devuelve estado vigente; en `CreatedAt + AttemptTTL` (comparación inclusiva) ejecuta UPDATE acotado a cuenta/ID para CREATED/PENDING_USER_INPUT→EXPIRED y devuelve la fila real. Si alcanza retención devuelveNotFound; la prueba no debe mezclar esos límites.

- Decorador solo `_test.go`, habilitado explícitamente en harness: primera lectura valida cuenta/ID/declaración contra el journal del ensayo, después invoca el mismo adaptador con `now=CreatedAt+TTL`, estrictamente antes de retention. Debe exigir EXPIRED leído realmente, sin construir un DTO falso, cambiar reloj del host ni ejecutar SQL desde browser.
- Lecturas posteriores usan el `now` normal y deben conservar EXPIRED duradero; un único clock override agregado. Mismatch/ajenos/desconocidos no autorizan override. Error incierto conserva evidencia y no repite automáticamente la transición.
- Probe dedicado: loginHTTPS, save201/readback200, cero creates al guardar; consentimiento separado/start201 y uncreatefake. Retorno con queryApproved/token falsificados limpia antes de GET200/EXPIRED/NOT_VERIFIED, muestra sesión vencida sin rechazo/aprobación. Nuevo ingreso confirma mismo estado y ausencia de polling/creates extra.
- Click real `Revisar o reintentar` debe navegar a cuenta/verificaciones y leer la identidad guardada intacta sin crear otro intento por mera navegación. No iniciar nueva sesión como parte de este gate.
- Harness comprobará READ ONLY estadoEXPIRED único, timestamps coherentes, declaración/claim conservados y cero eventos/outbox propios antes de cleanup. Cleanup por journal y baseline global, finalización/readback y retirada solo de procesos propios.

Esto prueba la frontera TTL y su UI con **reloj de argumento controlado solo en ensayo**, no paso real de30min, programación de worker ni SLA de purga. No acredita supersesión/revocación, sandboxApproved ni todas las políticas C10. N08global permanece parcial.

## Estado inicial (superado por la ejecución siguiente)

Helper y probe asignados en slices independientes; launcher scratch acepta `expiry`. Falta recepción/revisión de fuentes, wiring parental del harness, pruebas/race/preflight y ejecución autorizada con backup/grants mínimos y readback. No permisos abiertos ni mutaciones de datos de N08-I todavía.

## Revisión e integración parental

Entregas `deleg_5c91a47b` recibidas como source-only. Padre leyó ambas completas y corrigió antes de mutar:

- Helper: tras error en la llamada con reloj avanzado podía volver a intentarlo. Nueva regresión conductual parental falló con `uncertain override was retried or accepted`; se registra el override **antes** de delegar, se conserva incertidumbre y se rechaza otro override. Solo un readback válido y mismo scope marca Expired=true. Sin reintentar mutación incierta ni alterar reloj host.
- Tests de scope usaban time.Now frente a fixture de enero y podían salir por retención sin probar la condición pretendida. Se usa el reloj fijado de la propia fixture, antes del TTL.
- Probe: eliminada lectura de localStorage en about:blank (origen opaco); privacidad se comprueba en retornoHTTPS antes del click y también en cuenta. Las respuestas GET se observan directamente en el hilo principal, sin un relay/throw asíncrono innecesario en route. Se verifica ID real/EXPIRED/NOT_VERIFIED en ambas respuestas y campos exactos del GET de identidad después del click, no solo200.

Harness integra modo exclusivo `NATIVE_KYC_HTTP_EXPIRY=1`, incompatible con otros modos de transición. Conserva comparación de journal/intent para cleanup y callback READ ONLY previo: un intentoEXPIRED propio con `updated_at=created_at+30min`, declaración y claim conservados, cero eventos/outbox propios; Snapshot exige ClockOverrides1/Expiredtrue. El TTL30min coincide con DefaultDiditAttemptTTL real. El callback FAILED de N08-H conserva su expectativa independiente; el código de producto no cambia.

## Gates y backup

Suite parental cmd/api/verificación/crypto/HTTP/adaptador/fake cero caché/race y vet PASS; preflight READ ONLY PASS. Probe Node syntax/Prettier/ESLint PASS. Regresión final JSON parseada:530eventos de testPASS,10SKIP,0FAIL, incluyendo subtests; ambos lifecycle nativos antiguos confirmadosSKIP sin opt-in. Diff-check PASS.

Backup privado `pre-n08-expiry-20261109.dump`,0600,8,323,752bytes, SHA256 `df49cd235b60be1af5820eac856b3395e0a0b672520baff4ac48d866cb5e8e6a`; TOC y consumo completo pg_restore PASS, sin restore. Prepare/finalize versionados con sudo directo parental. Readback de permisos preparados: auditDELETE/outboxDELETE/auditSELECT=true, ownerCREATE=false. DELETE temporal sigue siendo de tabla, no controlSQLporfila; scope lo restringe el helper revisado.

## Ensayo nativo: PASS

`run_kyc_n07_bff.py expiry` con API18090 worker-free, frontendHTTPS4219/Next4201 y fake19091. Build N08-G reutilizado tras verificar fuentes de producto coincidentes; prueba-only no requiere rebuild. Chromium aislado sin perfil usuario y navegación del proveedor interceptada localmente.

| Gate | Resultado observado |
| --- | --- |
| Login/cookies | Login200; Secure/HttpOnly/SameSiteLax/path comprobados |
| Guardar/readback | 201/200, resumen guardado; deltafake0 |
| Consentimiento separado/inicio |201, deltafake1 |
| Retorno propio con queryApproved/token falsa | GET200/EXPIRED/NOT_VERIFIED, query retirada antes de dispatch |
| Copy/autoridad | Sesión de verificación vencida y explicación; sin verificado ni rechazo |
| TTL | Override exactoCreatedAt+30min una vez; estadoEXPIRED persistido por adaptadorPG real, confirmadoSQL antes de cleanup |
| Nuevo ingreso al retorno | GET200 mismoID/EXPIRED/NOT_VERIFIED; dos lecturas de estado total, sin polling extra en ventanas2.5s |
| Click Revisar o reintentar | Navega a mi-cuenta/verificaciones; GET200 con mismos nombres/fecha/RUT normalizado y resumen |
| No duplicación | UnPOSTguardar/unPOSTiniciar, deltafakepermanece1 después del click |
| Privacidad/red | Sin campos seleccionados en storage/URL; cero exterior inesperado |
| Logout |204/sesión401/cookies propias ausentes |

**probe_exit=0, harness_exit=0, native_run_exit=0, finalize_exit=0.** Harness PASS10.36s, paquete11.434s con race. No hubo fallo previo de este ensayo contraDB: las correcciones se hicieron antes de la ejecución.

## Postflight y retirada

Readback independiente READ ONLY: destino/OID/motor/ledger90 correctos; auditDELETE/outboxDELETE/ownerCREATE/auditSELECT=false; outbox50/pendientes0 estable. Preflight posterior PASS para política/fixtures/ausencia de artefactos propios. Ambos journals ausentes y marker ready ausente; stop vacío0600 propio validado/retirado. No se borraron ACK históricos por prefijo ni cuentas.

API cerrada por harness. Next/proxy/fake propios retirados medianteSIGTERM después de UID/cwd/exe; `ss` confirma4201/4219/18090/19091sin listeners. PostgreSQL/recursos ajenos, fixtures autorizados y backups privados conservados.

| Fuente probada | SHA256 |
| --- | --- |
| cmd/api/identity_native_expiry_test.go | e8e4f4c268f4d7fb00c1c3d92970e7f591595668cdf8b27217b1a433eb0147b6 |
| cmd/api/identity_native_http_harness_test.go | ea6a4e17c61903c9f02bfd42efbf40cba2572c939bbe97f78f1ddc0f5c532b24 |
| frontend/tooling/kyc-native-expiry.mjs | 3af5ae324c61a0ab27db44a97951629da0ba57bb552d5d78ee89255e87c687a9 |

**N08-I completa en la frontera TTL/estadoEXPIRED y regreso visible descritos.** No son30min transcurridos realmente, purga de snapshot por retención, worker programado ni SLA. No prueba supersesión/revocación, sandboxApproved o tratamiento de errorSQL al expirar. N08global sigue parcial; sin migración/commit/push/main/despliegue ni aceptación humana.
