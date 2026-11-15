# Revalidación de auditoría del fixture nativo — 2026-11-09

Rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD base `274f37e483a078890e92382a159fa49fd23c2fe8`, árbol sin commit. Complementa `evidencia_native_persistencia_20261109.md`; no convierte los PASS previos en aprobación de toda N03.

## Disposición por hallazgo

1. **Receipt y política:** son dos archivos y no una transacción atómica. El receipt se usa como journal recuperable. El código ya podía retomar un ID conocido y reconstruir su entrada de inventario tras leer la misma cuenta en DB; por tanto, no se confirmó la afirmación de que necesariamente quedaría irrecuperable. Sí se reprodujo otro fallo concreto del mismo límite: la implementación anterior escribía el receipt antes de rechazar un inventario lleno. La regresión observó RED `writes=1 receipt_changed=true` cuando debían ser cero. Se extrajo `persistNativeFixtureIdentity`, que valida identidad, inventario, duplicados, tipos y capacidad antes de cualquier escritura, conserva el ID conocido y después escribe receipt→política. Se agregó fsync del directorio tras rename además del fsync del archivo.
2. **RegistrationAttempted:** el informe citó líneas anteriores a la adición del guard. Al releer el candidato, el flag ya se persistía antes de Register y `validateNativeFixtureResume` ya impedía adoptar una cuenta sin intención o recrear un ID conocido ausente. `AuthenticationService.Register` retorna el ID junto al posible error de emisión/entrega posterior al commit (`internal/identity/auth_service.go:182`). La nueva prueba de ese caso confirma receipt pendiente antes de devolver ErrDelivery y cero VerifyEmail/reintentos de Register. No se ejecutó una falla de entrega contra la DB.
3. **Cleanup dependiente de response/header:** hallazgo válido. Se asignó corrección source-only exclusiva de `identity_declaration_runtime_test.go` y nuevo `identity_native_cleanup_test.go`: captura del input confiable antes de escribir, journal privado create-only y limpieza independiente de la respuesta. Todavía no se declara entrega ni prueba nativa de esa corrección.
4. **ID de respuesta como fallback de borrado:** hallazgo válido. En el mismo corte de cleanup se exige confirmar el objetivo por cuenta/ID y vinculación con el intento/auditoría antes de DELETE, sin permitir que una respuesta inconsistente amplíe el alcance. Pendiente de entrega/revisión y ensayo real.

## Pruebas verificadas del cambio receipt/política

`go test -race -v -count=1 ./cmd/api -run '^TestNativeFixture'` PASS; `go vet ./cmd/api` y diff-check PASS en ese momento. Incluye:

- Receipt pendiente antes de VerifyEmail y conservación ante fallo de verificación.
- ErrDelivery posterior al commit preserva el ID y no avanza a verificar.
- Guards de reuso/mismatch/cuenta conocida ausente/intención previa.
- Inventario lleno rechazado antes de mutar receipt o llamar al escritor (RED→GREEN).
- Fallo de escritura de política inyectado usando **archivos sintéticos temporales 0700/0600**: se recargan ambos archivos como una invocación nueva, se comprueba que la política vieja sigue intacta y el receipt conserva el ID; se reconstruye una sola entrada y se actualiza verified sin duplicarla ni perder otros campos. Es prueba de recuperación de archivos, no de crash físico ni de transacción distribuida.

La prueba posterior de reuso del fixture nativo no llegó a iniciarse: coincidió con edición del archivo del subagente y la compilación falló en `identity_native_cleanup_test.go:121` (`expected '(', found nativeIntentInput`). Se notificó al implementador y se suspendió la compilación concurrente. No se ejecutaron SQL, nuevos registros, migraciones ni grants por ese intento. El padre repetirá el ensayo solo sobre la entrega congelada y compilable.

## Gates restantes

- Entrega funcional de cleanup → revisión padre y regresión combinada → ensayo nativo con preparación/finalización y readback de revocación.
- No declarar aprobado el candidato por los tests default del auditor: las integraciones nativas de esa invocación estuvieron omitidas.
- N03 sigue abierta por aislamiento entre dos cuentas en la misma base. N05–N08 conservan los pendientes del plan; V89 no se aplica por este cambio de fixture.

## Entrega de cleanup y revisión padre — candidato congelado

