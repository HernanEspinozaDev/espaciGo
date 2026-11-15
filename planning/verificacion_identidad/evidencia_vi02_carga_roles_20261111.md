# Evidencia — VI-02, carga frontend sintética por rol

- **Fecha:** 2026-11-11
- **Estado:** subcorte frontend verificado con mocks y re-revisado independientemente el 2026-11-11; VI-02 permanece parcial.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y previamente dirty; se preservó. Los archivos de feature y E2E ya tenían cambios VI-02 anteriores; este recibo atribuye solo el delta de roles/carga probado aquí.

## Alcance verificado

- La UI presenta selección para `identity_front`, `identity_back` y `identity_selfie`, recibe solo JPEG/PNG y muestra el estado de cada rol sin exponer nombres ni contenido de archivos.
- El botón de envío solo aparece cuando la respuesta de expediente contiene exactamente tres archivos, uno por cada rol y todos en estado `limpio`. Un fixture adicional devuelve cuatro archivos con IDs únicos y `identity_back` duplicado exactamente dos veces; aserciones del mock verifican los cuatro objetos y los IDs distintos antes de devolver la respuesta. La UI presenta «Revisar archivos» y no ofrece ni envía la revisión. Al reemplazar el rol duplicado con un archivo sintético, el estado pendiente sigue sin habilitar el envío; tras una lectura fixture limpia, la solicitud puede enviarse con el ETag nuevo. El servidor sigue siendo la autoridad; este gate UI no sustituye su validación.
- El rol se propaga por fachada/puerto y el adapter serializa `file` + `evidence_role` en multipart. La fachada sustituye en su snapshot solo el archivo anterior del mismo rol y conserva los otros roles cuando el upload devuelve una respuesta válida.
- El contrato de espacio conserva multipart de un solo campo `file`; una regresión E2E comprueba que no se añada `evidence_role`.
- El mock E2E simula la limpieza tras una recarga y confirma que el usuario puede enviar después de los tres roles. La revisión humana permanece en un flujo separado para revisores autorizados; el panel propietario no presenta el contenido de sus imágenes.

## Verificación local ejecutada

- Revisión independiente read-only `deleg_9148374c` encontró que el primer fixture duplicado reutilizaba el mismo ID de archivo; se corrigió usando un ID sintético distinto y se añadieron aserciones de cuatro archivos, cuatro IDs únicos y dos registros `identity_back`. La re-revisión `deleg_68d23ef7` confirmó esas aserciones antes de responder el GET y que la UI bloquea el envío hasta la lectura fixture limpia; no reportó defectos ni preocupaciones de seguridad en el alcance. Esto sigue siendo evidencia de UI con mocks, no de Go/CAS/persistencia.

- `node --test tooling/document-verification-facade.test.mjs` — PASS.
- `env -u ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL -u IDENTITY_TEST_DATABASE_URL -u DATABASE_URL -u PGPASSFILE node --test tooling/*.test.mjs` — PASS, exit code 0.
- `node_modules/.bin/tsc --noEmit` — PASS.
- ESLint focalizado en los archivos de feature y tests — PASS, sin salida de errores/warnings.
- `node tooling/api-check.mjs` — PASS; OpenAPI/types/allowlist/inventario vigentes, 199 operaciones. OpenAPI ya incluía `evidence_role` en el worktree por cambios VI-02 previos; no se modificó en esta continuación.
- Prettier focalizado (`--check`) — PASS.
- `node_modules/.bin/next build` — PASS.
- Playwright local Chromium, `tests/phase-f6-document-verification.spec.ts`, Next standalone en `127.0.0.1:4203` — PASS, 3/3. Incluye upload de los tres roles, rechazo fail-closed de un rol duplicado, reemplazo sintético versionado del mismo rol, no mostrar envío mientras queda pendiente, envío tras refrescar el mock, multipart identity y compatibilidad del multipart space.
- No se repitió Next build tras esta ampliación: solo cambió el test Playwright; no se modificó código de producción.
- `GET /api/health` del Next standalone — HTTP 200. Se detuvo el servidor propio y se verificó que el puerto 4203 quedó sin listener.
- `git diff --check` — PASS después de la actualización documental; una comprobación específica confirmó que este archivo nuevo no tiene espacios finales.

## Límites y decisiones no tomadas

