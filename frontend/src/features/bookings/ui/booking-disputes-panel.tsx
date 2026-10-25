"use client";

import { useEffect, useState } from "react";
import { useAuth } from "@/features/auth/ui/use-auth";
import { useBookings } from "./use-bookings";
import { validateDisputeReasonCode } from "../domain/booking-rules";
import { Button } from "@/components/workspace/ui/button";
import {
  AlertTriangle,
  ShieldAlert,
  FileQuestion,
  CheckCircle,
  Loader2,
} from "lucide-react";

interface BookingDisputesPanelProps {
  reservationId: string;
}

export function BookingDisputesPanel({
  reservationId,
}: BookingDisputesPanelProps) {
  const { accessToken } = useAuth();
  const {
    disputes,
    damageClaim,
    actionLoading,
    error,
    loadDisputes,
    openDispute,
    loadDamageClaim,
    claimSecurityDeposit,
    submitDamageClaimDefense,
  } = useBookings();

  const [reasonCode, setReasonCode] = useState<string>(
    "condiciones_diferentes",
  );
  const [claimDescription, setClaimDescription] = useState<string>("");
  const [defenseText, setDefenseText] = useState<string>("");
  const [statusMessage, setStatusMessage] = useState<string | null>(null);
  const [validationError, setValidationError] = useState<string | null>(null);

  useEffect(() => {
    if (accessToken && reservationId) {
      loadDisputes(accessToken, reservationId).catch(() => {});
      loadDamageClaim(accessToken, reservationId).catch(() => {});
    }
  }, [accessToken, reservationId, loadDisputes, loadDamageClaim]);

  const handleOpenDispute = async (e: React.FormEvent) => {
    e.preventDefault();
    setValidationError(null);
    setStatusMessage(null);

    if (!validateDisputeReasonCode(reasonCode)) {
      setValidationError("Código de motivo de disputa inválido.");
      return;
    }

    if (!accessToken) {
      setValidationError("Debes iniciar sesión para abrir una disputa.");
      return;
    }

    try {
      await openDispute(accessToken, reservationId, reasonCode);
      setStatusMessage(
        "Disputa abierta formalmente bajo entorno seguro de mediación.",
      );
    } catch (err) {
      setValidationError(
        err instanceof Error ? err.message : "Error al abrir disputa.",
      );
    }
  };

  const handleClaimSecurity = async (e: React.FormEvent) => {
    e.preventDefault();
    setValidationError(null);
    setStatusMessage(null);

    if (!claimDescription.trim()) {
      setValidationError("Debes describir los daños o incumplimiento.");
      return;
    }

    if (!accessToken) return;

    try {
      await claimSecurityDeposit(
        accessToken,
        reservationId,
        claimDescription.trim(),
      );
      setStatusMessage(
        "Reclamo de garantía registrado para resolución arbitral.",
      );
      setClaimDescription("");
    } catch (err) {
      setValidationError(
        err instanceof Error ? err.message : "Error al reclamar garantía.",
      );
    }
  };

  const handleSubmitDefense = async (e: React.FormEvent) => {
    e.preventDefault();
    setValidationError(null);
    setStatusMessage(null);

    if (!defenseText.trim()) {
      setValidationError("Debes ingresar el texto de descargo.");
      return;
    }

    if (!accessToken) return;

    try {
      await submitDamageClaimDefense(
        accessToken,
        reservationId,
        defenseText.trim(),
      );
      setStatusMessage("Descargo registrado exitosamente.");
      setDefenseText("");
    } catch (err) {
      setValidationError(
        err instanceof Error ? err.message : "Error al registrar descargo.",
      );
    }
  };

  return (
    <section className="rounded-2xl border border-border bg-card p-4 sm:p-6 shadow-sm">
      <div className="border-b border-border pb-3 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <AlertTriangle className="h-5 w-5 text-rose-500" />
          <h2 className="text-lg font-semibold text-foreground">
            Disputas, Mediación y Reclamos de Garantía
          </h2>
        </div>
        <span className="text-xs text-muted-foreground">
          {disputes.length} disputa{disputes.length !== 1 ? "s" : ""}
        </span>
      </div>

      {(validationError || error) && (
        <div
          role="alert"
          className="mt-4 flex items-start gap-2.5 rounded-xl border border-destructive/20 bg-destructive/10 p-3 text-xs text-red-800 dark:text-red-300"
        >
          <ShieldAlert className="mt-0.5 h-4 w-4 shrink-0" />
          <div>
            <p className="font-medium">Aviso de Disputa</p>
            <p>{validationError || error}</p>
          </div>
        </div>
      )}

      {statusMessage && (
        <div
          role="status"
          className="mt-4 flex items-start gap-2.5 rounded-xl border border-emerald-500/20 bg-emerald-500/10 p-3 text-xs text-emerald-700 dark:text-emerald-400"
        >
          <CheckCircle className="mt-0.5 h-4 w-4 shrink-0" />
          <p>{statusMessage}</p>
        </div>
      )}

      <div className="mt-5 grid grid-cols-1 gap-6 md:grid-cols-2">
        {/* 1. Apertura de Disputa */}
        <div className="rounded-xl border border-border bg-muted/20 p-4 space-y-4">
          <h3 className="text-sm font-semibold text-foreground flex items-center gap-1.5">
            <FileQuestion className="h-4 w-4 text-primary" />
            Apertura de Disputa por Incumplimiento
          </h3>
          <p className="text-xs text-muted-foreground">
            Si existió alguna discrepancia sobre el acceso, condiciones o normas
            del espacio, puedes abrir una disputa para mediación de la
            plataforma.
          </p>

          <form onSubmit={handleOpenDispute} className="space-y-3">
            <div>
              <label
                htmlFor="dispute-reason"
                className="block text-xs font-medium text-foreground mb-1"
              >
                Motivo principal:
              </label>
              <select
                id="dispute-reason"
                value={reasonCode}
                onChange={(e) => setReasonCode(e.target.value)}
                className="w-full rounded-lg border border-input bg-background p-2 text-xs shadow-sm"
              >
                <option value="espacio_no_disponible">
                  Espacio no disponible
                </option>
                <option value="condiciones_diferentes">
                  Condiciones diferentes a las publicadas
                </option>
                <option value="dano_inmueble">Daño al inmueble</option>
                <option value="problemas_acceso">
                  Problemas de acceso o entrega de llaves
                </option>
                <option value="incumplimiento_normas">
                  Incumplimiento de normas
                </option>
                <option value="otro">Otro motivo justificado</option>
              </select>
            </div>

            <Button
              type="submit"
              variant="destructive"
              size="sm"
              disabled={actionLoading || !accessToken}
              className="w-full"
            >
              {actionLoading ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                "Abrir Disputa Formal"
              )}
            </Button>
          </form>
        </div>

        {/* 2. Reclamo de Garantía por Daños */}
        <div className="rounded-xl border border-border bg-muted/20 p-4 space-y-4">
          <h3 className="text-sm font-semibold text-foreground flex items-center gap-1.5">
            <ShieldAlert className="h-4 w-4 text-amber-500" />
            Reclamo sobre Depósito de Garantía
          </h3>
          <p className="text-xs text-muted-foreground">
            El anfitrión puede solicitar retención o captura de la garantía en
            caso de daños comprobables.
          </p>

          <form onSubmit={handleClaimSecurity} className="space-y-3">
            <div>
              <label
                htmlFor="damage-description"
                className="block text-xs font-medium text-foreground mb-1"
              >
                Descripción detallada del daño:
              </label>
              <textarea
                id="damage-description"
                value={claimDescription}
                onChange={(e) => setClaimDescription(e.target.value)}
                rows={3}
                placeholder="Detalla los daños constatados tras el check-out..."
                className="w-full rounded-lg border border-input bg-background p-2 text-xs shadow-sm placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-primary"
              />
            </div>

            <Button
              type="submit"
              variant="outline"
              size="sm"
              disabled={
                actionLoading || !accessToken || !claimDescription.trim()
              }
              className="w-full text-destructive hover:bg-destructive/10 border-destructive/30"
            >
              {actionLoading ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                "Reclamar Retención de Garantía"
              )}
            </Button>
          </form>
        </div>
      </div>

      {/* Historial de disputas abiertas */}
      {disputes.length > 0 && (
        <div className="mt-6 space-y-3">
          <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            Historial de Disputas Registradas ({disputes.length})
          </h3>
          <div className="space-y-3">
            {disputes.map((disp) => (
              <div
                key={disp.id}
                className="rounded-xl border border-border bg-card p-4 text-xs space-y-2"
              >
                <div className="flex items-center justify-between">
                  <span className="font-semibold text-foreground capitalize">
                    Motivo: {disp.opening_reason_code.replace(/_/g, " ")}
                  </span>
                  <span
                    className={`rounded-full px-2.5 py-0.5 text-[10px] font-medium ${
                      disp.state === "abierta"
                        ? "bg-rose-500/10 text-rose-600 dark:text-rose-400 border border-rose-500/20"
                        : "bg-zinc-500/10 text-zinc-600 dark:text-zinc-400 border border-zinc-500/20"
                    }`}
                  >
                    {disp.state}
                  </span>
                </div>
                <p className="text-muted-foreground font-mono text-[11px]">
                  Iniciada por: {disp.opened_by} · Fecha:{" "}
                  {new Date(disp.opened_at).toLocaleString("es-CL")}
                </p>
                {disp.closed_at && (
                  <p className="text-muted-foreground text-[11px]">
                    Cerrada el{" "}
                    {new Date(disp.closed_at).toLocaleString("es-CL")}:{" "}
                    {disp.close_reason_code}
                  </p>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Reclamo de Daños Activo y Descargo */}
      {damageClaim && (
        <div className="mt-6 rounded-xl border border-border bg-card p-5 text-xs space-y-4">
          <div className="flex items-center justify-between border-b border-border pb-2">
            <h4 className="text-sm font-semibold text-foreground">
              Estado del Reclamo de Daños
            </h4>
            <span className="font-mono text-[10px] text-muted-foreground">
              ID: {damageClaim.id}
            </span>
          </div>

          <p className="text-foreground leading-relaxed">
            <strong>Descripción del anfitrión:</strong> &quot;
            {damageClaim.description}&quot;
          </p>

          {damageClaim.defense ? (
            <div className="rounded-lg border border-border bg-muted/30 p-3 space-y-1">
              <p className="font-semibold text-foreground">
                Descargo del arrendatario:
              </p>
              <p className="text-muted-foreground">
                {damageClaim.defense.comments ?? "Sin comentarios adicionales."}
              </p>
            </div>
          ) : (
            <form
              onSubmit={handleSubmitDefense}
              className="space-y-2 border-t border-border pt-3"
            >
              <label className="block font-medium text-foreground">
                Presentar Descargo / Respuesta al Reclamo:
              </label>
              <div className="flex flex-col sm:flex-row gap-2">
                <input
                  type="text"
                  value={defenseText}
                  onChange={(e) => setDefenseText(e.target.value)}
                  placeholder="Ingresa tu descargo o justificación..."
                  className="flex-1 rounded-lg border border-input bg-background p-2 text-xs shadow-sm focus:outline-none focus:ring-1 focus:ring-primary"
                />
                <Button
                  type="submit"
                  size="sm"
                  disabled={
                    actionLoading || !accessToken || !defenseText.trim()
                  }
                >
                  Enviar Descargo
                </Button>
              </div>
            </form>
          )}

          {damageClaim.resolution && (
            <div className="rounded-lg border border-primary/20 bg-primary/5 p-3 text-xs space-y-1">
              <p className="font-bold text-foreground">
                Resolución del Reclamo:{" "}
                <span className="uppercase text-primary">
                  {damageClaim.resolution.outcome}
                </span>
              </p>
              <p className="text-muted-foreground">
                Motivo: {damageClaim.resolution.reason_code} · Resuelto el{" "}
                {new Date(damageClaim.resolution.resolved_at).toLocaleString(
                  "es-CL",
                )}
              </p>
            </div>
          )}
        </div>
      )}
    </section>
  );
}
