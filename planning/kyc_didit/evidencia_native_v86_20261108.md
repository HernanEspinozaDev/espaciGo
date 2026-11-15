# N04 — V86 aplicada en PostgreSQL nativo local (2026-11-08)

## Destino y alcance

- Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- Rama: `codex/marketplace/fix/kyc-06-save-identity-first`.
- HEAD base: `274f37e483a078890e92382a159fa49fd23c2fe8`, código probado con modificaciones sin commit.
- Autorización: instrucción humana de desarrollo sobre una única base local, registrada en §25 del plan y `AGENTS.md`. Sin bases efímeras, Cloud SQL, proveedor externo ni publicación.
- Base: `127.0.0.1:5432/espacigo_dms_stage_20261020`, OID 16388, PostgreSQL `180006`.

## Ruta ejecutada

1. Backup privado V85 en disco ext4, custom format, 0600; listado y parse completo de `pg_restore` válidos. SHA-256 `ee1827698341516f49877111730cd94749fed0ef5139ccc9b9894a5cae8c457c`.
2. Preparación administrativa versionada `db/repair/prepare_native_kyc_v86.sql`: guard de destino/ledger85/ausencia de función y CREATE preexistente, lock advisory y GRANT temporal de CREATE al owner. Ningún permiso runtime ampliado por esta preparación.
3. CLI `cmd/dbmigrate -native-kyc-v86` con confirmación exacta y fingerprint revisado del prefijo V1–V86 `d3ca3619312168dddebe1b7a722252d24381efcfd6bb60349a4dbd86007ec807`. Passfile designado por el código; secreto no leído ni impreso. Login TCP real `espacigo_migrator_login`, `SET ROLE espacigo_schema_owner`.
4. Staging exacto V1–V86; runner acotado al delta V85→V86, validación del ledger bajo lock, target/OID/engine/login/role revalidados después del lock y dentro de la transacción. Autoridad/ACL/membresías por el contrato canónico existente dentro de la transacción antes del SQL V86.
5. Finalizador versionado `db/repair/finalize_native_kyc_v86.sql` invocado por trap de salida: REVOKE de CREATE temporal y comprobación de que no queda efectivo.
6. Readback independiente con conexión nueva del migrador en transacción READ ONLY.

## Resultado observado

- Comando: exit 0, `native V86 applied; source-backed ledger V1..V86 verified; later migrations not applied`.
- Ledger: 86 filas, mínimo 1, máximo 86.
- V86: `V000086__identity_declaration_retention_purge.sql`.
- Checksum: `af864b793d8f4cbc9cf28dbfe65b8ed6d2388a0ade9a739fc8b480470bd3dc8e`.
- Función: `public.purge_expired_identity_declaration_revisions()`, owner `espacigo_schema_owner`, SECURITY DEFINER, `search_path=pg_catalog`.
- `has_schema_privilege('espacigo_schema_owner','public','CREATE') = false` tras finalización.
- `espacigo_runtime` conserva EXECUTE según el contrato intermedio V86. Su revocación exige V87→V88; no se presenta como contrato final ni se ejecutó la función.

## Fallo previo conservado y diagnóstico

El primer ensayo aplicó la ventana temporal de CREATE, pero el preflight de identidad rechazó una cuenta incompleta de membresías y no ejecutó V86. El trap retiró CREATE; una lectura nueva confirmó ledger85 y CREATE=false. La consulta parental encontró las dos aristas ya definidas en `authority_catalog.go`: schema_owner concedido al migrator y document_verification_fn concedido al schema_owner, ambas SET TRUE/INHERIT FALSE/ADMIN FALSE. Se retiró el conteo paralelo erróneo del preflight, conservando el contrato completo canónico en Plan y en la transacción bajo lock. El segundo ensayo completó V86 y su readback. No se relajó el contrato canónico ni se cambió una membresía.

La revisión delegada también afirmó que pgx no carga PGPASSFILE. Esa afirmación se refutó contra el módulo instalado pgx v5.11.0 (`pgconn/config.go`, carga de passfile y `FindPassword`) y un test con passfile temporal sintético sin conexión. No se añadieron contraseñas a DSN/argumentos ni se leyeron archivos de credenciales.

## Pruebas y límites

- TDD del gate target bajo lock: RED por helper ausente; GREEN de `TestNativeV86ExecutionTargetFailsClosed`, negativos base/OID/motor/session/current/query.
- `go test -race -count=1 ./cmd/dbmigrate -run '^TestNativeV86'` PASS; incluye consumo de passfile sintético, staging/pinning y rechazo CLI antes de configuración.
- `go vet ./internal/migrator ./cmd/dbmigrate` y `git diff --check` PASS.
- Backup posterior V86 listo para el siguiente delta: `.local/testing/kyc-native/pre-kyc-v86-20261108.dump`, 8289482 bytes, 0600, listado/parse completos válidos, SHA-256 `d1f3c3178a28b5d62df4727fea433007ce55a0074b61d4e528707bb542b758df`.
- No se inició API ni worker; no se invocó purga, no se prometió SLA, no se crearon/modificaron cuentas de aplicación ni sus contraseñas. No se aplicaron V87–V89 en esta evidencia.
- La prueba de guardado/readback KYC sigue sin ejecución por falta del acceso de login API y fixture registrado; aplicar V86 no la sustituye. Su gate histórico exacto V85 requiere actualización explícita para el ledger86 antes de ejecutarla.
