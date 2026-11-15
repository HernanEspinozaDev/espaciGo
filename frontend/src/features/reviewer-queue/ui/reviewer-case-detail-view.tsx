"use client";

import Link from "next/link";
import { useState } from "react";
import type { ReviewerIdentityFile } from "../domain/reviewer-case";
import type { ReviewerIdentityDecisionResult } from "../domain/reviewer-case";
import { ReviewerEvidencePreview } from "./reviewer-evidence-preview";
import { useReviewerCaseDetail } from "./use-reviewer-case-detail";
import { ReviewerIdentityDecisionForm } from "./reviewer-identity-decision-form";

const evidenceRoleLabels: Readonly<Record<string, string>> = {
  identity_front: "Anverso",
  identity_back: "Reverso",
  identity_selfie: "Selfie",
};
const fileStateLabels: Readonly<Record<ReviewerIdentityFile["state"], string>> =
  {
    pendiente: "Pendiente",
    limpio: "Limpio",
    bloqueado: "Bloqueado",
    error: "Error",
  };

function evidenceRoleLabel(file: ReviewerIdentityFile): string | null {
  return file.evidenceRole ? evidenceRoleLabels[file.evidenceRole] : null;
}

function evidenceLabel(file: ReviewerIdentityFile): string {
  const role = evidenceRoleLabel(file) ?? "Evidencia";
  return `${role} · ${file.mime} · ${file.pages} pág. · ${fileStateLabels[file.state]}`;
}

const decisionStateLabels: Readonly<
  Record<ReviewerIdentityDecisionResult["state"], string>
> = {
  aprobada: "Aprobada",
  rechazada: "Rechazada",
  requiere_antecedentes: "Antecedentes solicitados",
};

