# N07/N08 — reanudación del ensayo nativo (2026-11-09)

## Alcance y baseline

- Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- Rama: `codex/marketplace/fix/kyc-06-save-identity-first`.
- HEAD: `274f37e483a078890e92382a159fa49fd23c2fe8`; implementación en árbol dirty compartido, incluidos archivos nuevos. El SHA no representa por sí solo estos cambios sin commit.
- Se preservaron todos los cambios preexistentes. Sin commit, push, integración a main ni despliegue.
- Destino exclusivo: PostgreSQL nativo `127.0.0.1:5432/espacigo_dms_stage_20261020`, OID 16388, motor 180006. Dos cuentas sintéticas existentes autorizadas por política privada; no se crearon cuentas.
- N03, N05 y N06 mantienen sus cierres sintéticos documentados. N07/N08 no estaban cerradas: las revisiones source-only y el build no demostraban recorrido browser→BFF→Go→DB.

## Estado recuperado y ejecución parental

1. `git status --short`, `git branch --show-current`, `git rev-parse HEAD`: baseline anterior confirmado y cambios dirty preservados.
2. Inspección de listeners y `/proc` limitada a identidad/cwd/executable y configuración pública: frontend standalone propio en `127.0.0.1:4201`, proxy HTTPS en `127.0.0.1:4219`, proveedor falso en `127.0.0.1:19091`. API `18090` ausente. Estos listeners no son prueba funcional.
3. `python3 /home/nandev/.hermes/cache/scratch/launch_kyc_n08_http_test.py preflight`: PASS (`cmd/api`, 1.192s). Ejecuta preflight de arranque y de fixtures en modo explícito READ ONLY; valida política/receipts privados, ambas cuentas activas, destino/identidades/ledger y ausencia de artefactos de las cuentas. No habilita HTTP ni workers.
4. Se encontró un defecto adicional en el harness HTTP: `inet_server_addr()::text` puede devolver CIDR y el harness usaba `net.ParseIP`, a diferencia del preflight ya corregido. La regresión `TestNativeKYCRuntimeServerIP` reprodujo RED con `127.0.0.1/32` y `::1/128`. El helper ahora admite CIDR e IP simple, manteniendo rechazo de direcciones no-loopback. GREEN en la regresión/suite combinada. Es corrección del harness de prueba, no relajación del destino.
5. Regresión parental sin caché: `GOTOOLCHAIN=local go test -race -count=1 ./internal/verification/... ./internal/adapters/postgres/identitydeclaration/... ./cmd/api ./cmd/kyc-fake-provider`: PASS en los seis paquetes mostrados por Go. `go vet` sobre esos paquetes, `git diff --check` y `node --check` del probe: PASS. Los opt-ins nativos mutativos no se ejecutan mediante esta suite por defecto.
6. Backup nuevo previo a cualquier ventana de ACL: `.local/testing/kyc-native/pre-n07-resume-20261109.dump`, archivo privado 0600, 8,320,594 bytes, SHA-256 `d278115f9ea671654ed98fafb27ce4abc5e23981e74e72e563a1ad6c22e7767b`. `pg_restore --list` y consumo completo `pg_restore --exit-on-error --file=/dev/null`: PASS. Comando `sudo` directo en terminal Hermes, sin leer/imprimir la contraseña. No se restauró la base.

## Revisión y gates pendientes

- El cleanup ya dejó el patrón fijo N06 y deriva el scope desde intentos/sesiones de la declaración journaled. Se solicitó nueva revisión independiente de fuente sobre el estado actual, con especial atención al formato real de `agregado_id`, guard sin revisión/auditoría y conservación de outbox ajeno. No se acepta la revisión histórica condicional como aprobación del candidato actual.
- Antes del ensayo mutativo: resolver hallazgos concretos, repetir regresiones/preflight y confirmar la revocación garantizada de permisos temporales.
- Ejecutar prepare→readback de permisos→harness opt-in→probe HTTPS con auth normal sintética→stop/cleanup por journal→finalize→readback independiente, incluso ante fallo.
- El DELETE temporal de auditoría/outbox es un privilegio de tabla, no autorización SQL por fila; solo el helper revisado restringe los registros por journal. No elevar runtime.
- N07 sigue parcial hasta acreditar BFF/Go/DB reales; N08 sigue parcial hasta cubrir y mapear cada criterio C01–C11, incluyendo los estados/errores/retorno que no ejercita aún el probe básico.
- Consentimiento y retención no tienen aceptación legal externa demostrada. Sin usuarios/documentos reales, Didit externo, sandbox remoto, workers generales ni aceptación humana live.

