# Preparación de prueba manual Didit Sandbox local — 2026-11-10

## Autorización y coordinación

Usuario: «continua con sub agentes gpt 6 luna para preparar el flujo asi lo pruebo en local cuando quede listo genera una guia para montar el front end y backend en local o montalos tu y dame la url».

Delegación comprobada por lectura filtrada de configuración: provider openai-codex/model gpt-6-luna. Padre revisa/prueba/monta; ejecutores con archivos exclusivos. Rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD base `274f37e483a078890e92382a159fa49fd23c2fe8`, dirty compartido preservado. No commit/push/main/despliegue/SQL/IAM por inferencia.

## Resultado requerido

Frontend local HTTPS y backend nativo worker-free disponibles para que el usuario haga registro→verificación de correo→guardar identidad→inicio explícito Didit Sandbox→retorno→resultado. El agente no crea sesiones ni captura datos/biometría del usuario. Preparar material de ensayo y avisar que Sandbox no concede elegibilidad real. El buzón es captura local, no entrega a un correo externo.

- API:127.0.0.1:18090, base autorizada espacigo_dms_stage_20261020/5432, api_login→runtime, passfile privado existente.
- Frontend Next standalone:127.0.0.1:4201 tras HTTPS127.0.0.1:4219.
- SMTP captura:127.0.0.1:11025; buzón manual127.0.0.1:11080, RAM acotada, sin correo/PII en archivos o stdout.
- Host público fijo didit.testingpage.store; cambiar solo origen del túnel propio a18090 al levantar backend, sin cambiar URL en Didit. Mantener única ruta webhook y catch-all404.
- Credenciales Didit existentes en worktree histórico: leer privadamente al ejecutar, no imprimir/copiar/rotar valores. Archivo privado0600 verificado.

## Cortes y aceptación

1. Perfil separado local-kyc-didit-sandbox-manual: guards target/entorno/host/origin, claves privadas/no overrides, composición durable KYC-only, sin workers; captura correo explícita; preservar perfil fake.
2. Retorno: callback servidor-configurado exacto frontend/verificaciones/retorno con attempt_id generado en servidor; validación y regresiones sin cambiar contrato público arbitrariamente.
3. Buzón: SMTP real local y enlace de verificación con token en fragment, UI escapada/loopback/no-store, cuotas/TTL, no relay.
4. Launcher reproducible: entorno allowlist separado Next/API, claves locales privadas persistidas, sin fallthrough exterior; TLS local autocertificado con instrucción manual, sin alterar trust global del usuario.
5. Gates: race/vet/tests de slices y combinado; build frontend; preflight DB READ ONLY, backup antes de habilitar nuevas escrituras manuales, readiness/auth/webhook inválido denegado, puertos/propiedad, lectura independiente Cloudflare y URL estable. No contar un rechazo firmado inválido como aprobación real ni usar fixture para suplir respuesta Didit.
6. Guía final con URLs/comandos exactos, certificado local, buzón y pasos manuales, estado esperado Sandbox y límites; preservar recursos para el usuario y documentar stop/restart.

## Preflight parental ya ejecutado

- Native READ ONLY:ledger94, cero declaraciones y cero claims existentes. No se consultaron PII/ciphertext. API passfile propio regular0600.
- Se generaron create-only keyrings manuales de propósito distinto AES/HMAC en `.local/testing/kyc-manual/`0700/archivos0600, versión manual-20261110, con bytes aleatorios; no se sobrescribió ninguna clave previa ni se rotaron secretos reales. Certificado local autofirmado SAN127.0.0.1/clave privada0600,30d; no trust global instalado. Deben conservarse mientras haya declaraciones/claims dependientes, no recrearlas al reiniciar.
- Frontend build producción real (`next build` con entorno allowlist, sin dotenv) PASS. No arrancado todavía.
- Workflow MCP READ ONLY de app806fe3f4-4c9b-4567-bf6f-ce8c3e688fce: cuatro filas; único publicado Compliance workflow IDa4b973d4-4578-44fc-8580-7554e39821ce/v1, grafo OCR→LIVENESS ACTIVE_3D→FACE_MATCH, CHL ID habilitada y mismatchREVIEW. No callback configurado. response_attributesnull y flags globales liveness/facefalse se conservan como observación; el grafo custom explícito tiene los nodos, no afirmar captura/decisión real ejecutada.
- El WORKFLOW_ID del archivo histórico apunta a **Free KYC**, listado como draft, no al Compliance publicado. Para el launcher usar explícitamente el publicado inspeccionado, sin modificar workflow/proveedor ni las claves. La aprobación estricta Go mantiene exigencias de módulos/datos; si el workflow real no satisface esas exigencias, no conceder aprobación por inferencia.
- La única lectura remota realizada hasta aquí fue workflow list/get, sin crear sesión ni consumir un recorrido.

