"use client";

import { useEffect, useId, useState } from "react";
import Link from "next/link";

interface Party {
  role: "arrendador" | "arrendatario";
  legal_name: string;
  rut: string;
}

interface ContractPreviewData {
  state: "preview";
  quote_id: string;
  space_id: string;
  space_title: string;
  rate_unit: "hora" | "dia" | "mes";
  unit_price_clp: number | string;
  units: number | string;
  subtotal_clp: number | string;
  currency: "CLP";
  start_at: string;
  end_at: string;
  time_zone: string;
  parties: Party[];
  generated_at: string;
}

interface PublicRentalContractPreviewProps {
  quoteId: string;
  onClose: () => void;
}

export function PublicRentalContractPreview({
  quoteId,
  onClose,
}: PublicRentalContractPreviewProps) {
  const dialogId = useId();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [preview, setPreview] = useState<ContractPreviewData | null>(null);
  const [kycRequired, setKycRequired] = useState(false);

  useEffect(() => {
    let active = true;
    const controller = new AbortController();

    fetch(`/api/backend/rental-quotes/${encodeURIComponent(quoteId)}/contract-preview`, {
      method: "POST",
      headers: { Accept: "application/json" },
      signal: controller.signal,
    })
      .then(async (res) => {
        if (!active) return;
        if (res.status === 409) {
          setKycRequired(true);
          setError("Se requiere verificación de identidad vigente (RUT y cédula oficial) para preparar el borrador del contrato.");
          return;
        }
        if (!res.ok) {
          setError("No fue posible generar el borrador de contrato en este momento.");
          return;
        }
        const json = await res.json();
        if (active && json?.data) {
          setPreview(json.data);
        }
      })
      .catch(() => {
        if (active && !controller.signal.aborted) {
          setError("Error de conexión al cargar la vista previa del contrato.");
        }
      })
      .finally(() => {
        if (active) setLoading(false);
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

  const formatCLP = (val: number | string) => {
    const num = typeof val === "number" ? val : parseInt(val, 10);
    if (Number.isNaN(num)) return `$${val} CLP`;
    return `$${num.toLocaleString("es-CL")} CLP`;
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
            <h2 id={dialogId} className="text-xl font-bold tracking-tight text-foreground">
              Borrador de Condiciones Contractuales
            </h2>
            <p className="text-xs text-muted-foreground mt-1">
              Revisión previa sin efecto vinculante, firma ni retención de horario
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
        ) : kycRequired ? (
          <div className="space-y-4 rounded-xl border border-amber-500/30 bg-amber-500/10 p-5 text-sm">
            <h3 className="font-semibold text-amber-700 dark:text-amber-400">
              Verificación de identidad requerida
            </h3>
            <p className="text-muted-foreground leading-relaxed">
              {error}
            </p>
            <p className="text-xs text-muted-foreground">
              Para proteger la seguridad de ambas partes, EspaciGo exige validar tu identidad oficial antes de redactar o mostrar el borrador contractual.
            </p>
            <div className="pt-2">
              <Link
                href="/verificacion"
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
              <h3 className="font-semibold text-foreground">Partes del Arriendo</h3>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                {preview.parties.map((party) => (
                  <div key={party.role} className="rounded-lg border border-border bg-background p-3">
                    <span className="text-xs uppercase font-medium text-muted-foreground block">
                      {party.role === "arrendador" ? "Propietario / Anfitrión" : "Arrendatario"}
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
              <h3 className="font-semibold text-foreground">Condiciones del Espacio e Intervalo</h3>
              <dl className="grid grid-cols-2 gap-x-4 gap-y-2 text-sm">
                <dt className="text-muted-foreground">Publicación:</dt>
                <dd className="font-medium text-foreground">{preview.space_title}</dd>
                <dt className="text-muted-foreground">Inicio:</dt>
                <dd className="font-medium text-foreground">{formatTimestamp(preview.start_at)}</dd>
                <dt className="text-muted-foreground">Término:</dt>
                <dd className="font-medium text-foreground">{formatTimestamp(preview.end_at)}</dd>
                <dt className="text-muted-foreground">Zona Horaria:</dt>
                <dd className="font-medium text-foreground">{preview.time_zone}</dd>
                <dt className="text-muted-foreground">Unidades:</dt>
                <dd className="font-medium text-foreground">{preview.units} ({preview.rate_unit})</dd>
                <dt className="text-muted-foreground">Subtotal estimado:</dt>
                <dd className="font-bold text-foreground text-base">{formatCLP(preview.subtotal_clp)}</dd>
              </dl>
            </section>

            <div className="rounded-lg border border-border bg-muted/40 p-4 text-xs leading-relaxed text-muted-foreground">
              <p className="font-semibold text-foreground mb-1">Advertencia Legal y Alcance:</p>
              Este documento es únicamente un borrador de revisión previa generado automáticamente. No constituye contrato firmado, aceptación legal, promesa de arriendo, comprobante de pago ni reserva confirmada.
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
