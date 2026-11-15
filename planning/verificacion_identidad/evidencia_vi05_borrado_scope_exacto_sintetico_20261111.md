# Evidencia — VI-05, borrado identity exige scope exacto (sintético)

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron los cambios previos.

## Alcance

- `internal/adapters/documentfs/store_test.go` añade `TestPrivateStoreIdentityDeleteRequiresMatchingScope`.
- En `t.TempDir()` y con contenido sintético, escribe un objeto identity para owner/case/file válidos; intenta borrados con owner, case y file alternativos (UUIDs sintéticos válidos), y tras cada intento confirma que los bytes del objeto original siguen legibles.
- El control positivo borra con el tuple exacto y comprueba `os.ErrNotExist` al leer.
- Caracterización de `documentfs.Store` local existente; primera ejecución PASS, sin RED/GREEN ni cambio productivo. El filesystem temporal no representa una cuenta, bucket o almacenamiento remoto.

## Verificación

- `go test ./internal/adapters/documentfs -run '^TestPrivateStoreIdentityDeleteRequiresMatchingScope$' -count=1` — PASS.
- `go test -race ./internal/adapters/documentfs -count=1` — PASS.
- `go vet ./internal/adapters/documentfs`, gofmt focal y `git diff --check` — PASS.

## Límites

La revisión independiente read-only `deleg_883a5e82` aprobó sin hallazgos de seguridad o lógica; confirmó que owner/case/file se varían por separado y cada intento fallido se sigue de lectura/comparación del contenido, con control positivo exacto. Aclaró que no prueba autorización del caller, concurrencia ni otros backends. No prueba borrado durable/forense, bucket/GCS, IAM, APIs de almacenamiento, PostgreSQL, workers/leases, backup/recuperación, métricas/alertas ni logs/exportaciones. No se ejecutaron SQL/DDL, migraciones, consultas/conexiones a bases, Cloud, proveedores ni datos reales. Sin commit, push, integración a `main` o despliegue.
