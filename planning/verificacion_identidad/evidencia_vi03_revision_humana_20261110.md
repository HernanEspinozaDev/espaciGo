# Evidencia — VI-03, revisión humana con fixtures sintéticos

**Fecha:** 2026-11-10
**Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`
**Rama / HEAD observados:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`
**Estado:** implementación y pruebas locales con mocks sintéticos completadas; el revisor autorizado validó el gate local de código/fixtures el 2026-11-11. La semántica de fecha/zona sigue como limitación antes de integración o uso real. No cierra el producto ni demuestra persistencia PostgreSQL.

## Implementación incluida

- La cola lista casos revisables mediante grants `review_identity` con scope `identity_subject` ligado al titular; excluye autocontrol, casos reemplazados y estados no revisables. El detalle exige el mismo actor/grant/scope, vuelve a autorizar después de cargar la declaración y compara revisión/estado antes de entregarla.
- El detalle de identidad expone solo nombre, apellido, RUT y fecha de nacimiento guardados; la cola no incluye esa declaración. La lectura de archivos conserva el control de autorización, titular, caso, estado limpio, retención, tamaño y hash.
- La UI muestra las evidencias en visor privado; las Blob URL se revocan al cerrar, cambiar caso/sesión o decidir. Casos, checklist, formato observado y fechas quedan aislados por selección.
- La revisión requiere resultado explícito para cada criterio. No se aprueba con criterio faltante/negativo, formato `undetermined`, vencimiento ausente o expirado. Se guarda por separado el vencimiento observado y la vigencia que calcula el servidor; el cliente no decide vigencia.
- La decisión incluye observaciones allowlisted en el evento, actualiza el caso y emite el evento en la misma transacción del repositorio. El fake comprueba orden, argumentos y commit; esto no prueba la transacción de PostgreSQL.
- Se preparó `V000096__identity_reviewer_observation_audit.sql`, registrada en `db/release-policy.json` como `security-tightening`, `automatic: false`, checksum SHA-256 `3a11c71c0eb098881247e5e4926eea25648d42d71ed5755984d3f6f411e314e4`. La migración es fuente solamente: no se ejecutó ni aplicó.
- Se actualizó el test de normalización de creación identity que aún esperaba el payload anterior sin consentimiento, para que afirme el contrato vigente de formato declarado y consentimiento sintético explícito.

## Verificación local ejecutada

