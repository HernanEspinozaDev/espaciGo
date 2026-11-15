# Evidencia — VI-03, constructores y delegación en ReviewerAccessService y mapeo de copias en OwnerDecisionCopy

- **Fecha:** 2026-11-14.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / commit base `7d6d7c67425d0c72c918efeb10344b584d436df7`.
- **Entorno:** Local aislado Linux x86_64, Go 1.24 toolchain, PostgreSQL persistente ausente y no invocado.
- **Gate local:** Unitario/caracterización test-only con fakes en memoria (`reviewerAccessRepo`, `reviewerIdentityDecisionRepo`, `reviewerAccessStore`, `testRepository`). Sin SQL/DDL, sin acceso DB persistente ni Cloud Console / Cloud Storage.

## Verificaciones cubiertas

1. **Fronteras y sanitización de copia en `OwnerDecisionCopy` (`TestOwnerDecisionCopy`):**
   - Propósitos no identitarios (`PurposeSpace`, propósitos desconocidos o vacíos) devuelven cadena vacía sin importar el estado o el motivo.
   - Estados distintos de `CaseNeedsEvidence` (`CaseDraft`, `CaseSubmitted`, `CaseInReview`, `CaseApproved`, `CaseRejected`, `CaseRevoked`, `CaseExpired`) devuelven cadena vacía.
   - Estado `CaseNeedsEvidence` con código de motivo vacío devuelve cadena vacía.
   - Estado `CaseNeedsEvidence` con `ReasonEvidenceIncomplete` devuelve el texto aprobado específico (`ownerNeedsEvidenceCopy`).
   - Estado `CaseNeedsEvidence` con otros códigos (`ReasonRetentionDue`, `ReasonDocumentInvalid`, códigos personalizados) devuelve el texto aprobado de fallback (`ownerNeedsEvidenceFallback`).
   - Se comprueba explícitamente que los códigos de motivo internos evaluados (`ReasonEvidenceIncomplete`, `ReasonRetentionDue`, `ReasonDocumentInvalid`, etc.) no aparezcan contenidos como subcadenas en los textos expuestos.
   - Cobertura de `owner_decision_copy.go`: **100,0%**.

2. **Constructores y reloj en `ReviewerAccessService` (`TestReviewerAccessServiceConstructorAndClockBoundaries`):
   - `NewReviewerAccessService` valida dependencias no nulas (`repository`, `cases`, `store`), retornando `ErrInvalid` si cualquiera es nula.
   - Se comprueba explícitamente que la inicialización exitosa asigne un reloj no nulo en zona horaria `time.UTC`.
   - `NewReviewerAccessServiceWithIdentityDeclarationReader` rechaza reader nulo con `ErrInvalid` y propaga errores si el constructor base falla.
   - Asignación de reader exitosa preserva la interfaz adjunta.
   - `SetClock`: no-op seguro frente a receptor nulo o función de reloj nula; actualización precisa cuando se suministra reloj válido.

3. **Fronteras y delegación en `Review`, `ReviewIdentity` y `Revoke` (`TestReviewerAccessServiceReviewAndRevokeBoundaries`):
   - Receptor nulo en `Review`, `ReviewIdentity` y `Revoke` retorna fail-closed `ErrInvalid`.
   - Identificador inválido (no UUID) en `Review`, `ReviewIdentity` y `Revoke` retorna fail-closed `ErrInvalid`.
   - Ejecución válida de `Review` delega a `cases.ReviewWithReason`, transitando el caso y registrando la auditoría `review_decision`.
   - Ejecución válida de `ReviewIdentity` delega a `cases.ReviewIdentityWithReason`, transitando el caso y registrando la auditoría `review_decision`.
   - Ejecución válida de `Revoke` delega a `cases.Revoke`, transitando el caso a `CaseRevoked` y registrando la auditoría `revoked`.
   - Cobertura de todas las funciones de `reviewer_access.go`: **100,0%**.

## Comprobación local ejecutada

```bash
$ go test -race -count=20 -run '^(TestOwnerDecisionCopy|TestReviewerAccessService)' ./internal/documentverification
ok  	github.com/HernanEspinozaDev/espaciGo/internal/documentverification	1.091s

$ go test -race -count=1 ./internal/documentverification/...
ok  	github.com/HernanEspinozaDev/espaciGo/internal/documentverification	1.039s
ok  	github.com/HernanEspinozaDev/espaciGo/internal/documentverification/inspection	15.235s
ok  	github.com/HernanEspinozaDev/espaciGo/internal/documentverification/transport/http	1.454s

$ git diff --check
# (salida limpia, 0 errores)
```

## Cobertura final obtenida

- `internal/documentverification/owner_decision_copy.go`: **100,0%**.
- `internal/documentverification/reviewer_access.go`: **100,0%** en todos sus métodos y funciones.
- Paquete `internal/documentverification`: cobertura global del **90,9%**.
- Cobertura global de statements en todo el subsistema (`documentverification/...`): **82,9%**.

## Límites observados

- Test-only con fixtures sintéticos en memoria; sin dependencias externas de red o almacenamiento.
- Sin migraciones SQL ni ejecución DDL sobre bases de datos persistentes o Cloud SQL.
- Sin commit, push, deploy ni integración a `main`.