- Playwright intercepta en el navegador las solicitudes `/api/backend/**`; no ejercita BFF→Go, autenticación real, PostgreSQL ni almacenamiento de objetos. La respuesta multipart del mock no demuestra que Go haya recibido o persistido los bytes.
- No se ejecutaron SQL/DDL, migraciones, conexiones ni consultas a bases, pruebas PostgreSQL, Cloud, proveedores externos o uso de datos reales. No hubo commit, push, integración a `main` ni despliegue.
- No están demostrados el rechazo backend de roles faltantes/inválidos, límites efectivos de multipart, inspección/cuarentena, reemplazo CAS persistido y revocación inmediata del archivo anterior, cifrado/AAD, retención/borrado, acceso privado ni supervivencia a recarga/reinicio. VI-02 y VI-01 continúan parciales.
- La UI conserva el límite de 10 MiB del servicio fuente (`internal/documentverification/upload.go:11`), mientras §6 del plan mantiene una propuesta no ratificada de 8 MiB. Esta diferencia requiere ratificación antes de captura real; no se resuelve con esta prueba sintética.
- La semántica de `document_expires_at_observed` sigue sin definir como fecha civil/zona local o instante UTC. El código conserva la serialización actual de fin de día UTC (`23:59:59.999Z`); no se reinterpretó ni se afirma aptitud para integración o uso real. La limitación ya está registrada en el plan vigente y en `evidencia_vi03_revision_humana_20261110.md`.
- La aceptación del mock local no es aceptación de producto, legal/privacidad, revisión de seguridad integral ni autorización de captura real.

## Próximo gate

Seguir únicamente con criterios locales de código/fixtures que no dependan de DDL ni datos reales. Antes de integración o uso real se requiere una decisión explícita sobre semántica temporal y sobre el límite de tamaño; VI-01/VI-02 aún requieren evidencia PostgreSQL y de recorrido BFF→Go, que están fuera de la autorización vigente.

## Addendum — AAD de evidencia de identidad rechaza sustitución de scope (2026-11-11)

- `internal/adapters/documentcodec/codec_boundaries_test.go` añade `TestIdentityCodecRejectsOwnerCaseFileAndPurposeSubstitution`: genera con AES-GCM un envelope de evidencia sintética bajo `IdentityScope`, confirma que abre con el scope original y comprueba el rechazo autenticado (`ErrCorrupt`, sin bytes) si se intenta abrirlo bajo otro owner, caso, archivo o propósito `space`.
- La prueba caracteriza el codec de aplicación real en memoria; no altera código productivo ni ejecuta filesystem/GCS. La primera ejecución pasó, por lo que no se afirma RED/GREEN.
- Gates con `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `IDENTITY_TEST_DATABASE_URL`, `DATABASE_URL`, `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, `PGPASSWORD` y `PGPASSFILE` retirados: test focal PASS; `go test -race ./internal/adapters/documentcodec -count=1` PASS; `go vet ./internal/adapters/documentcodec` y `git diff --check` PASS.
- Límites: el test no demuestra selección/rotación durable de claves, GCS/CMEK, autorización de lectura, límites de custodia, PostgreSQL ni persistencia. No se ejecutaron SQL/DDL, consultas/conexiones a bases persistentes, migraciones, Cloud, proveedor o datos reales; sin commit, push, integración a `main` ni despliegue. Checkout `/home/nandev/Documentos/projects/espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD `274f37e483a078890e92382a159fa49fd23c2fe8`; VI-02 continúa parcial.

## Addendum — falla sintética de inspección rechazada antes de almacenar (2026-11-11)

- `internal/documentverification/upload_test.go` añade `TestTypedIdentityUploadFailsClosedWhenInspectionErrors`. Un expediente de identidad tipado sintético con consentimiento y rol válidos llega al fake inspector, que devuelve un error diagnóstico sintético; `UploadService.Upload` devuelve `ErrInvalid` sin exponer ese error ni entregar recibo.
- La regresión compara defensivamente Snapshot y auditoría antes/después, y afirma cero `Put`/`Delete` en el fake de almacenamiento; también comprueba que el inspector recibió propósito, ID y bytes esperados.
- Verificación: test focal PASS; `go test -race ./internal/documentverification/... -count=1` PASS; `go vet ./internal/documentverification/...` y `git diff --check` PASS. Variables `DATABASE_URL`, `TEST_DATABASE_URL`, `ESPACIGO_DATABASE_URL`, `DB_*`, `PG*` y `PGPASSFILE` retiradas. Fue caracterización de comportamiento existente (sin RED observado), solo cambió el test.
- Límite: el fake representa un fallo del límite de inspección, no un motor antimalware, escaneo de firmas, cuarentena ni un resultado real limpio. No acredita almacenamiento duradero ni Go→PostgreSQL/GCS. Sin SQL/DDL, DB, migraciones, Cloud, proveedor, datos reales, commit, push, integración a `main` ni despliegue; VI-02 continúa parcial.

## Addendum — processor mantiene fail-closed ante scanner no disponible (2026-11-11)

- Se reejecutaron los tests existentes `TestProcessorScannerFailureOrStaleMetadataRetries` y `TestProcessorRejectsMalformedScannerMetadata` en `internal/adapters/postgres/documentverification/processor_test.go`. Los fakes controlan el job, lectura del objeto y resultado/error del scanner; los casos no disponible, firmas desactualizadas y metadata malformada exigen retry, sin `ScanOutcome`.
- Verificación exacta: `go test ./internal/adapters/postgres/documentverification -run '^(TestProcessorScannerFailureOrStaleMetadataRetries|TestProcessorRejectsMalformedScannerMetadata)$' -count=1` y `go vet ./internal/adapters/postgres/documentverification` PASS. Se retiraron `DATABASE_URL`, `TEST_DATABASE_URL`, `ESPACIGO_DATABASE_URL`, `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `IDENTITY_TEST_DATABASE_URL`, `DB_*`, `PG*` y `PGPASSFILE`. No hubo cambio de código en este subcorte.
- Alcance: prueba solo la decisión en memoria de `ScanProcessor` frente a respuestas sintéticas. No demuestra ejecución de ClamAV, claim/cola PostgreSQL, estado de cuarentena durable, acceso privado, objeto GCS, escaneo en recorrido BFF→Go ni persistencia. Sin SQL/DDL, DB, migraciones, Cloud, proveedor o datos reales; sin commit, push, integración a `main` o despliegue. VI-02 sigue parcial.

