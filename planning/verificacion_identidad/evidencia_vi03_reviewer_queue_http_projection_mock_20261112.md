# Evidencia VI-03 — cola reviewer HTTP con servicio real y fakes

- **Fecha:** 2026-11-12.
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Cambio:** test-only en `internal/documentverification/transport/http/reviewer_queue_service_projection_test.go`; no se modificó código productivo.

## Comportamiento caracterizado

`TestReviewerQueueHandlerSerializesOnlyReviewableAssignedSnapshots` llama al handler HTTP real `ReviewerCasesHandler` y al `ReviewerAccessService` real. Authenticator, `ReviewerCaseReader`, servicio base requerido por el constructor y storage son fakes locales. La lista fake entrega dos snapshots sintéticos revisables (identity/space) y negativos de auto-revisión, revocación, supersession, propósito desconocido, `space` sin ID, IDs inválidos y estado terminal. La respuesta HTTP queda limitada a los dos IDs esperados; se verifica que el actor enviado al reader proviene del principal autenticado, las cabeceras privadas y que el DTO no serializa el ID de revisión legal.

Es una caracterización de primera ejecución del recorrido en memoria; no se afirma RED/GREEN ni hubo cambio productivo.

## Verificación ejecutada

Con `env -i` y sin variables de conexión/opt-in:

- Test focal `go test -race -count=1 ./internal/documentverification/transport/http -run '^TestReviewerQueueHandlerSerializesOnlyReviewableAssignedSnapshots$' -v` — PASS.
- Suites `go test -race -count=1 ./internal/adapters/postgres/documentverification ./internal/documentverification ./internal/documentverification/transport/http` — PASS.
- `go vet` en los tres paquetes — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt -d`, revisión de whitespace en los dos tests y `git diff --check` de cambios versionados — limpios.

## Revisión independiente

La primera revisión read-only `deleg_c47b5248` devolvió `passed: false` solo porque el archivo nuevo estaba untracked y no formaba parte del HEAD; no reportó concerns de seguridad ni lógica. En la aclaración acotada, `deleg_cd443c45` releyó el candidato del worktree y devolvió `passed: true`, sin hallazgos; su única sugerencia reiteró que el fake no prueba grants/SQL/DB/Cloud, límite ya registrado. El test permanece intencionalmente sin commit y eso no bloquea la verificación local. El usuario, como revisor autorizado, confirmó el 2026-11-12 que los cambios y evidencia fueron revisados conforme al plan y autorizó continuar; esto no afirma que el archivo esté en HEAD ni autoriza commit/push.

## Límites

El fake reader entrega resultados que conceptualmente ya son asignados; esta prueba no valida grants, su implementación, SQL ni persistencia. El test sí ejercita la proyección de `ReviewerAccessService.List`, el handler y la serialización DTO en memoria; no prueba autenticación real, asignación efectiva, autorización PostgreSQL, BFF, transacción, storage o Cloud. No ejecuté SQL/DDL ni me conecté a bases persistentes o Cloud; V95/V96 siguen sin aplicar. Sin datos reales, commit, push, deploy o integración a `main`.