## Primeros ensayos HTTPS: fallo reproducido antes de guardar

- Revisión independiente actual `deleg_f99c983a`: passed=true, sin hallazgos concretos de seguridad/lógica; limitada a fuente, sin DB/browser/archivos privados. Padre contrastó helper, scripts y formato de las dos rutas de outbox. No convierte por sí sola N07 en completa.
- Se repitió preflight READ ONLY: PASS. Se ejecutó `prepare_native_kyc_n07_http_cleanup.sql` y se releyeron destino/OID/motor y privilegios efectivos: owner DELETE audit/outbox=true, SELECT de columnas audit=true, owner CREATE=false, runtime DELETE outbox=false.
- Harness `TestNativeKYCAPIWithBFF` inició API propia; `/health/ready` 200. Primer probe falló en `identity-form` sin diagnóstico de status. Se detuvo por marker privado y harness PASS (27.62s), sin journal de declaración. El EXIT trap con `sudo -n` falló por autenticación no heredada; no se ocultó su exit1. Padre ejecutó finalizador directamente y verificó DELETE/SELECT temporales=false y owner CREATE=false.
- Se añadió salida diagnóstica del probe limitada a status/booleanos (sin cuerpos, credenciales, IDs ni PII). Segundo ensayo acotado con finalizador directo posterior: anonymous401, login200, cookies Secure/HttpOnly/SameSite=Lax/Path=/ correctas; GET inicial declaración415. Logout de finally204, sesión posterior401, cookies de sesión ausentes. Probe exit1; harness exit0 (5.65s), finalizador COMMIT. Sin save/intento de proveedor ni journal de declaración.
- Readback independiente posterior: audit DELETE=false, outbox DELETE=false, audit SELECT=false, owner CREATE=false, ledger90. API18090 sin listener. Marker stop propio retirado tras validar tipo/owner/0600 y ausencia de ready/journal. Preflight posterior PASS.
- Causa en fuente: `verification-api.ts` añade `Content-Type: application/json` a ambos GET (declaración y estado Didit), mientras el gateway valida correctamente `requestMediaTypes=[]` en operaciones sin cuerpo y devuelve415. No se modifica el gateway ni el contrato OpenAPI para admitir ese header inválido. Corrección TDD delegada exclusivamente al adaptador y su regresión ejecutable de contrato; build nuevo y repetición browser siguen pendientes.

## Corrección, recuperación y recorrido básico PASS

### Adaptador y build definitivo

- Entrega `deleg_065e65ab`, limitada a `frontend/src/features/verifications/verification-api.ts` y `frontend/tooling/verification-api-contract.test.mjs`: GET sin body/Content-Type, conservando same-origin/no-store; ambos POST conservan JSON e Idempotency-Key donde corresponde. Revisión parental de archivos y ejecución independiente: regresión de contrato 6/6 PASS.
- `tsc --noEmit` PASS; eslint global sin errores, con cuatro advertencias preexistentes (imagen, dos variables no usadas y expresión de tooling). Prettier de los dos archivos PASS. Suite `node --test tooling/*.test.mjs`: 484 casos, 483 PASS, 1 SKIP, 0 FAIL. El skip no acredita integración.
- Build desde snapshot nuevo sin `.env*`, entorno allowlist y dependencias locales: `next build` exit0, compilación/TypeScript/prerender PASS. Ruta: `/home/nandev/.hermes/cache/scratch/espacigo-kyc-n07-fixed-k653aibr`. Se detuvo solo el Next anterior tras verificar UID/executable/cwd/target; nuevo standalone propio en4201, proxy HTTPS4219 conservado. Readback del nuevo entorno confirmó ausencia de variables DATABASE/PG/IDENTITY/DIDIT. Health HTTPS200.
- SHA-256 del adaptador fuente y de su snapshot compilado, iguales: `cf58c42adf04e2272bd34e39e8012ee0609cc792f7be34b92f95e958b0b74da8`.

### Fallos reales y recuperación acotada