export function ReviewerCaseDetailView({ caseId }: { caseId: string }) {
  const { state, detail, canReview, reload, invalidate } =
    useReviewerCaseDetail(caseId);
  const [decisionUncertain, setDecisionUncertain] = useState(false);
  const [uncertainRefreshAttempts, setUncertainRefreshAttempts] = useState(0);
  const [decisionStatus, setDecisionStatus] = useState<{
    readonly caseId: string;
    readonly label: string;
  } | null>(null);
  const onDecisionCommitted = (result: ReviewerIdentityDecisionResult) => {
    setDecisionUncertain(false);
    setDecisionStatus({
      caseId: result.id,
      label: decisionStateLabels[result.state],
    });
    reload();
  };
  const onDecisionUncertain = () => {
    setDecisionStatus(null);
    setUncertainRefreshAttempts(0);
    setDecisionUncertain(true);
    invalidate();
  };

  const refreshUncertainDecision = () => {
    setUncertainRefreshAttempts((current) => current + 1);
    invalidate();
    reload();
  };
  const uncertaintyRequiresRefresh =
    decisionUncertain && canReview && !(state === "ready" && detail !== null);

  return (
    <section
      aria-labelledby="reviewer-case-detail-title"
      className="mx-auto max-w-5xl space-y-5 p-6"
    >
      <Link
        className="inline-flex rounded-md border px-3 py-2 text-sky-800"
        href="/mi-cuenta/revisiones"
      >
        Volver a revisiones
      </Link>
      <div>
        <h2 id="reviewer-case-detail-title" className="text-2xl font-semibold">
          Expediente de identidad
        </h2>
        <p className="mt-2 text-muted-foreground">
          Detalle privado de la asignación vigente. Esta vista es de solo
          lectura; no habilita decisiones ni abre el contenido de las
          evidencias.
        </p>
      </div>
      {decisionStatus?.caseId === caseId && (
        <p role="status">Estado actualizado: {decisionStatus.label}</p>
      )}

      {uncertaintyRequiresRefresh ? (
        <section
          aria-labelledby="reviewer-uncertain-title"
          className="space-y-3"
        >
          <h3 id="reviewer-uncertain-title" className="font-semibold">
            Resultado no confirmado
          </h3>
          <p>
            No se pudo confirmar la decisión. El expediente y sus evidencias
            permanecen ocultos; no reenvíes la decisión. Actualiza el expediente
            para consultar su estado actual.
          </p>
          {uncertainRefreshAttempts > 0 && state === "unavailable" && (
            <p role="alert">
              No se pudo actualizar el expediente. Vuelve a intentarlo.
            </p>
          )}
          <button
            className="rounded-md border px-3 py-2"
            disabled={uncertainRefreshAttempts > 0 && state === "loading"}
            onClick={refreshUncertainDecision}
            type="button"
          >
            {uncertainRefreshAttempts > 0 && state === "loading"
              ? "Actualizando…"
              : "Actualizar expediente"}
          </button>
        </section>
      ) : state === "loading" ? (
        <p role="status">Cargando expediente…</p>
      ) : state === "unavailable" || !detail ? (
        <p role="alert">Expediente no disponible.</p>
      ) : (
        <div className="space-y-5">
          <section aria-labelledby="reviewer-case-status-title">
            <h3 id="reviewer-case-status-title" className="font-semibold">
              Estado del expediente
            </h3>
            <dl className="mt-2 grid gap-2 sm:grid-cols-2">
              <div>
                <dt className="font-medium">Identificador</dt>
                <dd>{detail.id}</dd>
              </div>
              <div>
                <dt className="font-medium">Estado</dt>
                <dd>{detail.stateLabel}</dd>
              </div>
              <div>
                <dt className="font-medium">Revisión</dt>
                <dd>{detail.revision}</dd>
              </div>
              {detail.documentFormatDeclaredLabel && (
                <div>
                  <dt className="font-medium">Formato declarado</dt>
                  <dd>{detail.documentFormatDeclaredLabel}</dd>
                </div>
              )}
              {detail.documentFormatObservedLabel && (
                <div>
                  <dt className="font-medium">Formato observado</dt>
                  <dd>{detail.documentFormatObservedLabel}</dd>
                </div>
              )}
              {detail.documentExpiresAtObserved && (
                <div>
                  <dt className="font-medium">Vencimiento observado</dt>
                  <dd>{detail.documentExpiresAtObserved}</dd>
                </div>
              )}
            </dl>
          </section>

          <section aria-labelledby="reviewer-identity-declaration-title">
            <h3
              id="reviewer-identity-declaration-title"
              className="font-semibold"
            >
              Declaración de identidad
            </h3>
            <dl className="mt-2 grid gap-2 sm:grid-cols-2">
              <div>
                <dt className="font-medium">Nombre</dt>
                <dd>
                  {detail.declaration.firstName} {detail.declaration.lastName}
                </dd>
              </div>
              <div>
                <dt className="font-medium">RUN/RUT</dt>
                <dd>{detail.declaration.rut}</dd>
              </div>
              <div>
                <dt className="font-medium">Fecha de nacimiento</dt>
                <dd>{detail.declaration.dateOfBirth}</dd>
              </div>
            </dl>
          </section>

          <section aria-labelledby="reviewer-evidence-metadata-title">
            <h3 id="reviewer-evidence-metadata-title" className="font-semibold">
              Evidencias recibidas
            </h3>
            {detail.files.length === 0 ? (
              <p className="mt-2">No hay evidencias disponibles.</p>
            ) : (
              <ul className="mt-2 divide-y">
                {detail.files.map((file) => {
                  const roleLabel = evidenceRoleLabel(file);
                  return (
                    <li key={file.id} className="py-3">
                      <p>{evidenceLabel(file)}</p>
                      {file.state === "limpio" && roleLabel && (
                        <ReviewerEvidencePreview
                          caseId={detail.id}
                          file={file}
                          label={roleLabel}
                        />
                      )}
                    </li>
                  );
                })}
              </ul>
            )}
          </section>

          {(detail.state === "en_revision" ||
            detail.state === "requiere_antecedentes") && (
            <ReviewerIdentityDecisionForm
              caseId={detail.id}
              revision={detail.revision}
              onCommitted={onDecisionCommitted}
              onUncertain={onDecisionUncertain}
            />
          )}
        </div>
      )}
    </section>
  );
}
