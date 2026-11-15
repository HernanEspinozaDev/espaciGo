# N04/N02 — V87–V88 nativas y autenticación de desarrollo (2026-11-08)

## Referencia

Checkout `/home/nandev/Documentos/projects/espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD base `274f37e483a078890e92382a159fa49fd23c2fe8`, árbol con cambios sin commit. Único destino: PostgreSQL nativo `127.0.0.1:5432/espacigo_dms_stage_20261020`, OID 16388, motor 180006. Autorización humana vigente de datos/esquema necesarios para KYC local en §25 del plan. No bases efímeras, Cloud SQL ni Didit externo.

## Aplicación secuencial

- V86 estaba aplicada y verificada; evidencia separada `evidencia_native_v86_20261108.md`.
- Padre terminó la entrega fuente tail incompleta: flags con rechazo temprano, snapshot exacto por versión, fingerprint pinneado y postcheck del checksum final; agregó V88 al gate canónico de autoridad con CREATE requerido. Se mantienen target/OID/identidad bajo lock y dentro de la transacción, sin ampliar los otros modos.
- V87 se aplicó sola sobre ledger86 mediante `-native-kyc-tail-version 87`. No se concedió CREATE ni se creó un login worker. Resultado CLI exit 0; conexión nueva READ ONLY confirmó ledger87 y grant de worker (runtime todavía permitido como estado intermedio).
- Se tomó y validó backup V87 antes de V88.
- V88 se aplicó sola sobre ledger87 mediante `-native-kyc-tail-version 88`. Preparación/finalización versionadas `db/repair/prepare_native_kyc_v88.sql` y `finalize_native_kyc_v88.sql`, ventana mínima CREATE del owner, retirada por trap y comprobada. CLI exit 0.

## Readback independiente final

Conexión TCP nueva autenticada como `espacigo_migrator_login`, `SET LOCAL ROLE espacigo_schema_owner`, transacción READ ONLY:

- Ledger: 88 filas, mínimo 1, máximo 88. El runner y postplan cotejan el prefijo staged exacto con nombres/checksums fuente.
- V87: `V000087__identity_declaration_retention_worker.sql`, checksum `5eda043ddf3a3d5bb89e8d470eaf4e5450ed0964a9d59fe232bdcb37888b1d45`.
- V88: `V000088__revoke_runtime_identity_purge_execute.sql`, checksum `44cc5e74b8f8fcfba1633f01009024a7fdeae5683077b13677ed7bf2894083a8`.
- Fingerprints de prefijo `version name checksum\n`: V1–V87 `4ad03308b1a8e24c611c4f3327ac7c44b0f6e7793e5905db45d87bddfb78375f`; V1–V88 `8a88aea304ce45e34128017b61c56ab3b517b896a1a119a64d7f93abee2967bb`.
- Owner CREATE efectivo: **false**.
- Función de purga: worker EXECUTE **true**, runtime EXECUTE **false**.
- ACL directa observada: solo `espacigo_privacy_cleanup_worker` y owner `espacigo_schema_owner`, EXECUTE sin grant option; sin PUBLIC.
- No se invocó la purga ni se inició worker/API. No se afirma scheduler, SLA ni borrado tras VERIFIED.

## Backups privados

Todos custom format, 0600, almacenamiento ext4 privado excluido de Git; listado y parse completo mediante pg_restore terminaron exit 0. No son ensayos de restauración ni respaldo de roles globales.

| Boundary | Bytes | SHA-256 |
| --- | --- | --- |
| V86 antes de V87 | 8289482 | `d1f3c3178a28b5d62df4727fea433007ce55a0074b61d4e528707bb542b758df` |
| V87 antes de V88 | 8289810 | `7f2c54ced6422ff4e93ae0fb45a6146aece9ae4e7460ad7cc4eff9131d088301` |
| V88 después de aplicar | 8289691 | `8847b157f35019248be420bf7da080c87a9bf961311dcc3e231a6ac9e18c6455` |

## Bloqueo de login API resuelto

La inspección administrativa devolvió únicamente el booleano `rolpassword IS NULL=true` para `espacigo_api_login`, sin leer/imprimir ningún hash. El bloqueo no era una contraseña rechazada: el login nunca tuvo credencial inicializada. Se creó una primera credencial aleatoria propia del desarrollo local usando `initialize_native_api_password.sql.tmpl`, con guards de destino/OID/motor/ledger88/atributos, contraseña todavía NULL y ausencia de sesiones del login. No se sobrescribió ninguna contraseña existente ni se elevaron privilegios.

Passfile API guardado bajo `.local/testing/kyc-native/`, regular 0600/directorio0700, ignorado por Git; SQL privado renderizado retirado tras aplicar. No se tocaron passfiles existentes. Readback de conexión TCP nueva confirmó `session_user=espacigo_api_login`, `current_user=espacigo_runtime` en READ ONLY.

## Prueba nativa preparada, sin afirmar el recorrido completo

- La dependencia pgx consume passfiles explícitos por rol; helpers prueban archivos sintéticos y validan privacidad/path antes de conexión. API y cleanup usan fuentes distintas, sin password en URL ni fallback al passfile del otro rol.
- El test exige el UUID de una cuenta `verified=true` de la política privada, no solo un UUID existente. La cuenta aún debe prepararse por Register→VerifyEmail del servicio normal, sin proveedor de correo externo.
- El preflight por subprocess sudo falló al autenticarse fuera de la terminal principal. Se sustituyó por el mecanismo ya comprobado del migrador dedicado en una transacción READ ONLY, sin superuser ni cambio de permisos. **`TestNativeIdentityExternalReadOnlyPreflight` pasó contra esta base**: valida identidad/OID/motor/read-only y todos los nombres/checksums V1–V88.
- Pruebas focalizadas de cmd/api y CLI/migrador con `-race`, vet y diff-check pasaron. Regresiones nuevas observaron RED antes de sus helpers/actualización de ledger; validadores de archivos/política usan solo archivos de fixture.
- Guardar→pool/handler nuevo→readback KYC sigue pendiente de la cuenta sintética del corte. V89 todavía no está aplicada ni conectada al guardado; claim transaccional, outbox y purga de snapshots referenciados permanecen tareas distintas.
