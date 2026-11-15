"use client";

import { useState, type FormEvent } from "react";
import type {
  IdentityReviewChecklist,
  IdentityReviewChecklistField,
  IdentityReviewDecision,
  IdentityReviewDocumentFormat,
  ReviewerIdentityDecisionResult,
} from "../domain/reviewer-case";
import { useReviewerIdentityDecision } from "./use-reviewer-identity-decision";

const checklistLabels: readonly {
  readonly field: IdentityReviewChecklistField;
  readonly label: string;
}[] = [
  { field: "names_match", label: "Nombres coinciden" },
  { field: "rut_matches", label: "RUN/RUT coincide" },
  {
    field: "birth_date_and_majority_match",
    label: "Fecha de nacimiento y mayoría de edad verificadas",
  },
  { field: "portrait_matches_selfie", label: "Retrato coincide con selfie" },
  { field: "evidence_legible", label: "Evidencias legibles" },
  { field: "document_unexpired", label: "Documento vigente" },
  {
    field: "issue_date_and_format_verified",
    label: "Fecha de emisión y formato verificados",
  },
];

const initialChecklist: IdentityReviewChecklist = {
  names_match: null,
  rut_matches: null,
  birth_date_and_majority_match: null,
  portrait_matches_selfie: null,
  evidence_legible: null,
  document_unexpired: null,
  issue_date_and_format_verified: null,
};

export function ReviewerIdentityDecisionForm({
  caseId,
  revision,
  onCommitted,
  onUncertain,
}: {
  readonly caseId: string;
  readonly revision: number | string;
  readonly onCommitted: (result: ReviewerIdentityDecisionResult) => void;
  readonly onUncertain: () => void;
}) {
  const [decision, setDecision] =
    useState<IdentityReviewDecision>("needs_evidence");
  const [reasonCode, setReasonCode] = useState("antecedentes_incompletos");
  const [documentFormatObserved, setDocumentFormatObserved] =
    useState<IdentityReviewDocumentFormat>("undetermined");
  const [documentExpiresAtObserved, setDocumentExpiresAtObserved] =
    useState("");
  const [checklist, setChecklist] = useState(initialChecklist);
  const { failed, submitting, submit } = useReviewerIdentityDecision(
    caseId,
    revision,
    onCommitted,
    onUncertain,
  );

  const changeDecision = (value: IdentityReviewDecision) => {
    setDecision(value);
    setReasonCode(
      value === "reject"
        ? "documento_ilegible"
        : value === "needs_evidence"
          ? "antecedentes_incompletos"
          : "",
    );
  };

  const onSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    await submit({
      decision,
      reasonCode,
      documentFormatObserved,
      documentExpiresAtObserved,
      checklist,
    });
  };

  return (
    <form
      aria-labelledby="reviewer-identity-decision-title"
      className="space-y-5 rounded-lg border p-4"
      onSubmit={onSubmit}
    >
      <div>
        <h3
          id="reviewer-identity-decision-title"
          className="text-lg font-semibold"
        >
          Checklist y decisión
        </h3>
        <p className="mt-1 text-sm text-muted-foreground">
          Registra únicamente observaciones verificadas en las evidencias. La
          autorización final se valida en el servidor para esta asignación.
        </p>
      </div>

      <fieldset className="space-y-2">
        <legend className="font-medium">Checklist de identidad</legend>
        {checklistLabels.map(({ field, label }) => (
          <label key={field} className="grid gap-1 sm:grid-cols-2">
            <span>{label}</span>
            <select
              className="rounded-md border px-3 py-2"
              onChange={(event) => {
                const value = event.currentTarget.value;
                setChecklist((current) => ({
                  ...current,
                  [field]: value === "" ? null : value === "true",
                }));
              }}
              required
              value={checklist[field] === null ? "" : String(checklist[field])}
            >
              <option value="">Sin revisar</option>
              <option value="true">Sí</option>
              <option value="false">No</option>
            </select>
          </label>
        ))}
      </fieldset>

      <div className="grid gap-4 sm:grid-cols-2">
        <label className="grid gap-1">
          Decisión
          <select
            className="rounded-md border px-3 py-2"
            onChange={(event) =>
              changeDecision(
                event.currentTarget.value as IdentityReviewDecision,
              )
            }
            required
            value={decision}
          >
            <option value="approve">Aprobar</option>
            <option value="reject">Rechazar</option>
            <option value="needs_evidence">Solicitar antecedentes</option>
          </select>
        </label>

        <label className="grid gap-1">
          Formato observado por revisor
          <select
            className="rounded-md border px-3 py-2"
            onChange={(event) =>
              setDocumentFormatObserved(
                event.currentTarget.value as IdentityReviewDocumentFormat,
              )
            }
            required
            value={documentFormatObserved}
          >
            <option value="legacy">Antiguo</option>
            <option value="new">Nuevo</option>
            <option value="undetermined">Indeterminado</option>
          </select>
        </label>

        {decision !== "approve" && (
          <label className="grid gap-1">
            Motivo
            <select
              className="rounded-md border px-3 py-2"
              onChange={(event) => setReasonCode(event.currentTarget.value)}
              required
              value={reasonCode}
            >
              {decision === "reject" ? (
                <>
                  <option value="documento_ilegible">Documento ilegible</option>
                  <option value="documento_invalido">Documento inválido</option>
                  <option value="documento_vencido">Documento vencido</option>
                  <option value="cambio_titularidad">
                    Cambio de titularidad
                  </option>
                </>
              ) : (
                <option value="antecedentes_incompletos">
                  Antecedentes incompletos
                </option>
              )}
            </select>
          </label>
        )}

        <label className="grid gap-1">
          Fecha de vencimiento observada
          <input
            className="rounded-md border px-3 py-2"
            onChange={(event) =>
              setDocumentExpiresAtObserved(event.currentTarget.value)
            }
            required={
              decision === "approve" ||
              (decision === "reject" && reasonCode === "documento_vencido")
            }
            type="date"
            value={documentExpiresAtObserved}
          />
          <span className="text-sm text-muted-foreground">
            Fecha civil local YYYY-MM-DD, observada en el documento.
          </span>
        </label>
      </div>

      {failed && (
        <p role="alert">
          No se pudo validar la decisión. Revisa la fecha y los datos
          ingresados.
        </p>
      )}
      <button
        className="rounded-md bg-sky-800 px-4 py-2 text-white disabled:opacity-60"
        disabled={submitting}
        type="submit"
      >
        {submitting ? "Registrando…" : "Registrar decisión"}
      </button>
    </form>
  );
}
