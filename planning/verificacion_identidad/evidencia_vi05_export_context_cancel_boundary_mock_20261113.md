# VI-05 — cancelación en límites de exportación propia (mock local)

**Fecha:** 2026-11-13
**Rama / HEAD base:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`

## Cambio

`Service.ExportOwnArchive` ahora comprueba la cancelación del contexto antes de leer la fuente de identidad y después del retorno exitoso de cada fuente, antes de iniciar la siguiente lectura o generar el archivo ZIP. Si una dependencia devuelve éxito aunque su contexto haya sido cancelado, el servicio devuelve `context.Canceled` y retorna `archive == nil`.

La prueba `internal/privacy/identity_archive_cancellation_boundary_test.go` usa repositorio y módulo sintéticos que ignoran deliberadamente la cancelación y devuelven datos de prueba. Verifica tres límites:

- Contexto cancelado al entrar: cero lecturas de identidad y cero lecturas de módulos.
- Cancelación después de que la fuente de identidad devuelve éxito: una lectura de identidad, cero lecturas de módulos.
- Cancelación después de que la fuente de módulos devuelve éxito: una lectura de cada fuente y `archive == nil`.

## Evidencia de prueba

- **RED observado:** antes de los guardas, la corrida focal falló en los tres casos; cada uno recibió ZIP (838, 839 y 840 bytes) y `nil` como error, pese a la cancelación.
- **GREEN:** `gofmt -w internal/privacy/service.go internal/privacy/identity_archive_cancellation_boundary_test.go` — salida 0.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=20 -v ./internal/privacy -run '^TestExportOwnArchive(SkipsSourcesWhenContextIsAlreadyCancelled|StopsAfterIdentitySourceCancelsContext|StopsAfterModuleSourceCancelsContext)$'` — salida 0; los tres casos pasaron en las 20 repeticiones.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=1 ./internal/privacy` — salida 0.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go vet ./internal/privacy` — salida 0.
- Revisión source-only independiente `deleg_b8dc30a3` — PASS, sin hallazgos bloqueantes ni errores lógicos; no ejecutó pruebas.

## Alcance y límites

Prueba unitaria del servicio con dependencias sintéticas. La aserción verifica el retorno `archive == nil`; no inspecciona internamente la construcción del ZIP. Si una fuente devuelve un error propio al mismo tiempo que se cancela el contexto, ese error de fuente conserva precedencia y se propaga antes de consultar `ctx.Err()`; ese caso simultáneo no está cubierto aquí (véase `evidencia_vi05_export_source_error_cancel_precedence_mock_20261113.md`). La limitación original de cancelación posterior al último check fue abordada con un fence después de `zip.Writer.Close()`; véase `evidencia_vi05_export_archive_assembly_cancel_mock_20261113.md`. Ese fence descarta el ZIP al retornar si observa cancelación, pero no interrumpe el armado. No demuestra autorización HTTP efectiva, comportamiento de adaptadores reales, persistencia, privacidad de archivos en producción, ciclo de retención ni limpieza durable. No se usaron SQL/DDL, base persistente, Cloud, proveedores externos, navegador ni datos personales reales.

`internal/privacy/service.go` también contiene cambios locales preexistentes de validación de nombres de archivo; esos cambios se conservaron y no se atribuyen a esta evidencia.
