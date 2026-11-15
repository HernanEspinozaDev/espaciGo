# Evidencia — VI-05, delete identity rechaza scope con symlink (sintético)

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron los cambios previos.

## Alcance

- `internal/adapters/documentfs/store_test.go` añade `TestPrivateStoreIdentityDeleteRejectsSymlinkedScopeDirectory`.
- En `t.TempDir()`, crea un target y sentinel sintéticos dentro del root; instala un symlink en el directorio de case identity hacia ese target interno.
- `DeleteIdentityDocument` retorna `ErrStorage`; el sentinel permanece con bytes idénticos.
- Caracterización de `documentfs.Store` local existente. Primera ejecución PASS; no hubo RED/GREEN ni cambio productivo. El symlink es deliberadamente interno al root para distinguir el guard de directorios de solo protección contra escape exterior.

## Verificación

- Test focal `go test ./internal/adapters/documentfs -run '^TestPrivateStoreIdentityDeleteRejectsSymlinkedScopeDirectory$' -count=1` — PASS.
- Suite completa local `go test -race ./internal/adapters/documentfs -count=1` — PASS.
- `go vet ./internal/adapters/documentfs`, gofmt focal y `git diff --check` — PASS.

## Límites

La revisión independiente read-only `deleg_a032b512` aprobó sin hallazgos de seguridad o lógica; confirmó que el target interno hace sensible la prueba a borrado por symlink y que el sentinel detecta esa eliminación. El test no afirma que el symlink mismo permanezca. No prueba concurrencia TOCTOU, borrado durable/forense, otros backends, GCS, IAM, PostgreSQL, workers/leases, backup/recuperación, métricas/alertas ni logs/exportaciones. No se ejecutaron SQL/DDL, migraciones, consultas/conexiones a bases, Cloud, proveedores ni datos reales. Sin commit, push, integración a `main` o despliegue.
