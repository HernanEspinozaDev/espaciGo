# Evidencia VI-03 — límite de vencimiento ante `verified_until` del cliente

- **Fecha:** 2026-11-12
- **Rama:** `codex/marketplace/fix/kyc-06-save-identity-first`
- **HEAD base:** `274f37e483a078890e92382a159fa49fd23c2fe8`
- **Alcance:** se añadió `internal/documentverification/transport/http/reviewer_approval_requested_expiry_test.go`; sin cambios productivos.

## Caracterización

`TestReviewerIdentityApprovalCapsCallerExpiryAtObservedDocumentDate` ejecuta el handler HTTP reviewer y el servicio de dominio reales sobre el repositorio fake local usado por las pruebas existentes. El POST sintético incluye `verified_until=2099-01-01T00:00:00Z` y vencimiento documental observado `2031-04-05`. Con reloj fijo `2030-03-01T12:00:00Z`, se comprueba que `VerifiedUntil` y `DocumentExpiresAt` quedan limitados a la frontera civil exclusiva calculada por el helper para la fecha observada; confirma también `VerifiedAt`, ETag/revisión y auditoría sintética.

El test pasó al primer intento. Fue una caracterización de un límite de seguridad descubierto mediante cobertura; no hubo cambio productivo y no se afirma RED/GREEN. El bloque del handler que deserializa el `verified_until` opcional quedó cubierto (reporte de cobertura del HTTP package).

## Verificación local

Ejecutado con `env -i`, sin configuración/credenciales DB/PG:

- Test focal `TestReviewerIdentityApprovalCapsCallerExpiryAtObservedDocumentDate`: PASS.
- `go test -race ./internal/documentverification/... -count=1`: PASS.
- `go vet ./internal/documentverification/...`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- HTTP package con cobertura: PASS, 85.3% de sentencias; el bloque de asignación del `verified_until` opcional muestra conteo 1.
- `gofmt -d` y `git diff --check`: PASS.

## Límites

El repositorio y auditoría son fakes en memoria: no se ejecutó SQL/DDL ni se probó persistencia, CAS, autorización PostgreSQL, auditoría durable o concurrencia. No se accedió a bases persistentes o Cloud, no se usaron datos reales y no hubo commit, push, integración ni deploy.
