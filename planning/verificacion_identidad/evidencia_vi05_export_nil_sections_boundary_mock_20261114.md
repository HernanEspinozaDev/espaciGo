# Evidencia — VI-05, normalización de secciones nil y ordenamiento de archivos en archivo propio

- **Fecha:** 2026-11-14.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron cambios ajenos.

## Alcance

En `internal/privacy/identity_archive_nil_sections_boundary_test.go`, se caracterizan tres condiciones de borde en el servicio real `privacy.Service.ExportOwnArchive` con `memoryRepository` y fakes sintéticos:

1. `TestExportOwnArchiveRejectsEmptyAccountIDBeforeReachingSources`: verifica que `ExportOwnArchive(ctx, "")` falle de inmediato devolviendo `ErrInvalid` y slice de bytes nil, con cero llamadas a la fuente de módulos (`calls == 0`).
2. `TestExportOwnArchiveReturnsUnavailableWhenCompleteModulesNil`: verifica que si el servicio se construye sin módulos completos (`complete == nil`), `ExportOwnArchive` falle con `ErrExportUnavailable` y slice de bytes nil.
3. `TestExportOwnArchiveNormalizesNilSectionsAndSortsManifestFiles`: verifica que cuando la fuente de módulos entrega `sections: nil` y múltiples archivos en orden desordenado (`files/zulu-doc.pdf`, `files/alpha-doc.pdf`):
   - Se normalicen las secciones internas a mapa vacío `{}` para que en `data.json` se serialice `"sections": {}` y nunca `"sections": null`.
   - En `manifest.json`, `sections` contenga exactamente `["identity"]`.
   - La lista `manifest.Files` quede ordenada lexicográficamente (`[alpha, zulu]`).
   - Los archivos privados y metadatos se lean y desempaquen correctamente del ZIP con el contenido sintético esperado.

Es test-only de caracterización: pasó en su primera ejecución válida; no hubo cambio productivo ni se afirma RED/GREEN.

## Verificación local

- Focal: `go test -race -count=20 -run '^(TestExportOwnArchiveRejectsEmptyAccountIDBeforeReachingSources|TestExportOwnArchiveReturnsUnavailableWhenCompleteModulesNil|TestExportOwnArchiveNormalizesNilSectionsAndSortsManifestFiles)$' ./internal/privacy` — PASS.
- Paquete completo: `go test -race -count=1 ./internal/privacy` — PASS.
- `go vet ./internal/privacy` — PASS.
- `gofmt -d` y `git diff --check` — limpios.

## Revisión independiente

`deleg_77a83a34` aprobó source-only: confirmó el rechazo de `accountID` vacío (`ErrInvalid`, nil archive, 0 llamadas a módulos), rechazo sin módulos completos (`ErrExportUnavailable`), serialización de secciones en `data.json` como `{}` y no `null`, secciones del manifest exactamente `["identity"]` y ordenamiento lexicográfico de `manifest.Files`. Su observación sobre precisar la redacción del desempaquetado de archivos fue incorporada. No ejecutó pruebas ni modificó archivos.

## Límites

Repositorio de identidad y fuente de módulos son sintéticos/fakes en memoria. Esto no demuestra autenticación HTTP, adapters productivos, aislamiento PostgreSQL, persistencia, storage/GCS ni exportación de datos reales. No se ejecutó SQL/DDL ni se accedió a DB persistente, Cloud o datos reales. Sin commit, push, deploy o integración a `main`.