## Estado inicial de trabajo

Subagentes: perfil/mailer API y buzón en implementación; descubrimiento frontend de solo lectura. Callback se implementará después del freeze del primer slice para no compilar sobre paquetes compartidos en modificación. Todavía no hay URL declarada lista para prueba manual.

### Avance recibido

- `deleg_df2cc0e9`: perfil separado y selección de mailer local implementados; race/vet cmd/api+credentials reportados PASS. Padre leyó guards y cableado; el delegado no conservó RED previo, por tanto no se presenta como TDD demostrado. Revisión independiente solicitada. Callback siguiente ya delegado con propiedad exclusiva, sin compilar el padre sobre el paquete en modificación.
- Descubrimiento frontend confirmó rutas registro/verificar-correo/mi-cuenta-verificaciones/retorno, tokens de verificación en fragment y la necesidad de captura de correo; sin cambios UI ni build del delegado.
- Launcher/proxy y callback en curso; buzón en entrega independiente. Padre realizó build frontend PASS y copió assets standalone. No servidores aún.
- Backup fresco previo a habilitación manual: `.local/testing/kyc-manual/pre-manual-sandbox-20261110.dump`, propio0600,8349249bytes,SHA256 `e2893deccc92c77bddcfebcf08ea87e5708038824a8c0bfde6247d5b784cca49`; listado/parse completo pg_restore exit0. No restaura ni respalda roles globales por sí solo.
- Callback primera entrega incompleta: padre reprodujo RED en fixtures Save (invalid verification input), comprobó que el campo CallbackURL no llegaba a la configuración runtime y redistribuyó solo el GREEN faltante. No contabilizar esa entrega como implementación terminada ni TDD de comportamiento demostrado.
- Buzón primera entrega fue leída; fallos concretos de lifecycle/errores de arranque y límite SMTP aplicado después de ReadString asignador quedaron para corrección en sus dos archivos exclusivos. No servicio montado aún.
- Padre ejecutó `node --test tooling/verification-api-contract.test.mjs tooling/verification-session.test.mjs tooling/verification-result.test.mjs tooling/verification-runtime-session.test.mjs`:28PASS/0FAIL/0SKIP. Son contratos y aislamiento sintéticos frontend, no prueba de proveedor ni recorrido manual. Advertencia Node MODULE_TYPELESS_PACKAGE_JSON no bloqueante; no se alteró package.json para ocultarla.
- Revisión fuente independiente `deleg_488a7a87` y contraste parental: el perfil dedicado no lanza workers, pero runLocalKYCOnly valida el nombre del perfil fake en vez del seleccionado; corregir el enlace antes del arranque. El guard inicial permite cualquier UUID/version positiva conforme al primer contrato, mientras el launcher fijará el publicado inspeccionado; endurecer también el guard al par aprobado. Se conserva separado de la entrega callback para no compilar sobre escritores compartidos. La advertencia condicional del revisor sobre política de clave no demuestra incompatibilidad: el launcher usa solo keyring y no ambos mecanismos, como exige el guard. No se necesitan cambios de file-loader por ese comentario.
- Callback corregido `deleg_19eef679`: padre ejecutó de nuevo `go test -count=1 -race ./internal/verification/... ./cmd/api`, vet mismos paquetes y diff-check, exit0. Service callback tiene cobertura de attemptID/omisión/rechazo; el test runtime aún era solo validación, por eso se asignó una prueba conductual del cableado junto con los dos guards finales. No se presenta como llamada Didit real.
- Launcher primera revisión parental rechazó ruta ROOT/manual/bin/api, Host reescrito al puerto interno4201 y lectura de secretos al arrancar frontend/proxy/mailbox. Corrección exclusiva delegada con regresiones. La inspección privada filtrada mostró cuatro asignaciones `export` (API_KEY, WEBHOOK_SECRET, BASE_URL, WORKFLOW_ID); parser debe soportar la forma literal sin source/eval, y el runtime sigue fijando host primario/workflow publicado en lugar de heredar los valores históricos.
- Cloudflare: GET previo confirmó configuraciónv1 exacta; PUT autorizado cambió únicamente origen8080→18090. GET independiente confirmó configuraciónv2, `didit.testingpage.store` + regex exacta `^/api/v1/verifications/didit/webhook$`→`http://127.0.0.1:18090`, catch-all404 conservado. Sin cambio DNS/hostname/claves/workflow. Aún no equivale a recepción de webhook: API no montada todavía.

