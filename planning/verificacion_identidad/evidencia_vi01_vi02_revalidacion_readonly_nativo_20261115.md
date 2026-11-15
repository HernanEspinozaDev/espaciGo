# Revalidación READ ONLY del PostgreSQL nativo — 2026-11-15

## Alcance y resultado

- Rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD `274f37e483a078890e92382a159fa49fd23c2fe8`; worktree ya tenía cambios ajenos/preexistentes, preservados.
- Listener observado en `127.0.0.1:5432`.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" IDENTITY_DECLARATION_NATIVE_PREFLIGHT_ONLY=1 go test -count=1 ./cmd/api -run '^TestNativeIdentityExternalReadOnlyPreflight$' -v` — PASS; el test terminó en 0.01 s y el paquete en 0.048 s. El guard del propio test validó base `espacigo_dms_stage_20261020`, OID `16388`, `server_version_num=180007`, sesión `espacigo_migrator_login`, rol efectivo `espacigo_schema_owner`, ledger fuente/aplicado continuo V1–V96 y checksum final revisado. La conexión usa el passfile privado fijado en el código; no se imprimió ni registró su contenido.
- Readback ACL independiente en transacción `BEGIN READ ONLY`, `SET LOCAL ROLE espacigo_schema_owner` y `ROLLBACK`: el target devolvió `espacigo_dms_stage_20261020|180007|espacigo_migrator_login|espacigo_schema_owner|f|f`. Los dos booleanos corresponden a `DELETE` efectivo sobre `public.evento_auditoria_local` y `SELECT` efectivo, conjuntamente, sobre `actor_id`, `recurso_tipo`, `recurso_id`, `accion`, `correlacion_id`.

## Límites y siguiente gate

Esta revalidación no leyó inventario de cuentas/fixtures, no inició workers y no ejecutó pruebas funcionales, DML ni DDL. Confirma que sigue faltando la autoridad de cleanup observada el 2026-11-14; no se modificaron ACL. Por tanto, `TestNativeIdentityRUTKeyRotationSequential` y cualquier recorrido nativo con escrituras permanecen detenidos hasta resolver el requisito mediante una transición mínima, versionada y revisada, o una alternativa de cleanup autorizada. No hubo Cloud SQL, GCS, Didit, commit, push, despliegue o integración a `main`.
