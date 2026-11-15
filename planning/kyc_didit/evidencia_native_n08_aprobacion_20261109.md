# N08-K/L — aprobación sintética, supersesión y retirada nativas (2026-11-09)

## Baseline y alcance

Checkout `espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD base `274f37e483a078890e92382a159fa49fd23c2fe8` más cambios dirty sin commit. No publicar, integrar ni desplegar. Única base: `127.0.0.1:5432/espacigo_dms_stage_20261020`, OID16388, PostgreSQL180006. Dos cuentas sintéticas autorizadas conservadas; sin nuevas cuentas ni datos/proveedor reales.

Frontend standalone N08-G sin cambios funcionales: 300 archivos `src` comparados byte a byte con el snapshot, dotenv ausente. Next4201, proxy HTTPS4219 y fake19091 son procesos propios con entornos separados; API18090 usa perfil dedicado sin workers generales. Fake Approved únicamente con opt-in `KYC_FAKE_SYNTHETIC_APPROVAL=1`; default sigue In Review y solo se acepta el fixture sintético allowlisted.

## V91: prerrequisito aplicado y verificado

- V90 permite `declaration_id=NULL` solo en APPROVED sin metadata cifrada; un webhook posterior REVOKED/EXPIRED no podía actualizar ese intento. V1–V90 se conservaron inmutables.
- Delta `V000091__identity_detached_terminal_withdrawal.sql`, SHA-256 `6ceee080736bd965a27226e3810d28ede0ecf2deb52477716a504e2b425c1250`, permite únicamente APPROVED/REJECTED/EXPIRED separados y sin metadata cifrada. No funciones nuevas, roles ni grants.
- Fingerprint V1–V91: `40a36dd98b9dc916ab1bb106da1c77c67f11a9a67de3b69140faf252e1462814`. CLI acotado exige exactamente ledger90, una pendiente91, identidad/destino bajo lock y transacción, y postcheck91; V91 no requiere CREATE.
- La objeción inicial de `deleg_9f8b85ef` sobre casts de PostgreSQL se contrastó creando únicamente una tabla TEMP con el CHECK fuente V90 en la base autorizada, bajo migrador/owner, dentro de BEGIN/ROLLBACK. El deparse observado coincidió exactamente con catálogo/pin. `deleg_821c9638` aprobó la revisión de fuente del SQL/runner. No se confundió esta prueba temporal con aplicación de migración.
- Backup previo `.local/testing/kyc-native/pre-n08-v91-20261109.dump`, 8323938 bytes, 0600/ext4, SHA `9dc7d2edf79543580e9dac2e0ae8cae29df7c2bb5607a24c3fd5a485b5b09bd1`; listado y parse completo pg_restore PASS.
- `dbmigrate -plan-only` devolvió READ ONLY, 90 aplicadas y únicamente V91 pendiente con checksum exacto. CLI tail91 terminó exit0. Readback independiente confirmó ledger91/checksum y el nuevo CHECK, CREATE=false, grants cleanup=false, outbox50/0. No migración cloud.

## Revisión, fallos preservados y correcciones

1. El journal de intento delegado comparaba su correlación con la del guardado. Producción genera ambas por separado. Fixture conductual observó RED; el helper ahora vincula cuenta/declaración y conserva la correlación propia del intento. Persistencia exclusiva0600/fsync antes de PrepareAttempt; ocupado o resultado incierto conserva evidencia.
2. El primer ensayo de aprobación alcanzó commit/purga pero no una respuesta de webhook satisfactoria. La cola capturaba `time.Now().UTC` como método de una instancia temporal: el ACK quedaba anterior a la creación posterior de la fila. Prueba `TestSessionDeletionQueueDefaultClockDoesNotFreezeAtConstruction` observó RED. Se corrigió a una función que obtiene la hora al invocarse; reloj inyectado sigue soportado. El ensayo posterior aprobó y confirmó ACK. Es corrección de producto, no relajación de SQL.
3. Un verificador parental exigía que guardar una nueva revisión eliminara la reserva HMAC histórica verificada. Se rechazó esa exigencia: la política y `claimRutForRevision` preservan la reserva mínima anti-reutilización. La aprobación efectiva se invalida en GetStatus al existir nueva declaración; no se alteró producción para satisfacer una expectativa inventada.
4. Otro ensayo falló solo al leer ACK/retirar_en como cleanup owner. Catálogo READ ONLY confirmó que esas columnas no son legibles por ese principal y sí por runtime. El verificador se trasladó al pool runtime; cleanup sigue en migrador/owner. No se ampliaron grants.
5. La revisión `deleg_b5441e4d` confundió el argumento `false` de `nativeV86IdentityPreflight` con omitir la llamada. El padre contrastó la definición: false exige CREATE ausente, no salta identidad/target. Se mantiene la llamada y prueba por versión. La revisión no fue usada como aprobación ciega.

Todos los ensayos fallidos finalizaron grants y retiraron exclusivamente sus filas mediante journals; preflight/readback posterior confirmó cuentas intactas, cero residuos y outbox50/0 antes de un ensayo nuevo. Nunca se repitió un guardado posiblemente confirmado para arreglar su observación.

## N08-K — supersesión PASS nativa

`run_kyc_n07_bff.py approval-supersession`: probe0, harness0 (5.81s; paquete6.908s), finalizador0.

- Login200/cookies Secure HttpOnly; primera declaración201 y GET200 normalizados; cero creates al guardar.
- Inicio201 con consentimiento separado: exactamente un create nuevo del fake.
- Webhook firmado Approved200/PROCESSED: decisión autoritativa completa del fake; purga local y borrado remoto sintético/ACK.
- Retorno real: status APPROVED, badge IN_REVIEW y texto En revisión, nunca Identidad verificada. Guard LIVE requerido activo sobre intento SANDBOX; no se fabricó una aprobación LIVE.
- GET declaración404 tras purga; formulario vacío permite un segundo guardado real/GET200 con revisión distinta. Intento anterior conserva historial APPROVED pero badge NOT_VERIFIED. Approved tardío PROCESSED no restaura aprobación efectiva; callback visible Identidad no verificada.
- Dos saves, un create; cuatro GET de estado; storage/URL sin marcadores de identidad; cero peticiones exteriores inesperadas; logout204/sesión401/cookies retiradas.
- Verificador SQL READ ONLY runtime: intento exacto desacoplado/sin ciphertext, primera revisión ausente y nueva revisión propia, dos auditorías exactas, dos eventos PROCESSED/hash32, reserva HMAC mínima retenida, una outbox propia ACK y retirar_en≥ACK+30d. Outbox global delta exactamente1 antes de cleanup.
- Observador READ ONLY antes del commit real: declaración retention=consent+30d; intento/evento con misma fecha; claim pendiente con mismo expiry. Después de supersesión, nueva declaración conserva candidato30d. No cambio de reloj host ni transcurso real.
- Cleanup por cuenta+declaraciones+intento confiables, validando vínculos antes de borrar; outbox global vuelve a50/0, journals retirados y grants revocados comprobados.

Backup de ejecución aceptada: `pre-n08-supersession-runtime-read-20261109.dump`, SHA `ba0273532b386d2e7405ad26efd477e0fe9d0dda616e8d1d33f6c6118ca46e6b`, listado/parse pg_restore PASS. Backups de intentos previos se conservan privados como historia, no como evidencia de PASS.

## N08-L — retirada de aprobación previa PASS nativa

`run_kyc_n07_bff.py approval-withdrawal`: probe0, harness0 (5.20s; paquete6.283s), finalizador0.

- Guardar/iniciar/aprobar sintéticamente por el mismo flujo real; status APPROVED SANDBOX y badge IN_REVIEW, snapshot purgado404 y sesión fake eliminada.
- Webhook firmado REVOKED200/PROCESSED sobre intento ya aprobado/desacoplado → REJECTED/REJECTED. Replay200 deduplicado; Approved tardío200/IGNORED, sin resurrección.
- Nuevo ingreso al callback confirma estado durable y texto Verificación rechazada. Una sola declaración/un solo create; tres GET de estado. Storage/URL sin PII, exterior0, logout204/sesión401/cookies ausentes.
- SQL READ ONLY: un REJECTED desacoplado sin ciphertext; cero revisiones; una auditoría y claim mínimo histórico retenido; tres eventos/hash32 (dos PROCESSED, uno IGNORED); outbox propia ACK+retiro≥30d. Cleanup y baseline50/0 comprobados.
- Backup `pre-n08-approved-withdrawal-20261109.dump`, 8324619 bytes,0600, SHA `e9697d51d1cd5858aa22e68145c8b7b0547fe26102a4825ae910f71750a91acf`; listado/parse PASS.

## Estado posterior y límites

Readback independiente posterior a K/L: target/OID/motor exactos, ledger91; audit/outbox DELETE=false, columnas audit SELECT temporales=false, owner CREATE=false; outbox50 filas/0pendientes. Preflight de ambas cuentas sin artefactos KYC PASS; journals/control propios retirados después de confirmar metadata y limpieza. Cuentas, credenciales privadas y backups se conservan para los gates de cierre.

K/L cierran sus transiciones sintéticas, no todo N08 por sí solos. Actualización posterior: M completó reintento explícito tras TTL y N completó purga100+1 rollback-only con readback0; evidencias dedicadas y `cierre_native_n08_20261109.md` registran el cierre global. No proveedor externo, biometría real, cumplimiento legal, worker programado/SLA, aprobación humana, publicación ni despliegue.
