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

function codeList(value: unknown): string {
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === "string").join(", ") : "";
}

export function formatSuppressionExecution(result: Record<string, unknown>): string {
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
