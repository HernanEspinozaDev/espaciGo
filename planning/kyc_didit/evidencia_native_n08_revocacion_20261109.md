# N08-J — revocación de intento pendiente y no resurrección (2026-11-09)

## Alcance en preparación; no ejecución nativa aún

Checkout `espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD base `274f37e483a078890e92382a159fa49fd23c2fe8` más árbol dirty conservado. Solo cuenta sintética A ya autorizada, un guardado journaled y un intento falso. Sin nuevas cuentas, migraciones, clocks alterados, proveedor externo ni worker general.

Preflight READ ONLY y regresión parental cmd/api/verificación/crypto/HTTP/adaptador/fake con race PASS; vetcmd/api/diff-checkPASS. Readback administrativo READ ONLY: `espacigo_dms_stage_20261020`, OID16388, motor180006/ledger90; auditDELETE/outboxDELETE/ownerCREATE=false; outbox50/pendientes0estable.

## Frontera seleccionada y límites

Este gate **no** demuestra retirar una aprobación anterior. Parte de PENDING_USER_INPUT y aplica un webhook firmado sintético `user.status.updated/REVOKED`: servicio real actualizaREJECTED y completaeventoPROCESSED. Repetir exactamente el evento debe deduplicar sin nueva fila. Un evento distinto posterior `status.updated/Approved` debe quedarIGNORED, sin revivir el intento; su replay conservaIGNORED.

El fake no cambia su decisiónInReview; esta ruta terminal no usa un Approved del proveedor ni transforma un payload en decisión autoritativa. El propósito es revocación terminal/no resurrección y dedupe, no verificación biométrica, revocación deVERIFIED o sandboxApproved. Son transiciones pendientes separadas.

## Gates previstos

- LoginHTTPS normal/cookies, save201/readback200, cero creates al guardar, consentimientoDidit separado/start201/uncreatefake; redirect del proveedor interceptado sin tráfico exterior y respuestaPOST genuina preservada enmemoria.
- Cuatro POSTwebhookloopback firmados rawHMAC con secreto fijo exclusivamente sintético: revocación+replay(PROCESSED), tardíoApproved+replay(IGNORED). Sin token/cuenta real ni cambios al gateway.
- Retorno propio con queryApproved/tokenfalsa: limpieza antes de GET200/REJECTED/REJECTED, UIrechazada/sinbadgeverificado; reingreso confirma estado. No atribuirREVOKED a rechazo documental por biometría real.
- ClickrealRevisaroreintentar lleva a identidad guardada propia intacta; sin nuevo save/intent/create por navegar. PrivacidadstorageURL y exterior0; logout204/sesión401/cookiesausentes.
- Harnessmodoexclusivo `NATIVE_KYC_HTTP_REVOCATION=1`, sin decoratorfault/clock: SQLREADONLYantesdecleanup exige unREJECTED propio/declaración/claim, dos eventosdurables (unoPROCESSEDuser.status.updated y unoIGNOREDstatus.updated, hash32/processed_atnonnull), cerooutboxpropio. Replays no deben añadirfilas.
- Backupfresco validado→preparemínimo/readback→ensayo→cleanupjournal→finalizerdirecto/readback. Baselineoutbox50/pendientes0, preservar cuenta/ajenos y retirar solo procesospropios.

## Estado inicial (superado por la ejecución siguiente)

Harness y launcher preparados; probe dedicado en implementación delegada source-only. Falta recibir/revisar probe, completar gates y ejecutar. Permisos temporales siguen cerrados; no mutaciones de datos de N08-J todavía. N08global y C08continúanparciales.

## Revisión parental y correcciones previas

Entrega `deleg_9a036eae` source-only recibida y leída completa. Padre corrigió errores de copia antes de cualquier mutación: segundo evento debe ser `status.updated` (no `user.status.updated`); el estadoREJECTED produce badge_resultREJECTED real (noNOT_VERIFIED); la UIrechazada debe estar presente exactamente una vez, no afirmarse ausente inmediatamente después de esperarla. Renombró el resultado persistido y stage de recarga para no atribuir expiración a revocación. No se cambió handler, servicio, gateway ni el proveedor falso.

Probe Node syntax/ESLint/Prettier PASS; contrato adaptador7/7PASS. Suite parental antes y después del ensayo cmd/api/verificación/crypto/HTTP/adaptador/fake con race cero caché y vet PASS; preflight READ ONLY PASS. Antiguos lifecycle nativos sin opt-in; no se usaron como cobertura de este gate. Snapshot frontend N08-G comprobado contra fuentes relevantes, sin rebuild por cambios solo de pruebas.

Harness habilitado únicamente con modo explícito y mutuamente exclusivo; no decorator ni clockoverride. SQL previo a cleanup usa cuenta/declaración confiables y exige exactamente unREJECTED, declaración/claim conservados, dos eventos (unoPROCESSEDuser.status.updated, unoIGNOREDstatus.updated, amboshash32/processed_atnonnull) y cerooutboxpropio. Esto prueba la deduplicación durable de los cuatro envíos en dos filas sin borrar evidencia antes de verificar.

## Backup y permisos

Backup privado fresco `pre-n08-revocation-20261109.dump`,0600,8,323,845bytes, SHA256 `a2286bf2cb4773c16c62bb6a90ec461b36a93036c08bb46907c745c61b7f7b49`; TOC y consumo completo pg_restore PASS, sin restore. Prepare/finalize versionados mediante sudo directo parental; readback independiente preparado auditDELETE/outboxDELETE/auditSELECT=true y ownerCREATE=false. DELETE temporal es de tabla; scope de filas restringido por helper/journal revisados, no por una políticaSQLporfila.

## Ejecución nativa PASS

`run_kyc_n07_bff.py revocation`, API18090 worker-free, Next4201/proxyHTTPS4219, fake19091, Playwright aislado sin perfil del usuario ni exterior. Se conservaron cuerpo/cabeceras de cada evento en memoria para el replay byte a byte; timestamp vigente/firmaHMAC local sintética, redirecterror/timeout10s.

| Gate | Resultado observado |
| --- | --- |
| Login/cookies |200; flagsSecure/HttpOnly/SameSiteLax/path correctos |
| Guardar/leer |201/200/resumen; deltafake0 |
| Consentimiento separado/inicio |201/uncreatefake |
| WebhookREVOKED y replay exacto |200/PROCESSED ambas veces |
| DistintoApprovedtardío y replay exacto |200/IGNORED ambas veces |
| Retorno propio con Approved/tokenfalsos |200/mismoID/REJECTED/REJECTED; query limpia antes de dispatch |
| UI |Verificación rechazada visible, sin badgeIdentidadverificada; no es un resultado biométrico real |
| Nuevo ingreso |200/mismoID/REJECTED/REJECTED; dosGETestado totales, sinpolling extra en ventana2.5s |
| ClickrealRevisaroreintentar |Ruta cuenta/verificaciones, GET200 con nombres/fecha/RUTnormalizado exactos y resumen intacto |
| No duplicación |Unsave/unintentPOST; createsfake0guardar/1inicio/1final |
| Privacidad/red |Sin campos seleccionados en storage/URL; exteriorinesperado0 |
| Logout |204/sesión401/cookiespropiasausentes |

**probe_exit=0, harness_exit=0, native_run_exit=0, finalize_exit=0.** HarnessPASS10.37s, paquete11.448s con race. SQL previo a cleanup PASS con las expectativas enumeradas; replays no añadieron filas ni reactivaron estado. No fallos nativos previos en este gate: los defectos del probe se corrigieron por inspección antes de ejecutar.

## Postflight y retirada

FinalizadorCOMMIT. Readback independiente READ ONLY posterior: destino/OID/motor/ledger90 correctos; auditDELETE/outboxDELETE/ownerCREATE/auditSELECT=false; outbox50/pendientes0 estable. Preflight posterior PASS para política/fixtures/ausencia de artefactos. Ambosjournalsausentes; ready ausente y stopvacío0600propio validado/retirado. No se borraron ACK históricos por prefijo.

APIcerrada porharness. Next/proxy/fake propios retirados conSIGTERM trasUID/cwd/exe verificados; ssconfirmó4201/4219/18090/19091sinlisteners. PostgreSQL, cuentasautorizadas, backups y datos ajenos conservados. Sin migraciones/commit/push/main/despliegue.

| Fuente probada | SHA256 |
| --- | --- |
| cmd/api/identity_native_http_harness_test.go | 014e9b60830c2af1debd0464bbb916f37c2509ea83b865cabdae9903e1f1f113 |
| frontend/tooling/kyc-native-revocation.mjs | 2b28e3bb088eff2645a36760cbab37b1429fe1b8cc1aa766dc518d2da61e459f |

**N08-J completa para revocar un intento pendiente y evitar su resurrección por unApprovedtardío.** No demuestra revocación de una aprobación previa, supersesión/nueva revisión, sandboxApproved ni purga/retenciónoperacional. C08/N08siguenparciales. Webhook emisor es el probe local sintético, noDiditexterno/biometría/aceptaciónhumana.
