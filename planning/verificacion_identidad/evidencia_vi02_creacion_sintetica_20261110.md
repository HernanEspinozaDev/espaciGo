# Evidencia — inicio del corte sintético de creación de caso de identidad

**Fecha:** 2026-11-10
**Estado:** parcial; el vertical slice permanece en curso.
**Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`
**Rama / HEAD verificados:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`
**Worktree:** compartido y preexistente; 316 entradas sucias en la observación inicial y 321 en el último estado consultado. No se limpiaron ni sobrescribieron cambios ajenos; el incremento no se atribuye íntegramente a este subcorte.

## Alcance del subcorte

- El contrato de creación `purpose=identity` exige formato declarado (`legacy`, `new` o `unsure`), confirmación separada y versión `identity-capture-local-v1`.
- Go valida esos campos y, en `CreateCase`, deriva la revisión de declaración vigente desde el titular autenticado bajo el lock por cuenta; el ID nunca viene del cliente. Si no hay declaración guardada, no ejecuta la creación.
- La vista de Mi cuenta presenta el aviso como expediente sintético local y deja deshabilitadas captura/carga, envío a revisión y descarga. La versión de aviso no autoriza datos personales reales ni captura de documentos.
- OpenAPI y los artefactos generados de tipos, inventario y allowlist reflejan el contrato de creación.

## Límite funcional que mantiene el corte abierto

Se preparó la fuente append-only V95 y se actualizó el repositorio para insertar y leer el vínculo caso→revisión, formato declarado y versión/fecha del consentimiento. La migración conserva casos históricos con metadatos totalmente NULL, limita el vínculo por FK compuesta al mismo titular, impide crear casos identity nuevos incompletos y añade barreras contra re-enlazar un caso abierto y contra purgar declaraciones aún enlazadas. La transición terminal limpia el vínculo; aprobación y estados abiertos lo conservan. El release-policy y el harness se actualizaron para V95.

Esto solo demuestra diseño/código fuente y pruebas automatizadas sin DB: **V95 no se ejecutó**, no hubo conexión PostgreSQL ni lectura de estado persistido. La prueba opt-in de integración actualizada está preparada para crear y releer el caso mediante el rol runtime y recorrer los prefijos fresh/upgrade, pero no fue ejecutada. Por tanto siguen sin probarse semántica real de CHECK/FK/triggers, persistencia tras recarga/reinicio, purga/concurrencia ni flujo BFF→Go→DB. No se aplicó SQL.

## Verificación ejecutada

- `frontend`: `node tooling/api-check.mjs` — PASS; inventario actual de 199 operaciones.
- `frontend`: `node --test tooling/document-verification-create.test.mjs tooling/verification-api-contract.test.mjs` — PASS, 6/6.
- `frontend`: `tsc --noEmit --pretty false`, ESLint focalizado y Prettier `--check` de los archivos del subcorte — PASS.
- `frontend`: `next build` — PASS en la validación previa del mismo código de producción; el cambio posterior fue solo del test.
- Playwright Chromium local, suite `tests/phase-f6-document-verification.spec.ts` — PASS, 3/3, contra Next standalone en `127.0.0.1:4203`. El caso identity comprueba el consentimiento, la creación mockeada, la ausencia de controles de captura y `externalRequests=[]`. Las solicitudes `/api/backend/**` están simuladas en ese test: no demuestra la ejecución real del BFF ni backend/DB.
- Go, con variables opt-in de DB y `PGPASSFILE` retiradas: `go test ./internal/documentverification ./internal/adapters/postgres/documentverification -count=1` y `go test ./internal/migrator -count=1` — PASS; `go test -race` de los dos paquetes de dominio/adaptador — PASS; `go vet` de esos tres paquetes y `go build -o /dev/null ./cmd/api` — PASS. El paquete migrator pasó sin DSN; sus subtests que requieren PostgreSQL no constituyen evidencia de integración.
- Python de release/harness: `scripts/gcp/test_check_migration_ci.py` (7), `test_migration_release_manifest.py` (5), `test_migration_execution.py` (7) y `test_main_release.py` (17) — PASS. La suma indica ejecuciones de esos cuatro archivos, no gates de release remota.
- El SHA-256 de `V000095__identity_document_case_declaration_binding.sql` coincide con el checksum registrado en `db/release-policy.json`; `git diff --check` — PASS.
- `TestIdentityV95CaseDeclarationBindingContract` — PASS como comprobación de fuente. El subtest PostgreSQL gated `api-schema-contract` ahora contiene casos sintéticos para falta de consentimiento, FK entre titulares, re-enlace de caso activo, bloqueo de borrado mientras está enlazado y liberación al cerrar; **ese subtest no se ejecutó**.
- La primera revisión independiente detectó que el ACL permitía re-enlazar un caso activo a otra declaración del mismo titular. Se añadió un trigger y una regresión gated. La segunda revisión encontró que el guard no manejaba explícitamente un estado NULL; se hizo fail-closed y se agregó el caso negativo. Su afirmación de que faltaban los casos SQL no coincidía con el árbol: `current_schema_integration_test.go` ejecuta `api-schema-contract` después de V95 y `api_schema_contract_test.go` contiene esos asserts. La revisión final y la aclaración acotada no identificaron defectos de fuente; la aclaración confirma que el `passed=false` previo era inconsistente con sus propios hallazgos. Los asserts siguen gated y no se ejecutaron.

