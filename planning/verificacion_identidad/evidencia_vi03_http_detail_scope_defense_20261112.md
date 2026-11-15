# VI-03 — defensa HTTP de detalle reviewer ante respuesta fuera de scope

- **Fecha:** 2026-11-12.
- **Checkout/rama/HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- El worktree estaba ampliamente dirty; se preservaron los cambios ajenos. Sin commit.

## Cambio y cobertura

`ReviewerCasesHandler.loadDetail` ahora rechaza con 404 neutral una respuesta de `LoadDetail` cuyo ID no coincide con el case ID solicitado o cuyo propósito no es uno de los dos propósitos admitidos (`identity`, `space`). La comprobación ocurre antes de fijar ETag o serializar campos; es una defensa de la frontera HTTP y no sustituye las comprobaciones de asignación del servicio.

- **RED observado:** `TestReviewerDetailRejectsMismatchedCaseFromService` falló antes del cambio con 200 y devolvió el ID y la declaración sintéticos de otro caso.
- **GREEN:** el caso discrepante recibe 404, sin ETag ni valores de declaración; `TestReviewerDetailRejectsUnsupportedPurposeFromService` caracteriza 404 sin datos privados ni ETag para propósito no permitido.
- El positivo existente `TestAssignedIdentityDetailIncludesOnlyAllowlistedDeclaration` confirma los cuatro campos identity allowlisted y que el actor pasado al servicio corresponde al principal autenticado. `TestReviewerDetailDoesNotExposeIdentityDeclarationForSpaceCase` conserva el caso `space` válido y omite la declaración.

El handler solo puede pasar el principal del autenticador al servicio; la autorización/grant efectivo sigue siendo responsabilidad de `ReviewerAccessService`/repository y no queda demostrado por estos fakes.

## Verificación ejecutada

Con entorno aislado (`env -i`, solo `PATH`, `HOME`, `TMPDIR`; sin variables de base/opt-in):

- Test RED focal antes del cambio — FAIL esperado por discrepancia de ID, observó 200 y datos privados sintéticos.
- Test focal después del cambio — PASS.
- `go test -race ./internal/documentverification ./internal/documentverification/transport/http -count=1` — PASS.
- `go vet ./internal/documentverification ./internal/documentverification/transport/http` — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt` y `git diff --check` — PASS.

## Límites

Pruebas de handler/servicio con fakes e identificadores/datos sintéticos. No demuestran grants/autorización efectiva, aislamiento/locks, SQL, transacción, persistencia, BFF, PostgreSQL o Cloud. No se ejecutó SQL/DDL ni se accedió a bases persistentes; V95/V96 no se aplicaron. Sin datos reales, commit, push, integración a `main` o despliegue.
