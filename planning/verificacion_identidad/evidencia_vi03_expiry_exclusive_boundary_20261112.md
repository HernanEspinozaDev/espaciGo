# VI-03 — expiración en el límite civil exclusivo

- Fecha: 2026-11-12.
- Checkout/rama/HEAD: `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- Worktree compartido, ampliamente dirty; se preservaron todos los cambios ajenos.

## Alcance

Se añadió `TestTypedIdentityApprovalHonorsExclusiveObservedExpiryBoundary` en `internal/documentverification/service_test.go`, con un límite de expiración fijo y reloj controlado. El control positivo aprueba un nanosegundo antes del límite y deja `VerifiedUntil` exactamente en ese límite. El caso de igualdad con el límite se rechaza con `ErrInvalid` y verifica que snapshot y auditoría permanezcan sin cambios.

Es una prueba unitaria de caracterización del servicio: la conversión HTTP de fecha civil a comienzo del día siguiente se prueba por separado en `reviewer_cases_test.go`, incluido el comienzo válido tras una brecha DST y la serialización de ese límite de vuelta a la misma fecha civil. Juntas, las pruebas distinguen fecha civil y límite interno exclusivo. No hubo cambio productivo nuevo en este subcorte.

## Verificación

Con variables de conexión PostgreSQL retiradas:

- Test focal `TestTypedIdentityApprovalHonorsExclusiveObservedExpiryBoundary` — PASS.
- `go test -race ./internal/documentverification ./internal/documentverification/transport/http ./internal/documentverification/inspection -count=1` — PASS (tres paquetes).
- `go vet` en esos tres paquetes — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `git diff --check` en los archivos del corte — PASS.

## Límites

El repositorio es un fake en memoria. Esto no prueba commit/auditoría durable, PostgreSQL/V96, BFF/UI, almacenamiento de evidencias ni autorización operativa real. No se ejecutó SQL/DDL, se conectó a bases persistentes o Cloud, se usaron datos reales, ni se hizo push, deploy o integración a `main`. El cliente OpenAPI generado y formulario siguen pendientes de alineación con `format: date`; `api-check` permanece bloqueado por drift en el checkout backend y su corrección pertenece al worktree frontend autorizado.
