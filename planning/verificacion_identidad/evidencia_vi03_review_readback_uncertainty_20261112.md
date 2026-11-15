# VI-03 — incertidumbre al fallar el readback de una decisión

- Fecha: 2026-11-12.
- Checkout/rama/HEAD: `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- Worktree compartido y ampliamente dirty; se preservaron sus cambios preexistentes.

## Cobertura

`TestReviewerIdentityApprovalReadbackFailureCanBeResolvedWithoutRepeatingDecision` recorre el handler HTTP y el servicio real de decisión con un repositorio en memoria. El POST aprueba un caso identity sintético, pero el fake falla únicamente en el readback posterior a la transición. Se comprueba el 503 neutral, estado/auditoría aprobados en el fake y exactamente una transición. Un GET explícito posterior recupera la revisión 8 y la fecha civil original sin repetir el POST/transición.

Una primera ejecución terminó fallando en el contador de lecturas: el fake heredaba `WithReviewerCaseForRead` y, por tanto, no aplicaba el fallo diferido en ese método. Se corrigió el fake para interponer ese puerto de lectura real; el test focal luego pasó. Fue una corrección del harness, no evidencia de defecto productivo ni RED/GREEN del producto. No se modificó código de producción.

## Verificación

Con variables de conexión PostgreSQL retiradas:

- Test focal — PASS.
- `go test -race ./internal/documentverification ./internal/documentverification/transport/http -count=1` — PASS.
- `go vet` de ambos paquetes — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt` y `git diff --check` — PASS.

## Límites

El fake no demuestra transacción/auditoría durable, V96, PostgreSQL, BFF/Next o comportamiento de navegador; tampoco acredita que un GET real recupere el mismo registro tras reinicio. No se ejecutó SQL/DDL ni se accedió a bases persistentes o Cloud; no hubo migración, datos reales o publicación.
