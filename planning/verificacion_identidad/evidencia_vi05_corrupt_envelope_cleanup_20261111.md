# Evidencia — VI-05, cleanup de envelope identity corrupto

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y muy dirty; cambios preexistentes preservados.

## Alcance

- `TestRetentionProcessorDeletesCorruptIdentityEnvelope` usa `documentfs.Store` y `RetentionProcessor` reales bajo `t.TempDir()` con claim/job fake.
- Sella evidencia sintética, altera un byte del envelope cifrado, comprueba que la lectura falla con `documentfs.ErrStorage` y luego verifica que el processor completa el borrado y elimina el path del envelope corrupto.
- Sin cambios productivos; caracteriza que el borrado no necesita descifrar el contenido.

## Verificación local

- Test focal: PASS.
- `go test ./internal/adapters/postgres/documentverification -count=1`: PASS.
- `go test -race ./internal/adapters/postgres/documentverification -count=1`: PASS.
- `go vet ./internal/adapters/postgres/documentverification ./internal/adapters/documentfs ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido archivo nuevo: PASS.
- Variables `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `DATABASE_URL`, y `PG*` retiradas de comandos Go.

## Límites

Claim y completion son fake; el filesystem es temporal. No prueba elegibilidad, job durable, transacción, GCS, recuperación o borrado forense. Revisión independiente `deleg_1200d23b` aprobó sin hallazgos; confirmó solo la eliminación local del objeto corrupto sin descifrado, no estado durable. Sin SQL/DDL, bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue.
