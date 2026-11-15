# Cierre KYC y preparación de GCP

> **RETIRADO COMO PLAN OPERATIVO — 2026-11-10.** Decisión humana posterior: reemplazar la integración externa por [verificación propia de EspaciGo](../verificacion_identidad/plan_verificacion_identidad_espacigo.md). El contenido siguiente conserva evidencia e instrucciones históricas; no continuar su recomendación REST/SDK, sesiones ni release. Este cambio documental no retira servicios, secretos o datos.

## Encargo vigente

Usuario: «y prepara para subir a gcloud hay que resolver todos los pendientes que queden de kyc ademas no se cual version hacer por api rest o sdk web».

Autoriza preparación y resolución de pendientes locales; no se interpreta como ejecución inmediata de release, aplicación Cloud SQL, cambios IAM, promoción de tráfico ni prueba Live con datos reales. Mantener ensayo manual, sesiones y cambios ajenos. No commit/push/main por inferencia. Esta preparación complementa el plan KYC y el flujo canónico, no los sustituye.

## Arquitectura recomendada

**REST en Go + Web Redirect alojado para este cierre.** REST/SDK no son opciones excluyentes: el backend crea la sesión y recibe/verifica eventos; Web Redirect y SDK Web son alternativas de captura/presentación. El SDK incrusta el recorrido, no sustituye backend, webhooks, decisión autoritativa ni retención. No migrar ahora la UX a SDK para resolver obligaciones de datos/operación/cloud. Una futura incrustación mantiene el mismo backend y requiere su propio corte de permisos de cámara, móviles y CSP.

Fuentes oficiales verificadas mediante búsqueda actual: `docs.didit.me/integration/web-sdks/web-redirect`, `help.didit.me/integration/integration-options`, `docs.didit.me/sessions-api/create-session`. La extracción directa falló por backend search-only; no se presenta como lectura completa de SDK. Búsqueda oficial confirma creación backend POST/v3/session, URL retornada y webhook para actualización backend. No se investigan precios ni se crean sesiones.

## Base verificada

Worktree espaciGo-main-sync, rama codex/marketplace/fix/kyc-06-save-identity-first. HEAD274f37e483a078890e92382a159fa49fd23c2fe8; dirty compartido preservado (incluye cambios ajenos de pipeline/AGENTS). Fetch origin--prune ejecutado; origin/main actual5830c9ce2dc61d4df13a4139da984b4cafd6d9ba. **No actualizar el worktree por fast-forward con cambios locales**, no descartar ni mezclar main por inferencia. El release no puede tratar este HEAD base como el contenido dirty probado.

Autoridad release leída desde `.agents/skills/espacigo-release-flow/SKILL.md`: pipeline versionado Cloud Build/release_main.py, migración mediante Job privado y no bastión/SSH/DMS. Verificar fuentes canónicas completas antes de una operación cloud. Preparar código no es autorización de invocar apply ni levantar workers generales.

## Matriz inicial: no equivale a cierre

| Frente | Estado inicial | Gate pendiente |
|---|---|---|
| N08 y R1 técnico | Evidencia local histórica acotada, conservada | No reabrir como si nunca se probó; comprobar regresiones afectadas actuales |
| R2 rotación / ACK | V93/V94 aplicadas solo al destino nativo según evidencia | Reconciliar guards94 y retiro de metadatos desacoplados; no inventar V95 ni deadline |
| R3 operación | Sin cierre global | Login mínimo, scheduler, backlog/recuperación, alertas y prueba del plazo real; no arrancar consumidor compartido |
| R4 proveedor | Ensayo Sandbox real parcial + aprobación humana de UI | Live, privacidad/consentimiento/retención efectiva, aceptación y KYC14 separados |
| Estados y UI | REST y badge de prueba visibles, regresión focal | Pruebas pendientes de polling cap/hidden/A→B; review de consultas/mutaciones y recuperación/reinicio |
| Runtime cloud | Solo perfil manual loopback demostrado | Callback/origin reales configurable+allowlist, secretos privados, bindings y aislamiento Sandbox/Live |
| Exposición webhook | Túnel local acotado demostrado | Endpoint público cloud compatible con API privada/IAM, firma/rate-limit/replay, sin publicar toda API |
| Datos/SQL cloud | No consultado en este encargo | Prefix/checksums/owners y release-policy exactos, backup/plan revisado; no copiar DB local |
| Entrega | Dirty frente a main avanzado | Inventario de archivos propios/ajenos, revisión completa, QA local y release SHA reproducible antes de integración autorizada |

