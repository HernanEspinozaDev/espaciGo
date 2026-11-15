# Evidencia — VI-05, borrado identity local aislado por propósito (sintético)

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron los cambios previos.

## Alcance

- `internal/adapters/documentfs/store_test.go` añade `TestPrivateStoreDeletesIdentityWithoutDeletingSpaceDocument`.
- Con un `Store` real en `t.TempDir()` y contenido/clave sintéticos, escribe un documento identity y otro space con owner/case/file iguales; el objeto space usa además su `spaceID`.
- Al borrar identity, `ReadIdentityDocument` retorna `os.ErrNotExist`, el contenido space sigue legible e idéntico, y repetir el borrado identity retorna nil.
- Caracterización de la implementación documentfs local existente; primera ejecución PASS, sin RED/GREEN ni cambio productivo. Es filesystem temporal local, no mock; no equivale a Cloud ni a una prueba de recuperación post-reinicio de host.

## Verificación

- Test focal `go test ./internal/adapters/documentfs -run '^TestPrivateStoreDeletesIdentityWithoutDeletingSpaceDocument$' -count=1` — PASS.
- Suite completa local `go test -race ./internal/adapters/documentfs -count=1` — PASS.
- `go vet ./internal/adapters/documentfs`, gofmt focal y `git diff --check` — PASS.

## Límites

La revisión independiente read-only `deleg_a602f730` aprobó sin hallazgos de seguridad o lógica; confirmó que el test acredita ausencia identity antes del segundo borrado y que la segunda llamada solo verifica retorno sin error. No demuestra borrado durable/forense, ausencia de generaciones/copias, comportamiento del adapter GCS, IAM, PostgreSQL, worker/claim/lease, reinicio de host, backups, métricas/alertas ni privacidad de logs/exportaciones. No se ejecutaron SQL/DDL, migraciones, consultas/conexiones a bases, Cloud, proveedores ni datos reales. Sin commit, push, integración a `main` o despliegue.
