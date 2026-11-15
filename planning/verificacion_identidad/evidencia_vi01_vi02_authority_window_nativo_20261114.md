# Evidencia — aplicación nativa separada V95/V96

- **Fecha:** 2026-11-14.
- **Checkout / rama / HEAD base:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`; worktree compartido y dirty, preservado sin reset/stash/stage/commit/push.
- **Destino:** PostgreSQL nativo `127.0.0.1:5432/espacigo_dms_stage_20261020`, OID `16388`, PostgreSQL `18.7 (Ubuntu 18.7-1.pgdg26.04+2)`, `server_version_num=180007`.
- **Resultado vigente:** V95 y V96 aplicadas en invocaciones de versión exacta separadas; ledger continuo V1–V96, 96 filas, nombres/checksums source-backed y cero pendientes. `public CREATE=false` al final. No se aplicó ni alteró Cloud SQL.

## Gates y respaldos

La revisión de fuente independiente `deleg_7d6b9262`, aclarada por `deleg_afbf7c81`, quedó aprobada para la fuente (`source_review_pass=true`, `source_blockers=[]`); no sustituyó los gates operativos. El runner `authority-window` desechable había pasado con cinco resultados DB requeridos, ninguno omitido/faltante; Go race/vet/build, suites Python focales, `py_compile`, formato y `git diff --check` también pasaron. Ese runner no demuestra persistencia nativa.

Antes de V95 se validó este backup custom, no incluido en Git:

- `/home/nandev/.local/share/espacigo-native-backups/espacigo_dms_stage_20261020-pre-kyc-v95-v96-authority-20261114T225325Z.dump`
- 7,631,089 bytes; modo `0600`; UID propietario `1000`; filesystem `ext4 /dev/nvme0n1p2`; SHA-256 `4780b3338ca21f0bb9644be79dc599e9462bbb553adc447418529b7005327650`.
- `pg_restore --list`: 1,405 entradas; incluye `TABLE DATA public schema_migrations`. `pg_restore --exit-on-error --file=/dev/null`: PASS (parseo íntegro). No se hizo restauración completa a otra instancia.

Después del cierre y readback V95, y antes de abrir V96, se tomó y validó un segundo backup:

- `/home/nandev/.local/share/espacigo-native-backups/espacigo_dms_stage_20261020-post-v95-pre-v96-20261114T231139Z.dump`
- 7,640,167 bytes; modo `0600`; UID propietario `1000`; filesystem `ext4 /dev/nvme0n1p2`; SHA-256 `14a4aace70b302fbb1e1dbb451f858aaf3c5083bfded2067374c6c6b47892dab`.
- `pg_restore --list`: 1,420 entradas; incluye `TABLE DATA public schema_migrations`. Parseo íntegro con `pg_restore --exit-on-error --file=/dev/null`: PASS. No se hizo restauración completa.

El recibo operativo privado, modo `0600`, conserva la secuencia, los pins y ambos hashes en `~/.local/share/espacigo-native-backups/native-v95-v96-operation-20261114T230618Z.json`.

## V95 — transición autorizada, aplicación y cierre

1. `cmd/dbmigrate -plan-only` READ ONLY confirmó target exacto, history existente, 94 aplicadas y solo V95/V96 pendientes. `identity_ddl_authority` reportó `session_user=espacigo_migrator_login`, `current_user=espacigo_schema_owner`, `public_create=false`, owners `postgres` para case/event y los roles/grants requeridos.
2. El cuerpo guard de `db/repair/prepare_native_kyc_v95.sql` (SHA-256 `5418142f731eb892a4caadd3713df4d813017d3ed477d74790db05c9eec59590`) se ejecutó primero en `BEGIN TRANSACTION READ ONLY`, con el advisory lock canónico; `DO` pasó y la transacción se revirtió sin DDL.
3. La preparación versionada terminó exit 0: solo transfirió `public.document_verification_case` y `public.document_verification_event` a `espacigo_schema_owner`, dejando que PostgreSQL transfiriera sus ocho índices existentes y `public.document_verification_event_id_seq`; abrió únicamente `public CREATE` temporal para esa ventana. El `DO` post-preparación confirmó owners, ACL/grantor, privilegios runtime, ledger V1–V94 exacto y ACL pública más el grant temporal.
4. Un readback independiente, en conexión/transacción `READ ONLY`, volvió a ejecutar el verificador fuente y pasó: las dos tablas, ocho índices y secuencia eran del owner autorizado; los tuples runtime conservaron grantee/privilege/grant option con grantor `espacigo_schema_owner`; ledger seguía en V94; `CREATE` estaba abierto solo para la ventana V95.
5. El executor nativo `cmd/dbmigrate -native-kyc-tail-version 95` terminó exit 0: `native V95 applied; source-backed ledger V1..V95 verified; later migrations not applied`. V95 SHA-256: `351a9bbdbd890e34b3b5db46ecffa971d49b1f5d4519d9bf7f2b1eae1e897275`; fingerprint fuente V1..V95: `67077c1996cc72f2d6177d801fbfbbc445a53ba23b6306fc83ee18adfcbf3554`.
6. `db/repair/finalize_native_kyc_v95.sql` (SHA-256 `35e9b8afa6cb2acf298f2c78ecc2028ebb0417f92980d453abcbdb82436849e9`) terminó exit 0 y revocó `CREATE`. Su comprobación posterior al commit y un readback independiente READ ONLY pasaron: `public CREATE=false`, ACL pública exactamente en baseline, owners autorizados y ledger V94/V95 dentro del límite esperado.
7. Plan READ ONLY posterior verificó 95 aplicadas, V95 exacta, solo V96 pendiente y `public CREATE=false`.

## V96 — ventana separada y cierre

1. Tras el backup post-V95 se ejecutó el guard fuente de `db/repair/prepare_native_kyc_v96.sql` (SHA-256 `91bc78644a4ec9a6197292557df26ec689c23910f259c62f2cf22b4f07b244fd`) dentro de transacción READ ONLY con lock canónico: PASS y rollback, sin DDL. Comprobó ledger V1–V95, owners, nueve índices ligados (ocho previos más el índice creado por V95), secuencia, ACL/grantors runtime, roles/membresías y ausencia de esquema V96 parcial.
2. La preparación V96 terminó exit 0: abrió únicamente el `CREATE` temporal; no cambió owners. El verificador de preparación pasó en transacción READ ONLY independiente: ledger V95 y objetos/ACLs V95 intactos, ACL pública baseline más el grant temporal.
3. `cmd/dbmigrate -native-kyc-tail-version 96` terminó exit 0: `native V96 applied; source-backed ledger V1..V96 verified; later migrations not applied`. V96 SHA-256: `a206e74995db47112c6b382c33ffab5badcab2ec9b9859af0fbd018478cc7ca5`; fingerprint fuente V1..V96: `c71a06dfac444537bd0b05ef12346c57bcfda686c7761d53b52c24af25d02ab7`.
4. `db/repair/finalize_native_kyc_v96.sql` (SHA-256 `2d7f17c324690de1061e57d3f4c825af00a44022e8425ba71cbb199347a7afc5`) terminó exit 0 y revocó `CREATE`. El verificador post-commit más un readback independiente READ ONLY pasaron para target, ACL pública exacta sin CREATE, owners de las 12 relaciones autorizadas, tuples/grantors runtime, ledger V1–V96 y objetos V96: columnas, constraints y función guard con owner esperado y sin `SECURITY DEFINER`.
5. Plan final READ ONLY: DB exacta, PostgreSQL 18.7, history presente, V1–V96 (96), V96 con checksum exacto y cero pendientes. No mostró `identity_ddl_authority` porque no quedó DDL pendiente; los readbacks versionados anteriores verificaron post-estado y authority.

## Límites y estado pendiente

- El respaldo se validó por listing y parseo íntegro, no por una restauración completa en otra instancia.
- Cloud SQL no se consultó ni modificó; el último ledger Cloud SQL verificado sigue siendo histórico del **2026-11-07** (V1–V83, V84 pendiente en esa fecha). Su estado actual permanece **no verificado**. La aplicación nativa no autoriza Cloud SQL ni ejecución GCP.
- No se ejecutó la API contra la base persistente, pruebas de aceptación visual/online, cuentas reales, Didit ni datos personales reales. No se demuestra todavía E2E funcional, storage GCS/CMEK, MFA, operadores humanos, scanner/cuarentena, durabilidad tras reinicio, revocación/expiración integral, accesibilidad ni aceptación de producto. VI-01/VI-02 y el MVP completo no se declaran cerrados por el ledger aplicado.
- Sin commit, push, integración a `main`, despliegue o cambio de infraestructura. En este corte no se modificó `AGENTS.md` ni se reintentó su gate protegido.
