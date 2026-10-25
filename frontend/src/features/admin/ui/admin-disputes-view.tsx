"use client";

import { useEffect, useState } from "react";
import {
  AlertCircle,
  AlertTriangle,
  Bell,
  CheckCircle2,
  Clock,
  Coins,
  EyeOff,
  Flag,
  Loader2,
  RefreshCw,
  RotateCcw,
  Scale,
  XCircle,
} from "lucide-react";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/workspace/ui/card";
import { Button } from "@/components/workspace/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Label } from "@/components/workspace/ui/label";
import { Input } from "@/components/workspace/ui/input";
import { useAdmin } from "./use-admin";
import { isValidAdminReason } from "../domain/admin-rules";
import type {
  AdminDamageClaim,
  AdminDamageClaimResolutionInput,
  AdminLocalDispute,
  AdminLocalNotice,
  AdminReviewReport,
} from "../domain/types";
import { AdminFeedbackBanners } from "./admin-feedback-banners";

function renderDisputeBadge(state: AdminLocalDispute["state"]) {
  switch (state) {
    case "abierta":
      return (
        <Badge
          variant="destructive"
          className="gap-1 bg-red-100 text-red-800 dark:bg-red-950/50 dark:text-red-200 font-medium"
        >
          <AlertCircle className="size-3.5" aria-hidden="true" />
          Abierta
        </Badge>
      );
    case "en_revision":
      return (
        <Badge
          variant="secondary"
          className="gap-1 bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-200"
        >
          <Clock className="size-3.5" aria-hidden="true" />
          En revisión
        </Badge>
      );
    case "resuelta":
      return (
        <Badge
          variant="default"
          className="gap-1 bg-emerald-700 text-white font-medium"
        >
          <CheckCircle2 className="size-3.5" aria-hidden="true" />
          Resuelta
        </Badge>
      );
    case "cerrada":
      return (
        <Badge variant="outline" className="gap-1 text-muted-foreground">
          Cerrada
        </Badge>
      );
    default:
      return <Badge variant="outline">{state}</Badge>;
  }
}

function renderDamageClaimBadge(state: AdminDamageClaim["state"]) {
  switch (state) {
    case "abierto":
      return (
        <Badge
          variant="destructive"
          className="bg-red-100 text-red-800 dark:bg-red-950/50 dark:text-red-200 font-medium"
        >
          Abierto
        </Badge>
      );
    case "en_evaluacion":
      return (
        <Badge variant="secondary" className="bg-amber-100 text-amber-800">
          En evaluación
        </Badge>
      );
    case "resuelto":
      return (
        <Badge variant="default" className="bg-emerald-700 text-white">
          Resuelto
        </Badge>
      );
    case "rechazado":
      return (
        <Badge variant="outline" className="text-muted-foreground">
          Rechazado
        </Badge>
      );
    default:
      return <Badge variant="outline">{state}</Badge>;
  }
}

