# VI-04 — cancelación del gate de identidad en aprobación de reserva

- **Fecha:** 2026-11-12.
- **Checkout/rama/HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- Worktree compartido con cambios preexistentes; no se alteraron los demás archivos ni artefactos.

## Cambio y cobertura

`identityverification.Service.RequireForBooking` ahora comprueba el contexto después de cada lectura de verificación y propaga `context.Canceled`/`context.DeadlineExceeded` antes de evaluar la aprobación o consultar a la segunda parte.

`TestRequireForBookingStopsWhenContextCancelsDuringFirstPartyLookup` usa verificaciones manuales vigentes para ambas partes y un repositorio fake que cancela tras devolver correctamente el registro del anfitrión. **RED observado:** la función consultaba también al arrendatario y devolvía `nil`, permitiendo que el gate pareciera satisfecho con la solicitud ya cancelada. **GREEN:** propaga `context.Canceled` y el fake registra solo la consulta del anfitrión. El control positivo existente `TestRequireForBookingAcceptsCurrentManualDocumentReviewForBothParties` permanece dentro de la suite ejecutada.

## Verificación

Con entorno limpio (`env -i`, sin variables DB/PG/opt-in):

- Test focal — **PASS** tras RED esperado.
- `go test -race ./internal/identityverification ./internal/booking -count=1` — **PASS**.
- `go vet ./internal/identityverification ./internal/booking` — **PASS**.
- `go build -o /dev/null ./cmd/api` — **PASS**.
- `gofmt` y `git diff --check` — **PASS**.

## Límites

Cobertura unitaria en memoria; no prueba lectura PostgreSQL, aprobación persistida de booking, transacción, endpoint HTTP, BFF ni navegación. No se ejecutó SQL/DDL, se accedió a bases persistentes o Cloud, ni se usaron datos reales. Sin migraciones, commit, push, despliegue o integración a `main`.