## Secuencia de trabajo

1. Auditar obligaciones R2/R3/R4 y runtime cloud frente a código y evidencias actuales (dos revisiones de solo lectura en curso).
2. Cerrar faltantes de código/pruebas locales en slices exclusivos con RED→GREEN, manteniendo la DB nativa seleccionada y sin nuevos datos reales. No relajar comparadores ni convertir resultado Sandbox en elegibilidad Live.
3. Decidir y documentar cualquier política de retiro de correlación incierta antes del delta SQL; no aplicar migración por el mero pedido de preparación.
4. Preparar configuración cloud del runtime/secretos/callback/webhook sin tocar recursos remotos. La excepción de presentación «Cuenta verificada — prueba Sandbox» permanece explícita y no concede permisos reales.
5. Ejecutar regresión amplia local y revisión independiente, separar tests unitarios/mock/native/proveedor y SKIPs. Artefactos reproducibles; no reconstruir un standalone vivo ni interferir sesiones humanas.
6. Reconciliar el trabajo con main actual sin reescribir historia, solo con alcance/autorización y gates correspondientes. El trigger puede desplegar al publicar main: no publicar para probar si funciona.
7. Preflight cloud de solo lectura cuando el corte lo requiera; release efectivo solo tras cierre de gates, permiso explícito y procedimiento canónico (plan privado→backup→migrador Job→postcheck→revisiones sin tráfico→promoción/readback). No se ejecuta en este documento.

## Resultado de esta apertura

Recomendación técnica fundamentada y preparación iniciada; **KYC no está declarado terminado ni listo para desplegar**. El siguiente cambio implementable se selecciona con los hallazgos de las revisiones, no con una lista inferida del historial. No se modificaron recursos GCP, workflow, DB, IAM, datos ni secretos.

## Contraste parental de las auditorías

- Revisiones de solo lectura recibidas: cierre R2/R3/R4 y readinessGCP. Se aceptan como inventario, no como ejecución ni cierre. La advertencia de guards94 históricos se contrastó con fuente actual: `nativeIdentityPreflightVersion=94`, constante runtimeledger94/checksumV94 y finalizadorN07 distingue93/94 y preserva dependencias del definer. Por ello **no repetir una modificación de guards ya implementada**; falta su gate nativo completo, no escribir otra vez la misma corrección.
- Readback parental nativo READ ONLY como APIlogin→runtime: funciónACK SECURITYDEFINER/schemaowner, runtimeEXECUTEfalse, workerEXECUTEtrue; ownerCREATEfalse; definerDELETE/UPDATEid/SELECTretirar true. Se confirmaron dependencias claves, no inventario ACL completo ni ensayo de limpieza/finalización. Sin DDL/grants/purga ni lecturas de datos personales. CheckerPythonmigración7PASS.
- Bloqueo cloud confirmado por fuente: release-policy permite automático soloV84expand; V85–V94security-tightening/automaticfalse. No cambiar etiquetas aexpand para saltar revisión. La evidenciaCloudSQL históricaV83del7nov no demuestra ledger actual ni autoriza aplicar.
- Callback configurado continúa limitado a localhost por runtime y servicio. Slice de código/test asignado para origenHTTPS canónico administrado por servidor, sin callback arbitrario de cliente, conservando perfil manual y seguridad de URL. No se modifica ningún dominio/secretoremoto por este slice.
- R2metadatos requiere plazo/tratamiento explícito para correlación de creación incierta y preservar resultado autoritativo de la cuenta antes de retirar sus filas; no inferir30dACK/5añosauditoría. R3LOGIN/scheduler/SLA y R4Live/privacidad/aceptación siguen gates distintos.
- Preparación siguiente: contrato cloud de callback +regresión, pruebas operacionales locales acotadas y validación de retención faltante. Decisiones/provisioning externos deben mantenerse visibles en la matriz y no presentarse como cerrados por pruebas frontend.

## Callback HTTPS — cierre del slice de código local

La primera entrega pasó regresiones pero conservaba el fallback local aun con origen explícito. La revisión posterior no se aceptó como GREEN: omitió pruebas de constructor/runtime y de solicitud saliente. El padre completó la frontera compartida `ValidateDiditCallbackConfig`, invocada antes de dependencias en runtime y en `NewDiditService`.

