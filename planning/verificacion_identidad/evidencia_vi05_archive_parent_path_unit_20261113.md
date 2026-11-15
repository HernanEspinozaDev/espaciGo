# Evidencia VI-05 — rechazo de ruta ZIP con segmento `..`

Fecha: 2026-11-13 (hora local `-03:00`)

## Alcance

Se amplió la tabla `TestExportOwnArchiveRejectsDirectoryAndDuplicateFilePaths` en `internal/privacy/identity_archive_boundary_test.go` con la entrada sintética `files/../manifest.json`. Comprueba que `ExportOwnArchive` devuelve `ErrInvalid` y ningún byte de archivo ZIP antes de escribir una entrada que asciende fuera del directorio permitido `files/` hacia el nombre de un miembro reservado del archivo.

La tabla conserva los casos existentes de directorio, separador final, segmento `.`, separador inverso y nombres duplicados. Es una caracterización test-only: pasó al primer intento y no se modificó producción. El repositorio y la fuente de módulos son fakes; no se creó ni extrajo un ZIP para demostrar cómo reaccionan herramientas externas ante Zip Slip.

## Verificación ejecutada

- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test ./internal/privacy -run '^TestExportOwnArchiveRejectsDirectoryAndDuplicateFilePaths$' -count=1 -v` — PASS (6 subcasos).
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=1 ./internal/privacy` — PASS (paquete completo).
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=20 ./internal/privacy -run '^TestExportOwnArchiveRejectsDirectoryAndDuplicateFilePaths$'` — PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go vet ./internal/privacy` — PASS.
- `gofmt -d internal/privacy/identity_archive_boundary_test.go` — sin diferencias.
- `git diff --check -- internal/privacy/identity_archive_boundary_test.go` — PASS.
- Preflight de tests del paquete: sin `TestMain`, `init`, clientes PG/SQL, requests HTTP ni `exec.Command` en `_test.go`; solo se ejecutó el paquete unitario local.

## Límites

No prueba autenticación/autoridad del export, datos de DB, ZIPs externos ni comportamiento de extractores, persistencia/reinicio, SQL/DDL, PostgreSQL, GCS, Cloud o proveedores. No ejecuté suites de integración.

- Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`
- Rama: `codex/marketplace/fix/kyc-06-save-identity-first`
- HEAD observado: `274f37e483a078890e92382a159fa49fd23c2fe8`

Sin SQL/DDL, acceso a DB/Cloud, datos reales, commit, push, deploy ni integración a `main`; se preservó el estado previo del worktree.