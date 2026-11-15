# Preflight de ledger — 2026-11-07

## Alcance

Lectura read-only de identidad, ledger y catálogos de roles/owners/privilegios seleccionados para el gate V84. No hubo DDL, cambios de permisos, backups, creación/ejecución de Jobs ni despliegues. No se leyeron archivos de credenciales.

## Fuente contrastada

- Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- Rama/candidato: `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD `274f37e483a078890e92382a159fa49fd23c2fe8`.
- `origin/main` tras `git fetch origin --prune`: `5830c9ce2dc61d4df13a4139da984b4cafd6d9ba`; `main` local: `75c8c21d30c70a4fd63f6109664d5c2a063410df`.
- Las comparaciones directas `git diff --name-status origin/main HEAD -- db/migrations db/release-policy.json` y `git diff --name-status origin/main main -- db/migrations db/release-policy.json` no produjeron diferencias. `git status --short -- db/migrations db/release-policy.json` quedó vacío; no se tocaron migraciones.
- Manifest local: 84 entradas consecutivas V001–V084, 84 archivos; cero hashes divergentes frente al TSV. V084: `d8cc68f3ea73c6e351a31bd8f88fd0606e8217c028777d5ebca7b406bb852b8f`. Raíz del manifest: `160b647b738ae04300803b1f690adf33cd54348ec29a1a55d8db7a8acd56d51f`.
- El worktree conserva cambios KYC y documentos no confirmados; no se descartaron ni mezclaron.

## PostgreSQL nativo persistente

- Target exacto: `127.0.0.1:5432/espacigo_dms_stage_20261020`; conexión administrativa peer documentada: `sudo -u postgres psql -X -p 5432 -d espacigo_dms_stage_20261020`.
- Preflight `sudo id`: exit 0. La lectura se ejecutó en `BEGIN TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY` y terminó con `ROLLBACK`.
- Snapshot: `2026-11-07 08:03:45.362377-03`; `session_user=current_user=postgres`; DB OID `16388`; puerto `5432`; PostgreSQL `18.6 (Ubuntu 18.6-1.pgdg26.04+2)`; inicio del servidor `2026-11-06 18:34:11.841242-03`; `public.schema_migrations` existe y la identidad de lectura tiene `SELECT`.
- Ledger: 76 filas y 76 versiones distintas, mínimo 1, máximo 76, prefijo V1–V76 continuo; cero huecos, duplicados o filas fuera del inventario fuente. Nombre y SHA-256 de cada fila aplicada coinciden con las migraciones V001–V076 del candidato indicado.
- Pendientes relativos a ese inventario fuente: V077–V084. No se aplicó ninguna.
- La salida detallada del query contenía únicamente versiones, nombres y checksums de migración; no contiene filas de negocio. La lectura es una instantánea de ledger, no una prueba de ACL/ownership completo ni un preflight de DDL.

## Snapshot local acotado de owner/autoridad V84

- Lecturas adicionales, cada una en `BEGIN TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY` y terminada con `ROLLBACK`, contra el mismo target nativo mediante la ruta peer documentada.
- Snapshot de catálogos `2026-11-07 08:10:56.092156-03`: `public.schema_migrations` y `public.usuario` son owned por `postgres`; `public` es owned por `pg_database_owner`; `espacigo_schema_owner` y `espacigo_migrator_login` no aparecen en `pg_roles`; `espacigo_runtime` existe como LOGIN, `NOINHERIT`, no superuser/CREATEDB/CREATEROLE/BYPASSRLS; no se observaron memberships que involucren esos roles seleccionados.
- El mismo snapshot devolvió `schema_owner_effective: null` porque el rol está ausente. Esto no prueba autoridad del rol release en Cloud SQL.
- Snapshot de privilegios efectivos `2026-11-07 08:11:33.092882-03`: propietario de la base `postgres`; para `espacigo_runtime`, CONNECT y USAGE en `public` sí, CREATE en `public` no, SELECT del ledger no, SELECT y UPDATE de `public.usuario` sí. Se consultaron solo estos derechos seleccionados; no es un inventario completo de ACL, default ACL o privilegios de otros objetos.
- La primera invocación del query compuesto de catálogos devolvió error de sintaxis antes de ejecutar el `SELECT`; se corrigió el cierre SQL y las dos lecturas posteriores pasaron. La transacción fue de solo lectura; no se hizo DDL ni se mutó ACL.
- `espacigo_runtime` es LOGIN en este destino local, lo que se registra como estado observado, no se modifica ni se extrapola a Cloud SQL. El destino local no contiene los roles de migración previstos por el Job de release.

## Cloud SQL

- No se consultó Cloud SQL en este registro. El recibo V83/cero pendientes del 2026-11-06 sigue siendo histórico, no estado actual.
- La ruta vigente es privada y orquestada. `scripts/gcp/release_main.py` no ofrece modo `preflight-only`: `execute()` toma lock, construye y valida imágenes/manifiesto, ejecuta Job de plan y luego continúa a backup, Job aplicador, postcheck y rollout (`scripts/gcp/release_main.py:223–271`). La CLI solo expone `--execute`, `--sha` y `--build-id` (`:280–284`). `migration_job_plan.py` solo renderiza una propuesta (no invoca gcloud) y exige digest/provenance, identidad/grants runtime, versión de secreto habilitada y revisión de Job preexistente (`scripts/gcp/migration_job_plan.py:15–53`).
- No reconstruí fuera del orquestador una secuencia Cloud Build/Cloud Run ni inicié el release completo, porque eso cruzaría el alcance de preflight de solo lectura y podría seguir a backup/aplicación/despliegue. Se requiere una ruta independiente de preflight documentada o un alcance de release distinto, revisado y autorizado; no se usó bastión/túnel.

## Límites y siguiente gate

- El ledger local no establece el estado de Cloud SQL ni de demos/bases sintéticas.
- Se inspeccionó owner/rol y un conjunto acotado de privilegios efectivos del destino local (arriba); no se realizó inventario completo de ACL/default ACL ni preflight de autoridad en Cloud SQL.
- No se ejecutaron pruebas PostgreSQL ni se creó/seleccionó un destino sintético. El diseño y la ejecución de pruebas fresh/upgrade siguen pendientes de una selección/autorización exacta de destino aislado.
- Este registro no autoriza DDL, grants, backup, Job, release ni despliegue.