## Operaciones y gates no realizados

No hubo conexión a PostgreSQL, consulta de ledger, migración, modificación de datos persistentes, uso de credenciales reales, tráfico a proveedor, cloud, commit, push, integración a `main` ni despliegue. Las pruebas de UI citadas son las ya registradas en la observación anterior; no se volvió a ejecutar navegador ni se amplió el alcance pendiente `contract-ui`. El proceso temporal Next se detuvo y se comprobó que el puerto 4203 quedó sin listener en esa observación anterior.

**Siguiente criterio en esa observación:** la revisión de fuente estaba cerrada sin defectos concretos. La única puerta del vertical slice era ejecutar los subtests V94→V95 y fresh en el harness para verificar FK, CHECK, triggers y recorrido real del repositorio. La instrucción vigente prohíbe aplicar SQL en este corte: no se ejecutará mientras esa restricción esté activa; por ello el vertical slice sigue parcial y sin prueba de persistencia. En esa observación, `contract-ui` seguía pendiente y fuera de este slice.

## Continuación — contrato de creación en Mi cuenta

**Fecha:** 2026-11-10 21:19 -03
**Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.

- La inspección confirmó que OpenAPI define la variante identity con formato declarado y consentimiento local separado; el cliente TypeScript se genera desde OpenAPI, el allowlist BFF incluye el `GET`/`POST` de casos y el adaptador de la feature normaliza un payload allowlist, rechazando campos extra y un ID de declaración aportado por el cliente.
- En `/mi-cuenta/documentos`, la creación identity exige seleccionar formato y confirmar el aviso sintético local. El detalle identity no presenta carga, envío ni descarga; la prueba Playwright también observa cero solicitudes externas. La ruta Playwright intercepta `/api/backend/**`, por lo que esa evidencia es solo de UI/mock.
- Corregí y amplié `frontend/tooling/document-verification-openapi.test.mjs`: ahora encuentra las variantes por `purpose` (en vez de suponer que ambas incluyen vencimiento) y verifica los campos obligatorios, formatos, versión y límites de consentimiento, y que no se exponga un ID de declaración ni vencimiento del solicitante en la variante identity.
- Amplié `frontend/tooling/backend-gateway-credentials.test.mjs` para comprobar que el BFF reenvía sin cambios el payload exacto de creación identity con Bearer de sesión, sin cookies del navegador. Es una prueba unitaria del handler BFF con `fetch` upstream sustituido, no integración con Go.
- Verificación actual: `node tooling/api-check.mjs` PASS (199 operaciones); tres archivos focalizados con `node --test` PASS (8/8); ESLint focalizado y `tsc --noEmit` PASS; Prettier de los dos tests PASS; Playwright Chromium local contra Next standalone en `127.0.0.1:4203` PASS (1/1, mock); `git diff --check` PASS. No se reconstruyó Next porque no se modificó código de producción.
- No hubo tráfico a Didit/u otro proveedor, conexión a API Go, PostgreSQL, DDL/DML, datos personales reales, publicación ni despliegue. V95 sigue sin aplicar y la persistencia/recarga continúan sin evidencia PostgreSQL.

