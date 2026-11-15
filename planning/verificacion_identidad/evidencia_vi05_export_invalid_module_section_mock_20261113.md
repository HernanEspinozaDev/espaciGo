# Evidencia — VI-05, sección de módulo JSON inválida en archivo propio

- **Fecha:** 2026-11-13.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron cambios ajenos.

## Alcance

`TestExportOwnArchiveReturnsNoPartialArchiveWhenModuleSectionIsInvalidJSON` llama el servicio real `privacy.Service.ExportOwnArchive` con `memoryRepository` y fuente de módulos fake que devuelve `json.RawMessage` malformado para una sección sintética. Comprueba que la serialización falle, no se devuelva ningún byte de ZIP y la fuente fake sea llamada una vez con el ID de titular esperado. Es test-only de caracterización: pasó en su primera ejecución válida; no hubo cambio productivo ni se afirma RED/GREEN.

La prueba cubre el fallo al serializar `data.json`; no valida el contenido de fuentes productivas ni el mapeo de errores HTTP. No se acopla a la redacción concreta del error del paquete `encoding/json`.

## Verificación local

- Focal: `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=1 -v ./internal/privacy -run '^TestExportOwnArchiveReturnsNoPartialArchiveWhenModuleSectionIsInvalidJSON$'` — PASS (1/1).
- Grupo de cuatro tests del archivo/exportación: `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=20 ./internal/privacy -run '^(TestExportOwnArchiveOmitsOperationalRolesAndKeepsFileBytesOutOfMetadata|TestExportOwnArchiveReturnsNoPartialArchiveWhenModuleSourceFails|TestExportOwnArchiveReturnsNoPartialArchiveWhenModuleSectionIsInvalidJSON|TestExportOwnArchiveRejectsDirectoryAndDuplicateFilePaths)$'` — PASS.
- Paquete: `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=1 ./internal/privacy` — PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go vet ./internal/privacy` — PASS.
- `gofmt -d` y `git diff --check` — limpios.

## Revisión independiente

`deleg_711d700e` aprobó source-only, sin concerns ni errores lógicos. Confirmó que el test alcanza `ExportOwnArchive` con `json.RawMessage` malformado, exige error y archive nil, y que la evidencia conserva los límites fake-only. El revisor no ejecutó pruebas.

## Límites

Repositorio de identidad y fuente de módulos son sintéticos/fake. Esto no demuestra autenticación HTTP, adapters productivos, aislamiento PostgreSQL, persistencia, storage/GCS, operación en runtime ni exportación de datos reales. No se ejecutó SQL/DDL ni se accedió a DB persistente, Cloud o datos reales. Sin commit, push, deploy o integración a `main`.
