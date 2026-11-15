# VI-03 — scope de preflight y readback HTTP de decisión reviewer

- **Fecha:** 2026-11-12.
- **Checkout/rama/HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- El worktree está ampliamente dirty; se preservaron los cambios ajenos. Sin commit.

## Cambio y cobertura

Se centralizó la validación de snapshots reviewer en `reviewerSnapshotMatchesRequest`: el ID debe coincidir exactamente con el solicitado y el propósito debe ser `identity` o `space`. El handler la aplica al preflight de `POST /review`, al readback posterior a review/revoke mediante `load`, y al detalle HTTP. Rechaza con 404 neutral antes de mutar (en preflight) o antes de fijar ETag/serializar (en los readbacks).

- `TestReviewerDecisionRejectsMismatchedCaseOrUnsupportedPurposeBeforeWrite`: usa secuencia de snapshots sintéticos y cuerpos válidos para cada rama. Comprueba 404 sin ETag y que no haya llamada a `ReviewIdentity`/`Review` ni readback cuando el preflight devuelve ID distinto o propósito no admitido.
- `TestReviewerDecisionReadbackRejectsMismatchedCaseOrPurpose`: simula una decisión aceptada seguida de un readback con ID distinto o propósito no admitido; comprueba 404, sin ETag y sin serializar el ID de otro caso. El fake confirma la secuencia load→review identity→load.
- La cobertura previa de `TestReviewerDetailRejectsMismatchedCaseFromService` y `TestReviewerDetailRejectsUnsupportedPurposeFromService` permanece; los positivos existentes conservan el principal autenticado pasado al servicio, los campos identity allowlisted y el detalle `space` sin declaración identity.

Una primera ejecución del nuevo test de preflight se descartó como evidencia RED porque faltaba `DocumentFormatDeclared` en el fixture identity. Se corrigió con un caso completo y un body legacy específico para el propósito desconocido. La sensibilidad de la regresión corregida se verificó con Go overlay que quitó solo la guarda de preflight: ambos casos fallaron en la aserción exacta de no mutación (`calls=[load review_identity load]` y `calls=[load review load]`); la fuente del repo quedó byte a byte intacta. El test normal pasa con la guarda.

## Verificación ejecutada

Con entorno aislado `env -i` (solo `PATH`, `HOME`, `TMPDIR`, sin variables de DB/opt-in):

- Regresión de readback antes de su guarda — FAIL esperado: 200 con ETag `"8"` para ID/purpose discrepante; tras el cambio, PASS.
- Sensibilidad por overlay de preflight — FAIL esperado en la aserción de llamadas/side effects; fuente original sin cambios.
- `go test -race ./internal/documentverification ./internal/documentverification/transport/http -count=1` — PASS.
- `go vet ./internal/documentverification ./internal/documentverification/transport/http` — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt`, `gofmt -d` y `git diff --check` de los archivos tocados — PASS.

## Límites

Los tests usan un service fake con secuencia de resultados y una transición simulada. No demuestran asignación/grant efectiva, auditoría ni resultado durable, SQL, concurrencia, transacción, BFF, PostgreSQL o Cloud. La autenticación/grant real continúa en `ReviewerAccessService`/repository y no queda validada por esta defensa HTTP. No se ejecutó SQL/DDL ni se accedió a bases persistentes; V95/V96 no se aplicaron. Sin datos reales, commit, push, integración a `main` o despliegue.
