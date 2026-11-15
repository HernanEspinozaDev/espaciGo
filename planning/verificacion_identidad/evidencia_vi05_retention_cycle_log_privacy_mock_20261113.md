# Evidencia — VI-05, privacidad de logs del ciclo de retención

- **Fecha:** 2026-11-13.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron cambios ajenos.

## Alcance

`TestRetentionCycleDoesNotLogSyntheticPersonalIdentifiers` ejecuta `runIdentityRetentionCycle` real con purgers locales fake. Los errores centinela contienen correo sintético, UUIDs sintéticos de cuenta/solicitud/evidencia y texto privado sintético. El test comprueba que ambos purgers se ejecuten una vez, que se emitan una vez los mensajes genéricos esperados y que los marcadores sensibles y errores completos no aparezcan en el log capturado. Sus fakes se definen en este archivo y no dependen de helpers de otros tests.

Es test-only de caracterización: pasó al primer run válido; no se modificó producción ni se afirma RED/GREEN. Solo prueba el helper de ciclo de retención; no ejecuta el startup del worker ni cubre todos los logs de la aplicación.

## Verificación local

- Focal relacionado: `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=20 ./cmd/privacy-cleanup-worker -run '^(TestRetentionCycleDoesNotLogSyntheticPersonalIdentifiers|TestRetentionCycleIndependentlyRunsBothAndLogsNoErrors|TestRetentionCycleNilPurgersLogGenericFailures|TestRetentionCycleFullBatchLogsBacklog)$'` — PASS.
- Paquete `cmd/privacy-cleanup-worker`: `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=1 ./cmd/privacy-cleanup-worker` — PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go vet ./cmd/privacy-cleanup-worker` — PASS.
- `gofmt -d` y `git diff --check` — limpios.

## Revisión independiente

`deleg_0c55ae4d` aprobó source-only, sin concerns ni errores lógicos. Confirmó que el test verifica ambos purgers, mensajes genéricos y ausencia de marcadores sintéticos; no ejecutó pruebas. La revisión reiteró que el alcance es solo `runIdentityRetentionCycle` con fakes y no cubre privacidad global o runtime.

## Límites

Los errores, IDs y purgers son sintéticos. Esto no demuestra PostgreSQL, borrado efectivo, almacenamiento, configuración/runtime, ausencia de PII en todos los logs, cumplimiento de retención ni operación en Cloud. Se seleccionaron tests unitarios locales; no se ejecutó SQL/DDL ni se accedió a DB persistente, Cloud o datos reales. Sin commit, push, deploy o integración a `main`.
