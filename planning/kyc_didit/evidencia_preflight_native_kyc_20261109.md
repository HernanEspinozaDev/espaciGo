# KYC — preflight de ledger nativo y prueba de persistencia pendiente (2026-11-09)

## Alcance

- Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`
- Rama: `codex/marketplace/fix/kyc-06-save-identity-first`
- HEAD observado: `274f37e483a078890e92382a159fa49fd23c2fe8`
- Operaciones: inspección local de solo lectura, pruebas Go unitarias y análisis estático. No se ejecutaron API, workers, Didit, migraciones, DDL ni escrituras KYC.

## Preflight local

Consulta autenticada como `postgres` del sistema operativo, mediante `sudo -u postgres psql`; la consulta de catálogo se ejecutó dentro de `BEGIN TRANSACTION READ ONLY` y la conexión se cerró sin cambios.

- Base: `espacigo_dms_stage_20261020`
- OID: `16388`
- PostgreSQL `server_version_num`: `180006`
- Identidad de preflight: `session_user=postgres`, `current_user=postgres`
- Ledger: 85 filas; versiones V1–V85.
- Comparación automatizada, en orden numérico, de cada versión, nombre y SHA-256 del ledger contra los archivos locales V1–V85: **85/85 coincidentes**. Inventario local observado: 88 archivos de migración.
- V85: `V000085__identity_encrypted_declarations_and_minimal_events.sql`, checksum `7b78094558e2d9bde7804919b01c54d34caf2ac31ac34fb1b86a63999aeeaf51`.
- Membresías verificadas: `espacigo_api_login` puede `SET ROLE espacigo_runtime`; `espacigo_migrator_login` puede `SET ROLE espacigo_schema_owner`. Ambas aristas tienen `SET TRUE`, `INHERIT FALSE`, `ADMIN FALSE`. Los roles inspeccionados no son superuser ni tienen `CREATEDB`/`CREATEROLE`.

Este resultado describe solo el destino nativo local y el código fuente del checkout indicado; no establece estado de Cloud SQL ni prueba la autenticación de los dos logins.

## Cambios de prueba en el checkout

`cmd/api/identity_declaration_runtime_test.go` ahora:

- Rechaza DSN de aplicación distintos de `espacigo_api_login` o con password, host/socket, puerto, base o parámetros no permitidos.
- Mantiene el preflight de identidad del destino por socket peer, de solo lectura.
- Compara todo el ledger con nombres y checksums del inventario fuente V1–V85 y con el checksum fijado de V85.
- Exige `session_user=espacigo_api_login`, `current_user=espacigo_runtime` en cada conexión de aplicación.
- Usa un DSN separado de `espacigo_migrator_login`, ejecuta `SET ROLE espacigo_schema_owner` y comprueba ambas identidades antes de limpiar únicamente las filas sintéticas KYC de la correlación probada.
- Conserva la integración como opt-in; requiere un UUID de cuenta sintética autorizado y las DSN exactas. No crea cuentas ni modifica credenciales.

## Verificación ejecutada

- TDD para el validador de ledger: RED observado por ausencia de `validateIdentityDeclarationNativeLedger`; luego los casos coincidentes, faltante, extra, gap, renombre y checksum drift pasaron.
- `GOTOOLCHAIN=local go test -v -race -count=1 ./cmd/api -run '^Test(ValidateIdentityDeclarationRuntimeDSN|ValidateIdentityDeclarationCleanupDSN|ValidateIdentityDeclarationNativeLedgerMatchesSource|IdentityDeclarationRuntimeFailClosedWhenUnconfigured|IdentityDeclarationRuntimeRejectsInvalidKeyrings|IdentityDeclarationRuntimeRestartReadbackAndRotation|IdentityDeclarationRuntimePostgreSQLSaveRestartReadback)$'` — **PASS**, `cmd/api` terminó en `1.095s`; la prueba `TestIdentityDeclarationRuntimePostgreSQLSaveRestartReadback` quedó **SKIP** por falta de autorización/cuenta de fixture.
- `GOTOOLCHAIN=local go vet ./cmd/api` — **PASS**.
- `gofmt -d cmd/api/identity_declaration_runtime_test.go` — sin diferencias.
- `git diff --check -- cmd/api/identity_declaration_runtime_test.go` — **PASS**.

## Pendientes y límites

- `.local/testing/test-account-policy.json` no estaba presente en este checkout al comprobarlo; por eso no se leyó ni se tomó un UUID de cuenta.
- La persistencia real guardar → pool/handler nuevo → lectura posterior no se ejecutó. La comparación completa del ledger se hizo por separado en un preflight de solo lectura, no mediante la prueba opt-in.
- La autenticación `espacigo_api_login` no quedó verificada; el intento anterior terminó en `fe_sendauth: no password supplied`. No se reinició ninguna cuenta ni se cambió contraseña, secreto o rol.
- No se aplicó V86–V88; no se tocó el ledger.
- No hubo commit, push, merge ni despliegue.