- Go, con `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `IDENTITY_TEST_DATABASE_URL`, `DATABASE_URL` y `PGPASSFILE` retirados: última repetición `go test ./internal/documentverification/... ./internal/adapters/postgres/documentverification ./internal/migrator -count=1` — PASS; `go test -race ./internal/documentverification/... ./internal/adapters/postgres/documentverification -count=1` — PASS; `go vet` de los paquetes afectados y `go build -o /dev/null ./cmd/api` — PASS.
- Revalidación frontend del 2026-11-11: `tsc --noEmit` y `api-check` — PASS; inventario vigente de 199 operaciones. ESLint focalizado sobre la feature y tests VI-03 — PASS, 0 errores y 1 advertencia `@next/next/no-img-element` en la vista del Blob privado. Prettier focalizado en archivos VI-03 — PASS.
- Los comandos de ESLint/Prettier globales no pasan en el worktree compartido: ESLint encuentra 1 error (`no-assign-module-variable`, línea 19) en el archivo preexistente no seguido `tooling/account-canonical-navigation.test.mjs`, más 6 warnings; Prettier reporta ese archivo y artefactos bajo `.local/test-results-*`. No se modificaron esos artefactos ni el test fuera de alcance.
- Suite Node completa `node --test tooling/*.test.mjs` — PASS: 493 tests, 492 pass, 0 fail, 1 skipped.
- `node_modules/.bin/next build` — PASS en esta revalidación.
- Playwright Chromium, `tests/phase-document-review.spec.ts`, sobre Next standalone en `127.0.0.1:4203` — PASS, 7/7 en esta revalidación. La primera ejecución histórica expuso dos locators ambiguos; se limitaron al elemento de evidencia “anverso” y la repetición pasó. Las rutas `/api/backend/**` se simulan en la suite: no demuestra comunicación con Go ni PostgreSQL.
- Python, desde `scripts/gcp/`: `test_migration_release_manifest` y `test_check_migration_ci` — PASS, 12/12. Se comprobó el checksum de V96 contra `review_required_migrations` y el modo manual (`automatic: false`).
- `git diff --check` — PASS después de actualizar el estado/evidencia de esta validación.

## Seguimiento de revisión independiente

- La revisión independiente de fuente reportó cero preocupaciones de seguridad, pero no aprobó validación integral mientras la zona/semántica del vencimiento observado no esté definida. Señaló además cobertura negativa de auto-revisión/grants. El revisor autorizado validó después, el 2026-11-11, el gate local limitado a código y fixtures sintéticos; esta validación no sustituye la definición contractual para uso real ni evidencia PostgreSQL.
- La afirmación de que la UI envía medianoche UTC no coincide con el código: `observedDocumentExpiry` serializa la fecha de `<input type="date">` como `23:59:59.999Z`, y Playwright comprueba ese valor. Aun así, OpenAPI declara `date-time` y no explica si es una fecha civil chilena o un instante UTC; no se cambió esa semántica sin decisión de producto. La pregunta de aclaración no recibió respuesta dentro del plazo.
- Se revalidó que `LoadDetail` y `Download` rechazan `SubjectID == actor` antes de leer la declaración/los bytes. Se añadieron regresiones sintéticas para self-review en detalle y descarga, grant ausente, actor ajeno, capability incorrecta y capacidades mezcladas. `go test ./internal/documentverification/...`, `go test -race ./internal/documentverification/...` y `go vet ./internal/documentverification/...` — PASS con variables de integración retiradas. La consulta del repositorio filtra grants revocados/expirados en fuente; el comportamiento efectivo no se comprobó contra PostgreSQL.

## Addendum — descarga privada rechaza propósito no reconocido (2026-11-11)

- RED: `TestReviewerAccessDownloadRejectsUnknownPurposeBeforeReadingStore` se ejecutó con el propósito sintético `unsupported`, grant identity válido para el titular y evidencia limpia fake. Antes del guard, `Download` devolvió `synthetic evidence` y el store registró owner/case/file: la ruta elegía identity por defecto para cualquier propósito que no fuera `space`.
- GREEN: `ReviewerAccessService.Download` ahora limita el propósito a `PurposeIdentity` o `PurposeSpace` antes de evaluar la ruta de lectura. El mismo test verifica `ErrNotFound`, cero bytes y store sin invocar; pasa tras el cambio.
- `env -u ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL -u IDENTITY_TEST_DATABASE_URL -u DATABASE_URL -u PGHOST -u PGPORT -u PGDATABASE -u PGUSER -u PGPASSWORD -u PGPASSFILE go test -race ./internal/documentverification/... -count=1` — PASS; `go vet ./internal/documentverification/...` — PASS; `go build -o /dev/null ./cmd/api` — PASS; `git diff --check` — PASS.
- Alcance: servicio Go en memoria con fake repository/store; no prueba que un propósito inválido pueda existir en PostgreSQL ni la autorización/lectura integrada. No ejecuté SQL/DDL, consultas/conexiones a bases persistentes, migraciones, Cloud, proveedores o datos reales; no hubo commit, push, integración a `main` ni deploy. VI-03 conserva pendientes de semántica temporal e integración/persistencia.

## Addendum — 401/403 durante preview purgan estado privado (2026-11-11)

- `frontend/tooling/document-review-facade.test.mjs` carga y afirma en la lista el caso sintético, lo selecciona, genera un preview Blob y luego hace fallar la siguiente lectura de metadatos con `ApiError(401)` o `ApiError(403)`. Cada caso afirma lista vacía, caso/preview ausentes, mensaje allowlisted sin el texto crudo, revocación adicional en el `privateBlobUrls` fake y solo la descarga binaria previa (ninguna descarga tras la denegación).
- Suite `node --test tooling/document-review-facade.test.mjs` — 28/28 PASS; ESLint focal, Prettier focal y `git diff --check` — PASS. El test pasó en primera ejecución: es caracterización del comportamiento existente, no RED/GREEN ni modificación productiva frontend.
- La prueba transpila la fachada real y sustituye sus dependencias de puerto/registro de Blob; no prueba revocación nativa del navegador, BFF→Go, autorización efectiva ni persistencia. Sin SQL/DDL, bases persistentes, Cloud, proveedor o datos reales.

## Addendum — aprobación rechaza cada criterio negativo (2026-11-11)

- Se amplió `TestTypedIdentityApprovalRejectsAnyFailedChecklistItem` a `TestTypedIdentityApprovalRejectsEveryFailedChecklistItem`, con un caso identity sintético completo independiente por cada criterio: `names_match`, `rut_matches`, `birth_date_and_majority_match`, `portrait_matches_selfie`, `evidence_legible`, `document_unexpired` e `issue_date_and_format_verified`. Un solo valor false debe devolver `ErrInvalid` y conservar el Snapshot completo y el audit fake sin cambios.
- El test focal pasó en primera ejecución; no se modificó producción ni se afirma RED/GREEN. `go test ./internal/documentverification/... -count=1`, `go test -race ./internal/documentverification/... -count=1`, `go vet ./internal/documentverification/...` y `git diff --check` — PASS; variables DB/PG retiradas.
- Es prueba unitaria del servicio con repositorio en memoria; no demuestra commit/rollback ni restricciones PostgreSQL. Sin SQL/DDL, conexiones a bases, Cloud, proveedores o datos reales.

## Addendum — descarga de espacio conserva scope permitido (2026-11-11)

- Tras la revisión read-only `deleg_953236bd`, que aprobó el guard para `unsupported` y observó que el archivo inspeccionado no mostraba un caso positivo explícito de descarga `space`, se añadió `TestReviewerAccessSpaceDownloadUsesAssignedSpaceScopeAndStore`.
- Con fixture sintético, `CapabilitySpace` y `ScopeID` exactos, la prueba confirma owner distinto del revisor, lectura vía `ReadSpaceDocument` con owner/space/case/file exactos, y preservación del body/MIME tras validar tamaño/hash. Test focal, `go test -race ./internal/documentverification/... -count=1`, `go vet ./internal/documentverification/...` y diff-check — PASS; variables DB/PG retiradas.
- Caracterización del camino `space` en servicio/fake store; no demuestra autorización/repositorio PostgreSQL ni descarga real. No se ejecutó SQL/DDL ni hubo conexión a bases, Cloud, proveedor o datos reales.

## Addendum — purga visible ante 401 y 403 del preview (2026-11-11)

- `tests/phase-document-review.spec.ts` mantiene el caso de cambio de expediente, crea un preview Blob sintético y luego captura la respuesta real del browser test para el `GET /api/backend/document-verification/reviewer/cases/{id}` denegado. Comprueba método, path y status exactos (401 y 403 parametrizados), y que el caso/lista, declaración, archivos, formulario, checklist, controles y visor desaparezcan; registra `URL.createObjectURL`/`URL.revokeObjectURL` en la página y comprueba que el URL creado se revoque. No ocurre otra descarga binaria tras la denegación.
- Playwright Chromium focal con Next standalone local en `127.0.0.1:4203` — 2/2 PASS; interceptores sintéticos para `/api/backend/**`. ESLint, Prettier, `tsc --noEmit` y `git diff --check` focal — PASS. Para 401, el comportamiento observado de la app es cambiar a sesión no autenticada y mostrar el acceso de reingreso; para 403 conserva sesión y muestra la alerta allowlisted. La primera versión del test exigía alerta para ambos y falló en 401; la expectativa se ajustó al comportamiento real sin modificar producción. El servidor de ensayo se detuvo al terminar.
- Evidencia browser/UI con API simulada; la instrumentación confirma llamada a la API nativa `URL.revokeObjectURL`, no prueba liberación física de memoria ni BFF→Go, autorización backend, PostgreSQL o persistencia. Sin SQL/DDL, DB, Cloud, proveedor, datos reales, commit, push o despliegue.

El servidor Next temporal se inició con un entorno explícito sin credenciales ni variables de base y se terminó tras Playwright. Las pruebas del repositorio usan fakes; no se habilitó ningún DSN de integración.

## Límites y trabajo pendiente

No se ejecutó SQL/DDL, no hubo conexión/lectura/escritura de base, ni acceso a Cloud, proveedores, datos reales, publicación o despliegue. Por tanto no se demuestra que V96 sea SQL válido en PostgreSQL, que sus CHECK/triggers/ACL funcionen, que caso y evento persistan atómicamente, ni que el estado sobreviva recarga/reinicio. Tampoco se ejecutó el recorrido real Next/BFF → Go → PostgreSQL ni almacenamiento de objetos.

El alcance de asignación implementado es el grant `review_identity` scoped al titular (`identity_subject`), no una entidad separada de asignación por ID de caso. Si el requisito de producto exige asignación individual por caso, eso requiere una decisión/capacidad adicional; no se presenta como existente.

La implementación y cobertura sintética del gate local VI-03 fueron validadas por el revisor autorizado el 2026-11-11. La semántica del vencimiento observado continúa sin decisión y debe resolverse antes de integración/uso real; no se altera la convención actual de fin de día UTC. VI-01/VI-02 siguen parciales por sus gates de DB; VI-04/VI-05 no se iniciaron. El worktree compartido permanece sucio y preservado: no se hicieron commits, pushes ni integración a `main`.
