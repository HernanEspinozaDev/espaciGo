# Evidencia VI-05 — cancelación durante el armado del archivo propio

- Fecha: `2026-11-13`
- Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`
- Rama: `codex/marketplace/fix/kyc-06-save-identity-first`
- HEAD: `274f37e483a078890e92382a159fa49fd23c2fe8`

## Cambio

Se añadió `TestExportOwnArchiveDoesNotReturnArchiveWhenContextIsCancelledDuringAssembly` a `internal/privacy/identity_archive_cancellation_boundary_test.go`. Las fuentes sintéticas completan con éxito; el callback de reloj `service.now` cancela el contexto después de los checks de las fuentes y antes de serializar el resultado. Sin un fence final, el servicio generaba y devolvía el ZIP pese a la cancelación.

Se observó RED antes del cambio productivo: el test devolvió `archive bytes=818 err=<nil>`. El cambio mínimo en `Service.ExportOwnArchive` consulta `ctx.Err()` después de `zip.Writer.Close()` y antes de devolver los bytes; así descarta el archivo si el contexto se canceló durante el armado.

## Verificación local

Bajo `env -i`, conservando `PATH`, `HOME` y `TMPDIR`:

- Focal antes del cambio: RED esperado, salida `archive bytes=818 err=<nil>`; exit 1 del test.
- Focal posterior, `go test -race -count=20 ./internal/privacy -run '^TestExportOwnArchiveDoesNotReturnArchiveWhenContextIsCancelledDuringAssembly$'`: PASS.
- Suite `go test -race -count=1 ./internal/privacy`: PASS.
- `go vet ./internal/privacy`: PASS.
- `gofmt -d` sobre `internal/privacy/service.go` y el test nuevo no produjo diferencias; `git diff --check` de los archivos del corte: PASS.
- Revisión source-only independiente `deleg_14ab2fbf`: PASS, sin bloqueadores; no reejecutó pruebas.

## Límites

El callback de reloj y ambas fuentes son sintéticos. La prueba muestra que una cancelación posterior a los checks de las fuentes no entrega el ZIP; el servicio termina el armado y solo la detecta en el fence final, por lo que no demuestra interrupción temprana del trabajo, cancelación de I/O real, adapters, autorización HTTP, PostgreSQL, persistencia, storage, retención o runtime. No se ejecutó SQL/DDL ni se accedió a DB/Cloud o proveedores externos; sin datos reales, commit, push, deploy o integración a `main`.
