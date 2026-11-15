# VI-04 — contrato local de vencimiento para reservas

**Fecha:** 2026-11-12
**Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`
**Rama/HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`
**Alcance:** unidad local con servicios reales y fixtures/repositorio en memoria.

## Comportamiento caracterizado

`internal/documentverification/booking_identity_contract_test.go` añade `TestTypedIdentityApprovalSuppliesExclusiveBookingExpiry`. El test usa `documentverification.Service.ReviewIdentityWithReason` con reloj, autorización, archivos y checklist sintéticos; verifica que la aprobación guarda el vencimiento documental observado como `Snapshot.VerifiedUntil` cuando este antecede al límite de renovación de 24 meses. Luego construye una aserción mínima desde ese snapshot mediante un repositorio en memoria y ejecuta `identityverification.Service.RequireForBooking`: ambas partes satisfacen el gate un nanosegundo antes del límite y la identidad documental deja de satisfacerlo en la igualdad exacta.

La primera ejecución fue PASS: es una caracterización del contrato entre ambos servicios, no se observó RED/GREEN ni se modificó código productivo. El repositorio en memoria reproduce el mapeo `VerifiedUntil` → `Verification.ExpiresAt`; el adapter PostgreSQL y su consulta no forman parte de esta prueba.

## Verificación ejecutada

Con entorno de proceso limpio (`env -i`; sin configuración DB/PG/opt-in):

- Test focal `TestTypedIdentityApprovalSuppliesExclusiveBookingExpiry` — PASS.
- `go test -race ./internal/documentverification ./internal/identityverification ./internal/booking -count=1` — PASS.
- `go vet ./internal/documentverification ./internal/identityverification ./internal/booking` — exit 0.
- `go build -o /dev/null ./cmd/api` — exit 0.
- `gofmt -d` del archivo de test — sin salida; revisión explícita de newline final y trailing whitespace en test, evidencia y plan — PASS.

## Límites

No se ejecutó ni conectó SQL/DDL, base persistente, migración o Cloud. No se valida el adapter PostgreSQL, lectura durable, transacción/locks, handler HTTP de reservas, wiring runtime ni concurrencia. La evidencia confirma únicamente el cálculo de expiración del servicio de casos y la decisión del servicio de identidad/reservas con datos sintéticos en memoria; no cierra VI-04 ni habilita reservas reales.
