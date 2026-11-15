# N08-F — Unicode, RUN-DV-K y reanudación activa (2026-11-09)

## Baseline y revisión

- Checkout `espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD base `274f37e483a078890e92382a159fa49fd23c2fe8` más árbol dirty preservado.
- Probe probado `frontend/tooling/kyc-native-flow.mjs`, SHA-256 `6b230119312faeacee698bdcdd5006bb65fb800a32723ff1ba26da2568eabaa1`.
- Mismo build Next N07/HTTPS4219 y fake con contadores19091; API harness18090, PostgreSQL nativo `espacigo_dms_stage_20261020`/OID16388/motor180006. Dos cuentas existentes autorizadas, una declaración y un intento del ensayo.
- Entrega source-only `deleg_31ba2038`: variante opcional `KYC_NATIVE_UNICODE=1` y active-resume con otra key dentro del modo de idempotencia. Todos los marcadores de readback/storage/B se derivan de la identidad seleccionada y se pasan serializados al navegador.
- Padre detectó antes de mutar una referencia a `document` ejecutada en Node y una expresión coma introducidas en el check de retornoB; lo corrigió por lectura DOM vía Playwright y asignación/navegación separadas. La advertencia no era preexistente: tras corrección ESLint focalizado quedó sin errores/advertencias. No se relajó ningún assert ni producto.
- Un preflight anterior no llegó a ejecutarse porque el nuevo helper de journals estaba en edición concurrente y faltaba un import; se suspendieron compiles hasta recibir fuente estable. No hubo conexión ni escritura DB por ese fallo. Después de la entrega estable se repitieron suites y preflight PASS.

## Gates y ejecución

Node syntax/Prettier/ESLint/TypeScript PASS. Go sin caché/race en cmdapi/verificación/adaptador/fake, vet y diff-check PASS; antiguos native lifecycle continúan omitidos. Preflight nativo READ ONLY PASS.

Backup nuevo `.local/testing/kyc-native/pre-n08-unicode-20261109.dump`, 0600, 8,322,837bytes, SHA-256 `cda474efdb70bfa14613a0ef78ac0eeb1a0ccf54172436c7add8683a5213abdc`; TOC y consumo completo PASS. No restore.

Prepare/finalize versionados por sudo directo parental. `run_kyc_n07_bff.py unicode` habilitó FLOW/WEBHOOK/IDEMPOTENCY/UNICODE y la expectativa SQL del harness. No cambios de esquema ni perfil runtime, sin proveedor externo/worker general.

## Resultados reales

| Subcriterio | Resultado |
| --- | --- |
| Formulario con tildes, Ñ y guion en identidad sintética | Save201 y readback200, caracteres preservados |
| RUN de fixture con DV-K ingresado minúsculo y puntuación | Lectura autoritativa normalizada con Kmayúscula y resumen con formato esperado |
| Recarga/reingreso A | GET200, igualdad exacta de nombres/apellidos/fecha/RUN normalizado y render confirmado |
| A→B | Formulario inicial de B vacío; GET404 y retorno404/retry sin marcadores de A |
| Guardar/sin consentimiento Didit | Cero creación de sesión según contador autenticado del fake |
| Primer inicio | 201 y delta de creación1 |
| Replay de la misma key/body | 201, mismo intento/URL, delta aún1 |
| Reanudación del intento activo con otra key y mismo body | 201, mismo intento/URL, delta aún1 |
| Webhook/autoridad/retorno | Negativos401/401/404 y válido/replay200; IN_REVIEW por decisión fake autoritativa, sin badge de aprobación |
| Logout final | 204, sesión401 y cookies ausentes |
| Storage/URL y exterior | Sin campos seleccionados en storage/URL; cero peticiones browser externas inesperadas |

Reporte: identityVariantEnabled=true, identityVariantReadbackVerified=true; replaySameAttempt/replaySameRedirect y activeResumeSameAttempt/activeResumeSameRedirect=true; providerCreatesAfterStart/Replay/ActiveResume=1.

Expectativa SQL READ ONLY antes de cleanup (flag del modo webhook): un intentoIN_REVIEW y un eventoPROCESSED/hash32 en el ámbito confiable. **probe_exit=0, harness_exit=0**, `TestNativeKYCAPIWithBFF` PASS11.79s (paquete12.876s con race).

## Cleanup y readback independiente

Cleanup journaled y finalizador COMMIT. Readback READ ONLY posterior: ledger90; audit/outboxDELETE=false, auditSELECT=false, ownerCREATE=false; outbox50/pendientes0 estable. Preflight posterior PASS, ambas cuentas sintéticas conservadas y sin artefactos propios; journals A/B ausentes, markers retirados y API18090 detenida. No se eliminaron ACK históricos por prefijo.

**N08-F completa para normalización Unicode/DV-K y active-resume descritos.** No demuestra todas las combinaciones Unicode, carreras multiinstancia, nueva revisión/supersesión, TTL del reloj ni aceptación real. C01/C04 se componen con cobertura de unidad/crypto y pruebas nativas anteriores; no se les atribuye biometría ni sandbox externo. N08 global sigue parcial.

## Prerrequisito acotado para C09

Se recibió y revisó `cmd/api/identity_native_pair_journal_test.go`: selector test-only de exactamente dos cuentas, un journal exclusivo por cuenta, rutas fijas, rechazo de inputs/dir inseguros y de cuenta desconocida mediante el error real `identity.ErrForbidden`. El wrapper individual anterior no se modifica ni permite segunda escritura de la misma cuenta. Padre reforzó pruebas de Aexitoso/BErrConflict con journal previo y denegación de Create/GetRevision para desconocidos; race/vet focalizados PASS. Fuente del helper tras entrega inicial: SHA-256 `55a17673d2cd8234e71bbca5061554cc81866bce7ddf3b9809fd815207ad653f` corresponde al snapshot inicial, no a las mejoras parentales posteriores.

La integración de ese selector y el conflicto visible B409 está en implementación source-only separada. No se invocó el helper contra DB en N08-F, ni se declara C09 cerrada. El siguiente gate debe verificar rollback de B antes de limpiar, ambos journals e igualdad del baseline outbox, sin nueva cuenta ni DELETE amplio.
