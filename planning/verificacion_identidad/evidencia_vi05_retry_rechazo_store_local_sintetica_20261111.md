# Evidencia — VI-05, retry local ante rechazo del Store privado

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron los cambios previos.

## Alcance

- `TestRetentionProcessorRetriesWhenPrivateStoreRejectsUnsafeScope` siembra evidencia sintética con Store y filesystem reales bajo `t.TempDir()`, guarda el envelope cifrado, cambia el modo del directorio del caso a `0755` y procesa un claim identity mediante `RetentionProcessor`.
- El Store rechaza la eliminación del scope inseguro; el adaptador de job fake observa un intento, `retention_store_error`, retry de un segundo y cero completions. El envelope local sigue byte a byte igual y el directorio conserva modo `0755` tras el intento rechazado.
- Caracterización de comportamiento existente; no cambio productivo, sin secuencia RED/GREEN.

## Verificación

- Test focal `go test ./internal/adapters/postgres/documentverification -run '^TestRetentionProcessorRetriesWhenPrivateStoreRejectsUnsafeScope$' -count=1` — PASS.
- Suite de processor `go test -race ./internal/adapters/postgres/documentverification -run '^TestRetentionProcessor' -count=1` — PASS.
- `go vet ./internal/adapters/postgres/documentverification`, gofmt focal y `git diff --check` — PASS.
- Procesos Go ejecutados sin variables DB/PG, incluidas URLs de test, `DATABASE_URL`, `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, `PGPASSWORD` y `PGPASSFILE`.

## Límites

La revisión independiente read-only `deleg_f61b0f4b` aprobó sin hallazgos de seguridad/lógica. Confirmó el rechazo del Store y la igualdad byte a byte del envelope; sugirió comprobar que el processor no modifica el modo inseguro. Se añadió la aserción de que `0755` permanece y se repitieron test focal, suite `-race`, vet, gofmt y diff-check: PASS. El fake observa retry en memoria, no retry/claim/fencing/lease/completion durable. El test comprueba el rechazo de un directorio inseguro concreto y preservación del envelope local; no acredita contención general, PostgreSQL, GCS/IAM, backup/recuperación, worker real, borrado forense ni operación de producción. No hubo SQL/DDL, migraciones, conexiones/consultas a bases, Cloud, proveedores ni datos reales. Sin commit, push, integración a `main` o despliegue.
