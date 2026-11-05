"use client";
import { useEffect, useMemo, useState } from "react";
import { useAuth } from "@/features/auth/ui/use-auth";
import {
  canReview,
  reasonsFor,
  revokeReasons,
  validExpiry,
  type ReviewDecision,
} from "../domain/review";
import { useDocumentReview } from "./use-document-review";

const labels: Record<string, string> = {
  enviada: "Enviada",
  en_revision: "En revisión",
  requiere_antecedentes: "Requiere antecedentes",
  aprobada: "Aprobada",
  rechazada: "Rechazada",
  vencida: "Vencida",
  revocada: "Revocada",
  borrador: "Borrador",
  identity: "Identidad",
  space: "Espacio",
  limpio: "Limpio",
  bloqueado: "Bloqueado",
  pendiente: "Pendiente",
  error: "Error",
};
export function DocumentReviewView() {
  const { snapshot: auth } = useAuth();
  const sessionKey = JSON.stringify([
    auth.status,
    auth.accountId,
    auth.roles,
    auth.capabilities,
    auth.restrictedMode,
    auth.expiresAt,
  ]);
  return <DocumentReviewContent key={sessionKey} />;
}

function DocumentReviewContent() {
  const { snapshot, auth, load, select, download, decide, revoke } =
    useDocumentReview();
  const [formState, setFormState] = useState<{
    caseId: string | null;
    decision: ReviewDecision;
    expiry: string;
    reason: string;
    revokeReason: string;
  }>({
    caseId: null,
    decision: "approve",
    expiry: "",
    reason: "",
    revokeReason: "",
  });
  const current = snapshot.selected;
  const scopedForm =
    formState.caseId === (current?.id ?? null)
      ? formState
      : {
          caseId: current?.id ?? null,
          decision: "approve" as ReviewDecision,
          expiry: "",
          reason: "",
          revokeReason: "",
        };
  const updateForm = (patch: Partial<typeof scopedForm>) =>
    setFormState({ ...scopedForm, ...patch });
  const { decision, expiry, reason, revokeReason } = scopedForm;
  useEffect(() => {
    if (auth.status === "authenticated") void load();
  }, [auth.status, auth.accountId, load]);
  const expiryValue = useMemo(
    () =>
      validExpiry(
        expiry,
        undefined,
        current?.document_expires_at,
        current?.purpose === "space" ? 12 : 24,
      ),
    [expiry, current?.document_expires_at, current?.purpose],
  );
  const reasons = reasonsFor(decision);
  if (auth.status !== "authenticated")
    return (
      <section className="mx-auto max-w-5xl p-6">
        <h2 className="text-2xl font-semibold">Revisión documental</h2>
        <p className="mt-3 text-slate-700">
          {auth.status === "checking"
            ? "Comprobando sesión…"
            : "Inicia sesión con una cuenta revisora asignada para continuar."}
        </p>
      </section>
    );
  return (
    <div className="mx-auto max-w-5xl space-y-6 p-6">
      <header>
        <h2 className="text-2xl font-semibold">Revisión documental</h2>
        <p className="mt-2 text-slate-700">
          Expedientes asignados a tu sesión. El acceso y las decisiones se
          validan en el servidor.
        </p>
      </header>
      {snapshot.error && (
        <div role="alert" className="rounded-md bg-red-50 p-3 text-red-800">
          {snapshot.error}
        </div>
      )}
      <section
        aria-labelledby="assigned-heading"
        className="rounded-xl border border-slate-200 p-5"
      >
        <h3 id="assigned-heading" className="text-lg font-semibold">
          Expedientes asignados
        </h3>
        <button
          type="button"
          className="mt-3 rounded-md border px-3 py-2 text-sky-800"
          onClick={() => void load()}
        >
          Actualizar expedientes
        </button>
        {snapshot.loading && !current ? (
          <p className="mt-3">Cargando asignaciones…</p>
        ) : snapshot.cases.length === 0 ? (
          <p className="mt-3">No hay expedientes asignados disponibles.</p>
        ) : (
          <ul className="mt-3 divide-y">
            {snapshot.cases.map((c) => (
              <li
                key={c.id}
                className="flex flex-wrap items-center justify-between gap-3 py-3"
              >
                <span>
                  {labels[c.purpose]} · {labels[c.state] ?? c.state}
                </span>
                <button
                  data-review-case-id={c.id}
                  className="rounded-md border px-3 py-2 text-sky-800"
                  disabled={snapshot.pending}
                  onClick={() => void select(c.id)}
                >
                  Abrir expediente
                </button>
              </li>
            ))}
          </ul>
        )}
      </section>
      {current && (
        <section
          aria-labelledby="case-heading"
          data-review-case-state={current.state}
          className="space-y-5 rounded-xl border border-slate-200 p-5"
        >
          <div>
            <h3 id="case-heading" className="text-lg font-semibold">
              Expediente · {labels[current.purpose]}
            </h3>
            <p className="text-slate-700">
              Estado: {labels[current.state] ?? current.state} · Revisión{" "}
              {current.revision}
            </p>
          </div>
          <div>
            <h4 className="font-medium">Archivos</h4>
            {current.files.filter((f) => f.state === "limpio").length === 0 ? (
              <p>No hay archivos limpios disponibles.</p>
            ) : (
              <ul className="divide-y">
                {current.files
                  .filter((f) => f.state === "limpio")
                  .map((f) => (
                    <li
                      key={f.id}
                      className="flex items-center justify-between gap-3 py-3"
                    >
                      <span>
                        {labels[f.state]} · {f.mime} · {f.pages} pág.
                      </span>
                      {canReview(current) && (
                        <button
                          data-review-file-id={f.id}
                          className="rounded-md border px-3 py-2 text-sky-800"
                          disabled={snapshot.pending}
                          onClick={() => void download(f.id)}
                        >
                          Abrir archivo limpio
                        </button>
                      )}
                    </li>
                  ))}
              </ul>
            )}
          </div>
          {canReview(current) && (
            <form
              className="space-y-3 border-t pt-4"
              onSubmit={(e) => {
                e.preventDefault();
                void decide({
                  capability:
                    current.purpose === "identity"
                      ? "review_identity"
                      : "review_space",
                  decision,
                  ...(reasons.length ? { reason_code: reason } : {}),
                  ...(decision === "approve"
                    ? { verified_until: expiryValue ?? undefined }
                    : {}),
                });
              }}
            >
              <h4 className="font-medium">Registrar decisión</h4>
              <label className="block">
                Decisión
                <select
                  className="mt-1 block rounded border p-2"
                  value={decision}
                  onChange={(e) => {
                    updateForm({
                      decision: e.target.value as ReviewDecision,
                      reason: "",
                    });
                  }}
                >
                  <option value="approve">Aprobar</option>
                  <option value="reject">Rechazar</option>
                  <option value="needs_evidence">Solicitar antecedentes</option>
                </select>
              </label>
              {decision === "approve" && (
                <label className="block">
                  Vigencia hasta
                  <input
                    className="mt-1 block rounded border p-2"
                    type="datetime-local"
                    required
                    value={expiry}
                    onChange={(e) => updateForm({ expiry: e.target.value })}
                  />
                  <span className="text-sm text-slate-700">
                    Debe ser futura, máximo{" "}
                    {current.purpose === "space" ? 12 : 24} meses y no superar
                    el vencimiento documental.
                  </span>
                  {expiry && !expiryValue && (
                    <span className="block text-red-800">
                      Ingresa una fecha de vigencia válida.
                    </span>
                  )}
                </label>
              )}
              {reasons.length > 0 && (
                <label className="block">
                  Motivo
                  <select
                    required
                    className="mt-1 block rounded border p-2"
                    value={reason}
                    onChange={(e) => updateForm({ reason: e.target.value })}
                  >
                    <option value="">Selecciona un motivo</option>
                    {reasons.map((r) => (
                      <option key={r} value={r}>
                        {r.replaceAll("_", " ")}
                      </option>
                    ))}
                  </select>
                </label>
              )}
              <button
                className="rounded-md bg-sky-800 px-4 py-2 text-white disabled:opacity-50"
                disabled={
                  snapshot.pending ||
                  (decision === "approve" && !expiryValue) ||
                  (reasons.length > 0 && !reason)
                }
              >
                {snapshot.pending ? "Procesando…" : "Confirmar decisión"}
              </button>
            </form>
          )}
          {current.state === "aprobada" && (
            <form
              className="space-y-3 border-t pt-4"
              onSubmit={(e) => {
                e.preventDefault();
                void revoke({
                  purpose: current.purpose,
                  reason_code: revokeReason,
                });
              }}
            >
              <h4 className="font-medium">Revocar verificación</h4>
              <label className="block">
                Motivo
                <select
                  required
                  className="mt-1 block rounded border p-2"
                  value={revokeReason}
                  onChange={(e) => updateForm({ revokeReason: e.target.value })}
                >
                  <option value="">Selecciona un motivo</option>
                  {revokeReasons.map((r) => (
                    <option key={r} value={r}>
                      {r.replaceAll("_", " ")}
                    </option>
                  ))}
                </select>
              </label>
              <button
                className="rounded-md border border-red-700 px-4 py-2 text-red-800"
                disabled={snapshot.pending || !revokeReason}
              >
                Revocar
              </button>
            </form>
          )}
        </section>
      )}
    </div>
  );
}
