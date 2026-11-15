# Evidencia VI-03 — proyección fail-closed de cola reviewer

- **Fecha:** 2026-11-12.
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- Worktree compartido y dirty; se conservaron los cambios previos. Sin commit, push o integración.

## Alcance del subcorte

`ReviewerAccessService.List` ahora proyecta únicamente snapshots bien formados y accionables antes de devolver la cola. El helper `reviewerQueueCaseEligible` requiere ID y sujeto con formato válido, sujeto distinto del revisor, propósito `identity` o `space`, `SpaceID` válido para `space`, ausencia de marcadores de revocación/supersession y estado `enviada`, `en_revision` o `requiere_antecedentes`.

El contrato `ReviewerCaseReader.ListReviewerCases(ctx, actor)` sigue siendo responsable de devolver únicamente asignaciones exactas activas. Esta capa no dispone de grants por fila y, por tanto, no acredita ni sustituye autorización efectiva o filtrado SQL; es una defensa local ante resultados malformados/no accionables.

## Prueba y gates

- Se añadió `TestReviewerAccessListFailsClosedForUnreviewableSnapshots` con casos sintéticos positivos de `identity` y `space`, y negativos de auto-revisión, propósito desconocido, supersession, revocación, estado terminal, IDs inválidos y `space` sin ID.
- **RED observado antes del cambio:** la cola incluyó cinco snapshots del fake; además de dos casos válidos, expuso auto-revisión, propósito desconocido y caso superseded. El test falló en la aserción de proyección esperada. La revocación, los IDs inválidos y `space` sin ID se añadieron después del cambio como negativos adicionales; no se afirma RED previo para esos escenarios.
- Se ajustó `TestReviewerQueueContainsOnlyReviewableAssignedCases` para que sus fixtures de estado tengan IDs, sujeto y propósito válidos conforme al contrato; el fixture anterior omitía esos campos y dejó de ser un caso positivo legal después de introducir las guardas.
- **GREEN y gates finales**, ejecutados con `env -i` y sin variables de conexión/opt-in:
  - `go test -race -count=1 ./internal/documentverification ./internal/documentverification/transport/http` — PASS.
  - `go vet ./internal/documentverification ./internal/documentverification/transport/http` — PASS.
  - `go build -o /dev/null ./cmd/api` — PASS.
  - `gofmt -d` sobre los tres archivos — sin diferencias.
  - Revisión de whitespace final — 0 líneas; `git diff --check` para archivos versionados — PASS.

## Revisión independiente

- La primera revisión read-only `deleg_7466be82` no aprobó la versión inicial del test: señaló que no cubría revocación, IDs inválidos ni `SpaceID` ausente y que no demostraba todos los estados para space. Se añadieron esos negativos y positivos space en los tres estados elegibles; se conservó la limitación sobre grants exactos, que es responsabilidad del reader.
- La re-revisión read-only `deleg_69395ad9` del estado ampliado devolvió `passed: true`, sin hallazgos de seguridad ni lógica. Confirmó los casos positivos/negativos del helper; reiteró que el fake caracteriza proyección en memoria, no grants/autorización real ni SQL/persistencia. No ejecutó pruebas ni modificó archivos; los gates listados arriba son del parent sobre este estado final.

## Límites

El repositorio de la prueba es un fake que entrega snapshots; no prueba grants reales, lectura de asignaciones, expiración/revocación bajo bloqueo, SQL/DDL, índices, PostgreSQL, transacciones, persistencia, BFF, almacenamiento, Cloud ni datos reales. La regresión acredita solamente el filtrado defensivo del servicio en memoria. No se ejecutó SQL/DDL ni se conectó a bases persistentes o Cloud; V95/V96 permanecen sin aplicar. VI-03 y el MVP continúan parciales.
