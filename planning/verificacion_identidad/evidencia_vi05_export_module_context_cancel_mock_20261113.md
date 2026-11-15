# Evidencia — VI-05, cancelación de contexto durante exportación propia

- **Fecha:** 2026-11-13.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron cambios ajenos.

## Alcance

`TestExportOwnArchiveReturnsNoArchiveWhenModuleExportContextIsCancelled` ejerce `privacy.Service.ExportOwnArchive` real con `memoryRepository` y fuente de módulos fake bloqueante. El test espera que la llamada llegue a la fuente, cancela el contexto y comprueba que la exportación retorna archive nil, `context.Canceled`, y una única invocación scoped al ID sintético del titular. El fake espera `ctx.Done()` y dispone de un canal local de escape para terminar si el test falla por falta de forwarding; no se usa storage, PostgreSQL ni proveedor externo.

Test-only de caracterización al primer run válido; no hubo cambios de producción ni se afirma RED/GREEN.

## Verificación local

- Focal: `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=20 ./internal/privacy -run '^TestExportOwnArchiveReturnsNoArchiveWhenModuleExportContextIsCancelled$'` — PASS.
- Grupo de siete tests de exportación: `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=20 ./internal/privacy -run '^(TestExportOwnArchiveOmitsOperationalRolesAndKeepsFileBytesOutOfMetadata|TestExportOwnArchiveReturnsNoPartialArchiveWhenModuleSourceFails|TestExportOwnArchiveReturnsNoPartialArchiveWhenModuleSourceReturnsDataAndError|TestExportOwnArchiveReturnsNoPartialArchiveWhenModuleSectionIsInvalidJSON|TestExportOwnArchiveRejectsDirectoryAndDuplicateFilePaths|TestExportOwnArchiveStopsAfterIdentitySourceReturnsDataAndError|TestExportOwnArchiveReturnsNoArchiveWhenModuleExportContextIsCancelled)$'` — PASS.
- Paquete `internal/privacy`: `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=1 ./internal/privacy` — PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go vet ./internal/privacy` — PASS.
- `gofmt -d` y `git diff --check` — limpios. Tras la revisión inicial se volvió a ejecutar focal y grupo de siete con `-race -count=20`, suite package con `-race`, vet y formato: PASS.

## Revisión independiente

La revisión source-only inicial `deleg_d54b5cbe` aprobó sin concerns ni errores lógicos; señaló que un timeout podía dejar una goroutine esperando si se rompe el forwarding. Se añadió un canal local de escape para el fake y se repitieron las pruebas indicadas. Re-revisión source-only del delta pendiente; el revisor no ejecutó pruebas.

## Límites

Prueba solo forwarding/cancelación en el servicio con repository y módulo sintéticos. No demuestra comportamiento de adapters productivos, cancelación de I/O real, autenticación HTTP, PostgreSQL, persistencia, atomicidad, storage/GCS, runtime ni exportación de datos reales. No se ejecutó SQL/DDL ni se accedió a DB persistente o Cloud. Sin commit, push, deploy o integración a `main`.
