# Evidencia de preflight PostgreSQL nativo KYC — pre-V86

- **Fecha/hora:** 2026-11-08 (UTC; snapshot de esta sesión).
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Destino exacto:** PostgreSQL nativo `127.0.0.1:5432/espacigo_dms_stage_20261020`; no Docker ni Cloud SQL.
- **Alcance:** inspección de solo lectura, backup previo y verificación local sintética. No se aplicó DDL, no se cambió ACL/rol, no se inició API/worker, no se publicó ni desplegó.

## Identidad y ledger

Lectura en transacción `READ ONLY` con el login migrador y `SET ROLE espacigo_schema_owner`:

- PostgreSQL `18.6` (`server_version_num=180006`); `session_user=espacigo_migrator_login`, `current_user=espacigo_schema_owner`.
- Ledger continuo V1–V85, 85 filas. El checkout contiene V86/V87 pendientes; **no** se infiere que estén aplicadas.
- Existen `identity_declaration_revision`, `identity_provider_attempt` e `identity_provider_event`; las tablas consultadas de revisiones e intentos tenían 0 filas.
- La función `public.purge_expired_identity_declaration_revisions()` no existe.
- Roles presentes: `espacigo_runtime` y `espacigo_privacy_cleanup_worker` son `NOLOGIN` y sin atributos privilegiados consultados. Existe `espacigo_api_login`, `espacigo_migrator_login` y `espacigo_schema_owner`.
- `espacigo_privacy_worker_login` no existe; tampoco existe la membresía requerida `espacigo_privacy_worker_login` → `espacigo_privacy_cleanup_worker` con `ADMIN FALSE, INHERIT FALSE, SET TRUE`.

## Código y gates

- SHA-256 V86: `af864b793d8f4cbc9cf28dbfe65b8ed6d2388a0ade9a739fc8b480470bd3dc8e`.
- SHA-256 V87: `5eda043ddf3a3d5bb89e8d470eaf4e5450ed0964a9d59fe232bdcb37888b1d45`.
- V86 crea la purga y concede `EXECUTE` a `espacigo_runtime`.
- V87 exige el login y la membresía indicados, y preserva `EXECUTE` de runtime. Esto contradice la decisión registrada en `propuesta_retencion_kyc09_kyc10_20261108.md` §4.6 de dejar el permiso solo en el worker. No se modificaron V86/V87.
- La ruta nativa existente del migrador está limitada a V85 (`cmd/dbmigrate/native_local.go`, `cmd/dbmigrate/native_role_transition.go`, `scripts/run-native-v82-v85.sh`). No se usó como ruta de aplicación de V86+.
- La API activa en `127.0.0.1:18086` corresponde al ensayo wizard, no al destino nativo. Los perfiles worker-free existentes están fijados a bases loopback distintas; no se cambió esa configuración ni se dirigió esa API a este destino.

## Backup previo fresco

- Archivo: `/home/nandev/.local/share/espacigo-native-backups/espacigo_dms_stage_20261020-pre-v86-v88-20261108T233115Z.dump`.
- Formato custom; 8,286,348 bytes; propietario `nandev`; modo `0600`.
- SHA-256: `2e122107a50413eb4df59531831959e904079cd7b054cf5eaea2de8299bc149a`.
- `pg_restore --list`: PASS. Parseo completo con `pg_restore --exit-on-error --file=/dev/null`: PASS. Es validación del archivo, no ensayo de restauración.

## Pruebas ejecutadas

- `GOTOOLCHAIN=local go test -count=1 -race ./internal/verification/... ./internal/adapters/postgres/identitydeclaration/... ./cmd/api/...`: PASS.
- `bash scripts/test-migration-runner-postgres.sh full`: PASS, 120 resultados requeridos, ninguno faltante/omitido; PostgreSQL efímero de harness, migraciones V1–V87 y upgrade/failure/concurrency tests; contenedor propio retirado y ausencia verificada.
- `git diff --check`: PASS en el checkout inspeccionado.

Estas pruebas no validan el flujo KYC contra el PostgreSQL nativo persistente.

## Estado y siguiente gate

