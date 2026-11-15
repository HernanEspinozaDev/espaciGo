# VI-04 — caracterización del override de versión de consentimiento en Go

**Fecha:** 2026-11-13
**Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`
**Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`
**Estado:** test local sintético; revisión independiente source-only `deleg_be6e9eef` PASS, sin hallazgos.

## Alcance

Se agregó `TestIdentityDeclarationRuntimeUsesConfiguredConsentVersion` en `cmd/api/identity_declaration_runtime_test.go`. El test configura `IDENTITY_DECLARATION_CONSENT_VERSION` con una versión ficticia, usa un keyring temporal y `runtimeDeclarationRepo` en memoria, y ejecuta el handler/runtime Go con solicitudes HTTP de `httptest`:

- La versión por defecto enviada mientras el runtime está configurado con otra versión recibe `422`; el repositorio fake permanece sin revisiones.
- La versión sintética configurada recibe `201`; el fake conserva exactamente esa versión.
- Un GET posterior devuelve la misma versión configurada.

No se modificó código de producción. Es una caracterización del comportamiento de configuración vigente, no un cambio de política de consentimiento ni una prueba RED/GREEN.

## Verificación

`env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=1 ./cmd/api -run '^TestIdentityDeclarationRuntimeUsesConfiguredConsentVersion$' -v` — **PASS** (1 test).

El test usa únicamente archivos efímeros y un repositorio en memoria. No se habilitaron variables de conexión; no se ejecutó SQL/DDL, pruebas conectadas, migraciones, Cloud, proveedor externo ni sistema público.

## Límites

Esto no demuestra sincronización de la versión entre frontend/BFF y configuración del runtime, despliegue productivo, autenticación/autorización efectiva, PostgreSQL, persistencia/reinicio ni validez legal del aviso. Las pruebas previas con mocks tampoco cambian esos límites. No hubo commit, push, despliegue ni integración a `main`.
