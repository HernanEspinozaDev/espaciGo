# VI-03 — aprobación HTTP sintética con fecha civil

- Fecha: 2026-11-12.
- Checkout/rama/HEAD: `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- El worktree compartido estaba ampliamente dirty; se preservaron cambios preexistentes.

## Cobertura

`TestReviewerIdentityApprovalHTTPPreservesCivilExpiryThroughService` recorre el POST del handler HTTP, `ReviewerAccessService` y `Service.ReviewIdentity` reales con repositorio en memoria y grant identity sintético exacto. Envía `document_expires_at_observed: "2031-04-05"`; comprueba aprobación, ETag/revisión, que snapshot y audit reciban el límite exclusivo calculado en `America/Santiago`, y que la respuesta vuelva a exponer exactamente la fecha civil original. También verifica checklist completa y timestamps de aprobación controlados.

Es una caracterización de primera ejecución; no se observó RED ni se cambió código productivo en este subcorte. Complementa las pruebas unitarias del parser/DTO, frontera del servicio y caso DST.

## Verificación

Con variables de conexión PostgreSQL retiradas:

- Test focal — PASS.
- `go test -race ./internal/documentverification ./internal/documentverification/transport/http -count=1` — PASS.
- `go vet` de ambos paquetes — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `git diff --check` — PASS.

## Límites

El repositorio es fake en memoria: no demuestra transacción/auditoría durable, V96, persistencia ni reinicio; tampoco cliente TypeScript, formulario, BFF o integración PostgreSQL. No se ejecutó SQL/DDL ni se accedió a bases persistentes o Cloud; no se usaron datos reales, migraciones o publicación.
