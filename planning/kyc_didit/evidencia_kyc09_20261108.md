# KYC-09 — avance local y gates pendientes (2026-11-08)

## Alcance y referencia

- Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`
- Rama: `codex/marketplace/fix/kyc-06-save-identity-first`
- HEAD probado: `274f37e483a078890e92382a159fa49fd23c2fe8` (trabajo sin commit)
- Solo datos sintéticos. No se inició API/Next, no se abrió navegador, no se llamó a Didit ni a red externa, no se conectó a PostgreSQL persistente o Cloud SQL, no se ejecutó DDL persistente, ni se publicó/desplegó.

## Avance implementado

1. Códec y validación de declaración con JSON estricto, RUT chileno canónico, fecha civil no futura, límites Unicode de nombres y AAD vinculado a cuenta, UUID de declaración, revisión y schema.
2. Repositorio V85 con revisión serializada por cuenta mediante advisory transaction lock, cifrado invocado después de asignar revisión, `INSERT`/commit, lectura actual y lectura exacta account-scoped. El rol runtime del harness no necesita UPDATE/DELETE.
3. Servicio de aplicación: consentimiento afirmativo y versión vigente inyectada por configuración, política de retención inyectada sin valor predeterminado, sellado/descifrado AES-GCM y éxito condicionado a lectura exacta posterior al commit. Timestamps normalizados a precisión de microsegundos PostgreSQL.
4. Handler HTTP GET/POST opt-in por inyección, autenticación por cuenta del principal, decoder estricto, respuestas allowlist sin ciphertext/clave/retención y `Cache-Control: no-store`. El guardado no inicia Didit ni modifica perfil público.
5. OpenAPI agrega GET/POST `/verifications/identity-declaration`; tipos, inventario y allowlist se regeneraron desde la fuente. Inventario generado: 202 operaciones.
6. Se añadió auditoría atómica de cada guardado durable mediante `evento_auditoria_local`, usando solo actor/recurso/correlación y códigos fijos, sin campos de identidad, ciphertext ni clave. Se reutiliza la retención fija de cinco años de ese ledger; no se tocaron migraciones ni grants.
7. La integración del harness PostgreSQL desechable cubre guardado/readback, revisiones concurrentes, cifrado, fallo de sellado y limpieza. También cubre rollback conjunto cuando falla la inserción de auditoría, readback con servicio/pool nuevos, y POST→GET HTTP→PostgreSQL usando autenticador sintético. La correlación del audit coincide con el `X-Request-ID` generado por el servidor.
8. El servicio niega la lectura de una declaración vencida según su reloj y `retention_at`; el test usa un plazo sintético, no una política de producto.
9. V86 añade una función de purga parametrizada cero, limitada a 100 filas, con reloj de PostgreSQL y `FOR UPDATE SKIP LOCKED`; elimina solo revisiones vencidas no referenciadas por intentos del proveedor. El repositorio Go invoca esa función. El runtime conserva `EXECUTE` específico, sin `DELETE` directo; pruebas desechables verifican lote parcial, filas futuras/referenciadas, concurrencia de purgas y carrera entre inserción FK y purga. No se añadió scheduler/worker ni se aplicó V86 fuera del harness.

## Pruebas verificadas

- TDD de códec, repositorio, servicio y handler: se observaron RED esperados antes de implementar; regresión adicional de precisión temporal también falló antes de la corrección.
- `GOTOOLCHAIN=local go test -count=1 -race ./internal/verification/... ./internal/adapters/postgres/identitydeclaration` — PASS.
- `GOTOOLCHAIN=local go vet ./internal/verification/... ./internal/adapters/postgres/identitydeclaration ./internal/migrator` — PASS.
- `bash scripts/test-migration-runner-postgres.sh full` — PASS: 117 resultados DB requeridos, ninguno faltante/omitido; ejecutado en contenedor PostGIS 18 efímero, red/puertos/volúmenes externos deshabilitados; V1–V86 fresh y upgrades V75/V76; eliminación del contenedor propio verificada.
- Generación local: `node tooling/api-generate.mjs`, `node tooling/generate-api-inventory.mjs`, `node tooling/generate-api-allowlist.mjs`, `node tooling/api-check.mjs` — PASS; 202 operaciones y contrato generado actual.
- `git diff --check` y `gofmt` de archivos Go del corte — PASS.
- `pnpm` no está instalado en PATH; se usaron directamente los scripts Node locales ya instalados, sin descargar paquetes.

## Gates abiertos — KYC-09 sigue parcial

- `cmd/api/main.go:388` todavía instancia `verificationhttp.NewHandler(...)`, no `NewHandlerWithDeclaration`; el servicio no está montado en el runtime y el endpoint queda 503 allí. No se configuró ni cargó un keyring de runtime. No iniciar el API contra la DB persistente para probarlo.
- A solicitud expresa del usuario se creó `consentimiento_declaracion_identidad_v1_20261108.md`: versión candidata `id-consent-20261108` (19 caracteres) y copy separado de Didit. El plazo propuesto es 30 días desde el consentimiento, con eliminación dentro de las 24 horas siguientes al vencimiento. V86 implementa el mecanismo durable en SQL y Go, pero no hay scheduler/worker ni configuración de política activa; requiere revisión legal/privacidad y un corte separado para decidir/implementar su invocación.
- La ruta de auditoría del guardado usa el ledger append-only existente con `retirar_en = ocurrido_en + 5 years`, sin nuevas ACL/DDL. El usuario aceptó expresamente ese plazo para los metadatos de este evento. La prueba confirma que los datos declarados no se escriben en `detalle_codigos`; los IDs de actor/recurso y correlación siguen siendo metadatos vinculables.
- El test recrea servicio y `pgxpool` y recupera la fila previa, además de POST→GET a través del handler contra el mismo PostgreSQL efímero. No terminó ni reinició un proceso `cmd/api`; este sigue sin inyectar el servicio.
- UI/adapter de formulario, lectura en `/mi-cuenta`, conciliación tras reload y recorrido visual quedan para la fase dependiente; no se hicieron cambios de UI ni se afirmó una función visible completa.
- No se llamó a Didit, no se crearon sesiones, no se inició KYC-10/11 y no se afirmó aceptación humana.

## Decisión registrada y límite pendiente

El usuario pidió crear el texto y regla candidatos y aceptó la retención de cinco años de auditoría. V86 y su prueba desechable ya implementan/verifican la primitiva de purga; esto no activa la regla de 30 días ni prueba eliminación operativa dentro de 24 horas, porque no existe scheduler/worker y no se ejecutó en destinos persistentes. Antes de activar el copy: revisión legal/privacidad; además, un corte separado debe definir/integrar la invocación acotada y verificar configuración/inyección local aislada y reinicio real del proceso. No mostrar el formulario ni recibir datos de personas hasta cerrar esos gates. No llamar a Didit. Cualquier aplicación de SQL a un destino persistente requiere autorización separada y gates del runbook vigente.

## Addendum de avance — 2026-11-08

Este addendum corrige el estado operativo anterior sin borrar su historial: la frase «No se añadió scheduler/worker» en el punto 9 y el gate de invocación en el punto 35 describen el estado previo a esta actualización.

- `cmd/privacy-cleanup-worker/main.go` ahora construye el purgador de declaraciones únicamente cuando `DATABASE_SET_ROLE=espacigo_privacy_cleanup_worker` y llama una vez por ciclo a `PurgeExpiredRevisions`; el lote permanece limitado a 100. No se cambió el intervalo del worker ni se ejecutó el proceso. Una única invocación/ciclo no demuestra SLA de 24 horas.
- `db/migrations/V000087__identity_declaration_retention_worker.sql` concede `EXECUTE` al rol dedicado, sin `DELETE` directo; su checksum registrado es `5eda043ddf3a3d5bb89e8d470eaf4e5450ed0964a9d59fe232bdcb37888b1d45`. `db/release-policy.json` la mantiene como `security-tightening`, `automatic: false` y review-required. V86/V87 no se aplicaron a destinos persistentes.
- Verificaciones registradas tras esta slice: `bash scripts/test-migration-runner-postgres.sh full` — PASS en el harness efímero; `GOTOOLCHAIN=local go test -count=1 ./cmd/privacy-cleanup-worker` — PASS; `GOTOOLCHAIN=local go vet ./cmd/privacy-cleanup-worker ./internal/migrator` — PASS; `python3 scripts/gcp/test_check_migration_ci.py` — PASS, 4 pruebas. No se ejecutó el API ni el worker.
- Sin cambio en el gate de exposición: `cmd/api/main.go` aún usa `verificationhttp.NewHandler(...)`, no inyecta la declaración ni carga keyring runtime. La propuesta de consentimiento/retención sigue pendiente de revisión legal/privacidad; no habilitar UI ni recepción de datos personales. El harness y estos tests no sustituyen la prueba de reinicio/readback del proceso API.
- Estado: KYC-09 sigue parcial/abierto. Pendiente: gate legal/privacidad; inyección segura del servicio/keyring en API y prueba local aislada/reinicio/readback autorizada; mantener V86/V87 sin aplicar a DB persistente hasta autorización y gates específicos. No se inició KYC-10/11/12, no se llamó a Didit, no se publicó ni desplegó.

### Revalidación local posterior — 2026-11-08

- `GOTOOLCHAIN=local go test -count=1 -race ./internal/verification/... ./internal/adapters/postgres/identitydeclaration` — PASS en los cuatro paquetes reportados.
- `GOTOOLCHAIN=local go vet ./internal/verification/... ./internal/adapters/postgres/identitydeclaration ./internal/migrator`; `GOTOOLCHAIN=local go test -count=1 ./cmd/privacy-cleanup-worker`; `GOTOOLCHAIN=local go vet ./cmd/privacy-cleanup-worker ./internal/migrator` — todos PASS (exit 0).
- `python3 scripts/gcp/test_check_migration_ci.py` — PASS, 4 pruebas. `bash scripts/test-migration-runner-postgres.sh full` — PASS, 120 resultados DB requeridos, ninguno faltante/omitido; V1–V87 fresh y upgrades V75/V76 a V87; contenedor PostgreSQL efímero sin red/puertos/volúmenes externos, eliminado al terminar. `git diff --check` — PASS.
- No se ejecutó `cmd/api` ni el worker, no se aplicó DDL a destinos persistentes y no se llamó a Didit. Los tests/harness no cierran revisión legal/privacidad ni reinicio/readback del proceso API; KYC-09 permanece parcial.

### Preparación de revisión legal/privacidad — 2026-11-08

- Se creó `revision_legal_privacidad_20261108.md` con una matriz de preguntas para asesoría, no un dictamen. Contrasta la versión vigente de Ley 19.628 al 2026-11-08 con el texto de Ley 19.628 modificado por Ley 21.719 y con vigencia desde 2026-12-01; incluye las disposiciones de menores/datos sensibles y distingue el cambio transitorio de Ley 21.806.
- Gate pendiente: asesoría debe aprobar o corregir texto/versión, finalidad/base y campos, responsable/canales, retención/purga/backups, conservación de auditoría vinculable y consentimiento separado de proveedor/biometría. La matriz no habilita exposición ni recepción de datos reales.

### Regresión del estado API deshabilitado — 2026-11-08

- `TestIdentityDeclarationRouteFailsClosedWhenServiceIsNotConfigured` verifica GET/POST autenticados: 503, `Cache-Control: no-store` y sin eco del cuerpo cuando el servicio es nil, como en el constructor `NewHandler` que monta actualmente `cmd/api/main.go`.
- `GOTOOLCHAIN=local go test -count=1 ./internal/verification/transport/http -run '^TestIdentityDeclaration(RouteFailsClosedWhenServiceIsNotConfigured|RoutesRequireAuthenticatedHandlerConfiguration)$'` — PASS. Prueba de handler solamente; no se inició `cmd/api`, no demuestra el estado de un proceso real ni sustituye su integración futura.
- Tras añadirla, se repitieron `GOTOOLCHAIN=local go test -count=1 -race ./internal/verification/... ./internal/adapters/postgres/identitydeclaration` y `GOTOOLCHAIN=local go vet ./internal/verification/... ./internal/adapters/postgres/identitydeclaration ./internal/migrator`; ambos PASS. `git diff --check` PASS.

### Hallazgo de compatibilidad de retención — 2026-11-08

- V85 define FK no nulable de `identity_provider_attempt` a la revisión; V86 excluye de purga cualquier revisión referenciada. El worker solo invoca la purga de declaraciones y no limpia intentos. El test de V86 conserva intencionalmente filas referenciadas. Si se crea un intento Didit, la copia candidata que promete borrar cada declaración dentro de 24 horas después de 30 días no se satisface por el mecanismo actual mientras la referencia exista.
- No es una falla de la prueba: es una decisión de compatibilidad/retención pendiente antes de KYC-10. Requiere asesoría/producto y diseño versionado de retención de intentos/snapshots o corrección del copy; no modificar V85/V86 ni aplicar DDL persistente por inferencia.

### Revisión estática de privilegio de purga — 2026-11-08

- V86 concede `EXECUTE` a `espacigo_runtime` y V87 lo conserva al añadir el rol dedicado. El único consumidor de producción encontrado es el worker de limpieza, que usa `espacigo_privacy_cleanup_worker`; el test de ACL confirma runtime `EXECUTE` y ausencia de `DELETE` directo.
- Para verificar la frontera se invirtió temporalmente la aserción de la prueba PostgreSQL: el harness falló exactamente con `runtime_exec=true`, manteniendo el resto del contrato (owner, SECURITY DEFINER, search_path fijo, worker `EXECUTE`, sin PUBLIC ni DELETE directo). El cambio de prueba fue revertido; no se agregó migración ni se tocó el ACL. El permiso está acotado a una función sin argumentos que borra revisiones vencidas/no referenciadas por lotes de 100, pero sigue pendiente confirmar su necesidad frente al principio de mínimo privilegio.

## Cierre de gates e implementación KYC-09 runtime — 2026-11-08

- **Decisiones aprobadas en firme:** el Responsable del Proyecto cerró los gates legales, de privacidad y de producto en `revision_legal_privacidad_20261108.md`, `propuesta_retencion_kyc09_kyc10_20261108.md`, `consentimiento_declaracion_identidad_v1_20261108.md`, `aviso_y_consentimiento_didit_v1_20261108.md` y `plan_kyc_didit.md`.
- **Regla estricta de mayores de 18 años (`internal/verification`):** se implementó `ValidateAdultBirthDate(birthDate, now)` en `internal/verification/declaration.go` y su validación previa al cifrado/persistencia en `DeclarationService.Save` (`internal/verification/declaration_service.go`). Cualquier persona menor de 18 años (fecha civil UTC) recibe `ErrInvalid` (`422 invalid_input` en HTTP) sin cifrar, sin invocar el repositorio y sin reflejar PII en la respuesta.
- **Montaje runtime en `cmd/api`:** se creó `cmd/api/identity_declaration_runtime.go` y se conectó `verificationhttp.NewHandlerWithDeclaration(...)` en `cmd/api/main.go` usando `identitydeclarationpg.New(pool)`, keyring 0600 cargado por `IDENTITY_DECLARATION_KEYRING_FILE` (con fallback a `LOCAL_IDENTITY_DECLARATION_KEYRING_FILE`), versión de consentimiento por defecto `id-consent-20261108` y política de retención de 30 días (`consent_given_at + 30d`). Cuando la variable de keyring no está definida, el endpoint permanece fail-closed (`503 service_unavailable`); si el archivo de keyring es inválido, relativo, symlink o con permisos distintos de `0600`, el arranque falla cerrado sin filtrar rutas ni secretos.
- **Pruebas ejecutadas y verificadas:**
  - `GOTOOLCHAIN=local go test -count=1 -race ./internal/verification/... ./internal/adapters/postgres/identitydeclaration ./cmd/api ./cmd/privacy-cleanup-worker` — PASS.
  - `GOTOOLCHAIN=local go vet ./internal/verification/... ./internal/adapters/postgres/identitydeclaration ./cmd/api ./cmd/privacy-cleanup-worker ./internal/migrator` — PASS.
  - `TestIdentityDeclarationRuntimePostgreSQLSaveRestartReadback` en `cmd/api` contra contenedor PostgreSQL 18 efímero (`network=none`, `SET ROLE espacigo_runtime` tras aplicar V84 y V85 bajo `espacigo_migrator_login` / `espacigo_schema_owner`): rechazo `422` de menor de edad, `POST /api/v1/verifications/identity-declaration` -> `201 Created`, cierre de pool y rotación de keyring (`v1` -> `v2`), nueva instancia de servicio/handler y `GET /api/v1/verifications/identity-declaration` -> `200 OK` con readback exacto — PASS.
  - `bash scripts/test-migration-runner-postgres.sh full` — PASS (120 resultados DB requeridos, ninguno faltante/omitido, incluyendo rechazo HTTP `422` de menor de edad en `assertV85IdentitySchema`).
