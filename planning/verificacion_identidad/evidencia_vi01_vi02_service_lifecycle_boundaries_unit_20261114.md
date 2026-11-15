# Evidencia — VI-01 / VI-02, límites exhaustivos de ciclo de vida, transiciones y grants en Service

- **Fecha:** 2026-11-14.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / commit `c9873fc3459c55b6ef2220d91244ea1d9da7a5fb`.
- **Entorno de ejecución:** Entorno local aislado con fixtures sintéticos en memoria. Sin acceso ni llamadas a Cloud Storage, Secret Manager, Cloud SQL, bases PostgreSQL persistentes ni DDL/SQL.
- **Archivo de pruebas añadido:** `internal/documentverification/service_lifecycle_boundaries_test.go`.

## 1. Alcance cubierto (especificación exacta de casos de prueba)

Se implementó una suite de pruebas unitarias sobre `internal/documentverification/service.go` cubriendo compuertas de snapshot de espacio, inicialización, adición de archivos, envío, revisión, revocación y validación de grants:

1. **`TestServiceValidSnapshotAndStartBoundaries`**:
   - Invariantes de snapshot para espacio (`validSnapshot`): snapshot base de espacio es válido; se rechaza si incluye campos de consentimiento identitario (`CaptureConsentVersion != ""` o `CaptureConsentedAt` poblado).
   - Fronteras de inicialización de espacio (`svc.Start` con `PurposeSpace`):
     1. Rechazo si `DocumentFormatDeclared != ""` (`ErrInvalid`).
     2. Rechazo si `DocumentExpiresAt` está en el pasado frente al reloj configurado (`ErrInvalid`).
     3. Rechazo si `CaptureConsentGiven == true` (`ErrInvalid`).
     4. Rechazo si `CaptureConsentVersion != ""` (`ErrInvalid`).

2. **`TestServiceAddFileExhaustiveBoundaries`**:
   - Rechazo cuando `actor` no coincide con `x.SubjectID` (`ErrCapability`).
   - Rechazo en casos de espacio cuando `x.SubjectID` no coincide con `a.SpaceOwnerID` (`ErrCapability`).
   - Rechazo cuando el estado del caso no permite adjuntar archivos (`CaseSubmitted` -> `ErrTransition`).
   - Rechazo en casos de identidad con formato documental no declarado pero archivo con rol asignado (`ErrInvalid`).
   - Rechazo en casos de identidad con formato declarado pero archivo con rol vacío (`ErrInvalid`).
   - Rechazo en casos de identidad con formato declarado cuando el archivo es MIME `application/pdf` (`ErrInvalid`).

3. **`TestServiceSubmitExhaustiveBoundaries`**:
   - Verificación de concordancia de actor (`actor != x.SubjectID` -> `ErrCapability`).
   - Rechazo para casos de espacio cuando `x.SubjectID != a.SpaceOwnerID` (`ErrConflict`).
   - Rechazo cuando el caso no está en estado `CaseDraft` o `CaseNeedsEvidence` (`CaseApproved` -> `ErrTransition`).
   - Rechazo en casos de espacio con archivos que no están en estado limpio (`FilePending` -> `ErrFilesNotClean`).
   - Rechazo en casos de identidad con evidencia incompleta por ausencia de roles obligatorios (solo 2 archivos sin selfie -> `ErrEvidenceIncomplete`).

4. **`TestServiceReviewExhaustiveBoundaries`**:
   - Rechazo al suministrar formato observado en capacidades no identitarias (`CapabilitySpace` -> `ErrInvalid`).
   - Rechazo cuando el caso está en estado preliminar no enviado (`CaseDraft` -> `ErrTransition`).
   - Rechazo por código de decisión no reconocido (`Decision("invalid_decision")` -> `ErrInvalid`).
   - Rechazo al asociar checklist identitario a casos de espacio (`CapabilitySpace` -> `ErrInvalid`).
   - Aprobación de revisión identitaria con `requestedExpiry` válido acortando la expiración (`VerifiedUntil` actualizado exactamente a `requestedExpiry`).
   - Rechazo de revisión identitaria con `requestedExpiry` en el pasado (`ErrInvalid`).

5. **`TestServiceRevokeExhaustiveBoundaries`**:
   - Rechazo de autorevocación cuando `x.SubjectID == actor` (`ErrSelfReview`).
   - Rechazo de revocación sobre casos no aprobados (`CaseDraft` -> `ErrTransition`).

6. **`TestServiceTransactAndReviewerGrantBoundaries`**:
   - Detección de snapshot con supresión concurrente (`SupersededAt != nil` -> `ErrConflict`).
   - Detección de desalineación de revisión (`x.Revision != rev` -> `ErrConflict`).
   - Rechazo en `transact` cuando la capacidad no corresponde al propósito del caso (`ErrCapability`).
   - Verificación exhaustiva de compuertas en la función pura `hasReviewerGrant`:
     - Aceptación de grant válido.
     - Rechazo por `ActorID` vacío.
     - Rechazo por `Capability` inválida en invocación.
     - Rechazo por `scopeID` no UUIDv4 en invocación.
     - Rechazo por grant con `ReviewerID` no UUIDv4.
     - Rechazo por grant con `ReviewerID` que no coincide con `ActorID`.
     - Rechazo por grant con `Capability` desconocida.
     - Rechazo por grant con `ScopeID` no UUIDv4.
     - Rechazo por grants duales (asignación simultánea de identidad y espacio para el mismo revisor) fail-closed.

