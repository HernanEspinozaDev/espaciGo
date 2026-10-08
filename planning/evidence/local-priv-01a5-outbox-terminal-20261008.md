# Evidencia LOCAL-PRIV-01A5 — 2026-10-08

## Resultado

- La migración V27 se aplicó dentro del entorno PostgreSQL desechable del script de integración. No se aplicó a `espacigo_pgdata`; no se alteraron los secretos ni los datos de desarrollo persistentes.
- Pasaron `TestCredentialNoticeTerminalCycleAdminRecoveryAndRetention` y `TestCredentialNoticeInactiveRecipientCannotBeRecoveredOrSent`: octavo fallo terminal, no selección de un noveno intento, reinicio del servicio conservando el estado, no administrador rechazado, reapertura concurrente idempotente, total acumulado, entrega posterior, imposibilidad de reabrir evento ya entregado/cuenta inactiva, cancelación antes del despacho y purga al vencer 30 días mientras se preservan pendientes.
- Pasó `TestCredentialCoreRuntimeLeastPrivilege`: el rol runtime no borra directamente eventos del outbox; la purga acotada se ejecuta mediante función SQL protegida.
- Pasó `TestEligibleSuppressionPreservesTerminalAndCancelsPendingCredentialNoticeCycles` en sus dos subcasos PostgreSQL. La baja terminal conserva sin cambios fecha terminal, código, ciclo y vencimiento de 30 días; la baja pendiente actualiza evento y ciclo a `cancelada`, con fecha y motivo `baja_local_sin_finalidad`, en la misma transacción. En ambos casos la cuenta queda retirada, el worker no envía el aviso y la reapertura se rechaza por destinatario inactivo. También pasó la regresión amplia `TestM02LocalSuppressionMinimizesSyntheticAccountAndRecoversFileCleanup`.
- Pasó `TestCredentialNoticeMailpitDeliverySmoke`: el servicio de autenticación de prueba, conectado al PostgreSQL desechable, entregó el aviso genérico por SMTP a Mailpit. El test buscó el destinatario y asunto esperados y borró únicamente el ID del mensaje que creó. El buzón de Mailpit existente no se limpió globalmente.
- `npm --prefix mock run build` compiló el mock desde TypeScript.
- `go vet ./internal/identity/... ./internal/adapters/postgres/identity ./internal/dbbootstrap` pasó.
- El parser YAML de Python cargó `planning/openapi.yaml` correctamente. `@redocly/cli lint planning/openapi.yaml --skip-rule operation-summary` validó el contrato sin errores y dejó siete advertencias preexistentes. El lint recomendado sin esa excepción falla por falta de `summary` en varias operaciones existentes; los endpoints nuevos sí incluyen resumen. No se amplió el arreglo de esa regla histórica a todas las rutas.
- `git diff --check` se ejecutó tras la compilación como comprobación final.

## Comandos

```sh
GO_TEST_RUN='TestCredentialNotice|TestCredentialCoreRuntimeLeastPrivilege|TestEligibleSuppressionPreservesTerminalAndCancelsPendingCredentialNoticeCycles|TestM02LocalSuppressionMinimizesSyntheticAccountAndRecoversFileCleanup' \
  LOCAL_SMTP_ADDR=172.21.0.2:1025 \
  LOCAL_MAILPIT_API=http://127.0.0.1:8025 \
  bash scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/identity
npm --prefix mock run build
go vet ./internal/identity/... ./internal/adapters/postgres/identity ./internal/dbbootstrap
npx --yes @redocly/cli lint planning/openapi.yaml --skip-rule operation-summary
```

Se utilizó la instancia local existente de Mailpit solo como receptor de ensayo. El smoke test eliminó el único mensaje sintético que generó. No reinició los servicios ni modificó `espacigo_pgdata`.

## Límites

La prueba no demuestra entrega SMTP exactly-once ni envío a un proveedor productivo. El outbox durable y la operación de recuperación sobreviven reinicios; SMTP puede duplicar si acepta antes de una caída y el Backend no persiste la confirmación. El purgador elimina únicamente eventos entregados/cancelados/terminales vencidos sin lease; los pendientes no se eliminan.
