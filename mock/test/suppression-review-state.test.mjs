import test from "node:test";
import assert from "node:assert/strict";
import {
  clearSuppressionReviewPanelState,
  formatSuppressionExecution,
  initialSuppressionReviewPanelState,
  withSuppressionEvaluation,
  withSuppressionQueueCount,
} from "../../internal/mockserver/static/suppression-review-state.js";

test("refreshing the queue preserves the latest administrative evaluation", () => {
  const result = {
    outcome: "blocked",
    obligations_detected: ["active_booking"],
    pending_checks: ["dispute_review"],
    reviewed_at: "2026-10-07T16:20:00Z",
  };

  const evaluated = withSuppressionEvaluation(initialSuppressionReviewPanelState(), result);
  const refreshed = withSuppressionQueueCount(evaluated, 2);

  assert.equal(refreshed.queueStatus, "2 solicitud(es) en revisión.");
  assert.equal(refreshed.evaluationText, JSON.stringify(result, null, 2));
  for (const field of ["outcome", "obligations_detected", "pending_checks", "reviewed_at"]) {
    assert.match(refreshed.evaluationText, new RegExp(`"${field}"`));
  }
  assert.match(refreshed.evaluationText, /2026-10-07T16:20:00Z/);

  const cleared = clearSuppressionReviewPanelState();
  assert.match(cleared.queueStatus, /Inicia sesión/);
  assert.match(cleared.evaluationText, /Inicia sesión/);
});

test("suppression execution presents residual retention and recoverable file failures", () => {
  const text = formatSuppressionExecution({
    status: "limpieza_pendiente",
    outcome: "limpieza_pendiente",
    obligations_detected: [],
    pending_files: 1,
    removed: ["sesiones_y_tokens"],
    retained: ["ancla_tecnica_usuario", "auditoria_5_anios"],
    started_at: "2026-10-08T10:00:00Z",
    completed_at: null,
  });
  assert.match(text, /Resultado: limpieza_pendiente/);
  assert.match(text, /Archivos pendientes: 1/);
  assert.match(text, /sesiones_y_tokens/);
  assert.match(text, /auditoria_5_anios/);
  assert.match(text, /Término: pendiente/);
});
