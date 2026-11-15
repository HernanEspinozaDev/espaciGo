# N06 — Atomicidad y outbox durable; integración nativa completada (2026-11-09)

## Referencia

Checkout `/home/nandev/Documentos/projects/espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD base `274f37e483a078890e92382a159fa49fd23c2fe8`.
Destino local nativo exclusivo: `espacigo_dms_stage_20261020` (OID 16388, PostgreSQL 180006). V89 y V90 están aplicadas; ledger exacto V1–V90, checksum V90 `23bbacde2cf992f912926f84945c701422b097598e698b2e8f38158b61622592`. La reparación ACL estrecha de V90 permanece limitada al owner; no concede DELETE/UPDATE sobre outbox al runtime.

## Alcance implementado

1. `internal/adapters/postgres/identitydeclaration/session_deletion_queue.go` implementa `PostgresSessionDeletionQueue` sobre `public.outbox_evento_local` con clave idempotente, lectura limitada de pendientes y ACK por `entregada_en`/`retirar_en`. Usa `INSERT, SELECT, UPDATE` disponibles para `espacigo_runtime`; no borra físicamente la outbox.
2. `cmd/api/main.go` instancia la cola con el pool runtime y la pasa al servicio. `newRuntimeDiditService` reconoce `RutClaimStore` y `SessionDeletionQueue` sin cambiar la llamada previa de tests.
3. `DiditService` falla al construir si no puede leer la cola persistida; `RetryPendingDeletions` vuelve a consultar el outbox al comenzar cada reintento, conserva registros tras fallo de proveedor/ACK y propaga el primer error en vez de ocultarlo.
4. Eventos tardíos sobre intentos terminales `REJECTED`, `EXPIRED` o `FAILED` se marcan `IGNORED`, sin reactivar la sesión. El comportamiento existente para `APPROVED` conserva el estado y completa los eventos tardíos como `PROCESSED`.
5. Cleanup del fixture nativo ahora retira claims de RUT propios antes de eliminar la revisión y verifica residuo; no amplía privilegios.

## Pruebas ejecutadas

- `GOTOOLCHAIN=local go test -race -count=1 ./internal/verification/... ./internal/adapters/postgres/identitydeclaration/... ./cmd/api` — PASS. Incluye ciclo de outbox contra el PostgreSQL nativo con cuentas runtime y sessions UUID únicas, reintento del servicio en memoria, guardas de eventos terminales y wiring.
- `GOTOOLCHAIN=local go vet ./internal/verification/... ./internal/adapters/postgres/identitydeclaration/... ./cmd/api` — PASS.
- `git diff --check` — PASS.
- `python3 scripts/gcp/test_main_release.py` — PASS, 17 tests (regresión previa a la integración nativa).

Actualización del reintento duplicado:

- El ensayo nativo anterior alcanzó aprobación, pero falló porque un evento previamente almacenado como `FAILED` volvió a procesarse sin que el resultado indicara `Duplicate=true`. La salida fallida identificó el estado `IN_REVIEW`, `PROCESSED`, `Duplicate=false`; no se considera aceptación del ensayo.
- Se añadió la aserción de regresión al reintento recuperado en `TestDiditServiceFullLifecycleAndSecurityInvariants`; observó RED antes del cambio y GREEN después. `ProcessWebhook` ahora propaga el indicador de duplicado en todos sus retornos exitosos de procesamiento/reconciliación.
- La suite `go test -race -count=1 ./internal/verification/... ./internal/adapters/postgres/identitydeclaration/... ./cmd/api`, `go vet` de esos paquetes y `git diff --check` — PASS.
- Preflight separado por identidades mínimas: `espacigo_migrator_login`→`espacigo_schema_owner` confirmó destino/OID/motor/ledger/checksum; `espacigo_api_login`→`espacigo_runtime` confirmó ambas cuentas sintéticas autorizadas activas, cero revisiones, cero intentos/eventos/claims/outbox del ámbito de prueba y cola de borrado sin pendientes. Journal privado ausente.
- Backup fresco previo a los grants: `.local/testing/kyc-native/pre-n06-duplicate-retry-20261109.dump`, custom format, 8,319,180 bytes, modo 0600 sobre ext4; `pg_restore --list` y parse completo `pg_restore --exit-on-error --file=/dev/null` PASS. SHA-256 `9e0f5c3ab4a0bbba95882480d19ee0c294687aeff0795192a62100c9a390a54b`.
- Las operaciones de superusuario se ejecutaron como comandos `sudo ...` directos de nivel superior en el terminal Hermes, sin leer ni imprimir `SUDO_PASSWORD`. `prepare_native_kyc_fixture_cleanup.sql` terminó COMMIT; el readback verificó la ventana exacta para el owner y que runtime seguía sin CREATE/DELETE. `finalize_native_kyc_fixture_cleanup.sql` terminó COMMIT y retiró los grants temporales.
- Integración nativa real `GOTOOLCHAIN=local go test -race -count=1 ./cmd/api -run '^TestIdentityDeclarationRuntimePostgreSQLSaveRestartReadback$' -v` — PASS, 0.34s; paquete 1.411s. El test usó solo fixture y cliente Didit sintéticos, sin red/proveedor externo.
- La prueba indujo SQLSTATE `P0001` durante la purga y comprobó rollback íntegro: snapshot, intento, evento, claim y outbox conservados en sus estados previos; el cliente no ejecutó borrado remoto. El reintento del mismo evento fue `Duplicate=true`, confirmó `APPROVED`, claim verificado, evento procesado, snapshot cifrado purgado y outbox durable; la primera llamada sintética de borrado falló como estaba programado.
- Un servicio y pool nuevos recuperaron el estado `APPROVED` y la sesión pendiente desde PostgreSQL; el retry borró y confirmó ACK. El test verificó que la fila ACK permanece al menos 30 días y deja de estar pendiente.
- Readback posterior independiente: dos cuentas sintéticas activas conservadas; cero revisiones, intentos, eventos, claims y filas outbox de la prueba; cola sin pendientes; journal ausente; ledger V1–V90/checksum intactos; grants temporales revocados y privilegios runtime sin cambios. El backup permaneció privado y volvió a superar ambos checks de `pg_restore`.

## Resultado y límites

- La aprobación, claim verificado, evento, outbox y purga quedaron demostrados atómicamente en la base nativa, incluidos rollback inducido, retry duplicado, recuperación desde un pool nuevo, borrado/ACK sintético y retención.
- No se ejecutó un worker general ni proveedor externo; no se afirma SLA de borrado, ni aceptación de producto o cumplimiento legal. La prueba demuestra únicamente el flujo sintético local.
- **N06 queda resuelta/cerrada en el alcance local sintético.** N07 y N08 siguen pendientes; N07 es el siguiente corte del plan.
