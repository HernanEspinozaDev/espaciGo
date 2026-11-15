# Evidencia — VI-05, validación de rutas y colisiones del archivo propio

- **Fecha:** 2026-11-13.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron cambios ajenos.

## Brecha y alcance

Se amplió `privacy.Service.ExportOwnArchive` para validar todos los nombres antes de construir el manifest/ZIP: cada nombre debe conservar el prefijo `files/`, ser canónico según `path.Clean`, no contener separadores `\\` y no repetir exactamente otro nombre de entrada. Ante ruta inválida o nombre exacto duplicado retorna `nil, ErrInvalid`, sin devolver un ZIP parcial.

`TestExportOwnArchiveRejectsDirectoryAndDuplicateFilePaths` ejerce el servicio real con `memoryRepository` y una fuente de módulos fake. Los fixtures sintéticos cubren `files/`, separador final, segmento `.`, separador backslash y dos payloads con el mismo nombre. Cada escenario exige archivo nil y `ErrInvalid`; verifica también que la fuente recibió el ID de titular esperado. Las pruebas positivas existentes mantienen un nombre canónico y archivo válido. No se afirma política contra colisiones de mayúsculas/minúsculas o normalización Unicode.

Se inspeccionaron los nombres generados en `internal/adapters/postgres/ownerexport/repository.go`, pero esa fuente no se ejecutó: la cobertura de este subcorte permanece en el servicio con fakes. La prueba no caracteriza extracción del ZIP por consumidores externos.

## Verificación local

- Antes del cambio productivo, el focal nuevo salió con código 1 (`go test -race -count=1`), observando el estado RED para las entradas que el servicio aceptaba sin validación completa.
- Focales de límites y controles: `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=20 -v ./internal/privacy -run '^(TestExportOwnArchiveOmitsOperationalRolesAndKeepsFileBytesOutOfMetadata|TestExportOwnArchiveReturnsNoPartialArchiveWhenModuleSourceFails|TestExportOwnArchiveRejectsDirectoryAndDuplicateFilePaths)$'` — PASS.
- Paquete: `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=1 ./internal/privacy` — PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go vet ./internal/privacy` — PASS.
- `gofmt -d` para los dos archivos Go — limpio; `git diff --check` — PASS.

## Revisión independiente

`deleg_78553c14` revisó source-only los cuatro archivos del subcorte: PASS, sin concerns de seguridad ni errores lógicos. Confirmó que la validación ocurre antes de construir el ZIP y que el test/evidencia se mantienen dentro del alcance fake-only. Sugirió considerar casos de nombre vacío y duplicados equivalentes normalizados solo si forman parte de la política buscada. No se amplió: el nombre vacío ya falla el prefijo `files/`; la política de este subcorte rechaza rutas no canónicas y nombres duplicados exactos, sin definir colisiones de mayúsculas/minúsculas o Unicode.

## Límites

La prueba usa repositorio de identidad y fuente de módulos fake; no prueba autenticación HTTP, aislamiento de adaptadores productivos, consultas o escritura en PostgreSQL, persistencia, storage/GCS, ni compatibilidad de extracción en cada sistema operativo. Solo comprueba nombres exactos y fixtures sintéticos dentro del servicio. No se ejecutó SQL/DDL ni se accedió a DB persistente, Cloud o datos reales. Sin commit, push, deploy o integración a `main`.
