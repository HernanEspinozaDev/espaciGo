# Evidencia — VI-05, borrado identity rechaza directorio permisivo (sintético)

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron los cambios previos.

## Alcance

- `internal/adapters/documentfs/store_test.go` añade `TestPrivateStoreIdentityDeleteRejectsPermissiveScopeDirectory`.
- Con `documentfs.Store` real y datos sintéticos bajo `t.TempDir()`, guarda el envelope del objeto identity, cambia a `0755` el directorio de case y llama al borrado.
- El Store devuelve `ErrStorage`; el envelope cifrado permanece byte a byte en su ruta local.
- Caracterización del guard de permisos de directorio para filesystem local; primera ejecución PASS, sin RED/GREEN ni cambio productivo.

## Verificación

- Test focal `go test ./internal/adapters/documentfs -run '^TestPrivateStoreIdentityDeleteRejectsPermissiveScopeDirectory$' -count=1` — PASS.
- Suite completa local `go test -race ./internal/adapters/documentfs -count=1` — PASS.
- `go vet ./internal/adapters/documentfs`, gofmt focal y `git diff --check` — PASS.

## Límites

La revisión independiente read-only `deleg_dcd7cb2d` aprobó sin hallazgos de seguridad/lógica; confirmó que `PutIdentityDocument` deja el directorio case en `0700`, `chmod 0755` alcanza el guard y el envelope se compara byte a byte después del rechazo. No demuestra IAM, confidencialidad a nivel de sistema operativo, comportamiento de otros backends, GCS, PostgreSQL, borrado durable/forense, concurrencia/TOCTOU, worker/lease, backup/recuperación ni logs/exportaciones. No se ejecutaron SQL/DDL, migraciones, consultas/conexiones a bases, Cloud, proveedores ni datos reales. Sin commit, push, integración a `main` o despliegue.