El target está respaldado y el preflight de ledger/roles está documentado, pero V86–V87 no son aplicables de forma segura en su estado actual: falta el login/membresía de worker y V87 preserva el permiso runtime contra la decisión aprobada. La autorización del propietario para crear el login dedicado y una credencial nueva se recibió durante esta sesión; aún no se ejecutó esa operación. Antes de DDL se debe terminar y probar una ruta nativa explícita para el tramo append-only posterior a V85, diseñar la migración siguiente que retire `EXECUTE` runtime, comparar el inventario/checksums exactos con el ledger bajo el bloqueo del runner y cumplir los gates de autoridad/postcheck. La prueba de API requiere además un inicio local dedicado y acotado sin los workers ni rutas ajenas del perfil estándar.

No hay autorización ni evidencia de aplicación en Cloud SQL, despliegue, publicación o uso de Didit real.

## Continuación de código local — 2026-11-08

- Se agregó V88 append-only para revocar `EXECUTE` a `espacigo_runtime`; el rol dedicado worker conserva el permiso. Política: `review-required`, `automatic:false`. SHA-256 V88: `44cc5e74b8f8fcfba1633f01009024a7fdeae5683077b13677ed7bf2894083a8`.
- V86 y V87 conservan sus bytes/checksums arriba indicados. El tramo V86→V88 pasó el harness PostgreSQL desechable: 126 resultados requeridos, ninguno faltante/omitido; incluye V87 previo (runtime conserva el permiso) y V88 posterior (permiso revocado), upgrades V75/V76 y retiro verificado del contenedor. `go test -race ./internal/migrator`, `go vet ./internal/migrator` y `git diff --check`: PASS.
- Este resultado solo acredita código y fixtures sintéticos. No se volvió a consultar ni alterar el destino persistente. V86–V88 siguen sin aplicar al PostgreSQL nativo.
- El helper `internal/migrator/native_worker_bootstrap.go` aún no está integrado en la ruta `cmd/dbmigrate`; recibe evidencia suministrada por el llamador y no prueba el catálogo real por sí mismo. No usarlo para crear el rol. No se generó ni guardó credencial.
- Siguiente gate: implementar y revisar un flujo nativo que reúna preflight exacto bajo lock, cree el LOGIN dedicado con credencial privada 0600 y membresía de mínimo privilegio, aplique secuencialmente solo V86–V88 mediante runner versionado y haga postcheck; luego definir perfil de API local KYC sin workers ajenos. Requiere conservar el backup aprobado y la autorización ya otorgada para el destino/operación exactos; ninguna acción se ejecutó aquí.

## Revalidación del target y nuevo alcance local autorizado — 2026-11-08 20:56 -03

- El usuario instruyó continuar KYC realizando cambios y pruebas **solo** en el PostgreSQL local persistente, usando esa misma base como entorno de integración; quedan fuera Cloud SQL, proveedores externos, despliegue y publicación. Se conserva la prohibición de datos personales reales y el aislamiento de worktrees/demos ajenos.
- Preflight recién ejecutado en `127.0.0.1:5432/espacigo_dms_stage_20261020`: OID `16388`, PostgreSQL `18.6`, sesión administrativa local `postgres`, transacción `READ ONLY`; `pg_stat_activity` no mostró sesiones de cliente concurrentes. El listener local está en loopback `127.0.0.1:5432`.
- Inventario cluster-level: las bases accesibles observadas fueron `espacigo_dms_stage_20261020`, `postgres` y `template1` (más `template0` no conectable). El rol worker existente tiene 62 dependencias dentro de la base objetivo y una compartida de cluster; el nuevo LOGIN sigue ausente. No se cambió ninguna base ni rol.
- Preflight del runner usando `espacigo_migrator_login` + `SET ROLE espacigo_schema_owner` confirmó `READ ONLY`, base exacta, ledger V1–V85 continuo y coincidencia de nombres/checksums con el checkout. V86, V87 y V88 están pendientes con los hashes registrados arriba. El flujo fue solo `-plan-only`; no ejecutó DDL.
- Esta revalidación no equivale a autorización para actuar sobre otras bases del mismo clúster; cualquier bootstrap LOGIN/membresía se limita al rol/uso KYC acordado y deberá validar sus efectos globales. No se consultó ni modificó Cloud SQL.
