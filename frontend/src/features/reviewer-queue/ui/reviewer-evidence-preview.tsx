"use client";

import type { ReviewerIdentityFile } from "../domain/reviewer-case";
import Image from "next/image";
import { useReviewerIdentityEvidence } from "./use-reviewer-identity-evidence";

export function ReviewerEvidencePreview({
  caseId,
  file,
  label,
}: {
  readonly caseId: string;
  readonly file: ReviewerIdentityFile;
  readonly label: string;
}) {
  const { state, url, load } = useReviewerIdentityEvidence(caseId, file);
  const normalizedLabel = label.toLocaleLowerCase("es-CL");

  return (
    <div className="mt-2 space-y-2">
      <button
        className="rounded-md border px-3 py-2 text-sky-800 disabled:cursor-wait disabled:opacity-60"
        disabled={state === "loading" || state === "ready"}
        onClick={() => void load()}
        type="button"
      >
        {state === "loading"
          ? "Cargando evidencia…"
          : state === "ready"
            ? "Evidencia cargada"
            : `Ver ${normalizedLabel}`}
      </button>
      {state === "loading" && <p role="status">Cargando evidencia…</p>}
      {state === "unavailable" && <p role="status">Evidencia no disponible.</p>}
      {state === "ready" && url && (
        <figure className="relative h-[60vh] w-full max-w-3xl">
          <Image
            alt={`Evidencia de ${normalizedLabel}`}
            className="rounded-md object-contain"
            fill
            sizes="(max-width: 768px) 100vw, 768px"
            src={url}
            unoptimized
          />
        </figure>
      )}
    </div>
  );
}
