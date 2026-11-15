# N05 — V89 nativa y persistencia durable de claims HMAC de RUT (2026-11-09)

## Referencia

Checkout `/home/nandev/Documentos/projects/espaciGo-main-sync`, rama `codex/marketplace/feature/kyc-didit-integration-native-local-clean`.
Destino local nativo exclusivo: `espacigo_dms_stage_20261020` (OID 16388, PostgreSQL 180006).

## Alcance ejecutado (N05)

1. **Migración versionada V89 aplicada y verificada:**
   - Archivo: `db/migrations/V000089__identity_rut_claims.sql`
   - Checksum: `93671b484f3118c697eebc4654f4b32325cce4231582b488f2e1a8d9b7fece9c`
   - Fingerprint acumulado hasta V89: `94d2f333ee0d5649f54821ac4094e14e4df32cc2cf8c6b4c7d7a4cfe9dca30e5`
   - Script acotado de preparación: `db/repair/prepare_native_kyc_v89.sql` (otorga `CREATE` temporal al owner y `REFERENCES(id)` sobre `usuario` bajo advisory lock y guardias estrictas de OID 16388, motor 180006 y ledger de 88 filas).
   - Aplicación mediante CLI versionado: `PGPASSFILE=/home/nandev/.local/share/espacigo-native-secrets/pgpass DATABASE_URL="postgresql://espacigo_migrator_login@127.0.0.1:5432/espacigo_dms_stage_20261020?sslmode=disable" DATABASE_CONNECTION_MODE=direct DATABASE_SET_ROLE=espacigo_schema_owner ./bin/dbmigrate -native-kyc-tail-version 89 -confirm-native-kyc-tail espacigo_dms_stage_20261020 -native-kyc-tail-source-sha256 94d2f333ee0d5649f54821ac4094e14e4df32cc2cf8c6b4c7d7a4cfe9dca30e5`
   - Script finalizador verificado: `db/repair/finalize_native_kyc_v89.sql` (retira `CREATE` y `REFERENCES` del schema owner, valida ausencia residual de privilegios y confirma ledger de 89 filas).
   - Tabla creada: `public.identity_rut_claim` con clave primaria `fingerprint`, unicidad por `account_id`, claves foráneas a `usuario(account_id)` y a `identity_declaration_revision(declaration_id, account_id)` con `ON DELETE RESTRICT`, privilegios `SELECT, INSERT, UPDATE, DELETE` otorgados exclusivamente al rol `espacigo_runtime`.

2. **Implementación de adapter y cableado en runtime:**
   - [rut_claim.go](file:///home/nandev/Documentos/projects/espaciGo-main-sync/internal/adapters/postgres/identitydeclaration/rut_claim.go): interfaz desacoplada de base de datos para soporte transaccional y directo (`Exec`, `QueryRow`), método `MarkVerified` con semántica idempotente si el claim ya fue verificado para la misma cuenta y declaración, o detección de conflicto ante intento de sobreescritura ajena.
   - [main.go](file:///home/nandev/Documentos/projects/espaciGo-main-sync/cmd/api/main.go) y [identity_declaration_runtime.go](file:///home/nandev/Documentos/projects/espaciGo-main-sync/cmd/api/identity_declaration_runtime.go): cableado en runtime del `RutClaimStore` de PostgreSQL al `DiditService`.
   - Actualización de política en `db/release-policy.json` registrando V89 como migración manual bajo estrategia `security-tightening`.

3. **Pruebas y verificación (RED → GREEN):**
   - Unitarias: `TestRutClaimMarkVerifiedUnit` (5 casos cubriendo actualización exitosa, idempotencia, conflicto por distinto dueño, conflicto por claim no verificado y error en lookup).
   - Cableado runtime: `TestRuntimeDiditServiceWiresProvidedRutClaimStore`.
   - Integración nativa PostgreSQL: `TestRutClaimPostgresClaimLifecycle` adaptada a la política privada (`test-account-policy.json`), utilizando únicamente las 2 identidades autorizadas (`primary` y `secondary`), sin escrituras a `public.usuario`, ejerciendo claim inicial, unicidad global de fingerprint, reemplazo atómico por la misma cuenta, liberación (`Release`) y claim posterior por segunda cuenta.
   - Suite completa con `-race` en `./cmd/api`, `./cmd/dbmigrate`, `./internal/migrator`, `./internal/verification/...` e `./internal/adapters/postgres/identitydeclaration/...`: PASS sin data races.
   - Verificación de release canónico: `python3 scripts/gcp/test_main_release.py`: 17 tests PASS.
   - `git diff --check`: limpio (0 advertencias).

## Backup previo
- Archivo: `.local/testing/kyc-native/pre-v89-20261109.dump`
- Modo: 0600, formato custom pg_dump con lista y catálogo verificados.
- SHA-256: `56af1755700a141fa458c8aae9fbbe6330f22ac77fed18fc4ec3b68687549dbd`