**Estado:** el subcorte `contract-ui` queda verificado en su capa OpenAPI/BFF-generado/adaptador/Mi cuenta y pruebas locales, limitado a creación de expediente con aviso sintético y sin captura en la UI. VI-01/VI-02 siguen parciales por el gate PostgreSQL no ejecutado; captura real, carga, revisión humana y aprobación permanecen deshabilitadas/no implementadas.

## Continuación — revalidación local y ledger de solo lectura

**Fecha:** 2026-11-11
**Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
**Worktree:** compartido; 323 entradas dirty observadas y preservadas.

- Se revalidó `contract-ui` sin cambios de producción: `node tooling/api-check.mjs` PASS (199 operaciones); las pruebas Node focalizadas PASS (8/8); ESLint, TypeScript y Prettier focalizados PASS.
- Playwright local, proyecto `chromium-local`, prueba `identity case creation requires local notice and leaves capture disabled`: PASS (1/1) en `127.0.0.1:4203`. La prueba usa sesión sintética y simula `/api/backend/**`; verifica confirmación separada, cero POST antes del consentimiento, payload de creación, ausencia de controles de carga/envío/descarga y cero solicitudes externas. No demuestra BFF→Go/DB. El servidor temporal se detuvo y se verificó el puerto libre.
- Go sin variables de integración/credenciales DB: `go test ./internal/documentverification ./internal/adapters/postgres/documentverification ./internal/migrator -count=1`, `go test -race` de los paquetes de dominio/adaptador, `go vet` de los tres paquetes y `go build -o /dev/null ./cmd/api` — PASS. Los tests PostgreSQL gated no se ejecutaron.
- Preflight nativo READ ONLY con autenticación peer: `sudo id` PASS; destino `espacigo_dms_stage_20261020`, OID `16388`, motor `180006`, `postgres` como session/current user. La consulta en transacción `REPEATABLE READ READ ONLY` confirmó 94 versiones consecutivas V1–V94 y coincidencia exacta de nombre/checksum con el prefijo fuente; solo V95 está pendiente. No hubo DDL/DML ni inspección de filas de negocio.
- `git diff --check` PASS. Sin tráfico a Didit/proveedores, SQL aplicado, datos reales, cloud, publicación, integración a `main` ni despliegue.

**Estado actualizado:** `contract-ui` queda revalidado y completo dentro de su alcance local; la etiqueta pendiente del handoff no coincide con el código/evidencia actuales. El vertical slice sigue parcial: la restricción vigente prohíbe aplicar V95, por lo que FK/CHECK/triggers, persistencia y recarga/reinicio continúan sin prueba PostgreSQL. Captura real y revisión humana siguen fuera.

## Addendum — creación con formato declarado anterior (2026-11-11)

- `internal/documentverification/service_test.go` añade `TestIdentityStartAcceptsLegacyDeclaredFormat`. Con titular, revisión de declaración, consentimiento y formato válidos sintéticos, `Start` crea un caso identity `legacy`, captura en el snapshot el vínculo de declaración y el consentimiento server-side y no acepta vencimiento declarado por el solicitante.
- Verificación: test focal PASS; `go test -race ./internal/documentverification/... -count=1`, `go vet ./internal/documentverification/...`, gofmt y `git diff --check` PASS. Variables de integración/credenciales DB y `PGPASSFILE` retiradas. Solo se añadió cobertura; no cambió código productivo.
- Límite: fixture y repositorio en memoria; no demuestra admisibilidad/autenticidad ni vigencia de una imagen física, aprobación/revisión humana o persistencia PostgreSQL. Sin SQL/DDL, DB, migraciones, Cloud, proveedores, datos reales, commit, push, integración a `main` ni despliegue. VI-02 continúa parcial.