- `DIDIT_CALLBACK_URL` no vacío debe coincidir exactamente con `FRONTEND_PUBLIC_ORIGIN` más `/verificaciones/retorno`. Sin origen configurado se conserva exclusivamente el callback loopback histórico. Un origen explícito sustituye el fallback, no se suma a él. Callback vacío conserva compatibilidad.
- Rechazo de wildcard, hostname vacío, puerto vacío/fuera de rango, userinfo, query incluso `?` vacío, fragmento, ruta codificada y hostname con punto final. Perfil manual worker-free conserva sus guards exactos, sin tocar su proceso ni proveedor.
- RED parental: cinco orígenes malformados eran aceptados por el precheck runtime antes de construcción. GREEN posterior: pruebas de constructor con controles positivos local/cloud y negativos; `StartOrResume` usa origen HTTPS sintético y el ID del intento persistido, como único parámetro, sin valores de identidad en callback. Son dobles en memoria, no sesión remota ni persistencia PostgreSQL nueva.
- Verificación parental ejecutada: `go test -count=1 -race ./internal/verification/... ./cmd/api`, `go vet ./internal/verification/... ./cmd/api` y `git diff --check`, PASS. No atribuir estas pruebas a despliegue cloud ni a readback de configuración externa.
- Otro slice iniciado: conectar la función V94 al ciclo del worker restringido; el código previo solo llamaba purga de revisiones. Aún pendiente de entrega/revisión. No arrancar ese consumidor general en la base compartida para probarlo.

## V94 conectada al worker y preflight R3

- Entrega aceptada tras lectura y ejecución parental: `Repository.PurgeDeliveredSessionOutbox` llama exactamente a la función V94; el ciclo llama una vez a la purga de declaraciones y una vez al retiro de ACK. Solo el rol dedicado habilita esas llamadas; errores y lote completo tienen avisos estáticos independientes. Sin bucle de drenaje ni SQL nuevo.
- Regresión parental conjunta: `go test -count=1 -race ./cmd/privacy-cleanup-worker ./internal/adapters/postgres/identitydeclaration ./internal/verification/... ./cmd/api`, `go vet` en esos paquetes y `git diff --check`, PASS. Pruebas de helper/roles no prueban por sí solas scheduler, LOGIN ni recuperación del ciclo: slice sintético de esas fronteras en curso, sin arrancar `run()` ni consumidor real.
- Readback READ ONLY nuevo en destino nativo OID16388: worker NOLOGIN/no superuser/no createdb/no createrole/no bypassrls; `espacigo_privacy_worker_login` ausente y cero membresías hacia el worker. Es bloqueo real de autenticación R3, no contraseña rechazada. No se intentó login inexistente, no se creó/alteró rol ni se leyó secreto. Antes de aprovisionar necesita procedimiento acotado revisado y autorización correspondiente; el rol LOGIN tendría alcance de catálogo de clúster aunque su uso se limite a la base autorizada.
- R2metadatos todavía no se cierra: ACK V94 tiene ciclo, pero la retención de correlación incierta y el estado mínimo duradero de verificación no están sustituidos por esta función. No inferir un plazo universal del plazo ACK ni de auditoría.

## Ciclo periódico — evidencia sintética revisada

Se extrajeron `runIdentityRetentionCycle` y `runScheduler`, ambos llamados por el worker real. La prueba no invoca `run()` ni abre DB. La periodicidad se ejercita con callbacks sintéticos: ciclo inmediato, ticks posteriores, cancelación previa/dentro del callback, intervalo inválido sin invocación y ausencia de ciclos concurrentes por ejecución secuencial. La retención prueba recuperación tras fallo, continuidad del ACK ante error de declaración, una llamada por etapa/ciclo, roles, forwarding de contexto y mensajes estáticos sin errores crudos.

Verificación parental: `go test -count=20 -race ./cmd/privacy-cleanup-worker`, regresión conjunta cero-caché/race de adapter/verification/HTTP/cmdapi, `go vet` de los paquetes afectados y `git diff --check`, PASS. Las veinte repeticiones son suite sintética, no veinte ejecuciones nativas. No acredita LOGIN, alertas entregadas, backlog real ni SLA24h. Sin arranque/reinicio de servicios, cuentas, DDL/DML ni cambios remotos.

**Gate operativo siguiente:** preparar/revisar el bootstrap y obtener autorización específica para `espacigo_privacy_worker_login`→worker restringido antes de crearlo en el catálogo compartido. No usar login migrador/API como sustituto ni arrancar consumidores generales sobre datos retenidos. R2metadatos y R4externos siguen abiertos.

## Autorización de bootstrap local — 2026-11-10