- El primer recorrido con el adaptador corregido guardó201 y leyó200 tras recarga; creó intento201 y navegación al proveedor falso. Falló el probe al intentar obtener el cuerpo del POST después de una navegación completa. No se contó como PASS.
- El cleanup transaccional también falló y conservó journal/artefactos. El test de recuperación opt-in reprodujo RED con SQLSTATE42P18: la query final de conteos había dejado `$1` sin referencia tras un cambio de scope, usando `$2`–`$4`. Se corrigió la numeración/parámetros sin ampliar filtros ni privilegios. `TestNativeIdentityCleanupPersistedIntent` PASS nativo (0.06s), con journal retirado solo tras commit/verificación. El PASS source-only anterior no había detectado ese defecto y no se presenta como garantía de ejecución.
- Antes de recuperar, backup privado nuevo `.local/testing/kyc-native/pre-n07-journal-recovery-20261109.dump`, 0600, 8,321,744 bytes, SHA-256 `48fb47b91dd28b98ec29a6cc43dba0f70476e673baefaebd20a35f80db4e93a0`; TOC y consumo completo PASS. Sin restore.
- Se aisló `response-body-unavailable` mediante código diagnóstico fijo, sin imprimir cuerpos/URLs/PII. Leer el body inmediatamente en la promesa tampoco bastó. El probe ahora retransmite únicamente el POST local exacto de intent mediante `route.fetch({maxRedirects:0})`, conserva la respuesta auténtica en memoria y la entrega intacta al navegador. No fabrica el resultado BFF/Go/DB; solo la página de captura Didit se intercepta con HTML falso local. Cada ensayo fallido posterior limpió únicamente su declaración journaled antes del siguiente, sin repetición ciega de una mutación incierta.

### Resultado final padre — N07 completa localmente

`run_kyc_n07_bff.py` final: **probe_exit=0, harness_exit=0**. `TestNativeKYCAPIWithBFF` PASS (11.61s, paquete12.691s con `-race`). Secuencia real HTTPS→Next BFF→Go→base nativa:

| Criterio del recorrido básico | Resultado |
| --- | --- |
| Anónimo denegado / login A normal | 401 / 200 |
| Cookies sesión y expiración | Secure, HttpOnly, SameSite=Lax, Path=/ |
| Declaración inicial / guardar / readback / recarga | 404 / 201 / 200 / 200 |
| Consentimiento Didit separado | Sin preselección; bloqueo visible y cero llamadas antes de aceptar |
| Crear intento / consultar intento propio | 201 / 200; UUID y estado pendiente válidos |
| Redirección | Host HTTPS/path aprobados, interceptada localmente; cero peticiones externas inesperadas |
| Logout A / sesión posterior | 204 / 401 |
| Login B / declaración propia | 200 / 404, sin PII de A |
| UI de B | Formulario vacío y consentimientos no marcados |
| Logout B / sesión posterior | 204 / 401 |
| Reingreso A / readback / render | 200 / 200; datos sintéticos guardados coinciden y se muestran |
| Logout final / sesión posterior / cookies | 204 / 401 / ausentes |
| Storage y URL | Sin los campos sintéticos seleccionados en localStorage/sessionStorage/URL; no es auditoría de IndexedDB/Cache Storage/cuerpos |

Cleanup pasó por journal; conteo global outbox comparado con baseline sin cambio. Finalizador directo COMMIT. Readback independiente posterior READ ONLY: destino/OID/motor correctos, ledger90, checksumV90 exacto; owner DELETE audit/outbox=false, audit SELECT=false, CREATE=false, runtime DELETE outbox=false. Outbox Didit total46, pendientes0: se preservan filas ajenas, no se exige vaciar la cola. Preflight fixture posterior PASS (ambas cuentas activas autorizadas, cero artefactos de declaración/intentos/eventos/claims/auditoría propios); journal ausente; markers propios retirados; API18090 sin listener.

Regresión Go posterior sin caché con `-race` y vet en verificación/adaptador/cmdapi/fakeprovider, y `git diff --check`: PASS. Fuente cleanup SHA-256 `5551bd1ca1743a7f485a25ceeaac00758a54d2302f153c52dbeac4753d28f128`; harness `6dd25a78fcbbc30688e283f7da5eafc31368bb201fa775312d03f9d7233ab1b4`; probe scratch final `31a3345a0d3d49ff9c14af2849fff4dabcfc6c81b57766f98355f59d50c36a58`.

**N07 cerrada únicamente para el recorrido sintético local descrito. N08 parcial:** faltan matriz nativa C01–C11, webhook/retorno visible y estados/errores no recorridos, móvil/Axe/capturas y reproducibilidad durable del tooling scratch. No se acredita reinicio completo de API con sesión ni sandbox remoto, biometría, aceptación legal/humana o producto global.

Recursos propios retenidos por el agente operador exclusivamente para continuar N08: Next del build citado en4201, proxyHTTPS4219 y fakeprovider19091. Base nativa y ambas cuentas autorizadas conservadas; backups privados retenidos. API harness detenido, sin grants temporales. Retirar solo esos procesos cuando termine el ensayo, tras comprobar su identidad; no tocar demos ajenas.