### Montaje parental y último defecto acotado

- Buzón corregido: race/vet parentales PASS, binario construido y proceso propio PID103733/handleproc_68bf19462569, SMTP11025/HTTP11080. HTTPready/no-store, Host421 y SMTPbanner/QUIT PASS sin correo enviado. Receipt privado.
- Launcher corregido: Python11/Node2 parentales PASS. Frontend/proxy montados PID105137 y105185 con receipts privados y ausencia de variables backend/Didit comprobada sin imprimir entorno. Pantallas HTTPS200; primer paso registro renderizado por Playwright aislado sin envío. Un primer selector suponía «Crear» y tres campos; lectura real «Crea tu cuenta»/dos campos permitió corregir solo la prueba, no producto.
- API primer arranque falló porque la raíz del worktree tenía0775, rechazada por el guard del passfile. Se restringió solo esa raíz propia a0755, sin bajar el guard ni cambiar claves. Nuevo test opt-in `TestManualSandboxRuntimePreflight`, con el entorno real del lanzador y sin escrituras/proveedor: perfil/config/AES/HMAC/DB login API→runtime en READ ONLY/construcción servicios PASS. API arrancada PID112826/handleproc_f54589459389; receipt inicial privado. No se debe reutilizar ese receipt tras el reemplazo pendiente.
- APIready200, authterms200, declaración anónima401, términos por BFF HTTPS200. Curl externo raíz404/webhook no firmado401 y servicio túnelactive. urllib había devuelto403; no se cuenta como el mismo resultado de cliente.
- Revisión parental detectó que el mux dedicado heredado no montaba registro/verificación de correo. Se delegó agregar solo POSTregister/verification/status/reissue +OPTIONS exclusivamente al perfil manual, preservando fake y sin exponer auth genérico/admin/password. Falta entrega, regresión y reemplazo del binario propio. La guía queda marcada pendiente hasta ese gate.
- La prueba conductual runtime→HTTPcallback sigue pendiente pese a regresiones service/cableado fuente PASS; no se vuelve a presentar una entrega parcial como ese criterio completo.

### Entrega del entorno manual

- `deleg_917ec0ed` completó las cuatro rutas solo para manual; revisión parental y regresión final `go test -count=1 -race ./cmd/api ./cmd/local-kyc-mailbox ./internal/verification/... ./internal/adapters/credentials`, vet mismos paquetes y diff-check PASS. El preflight privado es opt-in y SKIP por defecto; su ejecución READ ONLY real está documentada arriba.
- Binario nuevo se construyó como api-next; se verificó UID/executable del API propio112826, SIGTERM y liberación18090 antes de reemplazarlo. API final PID120498/handleproc_8ddc1ddc41cf, receipt privado `api-process-final.json` con SHA del binario. No se tocó otro proceso ni se reinició la base.
- Probes finales: ready200, términos por BFF HTTPS200, cuatro POST de onboarding con JSON incompleto400, falta Origin403, declaración anónima401 y perfil ajeno404. Curl externo repetido: raíz404 y webhook no firmado401. Primer probe GET llevaba Content-Type indebidamente y fue corregido en la prueba; no se amplió la allowlist para aceptarlo.
- Entorno entregable para que el usuario pruebe, guía `guia_prueba_manual_sandbox_local_20261110.md`. No se declara recorrido completo aprobado ni cierre global: no cuenta/identidad/sesión nueva por el agente, callback runtime conductual aún pendiente, R2metadatos/R3/R4 se conservan separados. Recursos retenidos para el ensayo humano; HEADbase274f37e483a078890e92382a159fa49fd23c2fe8 y dirty compartido, sin commit/push.
