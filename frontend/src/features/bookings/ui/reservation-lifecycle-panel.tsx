"use client";

import { useEffect, useState } from "react";
import { useAuth } from "@/features/auth/ui/use-auth";
import { useBookings } from "./use-bookings";
import { formatBookingClp } from "./format";
import { getBookingStateConfig } from "./reservations-list";
import { Button } from "@/components/workspace/ui/button";
import {
  CreditCard,
  CheckCircle,
  XCircle,
  FileText,
  Download,
  KeyRound,
  LogOut,
  ShieldAlert,
  Clock,
  DollarSign,
  Calendar,
  AlertTriangle,
  History,
  Info,
  Loader2,
  Receipt,
  ShieldCheck,
} from "lucide-react";

interface ReservationLifecyclePanelProps {
  reservationId: string;
  role?: "arrendatario" | "anfitrion";
}

export function ReservationLifecyclePanel({
  reservationId,
}: ReservationLifecyclePanelProps) {
  const { accessToken } = useAuth();
  const {
    currentReservation,
    contract,
    operations,
    guarantee,
    settlement,
    settlementReceipt,
    cancellationPreview,
    businessClock,
    actionLoading,
    error,
    clearError,
    loadReservationDetail,
    submitSimulatedPayment,
    acceptReservation,
    rejectReservation,
    requestCancellationPreview,
    cancelReservation,
    simulateRefund,
    loadContract,
    signContract,
    rejectContract,
    downloadContractPdf,
    loadOperations,
    performCheckIn,
    performCheckOut,
    performReception,
    loadGuarantee,
    reconcileGuarantee,
    loadSettlement,
    submitFinancialDecision,
    loadReceipt,
    reconcileSettlement,
    getBusinessClock,
  } = useBookings();

  const [rejectReason, setRejectReason] = useState("");
  const [cancelReason, setCancelReason] = useState("");
  const [operationComments, setOperationComments] = useState("");
  const [receptionObservations, setReceptionObservations] = useState("");
  const [receptionResult, setReceptionResult] = useState<
    "registrada" | "recepcion_conforme" | "recepcion_con_observaciones"
  >("recepcion_conforme");
  const [evidenceCode, setEvidenceCode] = useState<
    "sin_evidencia" | "synthetic-png-v1" | "synthetic-damage-png-v1"
  >("sin_evidencia");
  const [localFeedback, setLocalFeedback] = useState<string | null>(null);

  // Initial load
  useEffect(() => {
    if (accessToken && reservationId) {
      loadReservationDetail(accessToken, reservationId).catch(() => {});
      loadContract(accessToken, reservationId).catch(() => {});
      loadOperations(accessToken, reservationId).catch(() => {});
      loadGuarantee(accessToken, reservationId).catch(() => {});
      loadSettlement(accessToken, reservationId).catch(() => {});
      getBusinessClock(accessToken).catch(() => {});
    }
  }, [
    accessToken,
    reservationId,
    loadReservationDetail,
    loadContract,
    loadOperations,
    loadGuarantee,
    loadSettlement,
    getBusinessClock,
  ]);

  if (!currentReservation) {
    return (
      <div className="flex flex-col items-center justify-center rounded-2xl border border-border bg-card p-12 text-center">
        <Loader2 className="h-8 w-8 animate-spin text-primary" />
        <p className="mt-4 text-sm text-muted-foreground">
          Cargando ciclo operativo de la reserva {reservationId}...
        </p>
      </div>
    );
  }

  const stateConfig = getBookingStateConfig(currentReservation.state);
  const StateIcon = stateConfig.icon;

  // Download contract PDF with private Blob cleanup
  const handleDownloadPdf = async () => {
    if (!accessToken || !contract) return;
    try {
      const blob = await downloadContractPdf(accessToken, contract.id);
      if (!blob) throw new Error("El archivo no está disponible.");
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `contrato-${currentReservation.id.slice(0, 8)}.pdf`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
    } catch (err) {
      setLocalFeedback(
        err instanceof Error ? err.message : "Error al descargar PDF",
      );
    }
  };

  return (
    <div className="space-y-8">
      {/* Feedback banner */}
      {(localFeedback || error) && (
        <div
          role="alert"
          className="flex items-start gap-3 rounded-xl border border-destructive/20 bg-destructive/10 p-4 text-sm text-red-800 dark:text-red-300"
        >
          <AlertTriangle className="mt-0.5 h-5 w-5 shrink-0" />
          <div className="flex-1">
            <p className="font-medium">Aviso de Operación</p>
            <p className="mt-0.5">{localFeedback || error}</p>
          </div>
          <button
            type="button"
            onClick={() => {
              setLocalFeedback(null);
              clearError();
            }}
            className="text-xs underline hover:no-underline"
          >
            Cerrar
          </button>
        </div>
      )}

      {/* 1. Header y Resumen */}
      <section className="rounded-2xl border border-border bg-card p-4 sm:p-6 shadow-sm overflow-hidden">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <div className="flex flex-wrap items-center gap-2.5">
              <span
                className={`inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs font-semibold ${stateConfig.badgeClass}`}
              >
                <StateIcon className="h-4 w-4" />
                {stateConfig.label}
              </span>
              <span className="rounded-md bg-muted px-2.5 py-0.5 text-xs text-muted-foreground font-mono">
                ID: {currentReservation.id}
              </span>
            </div>
            <h1 className="mt-2 text-xl font-bold tracking-tight text-foreground sm:text-2xl">
              Arriendo de Espacio
            </h1>
            <p className="text-xs text-muted-foreground break-all">
              Espacio: {currentReservation.space_id} · Anfitrión:{" "}
              {currentReservation.host_id} · Arrendatario:{" "}
              {currentReservation.renter_id}
            </p>
          </div>

          <div className="text-left sm:text-right">
            <p className="text-xs uppercase tracking-wider text-muted-foreground font-medium">
              Total Arriendo
            </p>
            <p className="text-2xl font-bold text-foreground">
              {formatBookingClp(currentReservation.subtotal_clp)} CLP
            </p>
            <p className="text-xs text-muted-foreground">
              {currentReservation.units} {currentReservation.rate_unit}
              {currentReservation.units > 1 ? "s" : ""} a{" "}
              {formatBookingClp(currentReservation.unit_price_clp)} CLP
            </p>
          </div>
        </div>

        <div className="mt-6 grid grid-cols-1 gap-4 border-t border-border pt-4 sm:grid-cols-3 text-xs">
          <div className="flex items-center gap-2 text-muted-foreground">
            <Calendar className="h-4 w-4 text-primary shrink-0" />
            <div>
              <p className="font-medium text-foreground">Inicio acordado</p>
              <p>
                {new Date(currentReservation.start_at).toLocaleString("es-CL")}
              </p>
            </div>
          </div>
          <div className="flex items-center gap-2 text-muted-foreground">
            <Clock className="h-4 w-4 text-primary shrink-0" />
            <div>
              <p className="font-medium text-foreground">Término acordado</p>
              <p>
                {new Date(currentReservation.end_at).toLocaleString("es-CL")}
              </p>
            </div>
          </div>
          <div className="flex items-center gap-2 text-muted-foreground">
            <ShieldCheck className="h-4 w-4 text-primary shrink-0" />
            <div>
              <p className="font-medium text-foreground">
                Garantía de Respaldo
              </p>
              <p>
                {formatBookingClp(
                  currentReservation.guarantee_expected_clp ?? 0,
                )}{" "}
                CLP
              </p>
            </div>
          </div>
        </div>

        {/* Historial de transiciones */}
        {currentReservation.history &&
          currentReservation.history.length > 0 && (
            <details className="mt-6 border-t border-border pt-4 text-xs">
              <summary className="cursor-pointer font-medium text-muted-foreground hover:text-foreground flex items-center gap-1.5">
                <History className="h-3.5 w-3.5" />
                Ver historial de transiciones de estado (
                {currentReservation.history.length})
              </summary>
              <div className="mt-3 space-y-2 pl-4 border-l-2 border-border/80">
                {currentReservation.history.map((transition) => (
                  <div
                    key={transition.sequence}
                    className="text-muted-foreground"
                  >
                    <span className="font-semibold text-foreground">
                      {transition.to}
                    </span>
                    {transition.from ? ` (desde ${transition.from})` : ""} —{" "}
                    {transition.reason}
                    <span className="ml-2 text-[10px] opacity-70">
                      {new Date(transition.at).toLocaleString("es-CL")}
                    </span>
                  </div>
                ))}
              </div>
            </details>
          )}
      </section>

      {/* 2. Pago Simulado Mercado Pago */}
      <section className="rounded-2xl border border-border bg-card p-4 sm:p-6 shadow-sm">
        <div className="flex items-center justify-between gap-2 border-b border-border pb-3">
          <div className="flex items-center gap-2">
            <CreditCard className="h-5 w-5 text-sky-500" />
            <h2 className="text-lg font-semibold text-foreground">
              Pasarela de Pago (Mercado Pago)
            </h2>
          </div>
          <span className="rounded-full bg-sky-500/10 px-2.5 py-0.5 text-xs font-medium text-sky-800 dark:text-sky-300">
            Ensayo Sandboxed
          </span>
        </div>

        {/* Permanent Warning Notice */}
        <div className="mt-4 flex items-start gap-2.5 rounded-xl border border-amber-500/20 bg-amber-500/10 p-4 text-xs text-amber-800 dark:text-amber-300">
          <Info className="mt-0.5 h-4 w-4 shrink-0" />
          <div>
            <p className="font-bold uppercase tracking-wider">
              Aviso Permanente de Entorno de Ensayo
            </p>
            <p className="mt-0.5 leading-relaxed">
              Este módulo simula la integración con Mercado Pago. No se conecta
              a servidores de producción, no debita fondos reales ni procesa
              tarjetas bancarias verdaderas.
            </p>
          </div>
        </div>

        <div className="mt-4">
          {currentReservation.state === "pendiente_de_pago" ? (
            <div className="space-y-3">
              <p className="text-sm text-foreground">
                El arriendo está pendiente de pago. Selecciona un resultado
                simulado para evaluar el flujo:
              </p>
              <div className="flex flex-wrap gap-3">
                <Button
                  onClick={async () => {
                    if (!accessToken) return;
                    await submitSimulatedPayment(
                      accessToken,
                      currentReservation.id,
                      "exito",
                    );
                  }}
                  disabled={actionLoading}
                  className="bg-emerald-700 text-white hover:bg-emerald-800"
                >
                  <CheckCircle className="mr-2 h-4 w-4" />
                  Simular Pago Exitoso
                </Button>
                <Button
                  variant="destructive"
                  onClick={async () => {
                    if (!accessToken) return;
                    await submitSimulatedPayment(
                      accessToken,
                      currentReservation.id,
                      "rechazo",
                    );
                  }}
                  disabled={actionLoading}
                >
                  <XCircle className="mr-2 h-4 w-4" />
                  Simular Pago Rechazado
                </Button>
                <Button
                  variant="outline"
                  onClick={async () => {
                    if (!accessToken) return;
                    await submitSimulatedPayment(
                      accessToken,
                      currentReservation.id,
                      "sin_respuesta",
                    );
                  }}
                  disabled={actionLoading}
                >
                  <Clock className="mr-2 h-4 w-4" />
                  Simular Timeout / Sin Respuesta
                </Button>
              </div>
            </div>
          ) : (
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <CheckCircle className="h-4 w-4 text-emerald-500" />
              <span>
                Estado de pago procesado:{" "}
                <strong>{currentReservation.state}</strong>.
              </span>
            </div>
          )}
        </div>
      </section>

      {/* 3. Decisión de Anfitrión (Aprobar / Rechazar) */}
      {currentReservation.state === "pagada" && (
        <section className="rounded-2xl border border-indigo-500/30 bg-indigo-500/5 p-4 sm:p-6 shadow-sm">
          <div className="border-b border-border pb-3">
            <h2 className="text-lg font-semibold text-foreground flex items-center gap-2">
              <ShieldCheck className="h-5 w-5 text-indigo-500" />
              Decisión del Anfitrión
            </h2>
            <p className="mt-1 text-xs text-muted-foreground">
              El usuario ha completado el pago simulado. El anfitrión debe
              aceptar o rechazar la solicitud.
            </p>
          </div>

          <div className="mt-4 space-y-4">
            <div className="flex flex-wrap gap-3">
              <Button
                onClick={async () => {
                  if (!accessToken) return;
                  await acceptReservation(accessToken, currentReservation.id);
                }}
                disabled={actionLoading}
                className="bg-indigo-600 text-white hover:bg-indigo-700"
              >
                <CheckCircle className="mr-2 h-4 w-4" />
                Aceptar y Aprobar Reserva
              </Button>
            </div>

            <div className="space-y-2 border-t border-border pt-4">
              <label
                htmlFor="reject-reason"
                className="block text-xs font-medium text-foreground"
              >
                Motivo de rechazo (si aplica)
              </label>
              <div className="flex flex-col gap-2 sm:flex-row">
                <input
                  id="reject-reason"
                  type="text"
                  value={rejectReason}
                  onChange={(e) => setRejectReason(e.target.value)}
                  placeholder="Ej: Espacio no disponible por mantenimiento inesperado..."
                  className="flex-1 rounded-lg border border-input bg-background px-3 py-1.5 text-sm shadow-sm focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
                />
                <Button
                  variant="destructive"
                  onClick={async () => {
                    if (!accessToken) return;
                    await rejectReservation(
                      accessToken,
                      currentReservation.id,
                      rejectReason,
                    );
                  }}
                  disabled={actionLoading}
                >
                  <XCircle className="mr-2 h-4 w-4" />
                  Rechazar Reserva
                </Button>
              </div>
            </div>
          </div>
        </section>
      )}

      {/* 4. Cancelación y Devolución */}
      {![
        "cerrada",
        "cancelada_por_firma",
        "cancelada_por_pago",
        "rechazada_arrendador",
        "vencida_pago",
        "vencida_host",
        "cancelada_arrendatario",
      ].includes(currentReservation.state) && (
        <section className="rounded-2xl border border-border bg-card p-4 sm:p-6 shadow-sm">
          <div className="border-b border-border pb-3 flex items-center justify-between">
            <h2 className="text-lg font-semibold text-foreground flex items-center gap-2">
              <LogOut className="h-5 w-5 text-destructive" />
              Cancelación de Reserva y Reembolso
            </h2>
            <Button
              variant="outline"
              size="sm"
              onClick={async () => {
                if (!accessToken) return;
                await requestCancellationPreview(
                  accessToken,
                  currentReservation.id,
                );
              }}
              disabled={actionLoading}
            >
              Consultar Política de Reembolso
            </Button>
          </div>

          {cancellationPreview && (
            <div className="mt-4 rounded-xl border border-border bg-muted/40 p-4 text-xs space-y-2">
              <div className="flex justify-between font-medium text-foreground">
                <span>Elegible para devolución:</span>
                <span>{cancellationPreview.eligible ? "Sí" : "No"}</span>
              </div>
              <div className="flex justify-between text-muted-foreground">
                <span>Proporción de reembolso:</span>
                <span>
                  {(cancellationPreview.refund_ratio * 100).toFixed(0)}%
                </span>
              </div>
              <div className="flex justify-between font-semibold text-emerald-600 dark:text-emerald-400 text-sm">
                <span>Monto a devolver:</span>
                <span>
                  {formatBookingClp(cancellationPreview.refund_amount_clp)} CLP
                </span>
              </div>
              {cancellationPreview.penalty_amount_clp > 0 && (
                <div className="flex justify-between text-destructive">
                  <span>Penalización aplicada:</span>
                  <span>
                    {formatBookingClp(cancellationPreview.penalty_amount_clp)}{" "}
                    CLP
                  </span>
                </div>
              )}
              <p className="pt-1 text-muted-foreground italic">
                {cancellationPreview.reason}
              </p>
            </div>
          )}

          <div className="mt-4 space-y-2">
            <label
              htmlFor="cancel-reason"
              className="block text-xs font-medium text-foreground"
            >
              Motivo de la cancelación
            </label>
            <div className="flex flex-col gap-2 sm:flex-row">
              <input
                id="cancel-reason"
                type="text"
                value={cancelReason}
                onChange={(e) => setCancelReason(e.target.value)}
                placeholder="Indica la razón de la cancelación..."
                className="flex-1 rounded-lg border border-input bg-background px-3 py-1.5 text-sm shadow-sm focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
              />
              <Button
                variant="destructive"
                onClick={async () => {
                  if (!accessToken) return;
                  await cancelReservation(
                    accessToken,
                    currentReservation.id,
                    cancelReason,
                  );
                }}
                disabled={actionLoading}
              >
                Ejecutar Cancelación
              </Button>
            </div>
          </div>

          {currentReservation.refund_id && (
            <div className="mt-4 rounded-xl border border-border p-4 text-xs space-y-2 bg-background">
              <div className="flex justify-between font-medium">
                <span>Reembolso registrado:</span>
                <span>{currentReservation.refund_id}</span>
              </div>
              <div className="flex justify-between">
                <span>Estado:</span>
                <span className="capitalize">
                  {currentReservation.refund_state ?? "pendiente"}
                </span>
              </div>
              {currentReservation.refund_amount_clp !== undefined && (
                <div className="flex justify-between font-semibold">
                  <span>Monto reembolso:</span>
                  <span>
                    {formatBookingClp(currentReservation.refund_amount_clp)} CLP
                  </span>
                </div>
              )}
              {currentReservation.refund_state === "pendiente" && (
                <Button
                  size="sm"
                  variant="outline"
                  className="mt-2"
                  onClick={async () => {
                    if (!accessToken) return;
                    await simulateRefund(
                      accessToken,
                      currentReservation.id,
                      "exito",
                    );
                  }}
                  disabled={actionLoading}
                >
                  Simular Devolución Exitosa
                </Button>
              )}
            </div>
          )}
        </section>
      )}

      {/* 5. Contrato Digital y Firmas */}
      <section className="rounded-2xl border border-border bg-card p-4 sm:p-6 shadow-sm">
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between border-b border-border pb-3">
          <div className="flex items-center gap-2">
            <FileText className="h-5 w-5 text-indigo-500" />
            <div>
              <h2 className="text-lg font-semibold text-foreground">
                Contrato Digital de Arriendo
              </h2>
              <p className="text-xs text-muted-foreground">
                Generación sintética con firmas electrónicas vinculantes y
                verificación SHA-256.
              </p>
            </div>
          </div>
          {contract && (
            <Button
              variant="outline"
              size="sm"
              onClick={handleDownloadPdf}
              className="inline-flex items-center gap-2"
            >
              <Download className="h-4 w-4" />
              Descargar PDF Seguro
            </Button>
          )}
        </div>

        {contract ? (
          <div className="mt-4 space-y-4">
            <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
              <div>
                <span>Estado del Contrato: </span>
                <span className="font-bold uppercase text-foreground">
                  {contract.state}
                </span>
                <span className="ml-2 font-mono">(v{contract.version})</span>
              </div>
              <span
                className="font-mono text-[10px] truncate max-w-xs"
                title={contract.sha256}
              >
                SHA256: {contract.sha256.slice(0, 16)}...
              </span>
            </div>

            {/* Signatures List */}
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              {contract.signatures.map((sig) => (
                <div
                  key={sig.role}
                  className="rounded-xl border border-border bg-muted/30 p-3.5 text-xs space-y-1"
                >
                  <div className="flex justify-between items-center">
                    <span className="font-semibold capitalize text-foreground">
                      {sig.role === "anfitrion" ? "Anfitrión" : "Arrendatario"}
                    </span>
                    <span
                      className={`rounded-full px-2 py-0.5 text-[10px] font-medium ${
                        sig.state === "firmada"
                          ? "bg-emerald-500/10 text-emerald-800 dark:text-emerald-300"
                          : sig.state === "rechazada"
                            ? "bg-destructive/10 text-destructive"
                            : "bg-amber-500/10 text-amber-800 dark:text-amber-300"
                      }`}
                    >
                      {sig.state}
                    </span>
                  </div>
                  <p className="text-muted-foreground font-mono text-[11px]">
                    Firmante: {sig.signer_id}
                  </p>
                  <p className="text-[10px] text-muted-foreground">
                    Actualizado:{" "}
                    {new Date(sig.updated_at).toLocaleString("es-CL")}
                  </p>
                </div>
              ))}
            </div>

            {/* Action Buttons for Signing */}
            {contract.state !== "firmado" && contract.state !== "anulado" && (
              <div className="flex flex-wrap gap-3 pt-2">
                <Button
                  onClick={async () => {
                    if (!accessToken) return;
                    await signContract(accessToken, contract.id);
                    await loadReservationDetail(
                      accessToken,
                      currentReservation.id,
                    );
                  }}
                  disabled={actionLoading}
                  className="bg-indigo-600 text-white hover:bg-indigo-700"
                >
                  <KeyRound className="mr-2 h-4 w-4" />
                  Firmar Contrato Digital
                </Button>
                <Button
                  variant="outline"
                  onClick={async () => {
                    if (!accessToken) return;
                    await rejectContract(accessToken, contract.id);
                  }}
                  disabled={actionLoading}
                >
                  Rechazar Contrato
                </Button>
              </div>
            )}
          </div>
        ) : (
          <p className="mt-4 text-xs text-muted-foreground">
            El contrato se generará automáticamente tras la aprobación de la
            reserva.
          </p>
        )}
      </section>

      {/* 6. Operación Física: Check-in, Check-out y Recepción */}
      <section className="rounded-2xl border border-border bg-card p-4 sm:p-6 shadow-sm">
        <div className="border-b border-border pb-3 flex items-center justify-between">
          <div>
            <h2 className="text-lg font-semibold text-foreground flex items-center gap-2">
              <KeyRound className="h-5 w-5 text-emerald-500" />
              Operación Física y Acceso
            </h2>
            <p className="mt-1 text-xs text-muted-foreground">
              Registro inmutable de entrega, devolución y recepción del espacio
              con registro de evidencias.
            </p>
          </div>
        </div>

        <div className="mt-4 space-y-5">
          {/* Check-in controls */}
          {currentReservation.state === "lista_para_checkin" && (
            <div className="rounded-xl border border-emerald-500/20 bg-emerald-500/5 p-4 space-y-3">
              <h3 className="font-medium text-emerald-800 dark:text-emerald-300 text-sm flex items-center gap-1.5">
                <CheckCircle className="h-4 w-4" />
                Paso 1: Check-in del Arrendatario
              </h3>
              <div>
                <label className="block text-xs font-medium text-foreground">
                  Comentarios u observaciones de ingreso:
                </label>
                <input
                  type="text"
                  value={operationComments}
                  onChange={(e) => setOperationComments(e.target.value)}
                  placeholder="Ej: Llaves recibidas conforme, todo en orden..."
                  className="mt-1 w-full rounded-lg border border-input bg-background px-3 py-1.5 text-xs shadow-sm focus:border-primary focus:outline-none"
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-foreground">
                  Evidencia fotográfica sintética:
                </label>
                <select
                  value={evidenceCode}
                  onChange={(e) =>
                    setEvidenceCode(
                      e.target.value as
                        | "sin_evidencia"
                        | "synthetic-png-v1"
                        | "synthetic-damage-png-v1",
                    )
                  }
                  className="mt-1 rounded-lg border border-input bg-background px-3 py-1.5 text-xs shadow-sm"
                >
                  <option value="sin_evidencia">Sin evidencia</option>
                  <option value="synthetic-png-v1">
                    synthetic-png-v1 (Estado normal)
                  </option>
                </select>
              </div>
              <Button
                onClick={async () => {
                  if (!accessToken) return;
                  await performCheckIn(accessToken, currentReservation.id, {
                    comments: operationComments,
                    fixture_evidence_code: evidenceCode,
                  });
                  await loadReservationDetail(
                    accessToken,
                    currentReservation.id,
                  );
                  setOperationComments("");
                }}
                disabled={actionLoading}
                className="bg-emerald-700 text-white hover:bg-emerald-800"
              >
                Registrar Check-In Formal
              </Button>
            </div>
          )}

          {/* Check-out controls */}
          {currentReservation.state === "en_curso" && (
            <div className="rounded-xl border border-teal-500/20 bg-teal-500/5 p-4 space-y-3">
              <h3 className="font-medium text-teal-800 dark:text-teal-300 text-sm flex items-center gap-1.5">
                <LogOut className="h-4 w-4" />
                Paso 2: Check-out del Arrendatario
              </h3>
              <div>
                <label className="block text-xs font-medium text-foreground">
                  Comentarios de salida:
                </label>
                <input
                  type="text"
                  value={operationComments}
                  onChange={(e) => setOperationComments(e.target.value)}
                  placeholder="Ej: Espacio entregado limpio, llaves devueltas en casillero..."
                  className="mt-1 w-full rounded-lg border border-input bg-background px-3 py-1.5 text-xs shadow-sm focus:border-primary focus:outline-none"
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-foreground">
                  Evidencia fotográfica sintética:
                </label>
                <select
                  value={evidenceCode}
                  onChange={(e) =>
                    setEvidenceCode(
                      e.target.value as
                        | "sin_evidencia"
                        | "synthetic-png-v1"
                        | "synthetic-damage-png-v1",
                    )
                  }
                  className="mt-1 rounded-lg border border-input bg-background px-3 py-1.5 text-xs shadow-sm"
                >
                  <option value="sin_evidencia">Sin evidencia</option>
                  <option value="synthetic-png-v1">
                    synthetic-png-v1 (Entrega conforme)
                  </option>
                  <option value="synthetic-damage-png-v1">
                    synthetic-damage-png-v1 (Con daños reportados)
                  </option>
                </select>
              </div>
              <Button
                onClick={async () => {
                  if (!accessToken) return;
                  await performCheckOut(accessToken, currentReservation.id, {
                    comments: operationComments,
                    fixture_evidence_code: evidenceCode,
                  });
                  await loadReservationDetail(
                    accessToken,
                    currentReservation.id,
                  );
                  setOperationComments("");
                }}
                disabled={actionLoading}
                className="bg-teal-600 text-white hover:bg-teal-700"
              >
                Registrar Check-Out y Entrega
              </Button>
            </div>
          )}

          {/* Reception controls */}
          {(currentReservation.state === "finalizada" ||
            operations.some((o) => o.kind === "checkout")) && (
            <div className="rounded-xl border border-border bg-muted/30 p-4 space-y-3">
              <h3 className="font-medium text-foreground text-sm flex items-center gap-1.5">
                <ShieldCheck className="h-4 w-4 text-primary" />
                Paso 3: Recepción por parte del Anfitrión
              </h3>
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <div>
                  <label className="block text-xs font-medium text-foreground">
                    Resultado de la recepción:
                  </label>
                  <select
                    value={receptionResult}
                    onChange={(e) =>
                      setReceptionResult(
                        e.target.value as
                          | "registrada"
                          | "recepcion_conforme"
                          | "recepcion_con_observaciones",
                      )
                    }
                    className="mt-1 w-full rounded-lg border border-input bg-background px-3 py-1.5 text-xs shadow-sm"
                  >
                    <option value="recepcion_conforme">
                      Conforme (Sin daños)
                    </option>
                    <option value="recepcion_con_observaciones">
                      Con observaciones / Daños
                    </option>
                    <option value="registrada">Registrada genérica</option>
                  </select>
                </div>
                <div>
                  <label className="block text-xs font-medium text-foreground">
                    Observaciones:
                  </label>
                  <input
                    type="text"
                    value={receptionObservations}
                    onChange={(e) => setReceptionObservations(e.target.value)}
                    placeholder="Detalle de la inspección física..."
                    className="mt-1 w-full rounded-lg border border-input bg-background px-3 py-1.5 text-xs shadow-sm focus:border-primary focus:outline-none"
                  />
                </div>
              </div>
              <Button
                onClick={async () => {
                  if (!accessToken) return;
                  await performReception(accessToken, currentReservation.id, {
                    result: receptionResult,
                    observations: receptionObservations,
                  });
                  await loadReservationDetail(
                    accessToken,
                    currentReservation.id,
                  );
                  setReceptionObservations("");
                }}
                disabled={actionLoading}
              >
                Registrar Recepción de Inmueble
              </Button>
            </div>
          )}

          {/* Historial de operaciones */}
          {operations.length > 0 && (
            <div className="space-y-2 pt-2">
              <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                Historial de Operaciones Registradas ({operations.length})
              </h3>
              <div className="divide-y divide-border/60 rounded-xl border border-border">
                {operations.map((op) => (
                  <div
                    key={op.id}
                    className="p-3.5 text-xs flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between"
                  >
                    <div>
                      <div className="flex items-center gap-2">
                        <span className="font-bold uppercase text-foreground">
                          {op.kind}
                        </span>
                        <span className="rounded-full bg-muted px-2 py-0.5 text-[10px] text-muted-foreground">
                          {op.result}
                        </span>
                      </div>
                      {op.comments && (
                        <p className="mt-1 text-muted-foreground">
                          &quot;{op.comments}&quot;
                        </p>
                      )}
                      {op.observations && (
                        <p className="mt-1 text-muted-foreground italic">
                          Obs: {op.observations}
                        </p>
                      )}
                    </div>
                    <span className="text-[10px] text-muted-foreground font-mono">
                      {new Date(op.occurred_at).toLocaleString("es-CL")}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      </section>

      {/* 7. Garantía, Liquidación y Reloj Comercial */}
      <section className="rounded-2xl border border-border bg-card p-4 sm:p-6 shadow-sm">
        <div className="border-b border-border pb-3 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <DollarSign className="h-5 w-5 text-emerald-500" />
            <h2 className="text-lg font-semibold text-foreground">
              Garantía, Liquidación y Reloj Comercial
            </h2>
          </div>
          {businessClock && (
            <span className="text-xs text-muted-foreground font-mono flex items-center gap-1">
              <Clock className="h-3.5 w-3.5" />
              Reloj:{" "}
              {new Date(businessClock.current_time).toLocaleTimeString("es-CL")}
            </span>
          )}
        </div>

        <div className="mt-4 grid grid-cols-1 gap-6 md:grid-cols-2">
          {/* Garantía */}
          <div className="rounded-xl border border-border p-4 text-xs space-y-3">
            <h3 className="font-semibold text-foreground text-sm flex items-center gap-1.5">
              <ShieldAlert className="h-4 w-4 text-amber-500" />
              Depósito de Garantía
            </h3>
            {guarantee ? (
              <div className="space-y-1.5">
                <div className="flex justify-between">
                  <span>Esperada:</span>
                  <span className="font-medium">
                    {formatBookingClp(guarantee.expected_clp)} CLP
                  </span>
                </div>
                <div className="flex justify-between">
                  <span>Autorizada:</span>
                  <span className="font-medium text-emerald-700 dark:text-emerald-400">
                    {formatBookingClp(guarantee.authorized_clp)} CLP
                  </span>
                </div>
                <div className="flex justify-between">
                  <span>Capturada (Daños):</span>
                  <span className="font-medium text-destructive">
                    {formatBookingClp(guarantee.captured_clp)} CLP
                  </span>
                </div>
                <div className="flex justify-between">
                  <span>Liberada:</span>
                  <span className="font-medium">
                    {formatBookingClp(guarantee.released_clp)} CLP
                  </span>
                </div>
                <div className="flex justify-between font-semibold border-t border-border pt-1">
                  <span>Estado:</span>
                  <span className="uppercase">{guarantee.state}</span>
                </div>
              </div>
            ) : (
              <p className="text-muted-foreground">
                No se ha consultado o no aplica garantía.
              </p>
            )}
            <div className="flex flex-wrap gap-2 pt-2">
              <Button
                size="sm"
                variant="outline"
                onClick={async () => {
                  if (!accessToken) return;
                  await reconcileGuarantee(accessToken, currentReservation.id, {
                    kind: "autorizacion",
                    amount_clp:
                      currentReservation.guarantee_expected_clp || 50000,
                    outcome: "exito",
                  });
                }}
                disabled={actionLoading}
              >
                Autorizar
              </Button>
              <Button
                size="sm"
                variant="outline"
                onClick={async () => {
                  if (!accessToken) return;
                  await reconcileGuarantee(accessToken, currentReservation.id, {
                    kind: "liberacion",
                    amount_clp:
                      currentReservation.guarantee_expected_clp || 50000,
                    outcome: "exito",
                  });
                }}
                disabled={actionLoading}
              >
                Liberar
              </Button>
            </div>
          </div>

          {/* Liquidación */}
          <div className="rounded-xl border border-border p-4 text-xs space-y-3">
            <h3 className="font-semibold text-foreground text-sm flex items-center gap-1.5">
              <Receipt className="h-4 w-4 text-primary" />
              Liquidación al Anfitrión
            </h3>
            {settlement ? (
              <div className="space-y-1.5">
                <div className="flex justify-between">
                  <span>Arriendo Bruto:</span>
                  <span>{formatBookingClp(settlement.gross_rent_clp)} CLP</span>
                </div>
                <div className="flex justify-between">
                  <span>Comisión ({settlement.commission_bps / 100}%):</span>
                  <span className="text-muted-foreground">
                    - {formatBookingClp(settlement.commission_clp)} CLP
                  </span>
                </div>
                <div className="flex justify-between font-bold text-foreground border-t border-border pt-1 text-sm">
                  <span>Transferencia Neta:</span>
                  <span>{formatBookingClp(settlement.transfer_clp)} CLP</span>
                </div>
                <div className="flex justify-between font-semibold">
                  <span>Estado:</span>
                  <span className="uppercase text-primary">
                    {settlement.state}
                  </span>
                </div>
              </div>
            ) : (
              <p className="text-muted-foreground">
                La liquidación se habilita al finalizar el arriendo.
              </p>
            )}
            <div className="flex flex-wrap gap-2 pt-2">
              <Button
                size="sm"
                variant="outline"
                onClick={async () => {
                  if (!accessToken) return;
                  await submitFinancialDecision(
                    accessToken,
                    currentReservation.id,
                    "exito",
                  );
                }}
                disabled={actionLoading}
              >
                Solicitar Liquidación
              </Button>
              <Button
                size="sm"
                variant="outline"
                onClick={async () => {
                  if (!accessToken) return;
                  await reconcileSettlement(
                    accessToken,
                    currentReservation.id,
                    "exito",
                  );
                  await loadReceipt(accessToken, currentReservation.id);
                }}
                disabled={actionLoading}
              >
                Conciliar
              </Button>
            </div>
          </div>
        </div>

        {/* Comprobante de liquidación */}
        {settlementReceipt && (
          <div className="mt-4 rounded-xl border border-emerald-500/20 bg-emerald-500/10 p-4 text-xs text-emerald-800 dark:text-emerald-300">
            <div className="flex items-center gap-2 font-bold text-sm">
              <Receipt className="h-4 w-4" />
              <span>{settlementReceipt.banner}</span>
            </div>
            <p className="mt-1 text-muted-foreground">
              Comprobante N° {settlementReceipt.receipt_id} · Confirmado el{" "}
              {new Date(settlementReceipt.confirmed_at).toLocaleString("es-CL")}
            </p>
          </div>
        )}
      </section>
    </div>
  );
}
