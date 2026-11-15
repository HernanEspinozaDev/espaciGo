# Evidencia — VI-05, fuente de exportación con datos parciales y error

- **Fecha:** 2026-11-13.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron cambios ajenos.

## Alcance

`TestExportOwnArchiveReturnsNoPartialArchiveWhenModuleSourceReturnsDataAndError` llama `privacy.Service.ExportOwnArchive` real con `memoryRepository` y una fuente fake que, en la misma respuesta, entrega sección JSON válida, archivo binario de fixture, exclusión y un error centinela. Comprueba que el servicio devuelva archive nil, propague el error original (`errors.Is`) y haya llamado la fuente una vez con el ID sintético del titular. El escenario asegura que resultados parciales acompañados de error no produzcan ZIP.

Test-only de caracterización: pasó en su primera ejecución válida; no se modificó producción ni se afirma RED/GREEN. Los resultados y el error son sintéticos.

## Verificación local

- Focal: `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=1 -v ./internal/privacy -run '^TestExportOwnArchiveReturnsNoPartialArchiveWhenModuleSourceReturnsDataAndError$'` — PASS (1/1).
- Grupo de cinco tests de exportación: `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=20 ./internal/privacy -run '^(TestExportOwnArchiveOmitsOperationalRolesAndKeepsFileBytesOutOfMetadata|TestExportOwnArchiveReturnsNoPartialArchiveWhenModuleSourceFails|TestExportOwnArchiveReturnsNoPartialArchiveWhenModuleSourceReturnsDataAndError|TestExportOwnArchiveReturnsNoPartialArchiveWhenModuleSectionIsInvalidJSON|TestExportOwnArchiveRejectsDirectoryAndDuplicateFilePaths)$'` — PASS.
- Paquete: `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=1 ./internal/privacy` — PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go vet ./internal/privacy` — PASS.
- `gofmt -d` y `git diff --check` — limpios.

## Revisión independiente

`deleg_d8ef907d` aprobó source-only, sin concerns ni errores lógicos. Confirmó que el test combina datos válidos y error, exige archive nil y `errors.Is` del sentinel, y verifica el ID del titular. El revisor no ejecutó pruebas.

## Límites

El repositorio de identidad y la fuente de módulos son fakes; no se prueba el comportamiento de adaptadores productivos, autenticación HTTP, PostgreSQL, persistencia, atomicidad, storage/GCS, exportación runtime ni datos reales. No se ejecutó SQL/DDL ni se accedió a DB persistente o Cloud. Sin commit, push, deploy o integración a `main`.
