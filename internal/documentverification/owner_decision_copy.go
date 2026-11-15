package documentverification

const (
	ownerNeedsEvidenceFallback = "Se requieren antecedentes adicionales para continuar. Revisa la sección de documentos y completa lo que falta antes de enviar nuevamente."
	ownerNeedsEvidenceCopy     = "Faltan antecedentes para continuar con la revisión. Revisa los documentos solicitados y completa lo que falta antes de enviar nuevamente."
)

// OwnerDecisionCopy returns approved, minimized copy for an identity owner.
// Internal reason codes are never part of the returned value.
func OwnerDecisionCopy(purpose Purpose, state CaseState, reason ReasonCode) string {
	if purpose != PurposeIdentity {
		return ""
	}
	switch state {
	case CaseNeedsEvidence:
		if reason == "" {
			return ""
		}
		if reason == ReasonEvidenceIncomplete {
			return ownerNeedsEvidenceCopy
		}
		return ownerNeedsEvidenceFallback
	default:
		return ""
	}
}
