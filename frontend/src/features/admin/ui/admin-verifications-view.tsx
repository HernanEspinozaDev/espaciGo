"use client";

import { useEffect, useState } from "react";
import {
  AlertCircle,
  CheckCircle2,
  Clock,
  FileCheck2,
  Loader2,
  RefreshCw,
  RotateCcw,
  UserCheck,
  XCircle,
} from "lucide-react";
import { Card, CardContent, CardHeader } from "@/components/workspace/ui/card";
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
import type { AdminVerificationCase } from "../domain/types";
import { AdminFeedbackBanners } from "./admin-feedback-banners";

function renderStateBadge(state: AdminVerificationCase["state"]) {
  switch (state) {
    case "aprobada":
      return (
        <Badge
          variant="default"
          className="gap-1 bg-emerald-700 hover:bg-emerald-700 text-white font-medium"
        >
          <CheckCircle2 className="size-3.5" aria-hidden="true" />
          Aprobada
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
    case "rechazada":
      return (
        <Badge
          variant="destructive"
          className="gap-1 bg-red-100 text-red-800 dark:bg-red-950/50 dark:text-red-200 font-medium"
        >
          <XCircle className="size-3.5" aria-hidden="true" />
          Rechazada
        </Badge>
      );
    case "revocada":
      return (
        <Badge variant="outline" className="gap-1 text-muted-foreground">
          <RotateCcw className="size-3.5" aria-hidden="true" />
          Revocada
        </Badge>
      );
    default:
      return <Badge variant="outline">{state}</Badge>;
  }
}

export function AdminVerificationsView() {
  const {
    snapshot,
    loadVerifications,
    loadRejectedVerifications,
    reviewVerification,
    revokeVerification,
  } = useAdmin();

  const [selectedCase, setSelectedCase] =
    useState<AdminVerificationCase | null>(null);
  const [reviewDecision, setReviewDecision] = useState<"aprobar" | "rechazar">(
    "aprobar",
  );
  const [reviewReason, setReviewReason] = useState("");
  const [reasonError, setReasonError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<"pending" | "all">("pending");

  // Revocation modal state
  const [caseToRevoke, setCaseToRevoke] =
    useState<AdminVerificationCase | null>(null);
  const [revokeReasonCode, setRevokeReasonCode] = useState<
    "aprobacion_fixture_incorrecta" | "revision_fixture_actualizada"
  >("aprobacion_fixture_incorrecta");

  useEffect(() => {
    void loadVerifications();
    void loadRejectedVerifications();
  }, [loadVerifications, loadRejectedVerifications]);

  const { verifications, rejectedVerifications, loading, actionLoading } =
    snapshot;

  // Combine or filter items
  const allCasesMap = new Map<string, AdminVerificationCase>();
  verifications.forEach((c) => allCasesMap.set(c.id, c));
  rejectedVerifications.forEach((c) => allCasesMap.set(c.id, c));
  const allCases = Array.from(allCasesMap.values());

  const displayedCases =
    activeTab === "pending"
      ? allCases.filter((c) => c.state === "en_revision")
      : allCases;

  const handleOpenReview = (
    item: AdminVerificationCase,
    decision: "aprobar" | "rechazar",
  ) => {
    setSelectedCase(item);
    setReviewDecision(decision);
    setReviewReason("");
    setReasonError(null);
  };

  const handleSubmitReview = async () => {
    if (!selectedCase) return;

    if (reviewDecision === "rechazar") {
      if (!isValidAdminReason(reviewReason)) {
        setReasonError(
          "El motivo de rechazo es obligatorio y debe tener entre 5 y 500 caracteres sin caracteres de control.",
        );
        return;
      }
    } else if (
      reviewReason.trim().length > 0 &&
      !isValidAdminReason(reviewReason)
    ) {
      setReasonError(
        "Si ingresas un motivo o notas, debe tener entre 5 y 500 caracteres válidos.",
      );
      return;
    }

    const success = await reviewVerification(
      selectedCase.id,
      reviewDecision,
      reviewReason.trim() ? reviewReason.trim() : undefined,
    );
    if (success) {
      setSelectedCase(null);
      setReviewReason("");
    }
  };

  const handleConfirmRevoke = async () => {
    if (!caseToRevoke) return;
    const success = await revokeVerification(caseToRevoke.id, revokeReasonCode);
    if (success) {
      setCaseToRevoke(null);
    }
  };

  return (
    <div className="space-y-6">
      <AdminFeedbackBanners />

      {/* Encabezado */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h2 className="text-2xl font-bold tracking-tight text-foreground">
            Moderación de Verificaciones KYC / KYB
          </h2>
          <p className="text-sm text-muted-foreground mt-0.5">
            Evaluación de identidad sintética, acreditación documental y
            resolución de solicitudes.
          </p>
        </div>

        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => {
            void loadVerifications();
            void loadRejectedVerifications();
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

      {/* Filtros de pestaña */}
      <div className="flex items-center gap-2 border-b border-border pb-2">
        <Button
          type="button"
          variant={activeTab === "pending" ? "secondary" : "ghost"}
          size="sm"
          onClick={() => setActiveTab("pending")}
          className="text-sm font-medium"
        >
          En Revisión (
          {allCases.filter((c) => c.state === "en_revision").length})
        </Button>
        <Button
          type="button"
          variant={activeTab === "all" ? "secondary" : "ghost"}
          size="sm"
          onClick={() => setActiveTab("all")}
          className="text-sm font-medium"
        >
          Todos los Casos ({allCases.length})
        </Button>
      </div>

      {/* Lista de Casos */}
      {displayedCases.length === 0 ? (
        <Card>
          <CardContent className="py-12 text-center text-muted-foreground text-sm space-y-2">
            <UserCheck
              className="size-10 mx-auto text-muted-foreground/60"
              aria-hidden="true"
            />
            <p className="font-medium">No hay solicitudes en esta vista.</p>
            <p className="text-xs">
              {activeTab === "pending"
                ? "No hay solicitudes KYC/KYB pendientes de moderación en este momento."
                : "No se encontraron registros de verificación cargados en el sistema."}
            </p>
          </CardContent>
        </Card>
      ) : (
        <div className="grid grid-cols-1 gap-4">
          {displayedCases.map((item) => (
            <Card
              key={item.id}
              className="overflow-hidden shadow-sm hover:shadow transition-shadow"
            >
              <CardHeader className="pb-3 bg-muted/20 border-b border-border/40">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div className="flex items-center gap-2.5">
                    <Badge
                      variant="outline"
                      className="uppercase font-mono text-xs font-bold"
                    >
                      {item.type}
                    </Badge>
                    <span className="text-sm font-semibold font-mono text-foreground">
                      ID: {item.id.slice(0, 8)}...
                    </span>
                  </div>
                  <div>{renderStateBadge(item.state)}</div>
                </div>
              </CardHeader>
              <CardContent className="pt-4 space-y-4">
                <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 gap-3 text-sm">
                  <div>
                    <span className="text-xs text-muted-foreground block">
                      Solicitante
                    </span>
                    <span
                      className="font-mono text-xs truncate block"
                      title={item.applicant_id}
                    >
                      {item.applicant_id}
                    </span>
                  </div>
                  <div>
                    <span className="text-xs text-muted-foreground block">
                      Fecha Solicitud
                    </span>
                    <span className="text-xs">
                      {new Date(item.created_at).toLocaleString("es-CL")}
                    </span>
                  </div>
                  <div>
                    <span className="text-xs text-muted-foreground block">
                      Última Actualización
                    </span>
                    <span className="text-xs">
                      {new Date(item.updated_at).toLocaleString("es-CL")}
                    </span>
                  </div>
                  <div>
                    <span className="text-xs text-muted-foreground block">
                      Código Corrección
                    </span>
                    <span className="text-xs font-mono">
                      {item.correction_code ?? "Ninguno"}
                    </span>
                  </div>
                </div>

                {/* Evidencia sintética visualizada */}
                <div className="rounded-md bg-muted/40 p-3 text-xs space-y-1 border border-border/50">
                  <div className="flex items-center gap-1.5 font-medium text-foreground">
                    <FileCheck2
                      className="size-3.5 text-primary"
                      aria-hidden="true"
                    />
                    <span>Evidencia Sintética Asociada</span>
                  </div>
                  <p className="text-muted-foreground">
                    Documento de identidad simulado y cotejo biométrico
                    sintético registrado para el usuario{" "}
                    {item.applicant_id.slice(0, 8)}.
                  </p>
                  {item.notes && (
                    <p className="mt-1 text-foreground font-medium">
                      Notas previas:{" "}
                      <span className="italic text-muted-foreground">
                        {item.notes}
                      </span>
                    </p>
                  )}
                </div>

                {/* Botones de acción */}
                <div className="flex flex-wrap items-center justify-end gap-2 pt-2 border-t border-border/40">
                  {item.state === "en_revision" && (
                    <>
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        onClick={() => handleOpenReview(item, "rechazar")}
                        className="text-destructive hover:bg-destructive/10 border-destructive/30 gap-1.5"
                      >
                        <XCircle className="size-4" aria-hidden="true" />
                        <span>Rechazar</span>
                      </Button>
                      <Button
                        type="button"
                        variant="default"
                        size="sm"
                        onClick={() => handleOpenReview(item, "aprobar")}
                        className="bg-emerald-700 hover:bg-emerald-800 text-white gap-1.5"
                      >
                        <CheckCircle2 className="size-4" aria-hidden="true" />
                        <span>Aprobar</span>
                      </Button>
                    </>
                  )}

                  {item.state === "aprobada" && (
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => setCaseToRevoke(item)}
                      className="text-muted-foreground hover:text-foreground gap-1.5 text-xs"
                    >
                      <RotateCcw className="size-3.5" aria-hidden="true" />
                      <span>Revocar Aprobación</span>
                    </Button>
                  )}
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {/* Modal accesible de Resolución (Aprobar o Rechazar) */}
      <Dialog
        open={selectedCase !== null}
        onOpenChange={(open) => !open && setSelectedCase(null)}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              {reviewDecision === "aprobar" ? (
                <>
                  <CheckCircle2
                    className="size-5 text-emerald-600"
                    aria-hidden="true"
                  />
                  <span>Aprobar Verificación</span>
                </>
              ) : (
                <>
                  <AlertCircle
                    className="size-5 text-destructive"
                    aria-hidden="true"
                  />
                  <span>Rechazar Verificación</span>
                </>
              )}
            </DialogTitle>
            <DialogDescription>
              {reviewDecision === "aprobar"
                ? "Confirma la aprobación de la solicitud de identidad del usuario."
                : "Para rechazar la solicitud, debes especificar obligatoriamente un motivo claro (mínimo 5 caracteres)."}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <Label htmlFor="review-reason">
                {reviewDecision === "rechazar"
                  ? "Motivo del rechazo (obligatorio)"
                  : "Notas adicionales (opcional)"}
              </Label>
              <Input
                id="review-reason"
                value={reviewReason}
                onChange={(e) => {
                  setReviewReason(e.target.value);
                  if (reasonError) setReasonError(null);
                }}
                placeholder={
                  reviewDecision === "rechazar"
                    ? "Ej. Documento borroso o ilegible..."
                    : "Ej. Cumple validaciones sintéticas..."
                }
                aria-required={reviewDecision === "rechazar"}
                aria-invalid={reasonError !== null}
                className={reasonError ? "border-destructive" : ""}
              />
              {reasonError && (
                <p className="text-xs text-destructive flex items-center gap-1 mt-1">
                  <AlertCircle className="size-3.5" aria-hidden="true" />
                  {reasonError}
                </p>
              )}
            </div>
          </div>

          <DialogFooter className="gap-2 sm:gap-0">
            <Button
              type="button"
              variant="outline"
              onClick={() => setSelectedCase(null)}
              disabled={actionLoading}
            >
              Cancelar
            </Button>
            <Button
              type="button"
              variant={
                reviewDecision === "rechazar" ? "destructive" : "default"
              }
              onClick={handleSubmitReview}
              disabled={actionLoading}
              className={
                reviewDecision === "aprobar"
                  ? "bg-emerald-700 hover:bg-emerald-800 text-white"
                  : ""
              }
            >
              {actionLoading ? (
                <Loader2
                  className="size-4 animate-spin mr-1.5"
                  aria-hidden="true"
                />
              ) : null}
              Confirmar{" "}
              {reviewDecision === "aprobar" ? "Aprobación" : "Rechazo"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Modal accesible de Revocación */}
      <Dialog
        open={caseToRevoke !== null}
        onOpenChange={(open) => !open && setCaseToRevoke(null)}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <RotateCcw className="size-5 text-amber-600" aria-hidden="true" />
              <span>Revocar Verificación Aprobada</span>
            </DialogTitle>
            <DialogDescription>
              Esta acción revoca la certificación vigente del caso y exige un
              motivo operativo registrado.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <Label htmlFor="revoke-code">
                Código de motivo de revocación
              </Label>
              <select
                id="revoke-code"
                value={revokeReasonCode}
                onChange={(e) =>
                  setRevokeReasonCode(
                    e.target.value as
                      | "aprobacion_fixture_incorrecta"
                      | "revision_fixture_actualizada",
                  )
                }
                className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
              >
                <option value="aprobacion_fixture_incorrecta">
                  Aprobación de fixture incorrecta
                </option>
                <option value="revision_fixture_actualizada">
                  Revisión por fixture actualizada
                </option>
              </select>
            </div>
          </div>

          <DialogFooter className="gap-2 sm:gap-0">
            <Button
              type="button"
              variant="outline"
              onClick={() => setCaseToRevoke(null)}
              disabled={actionLoading}
            >
              Cancelar
            </Button>
            <Button
              type="button"
              variant="destructive"
              onClick={handleConfirmRevoke}
              disabled={actionLoading}
            >
              {actionLoading ? (
                <Loader2
                  className="size-4 animate-spin mr-1.5"
                  aria-hidden="true"
                />
              ) : null}
              Confirmar Revocación
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
