# Evidencia VI-03 — solicitud de antecedentes con formato observado indeterminado

Fecha: 2026-11-13 (hora local `-03:00`)

## Alcance

Se añadió `TestIdentityNeedsEvidenceCanRecordUndeterminedObservedFormat` en `internal/documentverification/identity_correction_cycle_test.go`. Invoca `Service.ReviewIdentityWithReason` con fixture sintético en memoria: declaración `unsure`, observación del revisor `undetermined`, decisión `DecisionNeedsEvidence`, motivo `ReasonEvidenceIncomplete`, checklist completa pero con legibilidad y formato/fecha no verificados, y vencimiento observado cero.

La prueba verifica que el caso pasa a `requiere_antecedentes` (estado `CaseNeedsEvidence`), conserva revisor/revisión, declaración y observación; no establece vencimiento del documento ni `VerifiedAt`/`VerifiedUntil`; y registra decisión, motivo, observación, vencimiento cero y checklist en el evento fake. Esto complementa las pruebas que permiten aprobar `unsure` solo con observación concreta y que bloquean aprobación con `undetermined`.

Caracterización test-only: pasó en la primera ejecución; no se observó RED ni se modificó producción.

## Verificación ejecutada

- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test ./internal/documentverification -run '^TestIdentityNeedsEvidenceCanRecordUndeterminedObservedFormat$' -count=1 -v` — PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=20 ./internal/documentverification -run '^TestIdentityNeedsEvidenceCanRecordUndeterminedObservedFormat$' -v` — PASS, 20/20.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go vet ./internal/documentverification` — PASS.
- `gofmt -d internal/documentverification/identity_correction_cycle_test.go` — sin diferencias.
- `git diff --check -- internal/documentverification/identity_correction_cycle_test.go` — PASS.

## Límites

El servicio de dominio es real; el repository/auth y sus transiciones son sintéticos en memoria. No prueba HTTP, permisos/grants efectivos, persistencia/reinicio, atomicidad durable, SQL/DDL, PostgreSQL, BFF, almacenamiento, GCS, Cloud ni proveedores. No ejecuté suites de integración.

- Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`
- Rama: `codex/marketplace/fix/kyc-06-save-identity-first`
- HEAD observado: `274f37e483a078890e92382a159fa49fd23c2fe8`

Sin SQL/DDL, acceso a DB/Cloud, datos reales, commit, push, deploy ni integración a `main`; se preservó el worktree previo.