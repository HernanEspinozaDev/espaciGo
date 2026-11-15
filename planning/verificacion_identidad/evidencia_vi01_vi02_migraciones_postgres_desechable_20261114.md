# Evidencia VI-01/VI-02 — runner PostgreSQL desechable V1–V96

**Fecha:** 2026-11-14
**Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`
**Rama:** `codex/marketplace/fix/kyc-06-save-identity-first`
**HEAD base observado:** `274f37e483a078890e92382a159fa49fd23c2fe8`; los archivos probados incluyen cambios locales sin commit. El worktree estaba dirty y contiene otros cambios no atribuidos a esta evidencia.

## Ejecución y resultado

Se ejecutó:

```text
TMPDIR="$HOME/.hermes/cache/scratch" scripts/test-migration-runner-postgres.sh full
```

Resultado de la salida real del runner:

- Contenedor propio `espacigo-migration-ci-983801`, imagen `postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677`.
- Aislamiento verificado por el script: `network=none`, sin puertos publicados, sin volúmenes, `PGDATA=tmpfs`; socket temporal bajo `$HOME/.hermes/cache/scratch`.
- **159 resultados de pruebas de base de datos requeridos; ninguno faltante ni omitido.**
- Pasaron instalación fresh V1–V96, upgrades V75/V76→V96, contratos V94→V95 y V95→V96, plan de solo lectura, no-op, recuperación de fallos, checksums, concurrencia y pruebas del trigger V19.
- El script verificó la retirada del contenedor de ensayo propio.
- Salida íntegra conservada temporalmente en `/home/nandev/.hermes/cache/scratch/kyc-migration-ci.FEsm9v.log`; el runner informa exit 0.

Verificaciones adicionales posteriores: `git diff --check`, `gofmt -d` para los tres tests migratorios editados, el test de pins `go test ./cmd/dbmigrate -run '^TestNativeKYCTailV95V96PinsReviewedSourceAndSnapshot$' -count=1`, el contrato `go test ./internal/migrator -run '^TestIdentityV96ReviewerObservationAuditContract$' -count=1` y `python3 -m json.tool db/release-policy.json`; todos terminaron exit 0. En una comprobación posterior, con variables opt-in de DB/credenciales retiradas del entorno, `go test -race -count=1 ./cmd/dbmigrate ./internal/migrator` y `go vet ./cmd/dbmigrate ./internal/migrator` terminaron exit 0; el acceso PostgreSQL del runner integral está evidenciado por separado arriba. `gofmt` focal y `git diff --check` también terminaron exit 0.

## Ajustes de cobertura que forman parte del resultado

- La integración de V95/V96 conserva negativos de ownership para tabla de caso y tabla de eventos, verifica que cada rechazo no altere el ledger ni deje columnas parciales, y comprueba los prefijos/aplicación/no-op hasta V96.
- El fixture sintético posterior a V90 reproduce la reparación de ACL mínima ya descrita por `db/repair/repair_native_kyc_v90_outbox_definer_acl.sql`: `SELECT` solo en las columnas que lee la función y `INSERT` solo en las columnas que escribe `commit_verified_identity_decision`; se relee que no exista permiso de `INSERT` a nivel de tabla. Esto corresponde al estado autorizado del ensayo desechable, no a una nueva concesión en destino persistente.
- La prueba API de liberación de vínculo usa un `case_transitioned` válido y satisface el CHECK histórico de `reviewer_id`; es una prueba del adaptador/guard de persistencia, no de autorización de revisor ni de auto-revisión.
- El flujo V85 usa un `RutClaimStore` PostgreSQL sintético y limpia su claim. Tras la aprobación, verifica que no se vuelva a enlazar como actual la declaración purgada, que el estado de intento no se convierta en `VERIFIED` cuando existe otra revisión actual, y que `encrypted_decision` quede nulo según V93. Estos asserts caracterizan persistencia sintética; no habilitan ni validan el proveedor externo en producto.

## Límites y estado de destinos

Esta ejecución aplicó migraciones únicamente a bases dentro del contenedor desechable aislado y eliminado. **No aplicó DDL al PostgreSQL nativo ni a Cloud SQL.** El último preflight nativo separado, READ ONLY, observó ledger V1–V94, V95/V96 pendientes, `public CREATE=false` y ownership no compatible con los guards; esos blockers siguen vigentes. Cloud SQL no se consultó en esta ejecución y su última evidencia de esquema sigue fechada 2026-11-07. Ningún test de este runner acredita GCS/CMEK, MFA efectiva, revisores reales, aceptación del producto, despliegue o persistencia/reinicio en los destinos KYC nombrados.

Huella calculada para el inventario de fuentes del checkout probado: 96 migraciones, V96 SHA-256 `a206e74995db47112c6b382c33ffab5badcab2ec9b9859af0fbd018478cc7ca5`, fingerprint V1–V96 `c71a06dfac444537bd0b05ef12346c57bcfda686c7761d53b52c24af25d02ab7`. No hubo commit, push ni integración a `main`.