## Addendum — descarga del revisor rechaza archivos no limpios antes del store (2026-11-11)

- `internal/documentverification/reviewer_access_test.go` añade `TestReviewerAccessDownloadRejectsNonCleanEvidenceBeforeStoreRead`. Con grant identity exacto y expediente/archivo sintéticos vigentes, recorre `pendiente`, `bloqueado` y `error`; cada estado debe producir `ErrNotFound`, retorno vacío y dejar intacto el scope observable del store fake.
- Verificación: test focal PASS; `go test -race ./internal/documentverification/... -count=1` PASS; `go vet ./internal/documentverification/...` y `git diff --check` PASS. Se retiraron variables `DATABASE_URL`, `TEST_DATABASE_URL`, `ESPACIGO_DATABASE_URL`, `DB_*`, `PG*` y `PGPASSFILE`. No cambió código productivo.
- Límite: prueba la decisión fail-closed del servicio antes de una lectura fake; no prueba GCS, cuarentena/scan durable, autorización integrada o persistencia. Sin SQL/DDL, DB, migraciones, Cloud, proveedor o datos reales; sin commit, push, integración a `main` o despliegue. VI-02 sigue parcial.

## Addendum — envío identity rechaza cada rol no limpio (2026-11-11)

- `internal/documentverification/authorization_parent_test.go` añade `TestTypedIdentitySubmitRejectsEveryNonCleanRole`: con los tres roles identity y los demás datos sintéticos válidos, recorre las nueve combinaciones de rol (`identity_front`, `identity_back`, `identity_selfie`) y estado (`pending`, `blocked`, `error`). Cada envío debe devolver `ErrFilesNotClean`, conservando íntegramente Snapshot y auditoría fake.
- Verificación: test focal PASS; `go test -race ./internal/documentverification/... -count=1` PASS; `go vet ./internal/documentverification/...`, gofmt y `git diff --check` PASS. Variables DB/PG retiradas. Solo se añadió cobertura; el servicio ya rechazaba evidencia no limpia.
- Límite: caracterización del servicio con repositorio en memoria; no acredita transacción ni persistencia PostgreSQL, autorización integrada, scanner real o cuarentena durable. Sin SQL/DDL, DB, migraciones, Cloud, proveedor o datos reales; sin commit, push, integración a `main` o despliegue. VI-02 sigue parcial.

## Addendum — el ID reemplazado no queda descargable por el revisor (2026-11-11)

- `internal/documentverification/reviewer_access_test.go` añade `TestReviewerAccessCannotDownloadReplacedIdentityFile`. Un expediente identity sintético completo conserva los tres roles limpios y una revisión posterior; al solicitar el ID anterior, ausente del conjunto vigente, el servicio devuelve `ErrNotFound`, sin contenido/MIME y con cero lecturas del store fake. Luego el ID vigente sigue descargándose con contenido y MIME correctos.
- El fake `reviewerAccessStore` ahora cuenta lecturas identity/space; el test `TestReviewerAccessDownloadRejectsNonCleanEvidenceBeforeStoreRead` afirma explícitamente cero lecturas para `FilePending`, `FileBlocked` y `FileError`, atendiendo una sugerencia no bloqueante de la revisión `deleg_7e9b5ac8`.
- Verificación: tests focales de ambos casos PASS; `go test -race ./internal/documentverification/... -count=1` PASS; `go vet ./internal/documentverification/...`, gofmt y `git diff --check` PASS. Variables DB/PG retiradas. No cambió código productivo.
- Límite: el snapshot post-reemplazo y el store son fakes; esto caracteriza que el servicio no expone IDs ausentes del conjunto vigente, no integra `UploadService` con acceso de revisor ni demuestra CAS durable, borrado físico, scanner/cuarentena real o persistencia PostgreSQL. Sin SQL/DDL, DB, migraciones, Cloud, proveedor o datos reales; sin commit, push, integración a `main` o despliegue. VI-02 sigue parcial.
