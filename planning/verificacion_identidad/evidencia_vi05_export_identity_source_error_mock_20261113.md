# Evidencia — VI-05, error en la fuente de identidad del archivo propio

- **Fecha:** 2026-11-13.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron cambios ajenos.

## Alcance

`TestExportOwnArchiveStopsAfterIdentitySourceReturnsDataAndError` ejerce `privacy.Service.ExportOwnArchive` real con repository fake que devuelve simultáneamente `OwnData` sintético parcial y un error centinela. Comprueba archive nil, propagación del error original con `errors.Is`, una llamada de `ExportOwnData` con el ID sintético del titular y cero llamadas a la fuente de módulos. Los fixtures contienen solo datos sintéticos. El repository fake delega el resto de la interfaz en `memoryRepository` y el recorder de módulos se define en el propio archivo.

Test-only de caracterización: pasó en su primera ejecución válida; no se modificó producción ni se afirma RED/GREEN.

## Verificación local

- Focal: `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=1 -v ./internal/privacy -run '^TestExportOwnArchiveStopsAfterIdentitySourceReturnsDataAndError$'` — PASS (1/1).
- Grupo de seis tests de exportación: `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=20 ./internal/privacy -run '^(TestExportOwnArchiveOmitsOperationalRolesAndKeepsFileBytesOutOfMetadata|TestExportOwnArchiveReturnsNoPartialArchiveWhenModuleSourceFails|TestExportOwnArchiveReturnsNoPartialArchiveWhenModuleSourceReturnsDataAndError|TestExportOwnArchiveReturnsNoPartialArchiveWhenModuleSectionIsInvalidJSON|TestExportOwnArchiveRejectsDirectoryAndDuplicateFilePaths|TestExportOwnArchiveStopsAfterIdentitySourceReturnsDataAndError)$'` — PASS.
- Paquete `internal/privacy`: `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=1 ./internal/privacy` — PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go vet ./internal/privacy` — PASS.
- `gofmt -d` y `git diff --check` — limpios.

## Revisión independiente

`deleg_4283f2ff` aprobó source-only, sin concerns ni errores lógicos. Confirmó early-return incluso con datos parciales: archive nil, error original, ID del titular y cero consultas a módulos. El revisor no ejecutó pruebas.

## Límites

El repository y la fuente de módulos son fakes; no se demuestra autenticación HTTP, adapters productivos, PostgreSQL, persistencia, atomicidad, storage/GCS, runtime ni exportación de datos reales. No se ejecutó SQL/DDL ni se accedió a DB persistente o Cloud. Sin commit, push, deploy o integración a `main`.
