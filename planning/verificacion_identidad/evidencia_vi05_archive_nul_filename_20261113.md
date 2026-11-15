# Evidencia VI-05 — rechazo de byte NUL en nombres del ZIP

Fecha: 2026-11-13 (hora local `-03:00`)

## Alcance y RED→GREEN

Se añadió a `TestExportOwnArchiveRejectsDirectoryAndDuplicateFilePaths` un nombre sintético con byte NUL (`files/identity\x00-document.pdf`). La primera ejecución observó RED: `archive bytes=1080 err=<nil>`, es decir, `safeExportFileName` permitía el nombre y el ZIP era devuelto.

La corrección mínima en `internal/privacy/service.go` hace que `safeExportFileName` rechace cualquier NUL mediante `strings.ContainsRune(name, 0)`. El test exige `ErrInvalid` y archive nil antes de la escritura del ZIP. La tabla conserva los controles de directorio, separador final, `.`, `..`, separador inverso y duplicados.

## Verificación ejecutada

- Focal previo al cambio: `go test ./internal/privacy -run '^TestExportOwnArchiveRejectsDirectoryAndDuplicateFilePaths/NUL_byte_in_filename$' -count=1 -v` — RED esperado (`1080` bytes y error nil).
- Focal tras el cambio: `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test ./internal/privacy -run '^TestExportOwnArchiveRejectsDirectoryAndDuplicateFilePaths$' -count=1 -v` — PASS (7 subcasos).
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=1 ./internal/privacy` — PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=20 ./internal/privacy -run '^TestExportOwnArchiveRejectsDirectoryAndDuplicateFilePaths$'` — PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go vet ./internal/privacy` — PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go build -o /dev/null ./cmd/api` — PASS.
- `gofmt -d internal/privacy/service.go internal/privacy/identity_archive_boundary_test.go` — sin diferencias.
- `git diff --check -- internal/privacy/service.go internal/privacy/identity_archive_boundary_test.go` — PASS.
- Preflight de tests del paquete: sin `TestMain`, `init`, clientes PG/SQL, requests HTTP ni `exec.Command` en `_test.go`; no se ejecutaron suites de integración.

## Límites

Esta validación cubre solo la aceptación de nombres al construir el ZIP con datos sintéticos. No prueba autenticación/autoridad del export, extractores externos, datos o persistencia en DB, SQL/DDL, PostgreSQL, GCS, Cloud ni proveedores. No valida la totalidad de reglas de exportación.

- Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`
- Rama: `codex/marketplace/fix/kyc-06-save-identity-first`
- HEAD observado: `274f37e483a078890e92382a159fa49fd23c2fe8`

Sin SQL/DDL, acceso a DB/Cloud, datos reales, commit, push, deploy ni integración a `main`; se preservó el estado previo del worktree.