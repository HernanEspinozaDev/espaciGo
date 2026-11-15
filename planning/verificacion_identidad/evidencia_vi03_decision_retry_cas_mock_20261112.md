# Evidencia VI-03 — reintento de decisión tras readback incierto

- **Fecha:** 2026-11-12
- **Rama:** `codex/marketplace/fix/kyc-06-save-identity-first`
- **HEAD:** `274f37e483a078890e92382a159fa49fd23c2fe8`
- **Alcance:** caracterización local con handler, servicio de acceso de revisores, servicio de dominio y repositorio sintético en memoria. Sin PostgreSQL ni proveedor externo.

## Comportamiento cubierto

Se amplió `TestReviewerIdentityApprovalReadbackFailureCanBeResolvedWithoutRepeatingDecision` en `internal/documentverification/transport/http/reviewer_review_uncertain_test.go`:

1. Un revisor autorizado aprueba el caso en revisión 7. El fake de repositorio conserva la aprobación, revisión 8 y evento/auditoría.
2. El readback HTTP posterior falla y la primera respuesta es incierta (503), aunque la decisión ya quedó en el estado sintético.
3. El mismo POST con la revisión 7 se reintenta. La transición encuentra la revisión obsoleta y devuelve conflicto neutral 409, sin ETag ni lectura posterior de respuesta. El estado aprobado, revisión 8, fecha civil observada y auditoría permanecen sin cambios.
4. Un GET explícito reconcilia el resultado: 200, ETag `"8"`, aprobado y la fecha civil esperada.

La prueba se agregó como caracterización de un comportamiento existente; no se observó una fase RED para esta extensión y no se afirma RED/GREEN. No requirió cambio de producción.

## Ejecución local

- Test focal `go test ./internal/documentverification/transport/http -run '^TestReviewerIdentityApprovalReadbackFailureCanBeResolvedWithoutRepeatingDecision$' -count=1`: PASS.
- `go test -race ./internal/documentverification/... -count=1`: PASS en dominio, inspección y transporte HTTP.
- `go vet ./internal/documentverification/...`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- `gofmt -d` del test y `git diff --check`: PASS.

## Límites

El fake en memoria simula la comprobación de revisión y rechaza la segunda transición; esto no prueba que el adaptador PostgreSQL real haga CAS, que evento y estado sean atómicos, ni que la semántica sobreviva a concurrencia o fallos de commit. No acredita idempotencia persistida. VI-03/VI-04 y los gates de aceptación del MVP permanecen abiertos.

No se ejecutó SQL/DDL, migración, consulta ni prueba conectada a base persistente o Cloud. No se usaron datos reales. Sin commit, push ni deploy.
