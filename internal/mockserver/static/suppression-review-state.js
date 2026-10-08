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
function codeList(value) {
    return Array.isArray(value) ? value.filter((item) => typeof item === "string").join(", ") : "";
}
export function formatSuppressionExecution(result) {
    return [
        `Estado: ${result.status ?? "desconocido"}`,
        `Resultado: ${result.outcome ?? "desconocido"}`,
        `Decisión: ${result.decision_code ?? "sin código"}`,
        `Obligaciones: ${codeList(result.obligations_detected) || "ninguna"}`,
        `Archivos pendientes: ${result.pending_files ?? 0}`,
        `Retirado: ${codeList(result.removed) || "nada"}`,
        `Conservado: ${codeList(result.retained) || "sin datos residuales"}`,
        `Inicio: ${result.started_at ?? "—"}`,
        `Término: ${result.completed_at ?? "pendiente"}`,
    ].join("\n");
}