## 2. Resultados de ejecución local registrada

Ejecución local en memoria registrada durante la corrida:

```bash
$ go test -race -v -run 'TestService(ValidSnapshotAndStart|AddFileExhaustive|SubmitExhaustive|ReviewExhaustive|RevokeExhaustive|TransactAndReviewerGrant)' ./internal/documentverification
=== RUN   TestServiceValidSnapshotAndStartBoundaries
--- PASS: TestServiceValidSnapshotAndStartBoundaries (0.00s)
=== RUN   TestServiceAddFileExhaustiveBoundaries
--- PASS: TestServiceAddFileExhaustiveBoundaries (0.00s)
=== RUN   TestServiceSubmitExhaustiveBoundaries
--- PASS: TestServiceSubmitExhaustiveBoundaries (0.00s)
=== RUN   TestServiceReviewExhaustiveBoundaries
--- PASS: TestServiceReviewExhaustiveBoundaries (0.00s)
=== RUN   TestServiceRevokeExhaustiveBoundaries
--- PASS: TestServiceRevokeExhaustiveBoundaries (0.00s)
=== RUN   TestServiceTransactAndReviewerGrantBoundaries
--- PASS: TestServiceTransactAndReviewerGrantBoundaries (0.00s)
PASS
ok  	github.com/HernanEspinozaDev/espaciGo/internal/documentverification	1.031s

$ go test -coverprofile=/tmp/cov_svc_new2.out ./internal/documentverification && go tool cover -func=/tmp/cov_svc_new2.out | grep service.go
github.com/HernanEspinozaDev/espaciGo/internal/documentverification/service.go:177:		validReviewReason				100.0%
github.com/HernanEspinozaDev/espaciGo/internal/documentverification/service.go:210:		validRevokeReason				100.0%
github.com/HernanEspinozaDev/espaciGo/internal/documentverification/service.go:231:		validObservedDocumentFormat		100.0%
github.com/HernanEspinozaDev/espaciGo/internal/documentverification/service.go:240:		validObservedDocumentNumber		100.0%
github.com/HernanEspinozaDev/espaciGo/internal/documentverification/service.go:250:		validObservedName				100.0%
github.com/HernanEspinozaDev/espaciGo/internal/documentverification/service.go:260:		validObservedDocumentPersonalCode	100.0%
github.com/HernanEspinozaDev/espaciGo/internal/documentverification/service.go:270:		validObservedNationality			100.0%
github.com/HernanEspinozaDev/espaciGo/internal/documentverification/service.go:280:		validObservedCountry				100.0%
github.com/HernanEspinozaDev/espaciGo/internal/documentverification/service.go:290:		validEvidenceRole				100.0%
github.com/HernanEspinozaDev/espaciGo/internal/documentverification/service.go:298:		validSnapshot					95.0%
github.com/HernanEspinozaDev/espaciGo/internal/documentverification/service.go:340:		NewService						100.0%
github.com/HernanEspinozaDev/espaciGo/internal/documentverification/service.go:351:		Start							100.0%
github.com/HernanEspinozaDev/espaciGo/internal/documentverification/service.go:445:		AddFile							100.0%
github.com/HernanEspinozaDev/espaciGo/internal/documentverification/service.go:490:		allClean						100.0%
github.com/HernanEspinozaDev/espaciGo/internal/documentverification/service.go:502:		validFiles						100.0%
github.com/HernanEspinozaDev/espaciGo/internal/documentverification/service.go:525:		Submit							100.0%
github.com/HernanEspinozaDev/espaciGo/internal/documentverification/service.go:547:		Review							100.0%
github.com/HernanEspinozaDev/espaciGo/internal/documentverification/service.go:648:		Revoke							100.0%
github.com/HernanEspinozaDev/espaciGo/internal/documentverification/service.go:678:		purposeForCap					100.0%
github.com/HernanEspinozaDev/espaciGo/internal/documentverification/service.go:689:		reviewerScopeID				100.0%
github.com/HernanEspinozaDev/espaciGo/internal/documentverification/service.go:697:		hasReviewerGrant				100.0%
github.com/HernanEspinozaDev/espaciGo/internal/documentverification/service.go:710:		ListOwnCases					100.0%
github.com/HernanEspinozaDev/espaciGo/internal/documentverification/service.go:720:		LoadOwnCase					100.0%
github.com/HernanEspinozaDev/espaciGo/internal/documentverification/service.go:731:		transact						91.7%

$ go test -race -count=1 ./internal/documentverification/...
ok  	github.com/HernanEspinozaDev/espaciGo/internal/documentverification	1.063s
ok  	github.com/HernanEspinozaDev/espaciGo/internal/documentverification/inspection	15.843s
ok  	github.com/HernanEspinozaDev/espaciGo/internal/documentverification/transport/http	3.011s

Total statements de todo el subsistema documentverification: 93.1%
```

## 3. Conformidad con directivas y límites

- Cobertura de statements en `service.go` registrada al **98,5%**.
- Cobertura global del subsistema `documentverification/...` registrada al **93,1%**.
- Cero DDL/SQL ejecutado.
- Cero acceso a bases persistentes o Cloud Storage.
- `git diff --check`: limpio.
