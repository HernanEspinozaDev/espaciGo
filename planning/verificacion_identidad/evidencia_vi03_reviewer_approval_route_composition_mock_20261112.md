# Evidencia VI-03 — aprobación identity por ruta registrada

- **Fecha:** 2026-11-12.
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Cambio:** test-only; `TestReviewerIdentityApprovalCapsCallerExpiryAtObservedDocumentDate` ahora atraviesa `http.ServeMux` y `RegisterReviewerCasesRoutes`.

## Comportamiento caracterizado

El POST `/review` de aprobación pasa por la ruta registrada, `ReviewerCasesHandler` real y servicios reviewer/domain reales. Auth y repository son fakes sintéticos. El fixture tiene tres roles limpios y checklist completo. La prueba comprueba status/ETag revision 8, límite exclusivo convertido desde la fecha observada `2031-04-05` pese a una expiración solicitada mucho mayor, snapshot fake con la fecha/`VerifiedAt` esperados y auditoría fake con el vencimiento observado.

La caracterización pasó al primer intento; no se afirma RED/GREEN ni cambio productivo.

## Verificación ejecutada

Con `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR"`:

- Test focal con `go test -race -count=1` — PASS.
- Suites completas `-race` de `internal/documentverification` y `internal/documentverification/transport/http` — PASS.
- `go vet` de ambos paquetes — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt -d` y whitespace del archivo candidato — limpios.

## Revisión

Revisión independiente read-only `deleg_84af168c`: aprobada (`passed: true`), sin errores lógicos. Confirmó el dispatch por la ruta registrada y las aserciones de status/ETag, límite civil exclusivo, snapshot/auditoría y una transición. El revisor resaltó que autenticación y grant son sintéticos; se conserva esa limitación explícita y el test no se presenta como prueba de autorización real.

## Límites

Auth, grant, estado y auditoría son sintéticos/en memoria. Se caracteriza el límite civil y el wiring mux/handler en Go; no prueba grants efectivos, atomicidad PostgreSQL, SQL, persistencia, BFF→Go, Cloud ni decisión humana. Sin SQL/DDL, acceso a persistencia/Cloud, datos reales, commit, push, deploy o integración a `main`.