export function AdminDisputesView() {
  const {
    snapshot,
    loadDisputes,
    closeDispute,
    loadDamageClaims,
    resolveDamageClaim,
    loadReviewReports,
    moderateReviewReport,
    loadNotices,
    reopenNotice,
  } = useAdmin();

  const [activeSection, setActiveSection] = useState<
    "disputes" | "damage" | "reviews" | "notices"
  >("disputes");

  // Dispute close dialog
  const [selectedDispute, setSelectedDispute] =
    useState<AdminLocalDispute | null>(null);
  const [closeReasonCode, setCloseReasonCode] = useState<
    "ensayo_finalizado" | "registro_erroneo" | "duplicada"
  >("ensayo_finalizado");

  // Damage claim resolution dialog
  const [selectedClaim, setSelectedClaim] = useState<AdminDamageClaim | null>(
    null,
  );
  const [claimDecision, setClaimDecision] = useState<
    "aceptado" | "rechazado" | "parcial"
  >("aceptado");
  const [claimNotes, setClaimNotes] = useState("");
  const [approvedAmount, setApprovedAmount] = useState<string>("");
  const [claimError, setClaimError] = useState<string | null>(null);

  // Review report moderation dialog
  const [selectedReport, setSelectedReport] =
    useState<AdminReviewReport | null>(null);
  const [reportAction, setReportAction] = useState<"dismiss" | "hide">(
    "dismiss",
  );
  const [reportReasonCode, setReportReasonCode] = useState<
    "sin_infraccion" | "contenido_inadecuado" | "duplicado" | "error_registro"
  >("sin_infraccion");

  useEffect(() => {
    void loadDisputes();
    void loadDamageClaims();
    void loadReviewReports();
    void loadNotices();
  }, [loadDisputes, loadDamageClaims, loadReviewReports, loadNotices]);

  const {
    disputes,
    damageClaims,
    reviewReports,
    notices,
    loading,
    actionLoading,
  } = snapshot;

  const handleOpenCloseDispute = (dispute: AdminLocalDispute) => {
    setSelectedDispute(dispute);
    setCloseReasonCode("ensayo_finalizado");
  };

  const handleConfirmCloseDispute = async () => {
    if (!selectedDispute) return;
    const success = await closeDispute(
      selectedDispute.dispute_id,
      closeReasonCode,
    );
    if (success) {
      setSelectedDispute(null);
    }
  };

  const handleOpenResolveClaim = (claim: AdminDamageClaim) => {
    setSelectedClaim(claim);
    setClaimDecision("aceptado");
    setClaimNotes("");
    setApprovedAmount("");
    setClaimError(null);
  };

  const handleConfirmResolveClaim = async () => {
    if (!selectedClaim) return;
    if (!isValidAdminReason(claimNotes)) {
      setClaimError(
        "Las notas de resolución son obligatorias y deben tener entre 5 y 500 caracteres válidos.",
      );
      return;
    }

    const amountNum = approvedAmount.trim()
      ? Number(approvedAmount)
      : undefined;
    if (
      claimDecision !== "rechazado" &&
      amountNum !== undefined &&
      (isNaN(amountNum) || amountNum < 0)
    ) {
      setClaimError("El monto aprobado debe ser un número positivo válido.");
      return;
    }

    const input: AdminDamageClaimResolutionInput = {
      decision: claimDecision,
      resolution_notes: claimNotes.trim(),
      approved_amount_clp: amountNum,
    };

    const success = await resolveDamageClaim(selectedClaim.claim_id, input);
    if (success) {
      setSelectedClaim(null);
      setClaimNotes("");
    }
  };

  const handleConfirmModerateReview = async () => {
    if (!selectedReport) return;
    const success = await moderateReviewReport(
      selectedReport.report_id,
      reportAction,
      reportReasonCode,
    );
    if (success) {
      setSelectedReport(null);
    }
  };

  const handleReopenNotice = async (notice: AdminLocalNotice) => {
    await reopenNotice(notice.notice_id, "reintento_operativo");
  };

  return (
    <div className="space-y-6">
      <AdminFeedbackBanners />

      {/* Encabezado */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h2 className="text-2xl font-bold tracking-tight text-foreground">
            Disputas, Reclamos y Moderación Operativa
          </h2>
          <p className="text-sm text-muted-foreground mt-0.5">
            Gestión de incidentes entre arrendatarios y anfitriones, retención
            de garantías y reportes.
          </p>
        </div>

        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => {
            void loadDisputes();
            void loadDamageClaims();
            void loadReviewReports();
            void loadNotices();
          }}
          disabled={loading}
          className="gap-1.5"
        >
          <RefreshCw
            className={`size-4 ${loading ? "animate-spin" : ""}`}
            aria-hidden="true"
          />
          <span>Actualizar</span>
        </Button>
      </div>

      {/* Subsecciones accesibles */}
      <div className="flex flex-wrap items-center gap-2 border-b border-border pb-2">
        <Button
          type="button"
          variant={activeSection === "disputes" ? "secondary" : "ghost"}
          size="sm"
          onClick={() => setActiveSection("disputes")}
          className="gap-1.5"
        >
          <Scale className="size-4" aria-hidden="true" />
          <span>Disputas ({disputes.length})</span>
        </Button>
        <Button
          type="button"
          variant={activeSection === "damage" ? "secondary" : "ghost"}
          size="sm"
          onClick={() => setActiveSection("damage")}
          className="gap-1.5"
        >
          <Coins className="size-4" aria-hidden="true" />
          <span>Reclamos por Daños ({damageClaims.length})</span>
        </Button>
        <Button
          type="button"
          variant={activeSection === "reviews" ? "secondary" : "ghost"}
          size="sm"
          onClick={() => setActiveSection("reviews")}
          className="gap-1.5"
        >
          <Flag className="size-4" aria-hidden="true" />
          <span>Reportes de Reseñas ({reviewReports.length})</span>
        </Button>
        <Button
          type="button"
          variant={activeSection === "notices" ? "secondary" : "ghost"}
          size="sm"
          onClick={() => setActiveSection("notices")}
          className="gap-1.5"
        >
          <Bell className="size-4" aria-hidden="true" />
          <span>Avisos del Sistema ({notices.length})</span>
        </Button>
      </div>

      {/* Sección 1: Disputas de Arriendo */}
      {activeSection === "disputes" && (
        <div className="space-y-4">
          {disputes.length === 0 ? (
            <Card>
              <CardContent className="py-12 text-center text-muted-foreground text-sm">
                <Scale
                  className="size-10 mx-auto text-muted-foreground/60 mb-2"
                  aria-hidden="true"
                />
                <p className="font-medium">No hay disputas registradas.</p>
              </CardContent>
            </Card>
          ) : (
            <div className="grid grid-cols-1 gap-4">
              {disputes.map((dispute) => (
                <Card key={dispute.dispute_id} className="shadow-sm">
                  <CardHeader className="pb-3 bg-muted/20 border-b border-border/40">
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <div className="space-y-0.5">
                        <span className="text-xs font-mono text-muted-foreground block">
                          Disputa ID: {dispute.dispute_id.slice(0, 8)}...
                        </span>
                        <CardTitle className="text-sm font-semibold">
                          Reserva #{dispute.reservation_id.slice(0, 8)}
                        </CardTitle>
                      </div>
                      <div>{renderDisputeBadge(dispute.state)}</div>
                    </div>
                  </CardHeader>
                  <CardContent className="pt-4 space-y-3">
                    <p className="text-sm font-medium">
                      Motivo:{" "}
                      <span className="text-muted-foreground font-normal">
                        {dispute.reason}
                      </span>
                    </p>

                    <div className="grid grid-cols-1 sm:grid-cols-3 gap-2 text-xs text-muted-foreground">
                      <div>
                        <span className="block font-medium">Arrendatario</span>
                        <span className="font-mono">
                          {dispute.renter_id.slice(0, 8)}...
                        </span>
                      </div>
                      <div>
                        <span className="block font-medium">Anfitrión</span>
                        <span className="font-mono">
                          {dispute.host_id.slice(0, 8)}...
                        </span>
                      </div>
                      <div>
                        <span className="block font-medium">
                          Fecha Creación
                        </span>
                        <span>
                          {new Date(dispute.created_at).toLocaleString("es-CL")}
                        </span>
                      </div>
                    </div>

                    {dispute.close_reason && (
                      <div className="rounded-md bg-muted/50 p-2 text-xs">
                        <span className="font-medium">Motivo de cierre: </span>
                        <span>{dispute.close_reason}</span>
                        {dispute.closed_at && (
                          <span className="text-muted-foreground ml-2">
                            (
                            {new Date(dispute.closed_at).toLocaleString(
                              "es-CL",
                            )}
                            )
                          </span>
                        )}
                      </div>
                    )}

                    {dispute.state !== "cerrada" && (
                      <div className="flex justify-end pt-2 border-t border-border/40">
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          onClick={() => handleOpenCloseDispute(dispute)}
                          className="text-destructive hover:bg-destructive/10 border-destructive/30 text-xs gap-1.5"
                        >
                          <XCircle className="size-3.5" aria-hidden="true" />
                          <span>Cerrar Disputa (Irreversible)</span>
                        </Button>
                      </div>
                    )}
                  </CardContent>
                </Card>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Sección 2: Reclamos por Daños / Garantía */}
      {activeSection === "damage" && (
        <div className="space-y-4">
          {damageClaims.length === 0 ? (
            <Card>
              <CardContent className="py-12 text-center text-muted-foreground text-sm">
                <Coins
                  className="size-10 mx-auto text-muted-foreground/60 mb-2"
                  aria-hidden="true"
                />
                <p className="font-medium">
                  No hay reclamos por daños registrados.
                </p>
              </CardContent>
            </Card>
          ) : (
            <div className="grid grid-cols-1 gap-4">
              {damageClaims.map((claim) => (
                <Card key={claim.claim_id} className="shadow-sm">
                  <CardHeader className="pb-3 bg-muted/20 border-b border-border/40">
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <div className="space-y-0.5">
                        <span className="text-xs font-mono text-muted-foreground block">
                          Reclamo #{claim.claim_id.slice(0, 8)}...
                        </span>
                        <CardTitle className="text-sm font-semibold">
                          Reserva #{claim.reservation_id.slice(0, 8)}
                        </CardTitle>
                      </div>
                      <div>{renderDamageClaimBadge(claim.state)}</div>
                    </div>
                  </CardHeader>
                  <CardContent className="pt-4 space-y-3">
                    <p className="text-sm text-foreground">
                      {claim.description}
                    </p>

                    {claim.resolution && (
                      <div className="rounded-md bg-muted/50 p-2.5 text-xs space-y-1">
                        <div className="font-medium flex items-center justify-between">
                          <span>
                            Resolución:{" "}
                            {claim.resolution.decision.toUpperCase()}
                          </span>
                          <span className="text-muted-foreground">
                            {new Date(claim.resolution.at).toLocaleString(
                              "es-CL",
                            )}
                          </span>
                        </div>
                        <p className="text-muted-foreground">
                          {claim.resolution.reason}
                        </p>
                      </div>
                    )}

                    {(claim.state === "abierto" ||
                      claim.state === "en_evaluacion") && (
                      <div className="flex justify-end pt-2 border-t border-border/40">
                        <Button
                          type="button"
                          variant="default"
                          size="sm"
                          onClick={() => handleOpenResolveClaim(claim)}
                          className="text-xs gap-1.5 bg-primary"
                        >
                          <CheckCircle2
                            className="size-3.5"
                            aria-hidden="true"
                          />
                          <span>Resolver Reclamo de Garantía</span>
                        </Button>
                      </div>
                    )}
                  </CardContent>
                </Card>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Sección 3: Reportes de Reseñas */}
      {activeSection === "reviews" && (
        <div className="space-y-4">
          {reviewReports.length === 0 ? (
            <Card>
              <CardContent className="py-12 text-center text-muted-foreground text-sm">
                <Flag
                  className="size-10 mx-auto text-muted-foreground/60 mb-2"
                  aria-hidden="true"
                />
                <p className="font-medium">
                  No hay denuncias de reseñas registradas.
                </p>
              </CardContent>
            </Card>
          ) : (
            <div className="grid grid-cols-1 gap-4">
              {reviewReports.map((report) => (
                <Card key={report.report_id} className="shadow-sm">
                  <CardHeader className="pb-3 bg-muted/20 border-b border-border/40">
                    <div className="flex items-center justify-between">
                      <span className="text-xs font-mono text-muted-foreground">
                        Reporte #{report.report_id.slice(0, 8)}
                      </span>
                      <Badge
                        variant={
                          report.state === "pendiente" ? "secondary" : "outline"
                        }
                      >
                        {report.state}
                      </Badge>
                    </div>
                  </CardHeader>
                  <CardContent className="pt-4 space-y-3">
                    <p className="text-sm">
                      <span className="font-medium">Motivo denunciado: </span>
                      <span className="text-muted-foreground">
                        {report.reason}
                      </span>
                    </p>
                    {report.state === "pendiente" && (
                      <div className="flex justify-end gap-2 pt-2 border-t border-border/40">
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          onClick={() => {
                            setSelectedReport(report);
                            setReportAction("dismiss");
                          }}
                          className="text-xs"
                        >
                          Desestimar
                        </Button>
                        <Button
                          type="button"
                          variant="destructive"
                          size="sm"
                          onClick={() => {
                            setSelectedReport(report);
                            setReportAction("hide");
                          }}
                          className="text-xs gap-1"
                        >
                          <EyeOff className="size-3.5" aria-hidden="true" />
                          <span>Ocultar Reseña</span>
                        </Button>
                      </div>
                    )}
                  </CardContent>
                </Card>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Sección 4: Avisos Locales */}
      {activeSection === "notices" && (
        <div className="space-y-4">
          {notices.length === 0 ? (
            <Card>
              <CardContent className="py-12 text-center text-muted-foreground text-sm">
                <Bell
                  className="size-10 mx-auto text-muted-foreground/60 mb-2"
                  aria-hidden="true"
                />
                <p className="font-medium">
                  No hay avisos de notificación registrados.
                </p>
              </CardContent>
            </Card>
          ) : (
            <div className="grid grid-cols-1 gap-4">
              {notices.map((notice) => (
                <Card key={notice.notice_id} className="shadow-sm">
                  <CardContent className="pt-4 flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 text-sm">
                    <div>
                      <div className="font-medium text-foreground">
                        {notice.event_type}
                      </div>
                      <div className="text-xs text-muted-foreground mt-0.5">
                        Destinatario: {notice.recipient_email} • Estado
                        terminal: {notice.terminal_status}
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <Badge variant="outline" className="font-mono text-xs">
                        Reaperturas: {notice.reopen_count ?? 0}
                      </Badge>
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        onClick={() => handleReopenNotice(notice)}
                        disabled={actionLoading}
                        className="text-xs gap-1"
                      >
                        <RotateCcw className="size-3.5" aria-hidden="true" />
                        <span>Reabrir</span>
                      </Button>
                    </div>
                  </CardContent>
                </Card>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Modal Cierre Irreversible de Disputa */}
      <Dialog
        open={selectedDispute !== null}
        onOpenChange={(open) => !open && setSelectedDispute(null)}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <AlertTriangle
                className="size-5 text-destructive"
                aria-hidden="true"
              />
              <span>Cierre Irreversible de Disputa</span>
            </DialogTitle>
            <DialogDescription>
              Esta acción concluye el arbitraje de la disputa y no podrá ser
              reabierta sin intervención de soporte técnico de nivel 3.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <Label htmlFor="dispute-close-reason">
                Código de motivo de cierre
              </Label>
              <select
                id="dispute-close-reason"
                value={closeReasonCode}
                onChange={(e) =>
                  setCloseReasonCode(
                    e.target.value as
                      "ensayo_finalizado" | "registro_erroneo" | "duplicada",
                  )
                }
                className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
              >
                <option value="ensayo_finalizado">
                  Ensayo finalizado satisfactoriamente
                </option>
                <option value="registro_erroneo">
                  Registro erróneo o infundado
                </option>
                <option value="duplicada">Disputa duplicada</option>
              </select>
            </div>
          </div>

          <DialogFooter className="gap-2 sm:gap-0">
            <Button
              type="button"
              variant="outline"
              onClick={() => setSelectedDispute(null)}
              disabled={actionLoading}
            >
              Cancelar
            </Button>
            <Button
              type="button"
              variant="destructive"
              onClick={handleConfirmCloseDispute}
              disabled={actionLoading}
            >
              {actionLoading ? (
                <Loader2
                  className="size-4 animate-spin mr-1.5"
                  aria-hidden="true"
                />
              ) : null}
              Confirmar Cierre Irreversible
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Modal Resolución Reclamo por Daños */}
      <Dialog
        open={selectedClaim !== null}
        onOpenChange={(open) => !open && setSelectedClaim(null)}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Coins className="size-5 text-primary" aria-hidden="true" />
              <span>Resolución de Garantía por Daños</span>
            </DialogTitle>
            <DialogDescription>
              Dictamina el resultado del reclamo de retención o liberación de
              fondos de garantía.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <Label htmlFor="claim-decision">Decisión</Label>
              <select
                id="claim-decision"
                value={claimDecision}
                onChange={(e) =>
                  setClaimDecision(
                    e.target.value as "aceptado" | "rechazado" | "parcial",
                  )
                }
                className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
              >
                <option value="aceptado">Aceptado (Retención Total)</option>
                <option value="parcial">
                  Aceptado Parcial (Monto Parcial)
                </option>
                <option value="rechazado">
                  Rechazado (Liberación a Arrendatario)
                </option>
              </select>
            </div>

            {claimDecision !== "rechazado" && (
              <div className="space-y-2">
                <Label htmlFor="approved-amount">
                  Monto Aprobado (CLP opcional)
                </Label>
                <Input
                  id="approved-amount"
                  type="number"
                  min="0"
                  value={approvedAmount}
                  onChange={(e) => setApprovedAmount(e.target.value)}
                  placeholder="Ej. 45000"
                />
              </div>
            )}

            <div className="space-y-2">
              <Label htmlFor="claim-notes">
                Notas de resolución (obligatorio, 5+ caracteres)
              </Label>
              <Input
                id="claim-notes"
                value={claimNotes}
                onChange={(e) => {
                  setClaimNotes(e.target.value);
                  if (claimError) setClaimError(null);
                }}
                placeholder="Ej. Daño verificado según fotografías de entrega..."
                aria-required="true"
                aria-invalid={claimError !== null}
                className={claimError ? "border-destructive" : ""}
              />
              {claimError && (
                <p className="text-xs text-destructive flex items-center gap-1 mt-1">
                  <AlertCircle className="size-3.5" aria-hidden="true" />
                  {claimError}
                </p>
              )}
            </div>
          </div>

          <DialogFooter className="gap-2 sm:gap-0">
            <Button
              type="button"
              variant="outline"
              onClick={() => setSelectedClaim(null)}
              disabled={actionLoading}
            >
              Cancelar
            </Button>
            <Button
              type="button"
              onClick={handleConfirmResolveClaim}
              disabled={actionLoading}
            >
              {actionLoading ? (
                <Loader2
                  className="size-4 animate-spin mr-1.5"
                  aria-hidden="true"
                />
              ) : null}
              Registrar Resolución
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Modal Moderación Reseña */}
      <Dialog
        open={selectedReport !== null}
        onOpenChange={(open) => !open && setSelectedReport(null)}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>Moderar Reporte de Reseña</DialogTitle>
            <DialogDescription>
              Selecciona el motivo administrativo para{" "}
              {reportAction === "dismiss" ? "desestimar" : "ocultar"} el
              reporte.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <Label htmlFor="report-code">Motivo administrativo</Label>
              <select
                id="report-code"
                value={reportReasonCode}
                onChange={(e) =>
                  setReportReasonCode(
                    e.target.value as
                      | "sin_infraccion"
                      | "contenido_inadecuado"
                      | "duplicado"
                      | "error_registro",
                  )
                }
                className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
              >
                <option value="sin_infraccion">
                  Sin infracción comprobable
                </option>
                <option value="contenido_inadecuado">
                  Contenido inadecuado o difamatorio
                </option>
                <option value="duplicado">Reporte duplicado</option>
                <option value="error_registro">Error de registro</option>
              </select>
            </div>
          </div>

          <DialogFooter className="gap-2 sm:gap-0">
            <Button
              type="button"
              variant="outline"
              onClick={() => setSelectedReport(null)}
              disabled={actionLoading}
            >
              Cancelar
            </Button>
            <Button
              type="button"
              variant={reportAction === "hide" ? "destructive" : "default"}
              onClick={handleConfirmModerateReview}
              disabled={actionLoading}
            >
              {actionLoading ? (
                <Loader2
                  className="size-4 animate-spin mr-1.5"
                  aria-hidden="true"
                />
              ) : null}
              Confirmar Acción
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
