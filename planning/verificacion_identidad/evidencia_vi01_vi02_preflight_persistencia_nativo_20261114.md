# Evidencia — VI-01 / VI-02, preflight de persistencia PostgreSQL nativo

- **Fecha:** 2026-11-14.
- **Checkout / rama / HEAD base:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8` más árbol compartido con cambios sin commit.
- **Destino:** PostgreSQL nativo `127.0.0.1:5432/espacigo_dms_stage_20261020`.
- **Alcance:** lecturas de preflight únicamente; datos de negocio no consultados. Sin DDL ni mutaciones de owner, ACL, ledger o fixtures.

## Readback del plan migratorio

Se ejecutó `cmd/dbmigrate -plan-only` con la conexión exacta al destino, role de migración dedicado y `DATABASE_SET_ROLE=espacigo_schema_owner`. El resultado devolvió `database=espacigo_dms_stage_20261020`, PostgreSQL `18.7 (Ubuntu 18.7-1.pgdg26.04+2)`, `read_only=true`, history presente y ledger continuo V1–V94. La fila V94 coincide con `V000094__identity_delivered_session_outbox_retirement.sql` y el checksum fijado por el código fuente. V95 y V96 quedaron pendientes. El planner validó nombres/checksums del historial frente al inventario del checkout.

El JSON del planner **no incluyó `ddl_authority`** a pesar de V95/V96 pendientes. En la fuente consultada, `needsIdentityDDLAuthority` no selecciona esas versiones; por tanto ese output no acredita los requisitos de propiedad/autoridad específicos de V95/V96.

## Readback independiente de autoridad (READ ONLY)

Se ejecutó una transacción `BEGIN TRANSACTION READ ONLY`, `SET LOCAL ROLE espacigo_schema_owner`, SELECT de catálogos/privilegios y `ROLLBACK`. Resultados observados:

- `session_user=espacigo_migrator_login`; `current_user=espacigo_schema_owner`; `transaction_read_only=on`.
- Owner de schema `public`: `pg_database_owner`; `public CREATE=false`.
- `document_verification_case` y `document_verification_event`: owner `postgres`.
- `identity_declaration_revision` y `purge_expired_identity_declaration_revisions()`: owner `espacigo_schema_owner`.
- La función de purga está marcada SECURITY DEFINER con `search_path=pg_catalog`; ejecución efectiva permitida al worker y denegada a runtime.
- `espacigo_schema_owner`, `espacigo_runtime` y `espacigo_privacy_cleanup_worker` están sin LOGIN, INHERIT, SUPERUSER, CREATEROLE, CREATEDB, REPLICATION o BYPASSRLS.
- Privilegios runtime consultados para SELECT del caso, UPDATE de `document_expires_at` e INSERT de evento: presentes.

**Conclusión del gate:** V95/V96 no se aplicaron. Sus SQL requieren `public CREATE` y que las tablas de caso/evento pertenezcan a `espacigo_schema_owner`; el readback muestra CREATE ausente y ambos owners como `postgres`. Los permisos runtime y la función de purga consultados sí cumplen las condiciones medidas. No se cambió ownership ni ACL: el DDL queda detenido por autoridad/owner no satisfecho y debe resolverse mediante el procedimiento versionado y un executor autorizado, sin debilitar guards ni ampliar privilegios globalmente.

## Verificación de código local asociada

Se reforzaron tests de titularidad y fallos de borrado/compensación en `internal/verification/evidence_service_boundaries_test.go`; se hizo determinista la aserción concurrente de `MemorySessionDeletionQueue` y se corrigieron los claims de la evidencia HTTPDidit. Resultados observados en este checkout:

- `go test -race -count=1 ./internal/verification -run 'TestMemoryEvidenceRepoRejectsOtherOwners|TestEvidenceService(Content|Delete|UploadCompensationDeleteFailure).*|TestMemorySessionDeletionQueueBoundaries|TestHTTPDiditClientBoundaries'` — PASS.
- `go test -race -count=1 ./internal/verification` — PASS.
- `go vet ./internal/verification` — PASS.
- `gofmt -d internal/verification/evidence_service_boundaries_test.go internal/verification/didit_client_and_queue_boundaries_test.go` — sin diferencias.
- `git diff --check` y whitespace de evidencias Markdown — PASS.

Los tests usan fakes y no sustituyen persistencia, almacenamiento GCS, MFA, operadores reales ni E2E. Cloud SQL no se consultó en esta operación; su estado de esquema vigente permanece no verificado. El lock de release Cloud se consultó separadamente el 2026-11-14: objeto canónico ausente (404) y cero builds `WORKING` del trigger en `southamerica-west1`; no se ejecutó unlock ni hubo mutación. El inspector `recover_release_lock.py` reportó refusal ante el 404 de lock ausente, que se registra como limitación de su comportamiento, no como lock activo. No commit, push, despliegue o integración a `main`.

## Relectura del plan tras implementar el reporte V95/V96 — READ ONLY

Después de implementar el campo `identity_ddl_authority` en `PlanResult`, se repitió `cmd/dbmigrate -plan-only -dir db/migrations` el 2026-11-14 contra el mismo destino nativo exacto, con `espacigo_migrator_login`, `DATABASE_SET_ROLE=espacigo_schema_owner` y el PGPASSFILE ya documentado. Antes de conectar se verificaron el listener local en `127.0.0.1:5432`, el servicio PostgreSQL 18 local y los permisos del archivo privado (directorio `0700`, archivo regular `0600`); no se leyó su contenido. El resultado filtrado confirmó `read_only=true`, base `espacigo_dms_stage_20261020`, PostgreSQL `18.7 (Ubuntu 18.7-1.pgdg26.04+2)`, history presente, 94 filas aplicadas y solo V95/V96 pendientes; el planner validó los nombres/checksums contra el checkout.

El nuevo `identity_ddl_authority` contiene `session_user=espacigo_migrator_login`, `current_user=espacigo_schema_owner`, rol de sesión con LOGIN y sin los flags privilegiados medidos; `public CREATE=false`; owners `postgres` para `document_verification_case` y `document_verification_event`; owners `espacigo_schema_owner` para `identity_declaration_revision` y `purge_expired_identity_declaration_revisions()`. Reporta además roles restringidos, `SECURITY DEFINER`, `search_path=pg_catalog`, grants del worker y permisos runtime consultados. El estado coincide con el readback independiente previo; **V95/V96 siguen bloqueadas por `public CREATE=false` y ownership ajeno de las dos tablas**. No se consultaron filas de negocio, no se cambió ACL/owner/ledger y no se aplicó DDL. Cloud SQL permanece sin lectura actual. La primera ejecución del subcorte no emitía este objeto; el resultado descrito aquí es la relectura posterior.

## Verificación del reporte Go→Python en PostgreSQL desechable

La ejecución más reciente de `TMPDIR="$HOME/.hermes/cache/scratch" scripts/test-migration-runner-postgres.sh full` terminó con exit 0: **165 resultados requeridos, ninguno faltante ni omitido**. El runner capturó los JSON reales de planes read-only V95/V96 generados desde PostgreSQL desechable por Go; `scripts/gcp/test_identity_tail_plan_wire.py` pasó esos objetos por `migration_release_manifest.check_plan`, con inventario local obtenido de `cmd/dbmigrate -inventory-only`. El nombre de base del contenedor se adaptó solo en memoria para esa validación. No es una lectura de Cloud SQL ni prueba de persistencia nativa.

Verificaciones locales posteriores, con variables de DB retiradas para Go: `go test -race -count=1 ./cmd/dbmigrate ./internal/migrator` PASS; `go vet ./cmd/dbmigrate ./internal/migrator` PASS; `go build -o /dev/null ./cmd/dbmigrate` PASS; suites `test_migration_release_manifest`, `test_release_plan_authority`, `test_migration_execution` y `test_check_migration_ci` PASS; `py_compile`, `gofmt -d` y `git diff --check` PASS. Estas suites Go no se presentan como integración con la base persistente; el runner PostgreSQL de este subcorte fue el contenedor desechable descrito arriba.
