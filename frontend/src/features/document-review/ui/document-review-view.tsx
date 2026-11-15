"use client";
import { useEffect, useMemo, useState } from "react";
import { useAuth } from "@/features/auth/ui/use-auth";
import {
  canReview,
  reasonsFor,
  revokeReasons,
  validExpiry,
  type IdentityReviewChecklist,
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
const checklistItems = [
  { key: "names_match", label: "Nombres y apellidos coinciden" },
  { key: "rut_matches", label: "RUN/RUT coincide" },
  {
    key: "birth_date_and_majority_match",
    label: "Fecha de nacimiento y mayoría de edad coinciden",
  },
  {
    key: "portrait_matches_selfie",
    label: "Retrato del documento coincide con la selfie",
  },
  { key: "evidence_legible", label: "Las evidencias son legibles" },
  { key: "document_unexpired", label: "El documento está vigente" },
  {
    key: "issue_date_and_format_verified",
    label: "Fecha de emisión y formato fueron revisados",
  },
] as const;
type ChecklistKey = (typeof checklistItems)[number]["key"];
function emptyChecklist(): IdentityReviewChecklist {
  return {
    names_match: null,
    rut_matches: null,
    birth_date_and_majority_match: null,
    portrait_matches_selfie: null,
    evidence_legible: null,
    document_unexpired: null,
    issue_date_and_format_verified: null,
  };
}
function observedDocumentExpiry(value: string): string | null {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return null;
  const timestamp = Date.parse(`${value}T23:59:59.999Z`);
  if (!Number.isFinite(timestamp) || timestamp <= Date.now()) return null;
  const iso = new Date(timestamp).toISOString();
  return iso.slice(0, 10) === value ? iso : null;
}
const evidenceLabels: Record<string, string> = {
  identity_front: "anverso",
  identity_back: "reverso",
  identity_selfie: "selfie",
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
  const {
    snapshot,
    auth,
    load,
    select,
    preview,
    closePreview,
    decide,
    revoke,
  } = useDocumentReview();
  const [formState, setFormState] = useState<{
    caseId: string | null;
    decision: ReviewDecision;
    expiry: string;
    observedFormat: "" | "legacy" | "new" | "undetermined";
    observedDocumentExpiry: string;
    checklist: IdentityReviewChecklist;
    reason: string;
    revokeReason: string;
  }>({
    caseId: null,
    decision: "approve",
    expiry: "",
    observedFormat: "",
    observedDocumentExpiry: "",
    checklist: emptyChecklist(),
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
          observedFormat: "" as const,
          observedDocumentExpiry: "",
          checklist: emptyChecklist(),
          reason: "",
          revokeReason: "",
        };
  const updateForm = (patch: Partial<typeof scopedForm>) =>
    setFormState({ ...scopedForm, ...patch });
  const {
    decision,
    expiry,
    observedFormat,
    observedDocumentExpiry: observedDocumentExpiryInput,
    checklist,
    reason,
    revokeReason,
  } = scopedForm;
  useEffect(() => {
    if (auth.status === "authenticated") void load();
  }, [auth.status, auth.accountId, load]);
  const observedExpiryValue = observedDocumentExpiry(
    observedDocumentExpiryInput,
  );
  const expiryValue = useMemo(
    () =>
      validExpiry(
        expiry,
        undefined,
        current?.purpose === "identity"
          ? (observedExpiryValue ?? undefined)
          : current?.document_expires_at_observed,
        current?.purpose === "space" ? 12 : 24,
      ),
    [
      expiry,
      observedExpiryValue,
      current?.document_expires_at_observed,
      current?.purpose,
    ],
  );
  const checklistComplete = checklistItems.every(
    ({ key }) => checklist[key as ChecklistKey] !== null,
  );
  const checklistPassed = checklistItems.every(
    ({ key }) => checklist[key as ChecklistKey] === true,
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
                        {f.evidence_role
                          ? `Evidencia · ${evidenceLabels[f.evidence_role] ?? "identidad"}`
                          : labels[f.state]}{" "}
                        · {f.mime} · {f.pages} pág.
                      </span>
                      {canReview(current) && (
                        <button
                          data-review-file-id={f.id}
                          className="rounded-md border px-3 py-2 text-sky-800"
                          disabled={snapshot.pending}
                          onClick={() => void preview(f.id)}
                        >
                          Abrir archivo limpio
                        </button>
                      )}
                    </li>
                  ))}
              </ul>
            )}
          </div>
          {current.identity_declaration && (
            <section aria-labelledby="identity-declaration-heading">
              <h4 id="identity-declaration-heading" className="font-medium">
                Declaración de identidad guardada
              </h4>
              <dl className="mt-2 grid gap-2 sm:grid-cols-2">
                <div>
                  <dt className="font-medium">Nombre</dt>
                  <dd>
                    {current.identity_declaration.first_name}{" "}
                    {current.identity_declaration.last_name}
                  </dd>
                </div>
                <div>
                  <dt className="font-medium">RUN/RUT</dt>
                  <dd>{current.identity_declaration.rut}</dd>
                </div>
                <div>
                  <dt className="font-medium">Fecha de nacimiento</dt>
                  <dd>{current.identity_declaration.date_of_birth}</dd>
                </div>
              </dl>
            </section>
          )}
          {snapshot.preview && (
            <section
              role="dialog"
              aria-modal="true"
              aria-labelledby="private-evidence-viewer-heading"
              className="space-y-3 rounded-lg border border-slate-300 p-4"
            >
              <div className="flex items-center justify-between gap-3">
                <h4
                  id="private-evidence-viewer-heading"
                  className="font-semibold"
                >
                  Visor privado de evidencia
                </h4>
                <button
                  type="button"
                  className="rounded border px-3 py-2"
                  onClick={closePreview}
                >
                  Cerrar visor
                </button>
              </div>
              {(() => {
                const file = current.files.find(
                  (item) => item.id === snapshot.preview?.fileId,
                );
                if (!file || file.state !== "limpio")
                  return <p>La evidencia ya no está disponible.</p>;
                const label =
                  evidenceLabels[file.evidence_role ?? ""] ?? "documento";
                return file.mime.startsWith("image/") ? (
                  // The URL is registry-owned and revoked on close, selection, session change, or unmount.
                  <img
                    className="max-h-[70vh] max-w-full object-contain"
                    src={snapshot.preview.url}
                    alt={`Evidencia privada: ${label}`}
                  />
                ) : (
                  <iframe
                    className="h-[70vh] w-full"
                    src={snapshot.preview.url}
                    title={`Evidencia privada: ${label}`}
                  />
                );
              })()}
            </section>
          )}
          {canReview(current) && (
            <form
              className="space-y-3 border-t pt-4"
              onSubmit={(e) => {
                e.preventDefault();
                const identityReview = current.purpose === "identity";
                if (
                  (decision === "approve" && !expiryValue) ||
                  (identityReview &&
                    (!observedFormat ||
                      !checklistComplete ||
                      (decision === "approve" &&
                        (observedFormat === "undetermined" ||
                          !observedExpiryValue ||
                          !checklistPassed))))
                )
                  return;
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
                  ...(identityReview && observedFormat
                    ? {
                        document_format_observed: observedFormat,
                        ...(observedExpiryValue
                          ? {
                              document_expires_at_observed: observedExpiryValue,
                            }
                          : {}),
                        identity_review_checklist: checklist,
                      }
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
              {current.purpose === "identity" && (
                <>
                  <label className="block">
                    Formato observado
                    <select
                      required
                      className="mt-1 block rounded border p-2"
                      value={observedFormat}
                      onChange={(e) =>
                        updateForm({
                          observedFormat: e.target
                            .value as typeof observedFormat,
                        })
                      }
                    >
                      <option value="">Selecciona el formato observado</option>
                      <option value="legacy">Anterior</option>
                      <option value="new">Nuevo</option>
                      <option value="undetermined">No determinable</option>
                    </select>
                  </label>
                  <label className="block">
                    Vencimiento del documento observado
                    <input
                      className="mt-1 block rounded border p-2"
                      type="date"
                      required={decision === "approve"}
                      value={observedDocumentExpiryInput}
                      onChange={(e) =>
                        updateForm({ observedDocumentExpiry: e.target.value })
                      }
                    />
                  </label>
                  <fieldset className="space-y-3 rounded border p-3">
                    <legend className="px-1 font-medium">
                      Checklist de revisión de identidad
                    </legend>
                    {checklistItems.map(({ key, label }) => (
                      <fieldset key={key} className="flex flex-wrap gap-4">
                        <legend className="w-full">{label}</legend>
                        {[true, false].map((value) => (
                          <label
                            key={String(value)}
                            className="inline-flex items-center gap-2"
                          >
                            <input
                              type="radio"
                              name={`identity-check-${key}`}
                              aria-label={`${label}: ${value ? "Sí" : "No"}`}
                              checked={checklist[key as ChecklistKey] === value}
                              onChange={() =>
                                updateForm({
                                  checklist: { ...checklist, [key]: value },
                                })
                              }
                            />
                            {value ? "Sí" : "No"}
                          </label>
                        ))}
                      </fieldset>
                    ))}
                  </fieldset>
                </>
              )}
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
                  (current.purpose === "identity" &&
                    (!observedFormat ||
                      !checklistComplete ||
                      (decision === "approve" &&
                        (observedFormat === "undetermined" ||
                          !observedExpiryValue ||
                          !checklistPassed)))) ||
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