El usuario respondió **«Autorizar el aprovisionamiento local revisado y la prueba de autenticación»** al alcance explícito: preparar, revisar y provisionar `espacigo_privacy_worker_login`→worker restringido para uso exclusivo en `espacigo_dms_stage_20261020`, reconociendo el catálogo compartido de clúster; **sin** consumidor general, purga de datos retenidos, CloudSQL ni despliegue. No autoriza cambiar roles existentes, sus contraseñas, ACL compartidas o pg_hba.

Preflight administrativo READ ONLY por socket: target/OID16388/engine180006 exactos; bases de clúster objetivo/postgres/template0/template1, dependencias worker solo en objetivo más su ACL compartida de base. API manual activa preservada. Ledger94 comparado completo con fuente, fingerprint94 exacto. Backup nuevo `pre-worker-login-20261110.dump`0600/ext4:8.353.643bytes/SHA256 `2333a813d3e33adc38e596cafff44f4352161b77536a4886418194e054282265`, listado+parse PASS. Archivo protege la base, no roles globales; baseline privado sin hashes/passwords conserva atributos/membresías/dependencias/ACL.

Bootstrap exact-target en preparación/revisión: crear únicamente el LOGIN ausente y una membresía ADMINfalse/INHERITfalse/SETtrue. No usar bootstrap de restore/GCP ni asignar privilegios directos adicionales. Antes de commit: ensayo transaccional con ROLLBACK y readback independiente; después: autenticación TCP real y SET ROLE, consultas READ ONLY y comparación de invariantes. No invocar purgadores durante este gate.

## Bootstrap y autenticación local — ejecutados y releídos

**Gate de LOGIN local completado, no operación R3 completa.** Fuentes: `db/repair/bootstrap_native_kyc_worker_login.sql.tmpl`, renderizador/pruebas `scripts/test_native_kyc_worker_bootstrap.py`, readbacks `db/repair/readback_native_kyc_worker_{login,auth}.sql`.

- Revisión parental corrigió separador de fingerprint SQL (`E'...\n'`), formato SCRAM real con componentes base64 de tamaños16/32/32 y cobertura positiva del renderizador, además de dependencia worker solo ACL57relaciones/1schema/6funciones/1base y membresía exacta en ambas direcciones. Cinco pruebas offline PASS. El helper GoV85 no se cambió: revisión independiente confirmó que es scaffold sin caller operacional y no un gate de este camino SQL versionado.
- Ensayo PostgreSQL18.6 con ROLLBACK PASS. Readback independiente idéntico al baseline privado registrado: roles, memberships, ledger, dependencias, owners/ACL de relaciones/columnas/funciones/schemas/defaults y ACL de bases. No se asumió éxito por salida del comando sin lectura.
- Aplicación por socket administrativo con lock/guard94/target/OID/engine y rechazo de LOGIN existente: **COMMIT exit0**. Creó solo `espacigo_privacy_worker_login`, LOGIN/NOINHERIT/NOSUPERUSER/NOCREATEDB/NOCREATEROLE/NOREPLICATION/NOBYPASSRLS, límite2; membresía única al worker con grantorpostgres/ADMINfalse/INHERITfalse/SETtrue. Sin grants directos nuevos ni cambios a roles existentes.
- Readback postcommit comparado programáticamente: exactamente un LOGIN y una membresía nuevos; baseline registrado restante y ledger94 exactos preservados. Autenticación TCP real SCRAM usando passfile privado0600, SET LOCAL ROLE worker y regreso a LOGIN tras ROLLBACK: PASS. Verificó permisos efectivos por catálogo, **sin invocar** funciones de purga. Negativo instalado: segundo bootstrap rechazado antes de DDL por LOGIN existente; autenticación posterior siguió PASS, sin reset de contraseña.
- Recibo privado `.local/testing/kyc-native/worker-bootstrap-20261110/result.json`; passfile `worker.pgpass` conservado0600. Se retiraron solo los dos SQL propios con verificador SCRAM; template y hashes/recibos sin secretos conservados. No se imprimieron password/verificador ni se pasaron por argumentos.
- La exclusividad es el destino operativo autorizado y fijado por las herramientas: **no se modificó** la política PUBLIC CONNECT ni pg_hba del clúster. No afirmar aislamiento de conexión a otras bases que no se implementó.
- No arrancó worker/API, no tocó cuentas/datos retenidos, no aplicó migraciones, no cambió proveedor/CloudSQL/GCP ni hizo commit/push. R3 sigue pendiente de ensayo nativo acotado del ciclo, backlog/alertas y SLA; R2metadatos y R4 permanecen abiertos.
