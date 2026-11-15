"use client";

import { useEffect, useId, useState } from "react";
import Link from "next/link";
import type { RentalContractPreview } from "../domain/contract-preview";
import { formatRentalCLP } from "../domain/contract-preview-format";
import { publicRentalContractPreview } from "../infrastructure/contract-preview-runtime";

interface PublicRentalContractPreviewProps {
  quoteId: string;
  onClose: () => void;
}

export function PublicRentalContractPreview({
  quoteId,
  onClose,
}: PublicRentalContractPreviewProps) {
  const dialogId = useId();
  const [state, setState] = useState({
    quoteId,
    loading: true,
    error: null as string | null,
    preview: null as RentalContractPreview | null,
    identityRequired: false,
  });
  const visibleState =
    state.quoteId === quoteId
      ? state
      : {
          quoteId,
          loading: true,
          error: null,
          preview: null,
          identityRequired: false,
        };
  const { loading, error, preview, identityRequired } = visibleState;

  useEffect(() => {
    let active = true;
    const controller = new AbortController();

    publicRentalContractPreview
      .getPreview(quoteId, controller.signal)
      .then((result) => {
        if (!active) return;
        if (!result.ok) {
          let message: string;
          if (result.error.kind === "identity_required") {
            message =
              "Se requiere verificación de identidad vigente para preparar el borrador del contrato.";
          } else if (result.error.kind === "not_found") {
            message =
              "La cotización ya no está disponible. Cierra esta ventana y solicita una cotización actualizada.";
          } else if (result.error.kind === "conflict") {
            message =
              "La disponibilidad o las condiciones cambiaron. Solicita una cotización actualizada antes de continuar.";
          } else {
            message =
              "No fue posible generar el borrador de contrato en este momento.";
          }
          setState({
            quoteId,
            loading: false,
            error: message,
            preview: null,
            identityRequired: result.error.kind === "identity_required",
          });
          return;
        }
        setState({
          quoteId,
          loading: false,
          error: null,
          preview: result.value,
          identityRequired: false,
        });
      })
      .catch(() => {
        if (active && !controller.signal.aborted) {
          setState({
            quoteId,
            loading: false,
            error:
              "No fue posible cargar la vista previa del contrato en este momento.",
            preview: null,
            identityRequired: false,
          });
        }
      });

    return () => {
      active = false;
      controller.abort();
    };
  }, [quoteId]);

  const formatTimestamp = (iso: string) => {
    try {
      return new Intl.DateTimeFormat("es-CL", {
        dateStyle: "medium",
        timeStyle: "short",
        timeZone: preview?.time_zone ?? "America/Santiago",
      }).format(new Date(iso));
    } catch {
      return iso;
    }
  };

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby={dialogId}
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm sm:p-6"
    >
      <div className="relative w-full max-w-2xl max-h-[90vh] overflow-y-auto rounded-2xl border border-border bg-background p-6 shadow-2xl sm:p-8 space-y-6">
        <header className="flex items-start justify-between border-b border-border pb-4">
          <div>
            <h2
              id={dialogId}
              className="text-xl font-bold tracking-tight text-foreground"
            >
              Borrador de Condiciones Contractuales
            </h2>
            <p className="text-xs text-muted-foreground mt-1">
              Revisión previa sin efecto vinculante, firma ni retención de
              horario
            </p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-md border p-1 text-muted-foreground hover:bg-muted hover:text-foreground"
            aria-label="Cerrar vista previa"
          >
            ✕
          </button>
        </header>

        {loading ? (
          <div className="py-12 text-center text-sm text-muted-foreground animate-pulse">
            Validando identidades y preparando borrador del contrato...
          </div>
        ) : identityRequired ? (
          <div className="space-y-4 rounded-xl border border-amber-500/30 bg-amber-500/10 p-5 text-sm">
            <h3 className="font-semibold text-amber-700 dark:text-amber-400">
              Verificación de identidad requerida
            </h3>
            <p className="text-muted-foreground leading-relaxed">{error}</p>
            <p className="text-xs text-muted-foreground">
              Para proteger la seguridad de ambas partes, EspaciGo exige validar
              tu identidad oficial antes de redactar o mostrar el borrador
              contractual.
            </p>
            <div className="pt-2">
              <Link
                href="/mi-cuenta/verificaciones"
                className="inline-flex min-h-11 items-center justify-center rounded-lg bg-primary px-5 py-2 text-sm font-medium text-primary-foreground shadow hover:bg-primary/90"
              >
                Ir a verificación de identidad (KYC)
              </Link>
            </div>
          </div>
        ) : error ? (
          <div className="py-6 text-center text-sm text-destructive">
            {error}
          </div>
        ) : preview ? (
          <div className="space-y-6 text-sm">
            <section className="space-y-3 rounded-xl border border-border bg-muted/20 p-4">
              <h3 className="font-semibold text-foreground">
                Partes del Arriendo
              </h3>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                {preview.parties.map((party) => (
                  <div
                    key={party.role}
                    className="rounded-lg border border-border bg-background p-3"
                  >
                    <span className="text-xs uppercase font-medium text-muted-foreground block">
                      {party.role === "arrendador"
                        ? "Propietario / Anfitrión"
                        : "Arrendatario"}
                    </span>
                    <span className="font-medium text-foreground block mt-1">
                      {party.legal_name}
                    </span>
                    <span className="text-xs text-muted-foreground block mt-0.5">
                      RUT: {party.rut}
                    </span>
                  </div>
                ))}
              </div>
            </section>

            <section className="space-y-2 rounded-xl border border-border bg-muted/20 p-4">
              <h3 className="font-semibold text-foreground">
                Condiciones del Espacio e Intervalo
              </h3>
              <dl className="grid grid-cols-2 gap-x-4 gap-y-2 text-sm">
                <dt className="text-muted-foreground">Publicación:</dt>
                <dd className="font-medium text-foreground">
                  {preview.space_title}
                </dd>
                <dt className="text-muted-foreground">Inicio:</dt>
                <dd className="font-medium text-foreground">
                  {formatTimestamp(preview.start_at)}
                </dd>
                <dt className="text-muted-foreground">Término:</dt>
                <dd className="font-medium text-foreground">
                  {formatTimestamp(preview.end_at)}
                </dd>
                <dt className="text-muted-foreground">Zona Horaria:</dt>
                <dd className="font-medium text-foreground">
                  {preview.time_zone}
                </dd>
                <dt className="text-muted-foreground">Unidades:</dt>
                <dd className="font-medium text-foreground">
                  {preview.units} ({preview.rate_unit})
                </dd>
                <dt className="text-muted-foreground">Subtotal estimado:</dt>
                <dd className="font-bold text-foreground text-base">
                  {formatRentalCLP(preview.subtotal_clp)}
                </dd>
              </dl>
            </section>

            <div className="rounded-lg border border-border bg-muted/40 p-4 text-xs leading-relaxed text-muted-foreground">
              <p className="font-semibold text-foreground mb-1">
                Advertencia Legal y Alcance:
              </p>
              Este documento es únicamente un borrador de revisión previa
              generado automáticamente. No constituye contrato firmado,
              aceptación legal, promesa de arriendo, comprobante de pago ni
              reserva confirmada.
            </div>
          </div>
        ) : null}

        <footer className="flex justify-end pt-2 border-t border-border">
          <button
            type="button"
            onClick={onClose}
            className="min-h-11 rounded-lg border border-border px-5 py-2 text-sm font-medium hover:bg-muted text-foreground"
          >
            Cerrar
          </button>
        </footer>
      </div>
    </div>
  );
}