Se recibió `identity_native_cleanup_test.go` y la conexión del wrapper en el ensayo nativo. El input cuenta/declaración/correlación queda en journal create-only 0600 con fsync de archivo/directorio antes de delegar al repositorio real. Un ID existente, journal ocupado o error de persistencia impide la escritura; un resultado incierto conserva el journal. No se usa el ID de la respuesta para determinar el DELETE; se confirma la revisión con bloqueo y la auditoría por correlación antes de borrar. El journal se retira únicamente tras cleanup exitoso y se sincroniza su directorio.

La revisión padre detectó y corrigió dos fallos adicionales con RED conductual: el directorio symlink era aceptado (`writes=1`) y un pool de limpieza ausente devolvía éxito. Ahora el directorio debe existir, ser del operador, 0700 y no symlink; el pool/scope incompleto devuelve error. También se valida metadata del journal leído y su igualdad con el intent capturado en memoria; archivo faltante ya no se silencia. La prueba positiva comprueba el journal en el límite de la llamada de escritura del fake, no solo después de terminar.

Se retiró `FOR UPDATE` de la lectura de auditoría: habría requerido ampliar permisos con UPDATE que el contrato temporal no concede. Se mantiene el bloqueo de la revisión propia; no se modificaron las ACL versionadas ni se amplió runtime. Fakes no prueban semántica de PostgreSQL ni concurrencia.

Sobre la entrega ya estable, pasaron las suites combinadas `go test -race -count=1` de cmd/api, cmd/dbmigrate, internal/migrator, identitydeclaration e internal/verification/..., vet y diff-check. `TestNativeIdentityPrepareAuthorizedFixture` pasó de nuevo contra el destino nativo reutilizando la misma cuenta; queda superado el fallo transitorio de compilación anterior. No se afirma todavía prueba nativa del nuevo cleanup.

Backup privado fresco antes de la siguiente ventana: `.local/testing/kyc-native/pre-cleanup-journal-20261109.dump`, **8,290,062 bytes**, 0600/ext4, SHA-256 `22bbe0b7fa1b059a940c36f86889ef1e9b3854b8b246df11a6db316685f9f738`; listado y parse completo pg_restore exit 0, sin restore real. No se abrieron grants en esta preparación.

Se solicitó revisión independiente final del candidato congelado. Tras su resultado: ensayo nativo del nuevo cleanup con el par prepare/finalize y readback independiente de filas/grants/journal, antes de cerrar estos hallazgos o continuar el aislamiento de N03. No se declara aprobación por anticipado.

## Cierre de hallazgos de cleanup — revisión y ejecución nativa

La revisión independiente final devolvió `passed=true`, sin problemas de seguridad/lógica. Su límite explícito fue no ejecutar PostgreSQL: esa evidencia se obtuvo después por el padre, no por inferencia de los fakes.

En la ventana posterior al veredicto, con el backup indicado y el mismo par prepare/finalize versionado, se ejecutaron con `-race -count=1` y pasaron:

- `TestIdentityDeclarationRuntimePostgreSQLSaveRestartReadback` (0.24 s): guardado y lectura reales, journal previo y cleanup por intención confiable.
- `TestNativeIdentityPrepareAuthorizedFixture` (0.13 s): reuso de la cuenta propia, sin otro Register.

El finalizador se ejecutó por trap. Conexión nueva READ ONLY independiente confirmó: cuenta propia activa conservada=1; revisiones propias=0; intentos propios=0; auditorías propias de guardado=0; DELETE temporal=false y SELECT temporal de las cinco columnas=false; owner CREATE=false; runtime EXECUTE de purga=false; ledger88/88 filas. El journal ya no existe, tampoco como symlink. Diff-check PASS. No se creó cuenta adicional, aplicó V89, ejecutó worker ni llamó Didit/Cloud SQL.

Los hallazgos 3/4 de esta auditoría quedan resueltos para el ensayo nativo revisado, sin afirmar prueba de crash físico, concurrencia de producción, JWT/BFF ni UI. El slice siguiente de N03 —aislamiento de dos cuentas— fue asignado source-only, preservando recibos separados y el límite autorizado máximo dos; la segunda cuenta aún no se creó en esta ventana. Esta evidencia corresponde al candidato de cleanup anterior a ese siguiente cambio.
