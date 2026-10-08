export interface SuppressionReviewPanelState {
  queueStatus: string;
  evaluationText: string;
}

export function initialSuppressionReviewPanelState(): SuppressionReviewPanelState {
  return {
    queueStatus: "Inicia sesión con rol administrador para consultar la cola.",
    evaluationText: "Inicia sesión con rol administrador para consultar la cola.",
  };
}

export function clearSuppressionReviewPanelState(): SuppressionReviewPanelState {
  return initialSuppressionReviewPanelState();
}

export function withSuppressionQueueCount(
  state: SuppressionReviewPanelState,
  count: number,
): SuppressionReviewPanelState {
  return { ...state, queueStatus: `${count} solicitud(es) en revisión.` };
}

export function withSuppressionEvaluation(
  state: SuppressionReviewPanelState,
  result: unknown,
): SuppressionReviewPanelState {
  return { ...state, evaluationText: JSON.stringify(result, null, 2) };
}
