# Evidencia VI-05 — precedencia de error de fuente ante cancelación simultánea

- Fecha: `2026-11-13`
- Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`
- Rama: `codex/marketplace/fix/kyc-06-save-identity-first`
- HEAD observado: `274f37e483a078890e92382a159fa49fd23c2fe8`

## Alcance

Se añadió `TestExportOwnArchivePreservesSourceErrorWhenSourceCancelsContext` a `internal/privacy/identity_archive_cancellation_boundary_test.go`. Los fakes de identidad y de módulos cancelan el contexto dentro de la llamada y devuelven un error sintético distinto de `context.Canceled`. La tabla cubre ambos límites: error de identidad (sin llamada posterior a módulos) y error de módulos (una llamada scoped al titular). En ambos casos se exige `archive == nil`, propagación detectable del error original y que la cancelación no lo sustituya como resultado.

Es una prueba de caracterización del orden actual: `ExportOwnArchive` devuelve el error de la dependencia antes de consultar `ctx.Err()` tras una respuesta fallida. No se observó RED: el primer run del test nuevo pasó; no hubo cambio de código productivo.

## Verificación local

Bajo entorno de proceso limpio (`env -i`, conservando `PATH`, `HOME` y `TMPDIR`):

- Test nuevo junto con las tres pruebas de límites de cancelación, `go test -race -count=20`: PASS.
- Suite `go test -race -count=1 ./internal/privacy`: PASS.
- `go vet ./internal/privacy`: PASS.
- `gofmt` y `git diff --check` de los archivos del corte: PASS.
- Revisión source-only independiente `deleg_6e889654`: PASS sin bloqueadores; el revisor no reejecutó tests. La sugerencia no bloqueante sobre permitir errores envueltos es consistente con la aserción `errors.Is`.

No se accedió a bases persistentes/Cloud ni a proveedores, y no se ejecutó SQL/DDL. No hubo commit, push, deploy ni integración a `main`.

## Límites

Los callbacks de cancelación y errores son sintéticos. El test caracteriza el servicio con repositorio/fuente fake; no demuestra cancelación de I/O productivo, adapters, autorización HTTP, PostgreSQL, persistencia, almacenamiento, retención ni comportamiento runtime. La aceptación del flujo completo VI-05 y sus gates persistentes/operativos siguen abiertos.
