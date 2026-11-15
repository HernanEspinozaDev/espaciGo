# Evidencia VI-03 — checklist obligatoria para solicitar antecedentes

Fecha: 2026-11-13 (hora local `-03:00`)

## Alcance

Se añadió `TestReviewerIdentityNeedsEvidenceRejectsMissingOrIncompleteChecklist` en `internal/documentverification/transport/http/reviewer_needs_evidence_checklist_test.go`. Con `decision=needs_evidence`, motivo sintético válido y formato observado presente, cubre tres cuerpos: checklist omitida, checklist `null` y checklist vacía. Cada caso espera `400 invalid request` y que el servicio fake solo haya recibido la carga inicial del expediente; no se invoca la transición ni el readback posterior.

La prueba llama al `ReviewerCasesHandler` real con `reviewerFake` y `reviewerAuth` sintéticos. El primer run pasó; es caracterización del guard existente, no se observó RED ni se cambió producción. El método de checklist trata nil como incompleta.

## Verificación ejecutada

- Preflight del paquete HTTP: sin `TestMain` ni `init`; el test elegido usa únicamente handler, fake de servicio y datos sintéticos.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test ./internal/documentverification/transport/http -run '^TestReviewerIdentityNeedsEvidenceRejectsMissingOrIncompleteChecklist$' -count=1 -v` — PASS (3 subcasos).
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=20 ./internal/documentverification/transport/http -run '^TestReviewerIdentityNeedsEvidenceRejectsMissingOrIncompleteChecklist$'` — PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go vet ./internal/documentverification/transport/http` — PASS.
- `gofmt -d internal/documentverification/transport/http/reviewer_needs_evidence_checklist_test.go` — sin diferencias.
- `git diff --check -- internal/documentverification/transport/http/reviewer_needs_evidence_checklist_test.go` — PASS.

## Límites

No se ejecutó la suite completa del paquete, sus harness BFF, ni pruebas de integración. El fake no prueba el servicio de dominio, grants efectivos, persistencia, SQL/DDL, PostgreSQL, reinicio, storage, GCS, Cloud ni proveedores externos.

- Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`
- Rama: `codex/marketplace/fix/kyc-06-save-identity-first`
- HEAD observado: `274f37e483a078890e92382a159fa49fd23c2fe8`

Sin SQL/DDL, acceso a DB/Cloud, datos reales, commit, push, deploy ni integración a `main`; se preservó el estado previo del worktree.