# Evidencia VI-03 — dominio rechaza checklist incompleta al pedir antecedentes

Fecha: 2026-11-13 (hora local `-03:00`)

## Alcance

Se añadió `TestIdentityNeedsEvidenceRejectsIncompleteChecklistWithoutMutation` en `internal/documentverification/identity_correction_cycle_test.go`. Con un caso sintético válido (`submitted`, declaración `unsure`, evidencias limpias), llama a `Service.ReviewIdentityWithReason` real para `needs_evidence`, motivo `antecedentes_incompletos` y observación `undetermined`. Cubre checklist nil y checklist con `IssueDateAndFormatVerified` ausente.

Cada subcaso exige `ErrInvalid`, snapshot y audit sin cambios, y contabiliza el intento de transición por el reviewer sintético. El primer run pasó: es caracterización de la validación de dominio existente, no se observó RED ni se modificó producción. El repository es `memoryRepository`; no implica rollback durable.

Este test complementa `TestReviewerIdentityNeedsEvidenceRejectsMissingOrIncompleteChecklist` del handler HTTP: ese comprueba el rechazo 400 antes de invocar la decisión; este comprueba la validación del servicio de dominio para llamadores que no pasan por ese handler.

## Verificación ejecutada

- Preflight `internal/documentverification`: sin `TestMain` ni `init`.
- Focal `go test ./internal/documentverification -run '^TestIdentityNeedsEvidenceRejectsIncompleteChecklistWithoutMutation$' -count=1 -v` — PASS (2 subcasos).
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=20 ./internal/documentverification -run '^(TestIdentityNeedsEvidenceCanRecordUndeterminedObservedFormat|TestIdentityNeedsEvidenceRejectsIncompleteChecklistWithoutMutation)$'` — PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go vet ./internal/documentverification` — PASS.
- `gofmt -d internal/documentverification/identity_correction_cycle_test.go` — sin diferencias.
- `git diff --check -- internal/documentverification/identity_correction_cycle_test.go` — PASS.

## Límites

No se ejecutó la suite completa. El fake no prueba autorización/grants efectivos, atomicidad durable, SQL/DDL, PostgreSQL, persistencia/reinicio, storage, GCS, Cloud ni proveedores externos.

- Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`
- Rama: `codex/marketplace/fix/kyc-06-save-identity-first`
- HEAD observado: `274f37e483a078890e92382a159fa49fd23c2fe8`

Sin SQL/DDL, acceso a DB/Cloud, datos reales, commit, push, deploy ni integración a `main`; se preservó el estado previo del worktree.