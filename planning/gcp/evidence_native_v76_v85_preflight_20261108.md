# Evidencia — preflight y backup nativo V76→V85

- Fecha: 2026-11-08.
- Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- HEAD: `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- Alcance autorizado por el usuario: destino nativo `127.0.0.1:5432/espacigo_dms_stage_20261020`; gates, transición específica roles/configuración, backup y aplicación secuencial V77–V85. No incluye Cloud SQL ni despliegue KYC.
- Autorización posterior explícita: generar una credencial nueva solo para `espacigo_migrator_login` y guardarla en `/home/nandev/.local/share/espacigo-native-secrets/pgpass` con modo 0600; no tocar ni rotar credenciales existentes. La credencial fue creada y la autenticación TCP del login se validó; su valor no se registra.
- Las secciones de preflight y backup inicial más abajo son instantáneas históricas anteriores a V77–V85; el resultado final se registra en “Aplicación nativa”.

## Preflight READ ONLY — snapshot previo a V77–V85

Se ejecutó `sudo id` con autenticación local; se verificó que `127.0.0.1:5432` pertenece al postmaster PostgreSQL local PID 7104. La conexión peer fue a la base exacta, no a un contenedor ni a Cloud SQL.

Lectura en transacción `REPEATABLE READ READ ONLY`:

- PostgreSQL `18.6`; DB OID `16388`; owner de base `postgres`; `session_user=current_user=postgres`.
- Ledger continuo V001–V076, 76 filas. Comparación programática del nombre y SHA-256 de cada fila aplicada con los archivos actuales: 0 discrepancias; todas las 76 coinciden. Pendientes del candidato actual: V77–V85.
- Entre los cinco roles contractuales consultados solo existe `espacigo_runtime`; está LOGIN y no tiene superuser, CREATEDB, CREATEROLE, replication, bypass RLS ni inherit. Faltan `espacigo_api_login`, `espacigo_migrator_login`, `espacigo_schema_owner` y `espacigo_document_verification_fn`.
- No se encontraron sesiones cliente en el clúster durante la lectura. El clúster tiene solo las bases `espacigo_dms_stage_20261020` y `postgres`. El rol runtime no figura como owner; sus ACL tienen dependencias catalogadas en el destino y CONNECT concedido por postgres. No se cambió ni revocó nada.
- `public.identity_verification_attempts` y `public.didit_webhook_inbox` no existen aún en este destino (V84 está pendiente).

La lectura es una instantánea del 2026-11-08, no una garantía sobre reconexiones futuras.

## Backup previo

- Archivo privado fuera del repositorio: `espacigo_dms_stage_20261020-pre-v77-v85-20261108T052111Z.dump`.
- Directorio privado: `/home/nandev/.local/share/espacigo-native-backups` (modo 0700, owner `nandev`); archivo modo 0600, owner `nandev`.
- Formato custom, 8,081,880 bytes; SHA-256 `e09ae684b9b82a9bdaaf49c99f4c70dbad5a18cc8717c04fd6dc343fce5665ad`.
- `pg_restore --list`: 1,199 entradas. `pg_restore --exit-on-error --file=/dev/null`: exit 0. Esto valida el catálogo y parseo completo del archivo; no es un ensayo de restauración.
- El backup no incluye globals/roles de clúster. El contrato de rol/ACL queda registrado por separado; no se exportaron hashes de contraseñas.

## Ensayos locales

- `bash scripts/test-migration-runner-postgres.sh full`: 111 resultados DB obligatorios PASS, ninguno faltante/omitido; fresh V001–V085 y upgrades V75/V76→V85, entre otros. PostgreSQL desechable aislado sin red, puertos ni volúmenes; contenedor propio retirado y ausencia verificada. La regresión incluye esquemas extra en `search_path` y el ACL predeterminado `PUBLIC TEMPORARY`.
- `GOTOOLCHAIN=local go test -count=1 ./cmd/dbmigrate ./internal/migrator ./internal/verification/declarationcrypto`: PASS.
- `GOTOOLCHAIN=local go vet ./cmd/dbmigrate ./internal/migrator ./internal/verification/declarationcrypto`: PASS.
- Pruebas Python focalizadas anteriores: 26 tests PASS. Después del ajuste de autoridad V84, `PYTHONPATH=scripts/gcp python3 -m unittest test_v84_authority test_migration_release_manifest`: 7 tests PASS.
- `git diff --check`: PASS.
- La suite Python completa de `scripts/gcp` intentó 84 tests pero terminó con 2 errores de importación por falta de `PyYAML` (`test_full_release_labels`, `test_web_release`); no se instaló dependencia. Los otros tests completaron.

## Aplicación nativa — 2026-11-08

- Destino revalidado: PostgreSQL nativo `127.0.0.1:5432/espacigo_dms_stage_20261020`, OID 16388, versión 18.6. No se conectó a Cloud SQL ni se desplegó KYC.
- El preflight final encontró ledger V1–V83 y pendientes V84–V85; identidad `session_user=espacigo_migrator_login`, `current_user=espacigo_schema_owner`. La autoridad V84 pasó validación Go/Python y el `search_path` efectivo fue restringido a `pg_catalog,public` dentro de las transacciones de plan/aplicación.
- Aplicación secuencial V84 y V85 completada mediante `cmd/dbmigrate`; el postcheck independiente releyó ledger V1–V85 continuo (85 filas) y comparó nombres/checksums con el inventario fijado. No quedan migraciones pendientes.
- V84 y V85 recibieron únicamente las ventanas autorizadas CREATE sobre `public` y REFERENCES(`usuario.id`); los finalizadores revocaron ambas después de cada fase. La lectura posterior confirmó `CREATE=false`, `REFERENCES=false`, ledger SELECT/INSERT disponibles, exactamente tres memberships contractuales y sin ACL PUBLIC sobre las tres tablas de V85.
- El OID de `espacigo_runtime` se conservó. Roles login/API y NOLOGIN permanecen sin atributos privilegiados; las cinco tablas de identidad/verificación examinadas están vacías. El postcheck validó además los grants de columnas de runtime y owners esperados.
- Backups previos a cada fase, fuera del repositorio, owner `nandev`, modo 0600, `pg_restore --list` y parseo completo exitosos:
  - V84: `/home/nandev/.local/share/espacigo-native-backups/espacigo_dms_stage_20261020-post-v83-pre-v84-20261108T082755Z.dump`, 8,212,763 bytes, SHA-256 `d978efb908cba1dce20c16bfc6475d2994f1b8712462f591b59865d2934b5e34`.
  - V85: `/home/nandev/.local/share/espacigo-native-backups/espacigo_dms_stage_20261020-post-v84-pre-v85-20261108T082756Z.dump`, 8,230,019 bytes, SHA-256 `78edd962bbbc1820b75c18d7564285b9e560b4d8a8508516abb796d31d6915e0`.
- Diagnóstico resuelto: el gate inicialmente rechazaba el ACL preexistente, no grantable y otorgado por el owner, `PUBLIC TEMPORARY` de la base. El contrato Go/Python y el harness ahora aceptan solo ese caso exacto; los casos con grant option o grantor distinto siguen rechazados. No se cambió el ACL de la base. La regresión también reproduce schemas adicionales en el `search_path`; el runner usa `SET LOCAL search_path TO public` para que `pg_catalog` conserve precedencia efectiva.
- Hubo intentos de V84 que fallaron cerrados; las transacciones se revirtieron, el finalizador retiró las ventanas temporales y el ledger permaneció en V83. La ejecución final creó backups frescos y aplicó V84 y V85 por separado.
- Revisión final READ ONLY: `sudo -u postgres psql` confirmó DB/OID/engine, ledger 1–85, membresías y flags; Go plan independiente coincidió con las 85 filas del inventario. Ambos backups se releen y validaron de nuevo.
- Estado de código: rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD `274f37e483a078890e92382a159fa49fd23c2fe8`; cambios locales no comprometidos preservados.
- Alcance restante: esto aplica el esquema local; **no** demuestra flujo KYC end-to-end ni autoriza despliegue. Cloud SQL no fue leído/aplicado en este corte; no declarar bases unificadas. `semantic_audit_closed=false` permanece.