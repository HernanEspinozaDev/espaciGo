# Evidencia — VI-05, borrado identity de symlink final

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y muy dirty; cambios preexistentes preservados.

## Alcance

- Test-only characterization: `TestPrivateStoreIdentityDeleteRemovesFinalSymlinkWithoutFollowingTarget` crea un objeto identity en `t.TempDir()`, lo sustituye por un symlink final a un sentinel fuera del root y ejecuta borrado con Store real.
- Verifica que la entrada symlink desaparece y que los bytes del target externo permanecen idénticos.
- Sin cambios de producción.

## Verificación local

- Test focal: PASS.
- `go test ./internal/adapters/documentfs -count=1`: PASS.
- `go test -race ./internal/adapters/documentfs -count=1`: PASS.
- `go vet ./internal/adapters/documentfs ./internal/adapters/postgres/documentverification ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido archivo nuevo: PASS.
- Variables `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `DATABASE_URL`, y `PG*` retiradas de comandos Go.

## Límites

Prueba secuencial de symlink final en filesystem temporal; no acredita TOCTOU, concurrencia, autorización, persistencia, GCS ni borrado forense. Revisión independiente `deleg_d0737ddd` aprobó sin hallazgos; verificó unlink del symlink final y bytes externos intactos. Sin SQL/DDL, bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue.
