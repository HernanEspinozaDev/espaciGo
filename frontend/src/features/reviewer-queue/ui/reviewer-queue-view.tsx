"use client";

import Link from "next/link";
import { useReviewerQueue } from "./use-reviewer-queue";

export function ReviewerQueueView() {
  const { state, rows } = useReviewerQueue();

  return (
    <section
      aria-labelledby="reviewer-queue-title"
      className="mx-auto max-w-5xl p-6"
    >
      <h2 id="reviewer-queue-title" className="text-2xl font-semibold">
        Revisión de identidad
      </h2>
      <p className="mt-2 text-muted-foreground">
        Resumen de casos de identidad asignados.
      </p>
      {state === "loading" ? (
        <p role="status">Cargando casos…</p>
      ) : state === "unavailable" ? (
        <p role="status">Casos no disponibles.</p>
      ) : state === "empty" ? (
        <p role="status">No hay casos asignados.</p>
      ) : (
        <ul className="mt-6 divide-y">
          {rows.map((row) => (
            <li key={row.id} className="py-4">
              <div>
                <p>Expediente: {row.id}</p>
                <p>Estado: {row.stateLabel}</p>
                <p>Revisión: {row.revision}</p>
                {row.documentExpiresAtObserved && (
                  <p>Vencimiento observado: {row.documentExpiresAtObserved}</p>
                )}
              </div>
              <Link
                className="mt-3 inline-flex rounded-md border px-3 py-2 text-sky-800"
                href={`/mi-cuenta/revisiones/${row.id}`}
              >
                Abrir expediente
              </Link>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
