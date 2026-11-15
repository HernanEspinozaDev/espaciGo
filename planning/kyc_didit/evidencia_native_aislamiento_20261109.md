# N03 — Preparación del aislamiento nativo entre dos cuentas (2026-11-09)

## Estado y alcance

Checkout `/home/nandev/Documentos/projects/espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD base `274f37e483a078890e92382a159fa49fd23c2fe8`, árbol sin commit. Única DB: `127.0.0.1:5432/espacigo_dms_stage_20261020`, OID16388, PostgreSQL180006, ledger88. No otras bases, Docker, proveedor Didit, API general, workers ni Cloud SQL.

El cleanup con journal previo tiene revisión y ejecución nativa PASS en `revision_fixture_native_20261109.md`. Esta evidencia documenta la preparación, revisión y ejecución nativa del aislamiento cruzado entre dos cuentas en la misma base; **GET cruzado nativo PASS y N03 completada**.

## Implementación fuente completada por el padre

La entrega delegada solo agregó selector y prueba de basename; se conservó como parcial, sin presentarla como aislamiento terminado. El padre completó:

- Slots explícitos vació/1→`fixture.json`, 2→`fixture-2.json`; cualquier otro valor/traversal se rechaza antes de leer política, credenciales o conectar.
- Nuevo `identity_native_isolation_test.go` con validación de dos UUID canónicos, distintos y ambos verified/autorizados por la política antes de conexión.
- El ensayo de persistencia exige ahora `IDENTITY_DECLARATION_NATIVE_OTHER_ACCOUNT_ID` además de su configuración previa. Una configuración parcial falla sin conectar; el modo default sigue omitido.
- Ambos estados activos se comprueban como runtime en READ ONLY. La cuenta secundaria debe empezar sin declaración; si ya tiene datos, aborta sin borrarlos.
- Después del POST/GET propios de A, GET desde handler de B con repositorio PostgreSQL real/otro pool debe ser 404 sin ID/campos personales de A; después A debe conservar el mismo DTO completo. Cleanup/journal siguen limitados al input de escritura de A, no se limpia B.
- Auth HTTP sigue inyectada solo para los fixtures: no es JWT/BFF ni evidencia visual de sesión real.

## RED→GREEN y regresión

Se observaron fallos conductuales antes de corregir: faltante/mismo UUID/malformado/no inventariado/no verificado para secundaria eran aceptados por la validación anterior de solo la primaria; slot inválido alcanzaba lectura de política en el subprocess con directorio limpio. Después de los guards:

`go test -race -count=1` de cmd/api, cmd/dbmigrate, internal/migrator, identitydeclaration e internal/verification/... PASS; vet y diff-check PASS. La prueba subprocess usa raíz temporal sin política ni credenciales y demuestra rechazo temprano; no crea otra DB. Integraciones omitidas de la suite default no se cuentan como pruebas nativas.

Código congelado y revisión independiente source-only solicitada. El GET cruzado real espera ese veredicto antes de abrir la ventana temporal de cleanup.

## Segunda cuenta efectivamente preparada

Se ejecutó:

```sh
IDENTITY_DECLARATION_NATIVE_PREPARE_FIXTURE=1 \
IDENTITY_DECLARATION_NATIVE_FIXTURE_SLOT=2 GOTOOLCHAIN=local \
  go test -v -count=1 ./cmd/api -run '^TestNativeIdentityPrepareAuthorizedFixture$'
```

**PASS** por Register→VerifyEmail normales, sin SMTP externo. Receipt separado y política actualizada, ambos 0600 e ignorados; no se copiaron aquí correo, UUID, contraseñas ni tokens. El receipt primario se conservó.

Readback independiente TCP APIlogin→runtime, transacción READ ONLY: exactamente **dos cuentas autorizadas distintas activas** y **cero revisiones propias entre ambas**. Se alcanzó el máximo permitido de dos; no crear una tercera ni borrar receipts para forzar altas. Reutilizar únicamente estas cuentas por sus recibos privados.

## Backup y gate siguiente

Backup fresco después de preparar ambas cuentas: `.local/testing/kyc-native/pre-native-isolation-20261109.dump`, **8,290,390 bytes**, 0600/ext4; SHA-256 `ba89c2a26f87dbec2e4876bcd8c315e63f65652d287106b842ae3432f199421f`. Listado y parse completo pg_restore exit0; no restore real ni respaldo de roles globales.

En esta preparación inicial no se abrieron grants ni se aplicó SQL de migración.

## Revisión independiente y corrección de unicidad de inventario

La revisión estricta de código observó que `validateNativeIdentityAccountPair` validaba ambos UUID individualmente contra el inventario, pero no exigía explícitamente que las entradas de `account_inventory` fueran únicas, lo que permitiría que un inventario duplicado con el mismo ID superara la comprobación de par. Se implementó la prueba conductual RED `TestNativeIdentityPolicyRejectsDuplicateInventory`, se reforzó `validateNativeIdentityPolicy` en `identity_native_credentials_test.go` para exigir UUIDs canónicos y únicos en el inventario, y se verificó en verde con `go test -race` y `go vet`.

## Ejecución nativa del aislamiento PostgreSQL (GET cruzado) y readback

Con backup fresco validado y los scripts de prepare/finalize versionados (`prepare_native_kyc_fixture_cleanup.sql` y `finalize_native_kyc_fixture_cleanup.sql`), se ejecutó la suite nativa con finalizador por `trap`:

```sh
=== RUN   TestIdentityDeclarationRuntimePostgreSQLSaveRestartReadback
--- PASS: TestIdentityDeclarationRuntimePostgreSQLSaveRestartReadback (0.28s)
PASS
ok      github.com/HernanEspinozaDev/espaciGo/cmd/api   1.368s
```

Verificaciones satisfechas en la base nativa `127.0.0.1:5432/espacigo_dms_stage_20261020`:
1. Guardar declaración de cuenta A bajo `espacigo_api_login` -> `espacigo_runtime` (201 Created) con ciphertext verificado.
2. Reconstrucción de pool/handler y lectura GET de cuenta A -> 200 OK con datos completos.
3. Solicitud GET autenticada como cuenta B (con pool y repositorio real separado) hacia la declaración de cuenta A -> **404 Not Found (`ErrNotFound`)**, sin devolver identificador ni campos PII de A.
4. Relectura posterior como cuenta A -> intacta y con el mismo hash/contenido.
5. Limpieza automática mediante journal durable de cuenta A, sin afectar a cuenta B ni dejar filas residuales.

Readback independiente final en transacción `READ ONLY` como `postgres`:
- `own_active_accounts_retained`: 2 (ambas cuentas sintéticas conservadas).
- `own_revisions_remaining`: 0.
- `own_provider_attempts_remaining`: 0.
- `own_declaration_audit_remaining`: 0.
- `cleanup_delete_remaining`: false.
- `cleanup_column_select_remaining`: false.
- `owner_create`: false.
- `runtime_purge_execute`: true.
- `ledger_count`: 88, `ledger_version`: 88.
- `journal_absent=true` (archivo temporal `declaration-test-intent.json` retirado tras cleanup exitoso).

Con esta verificación, el slice **N03 queda resuelto y cerrado**. No se ejecutaron migraciones nuevas, Cloud SQL ni llamadas a Didit externo. Siguiente tarea del plan: **N05** (atomicidad/unicidad RUT / revisión de concurrencia y flujo).
