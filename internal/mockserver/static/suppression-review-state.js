export function initialSuppressionReviewPanelState() {
    return {
        queueStatus: "Inicia sesión con rol administrador para consultar la cola.",
        evaluationText: "Inicia sesión con rol administrador para consultar la cola.",
    };
}
export function clearSuppressionReviewPanelState() {
    return initialSuppressionReviewPanelState();
}
export function withSuppressionQueueCount(state, count) {
    return { ...state, queueStatus: `${count} solicitud(es) en revisión.` };
}
export function withSuppressionEvaluation(state, result) {
    return { ...state, evaluationText: JSON.stringify(result, null, 2) };
}
