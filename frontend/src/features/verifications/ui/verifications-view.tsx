"use client";

import { useEffect, useState } from "react";
import Image from "next/image";
import {
  AlertCircle,
  CheckCircle2,
  Clock,
  Download,
  Eye,
  FileCheck2,
  FlaskConical,
  History,
  Loader2,
  RefreshCw,
  RotateCcw,
  ShieldCheck,
  Trash2,
  UploadCloud,
  XCircle,
} from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/workspace/ui/card";
import { Button } from "@/components/workspace/ui/button";
import { Badge } from "@/components/ui/badge";
import { useAuth } from "@/features/auth/ui/use-auth";
import { useVerifications } from "./use-verifications";
import type {
  VerificationCorrectionCode,
  VerificationState,
  VerificationType,
} from "../domain/verification";
import { privateBlobUrls } from "@/lib/api/private-blob-urls";

function renderStateBadge(state: VerificationState) {
  switch (state) {
    case "aprobada":
      return (
        <Badge
          variant="default"
          className="gap-1 bg-emerald-800 hover:bg-emerald-800 text-white font-medium"
        >
          <CheckCircle2 className="size-3.5" />
          Aprobada
        </Badge>
      );
    case "en_revision":
      return (
        <Badge
          variant="secondary"
          className="gap-1 bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-200"
        >
          <Clock className="size-3.5" />
          En revisión
        </Badge>
      );
    case "rechazada":
      return (
        <Badge variant="destructive" className="gap-1">
          <XCircle className="size-3.5" />
          Rechazada
        </Badge>
      );
    case "revocada":
      return (
        <Badge variant="outline" className="gap-1 text-muted-foreground">
          <RotateCcw className="size-3.5" />
          Revocada
        </Badge>
      );
    default:
      return <Badge variant="outline">{state}</Badge>;
  }
}

export function VerificationsView() {
  const { accessToken } = useAuth();
  const {
    snapshot,
    loadOverview,
    loadCaseDetails,
    startVerification,
    retryVerification,
    uploadEvidence,
    deleteEvidence,
    getEvidenceContent,
  } = useVerifications();

  const [selectedCaseId, setSelectedCaseId] = useState<string | null>(null);
  const [loadingAction, setLoadingAction] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [previewBlobUrl, setPreviewBlobUrl] = useState<string | null>(null);
  const [previewEvidenceId, setPreviewEvidenceId] = useState<string | null>(
    null,
  );

  useEffect(() => {
    if (accessToken) {
      loadOverview(accessToken).catch(() => {
        // Ignored
      });
    }
  }, [accessToken, loadOverview]);

  useEffect(() => {
    return () => {
      if (previewBlobUrl) {
        privateBlobUrls.revoke(previewBlobUrl);
      }
    };
  }, [previewBlobUrl]);

  const handleSelectCase = async (id: string) => {
    if (!accessToken) return;
    setSelectedCaseId(id);
    setError(null);
    setMessage(null);
    if (previewBlobUrl) {
      privateBlobUrls.revoke(previewBlobUrl);
      setPreviewBlobUrl(null);
      setPreviewEvidenceId(null);
    }
    try {
      await loadCaseDetails(accessToken, id);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Error al cargar detalle del caso",
      );
    }
  };

  const handleStartVerification = async (type: VerificationType) => {
    if (!accessToken) return;
    setError(null);
    setMessage(null);
    setLoadingAction(`start-${type}`);
    try {
      const key = crypto.randomUUID();
      const created = await startVerification(accessToken, key, type);
      setMessage(`Verificación ${type.toUpperCase()} iniciada correctamente.`);
      setSelectedCaseId(created.id);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Error al iniciar verificación",
      );
    } finally {
      setLoadingAction(null);
    }
  };

  const handleRetry = async (id: string, code: VerificationCorrectionCode) => {
    if (!accessToken) return;
    setError(null);
    setMessage(null);
    setLoadingAction(`retry-${id}`);
    try {
      const key = crypto.randomUUID();
      await retryVerification(accessToken, id, key, code);
      setMessage("Caso reintentado correctamente con nueva evidencia.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Error al reintentar caso");
    } finally {
      setLoadingAction(null);
    }
  };

  const handleUploadEvidence = async (id: string) => {
    if (!accessToken) return;
    setError(null);
    setMessage(null);
    setLoadingAction(`upload-${id}`);
    try {
      await uploadEvidence(accessToken, id);
      setMessage(
        "Evidencia sintética ('synthetic-png-v1') cargada exitosamente.",
      );
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Error al cargar evidencia",
      );
    } finally {
      setLoadingAction(null);
    }
  };

  const handleDeleteEvidence = async (id: string, evidenceId: string) => {
    if (!accessToken) return;
    setError(null);
    setMessage(null);
    setLoadingAction(`delete-ev-${evidenceId}`);
    if (previewEvidenceId === evidenceId && previewBlobUrl) {
      privateBlobUrls.revoke(previewBlobUrl);
      setPreviewBlobUrl(null);
      setPreviewEvidenceId(null);
    }
    try {
      await deleteEvidence(accessToken, id, evidenceId);
      setMessage("Evidencia eliminada.");
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Error al eliminar evidencia",
      );
    } finally {
      setLoadingAction(null);
    }
  };

  const handleViewEvidence = async (id: string, evidenceId: string) => {
    if (!accessToken) return;
    setError(null);
    setLoadingAction(`view-ev-${evidenceId}`);
    try {
      if (previewBlobUrl) {
        privateBlobUrls.revoke(previewBlobUrl);
        setPreviewBlobUrl(null);
        setPreviewEvidenceId(null);
      }
      const blob = await getEvidenceContent(accessToken, id, evidenceId);
      if (!blob) {
        setError("No se pudo descargar el contenido de la evidencia.");
        return;
      }
      const registered = privateBlobUrls.create(`evidence-${evidenceId}`, blob);
      setPreviewBlobUrl(registered.url);
      setPreviewEvidenceId(evidenceId);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Error al previsualizar evidencia",
      );
    } finally {
      setLoadingAction(null);
    }
  };

  const handleDownloadEvidence = async (id: string, evidenceId: string) => {
    if (!accessToken) return;
    setError(null);
    setLoadingAction(`dl-ev-${evidenceId}`);
    try {
      const blob = await getEvidenceContent(accessToken, id, evidenceId);
      if (!blob) {
        setError("No se pudo descargar el archivo de evidencia.");
        return;
      }
      const registered = privateBlobUrls.create(`dl-${evidenceId}`, blob);
      const link = document.createElement("a");
      link.href = registered.url;
      link.download = `evidencia-${evidenceId}.png`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      setTimeout(() => registered.revoke(), 2000);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Error al descargar evidencia",
      );
    } finally {
      setLoadingAction(null);
    }
  };

  const eligibility = snapshot.eligibility;
  const verifications = snapshot.verifications;
  const currentCase = snapshot.currentCase;

  return (
    <div className="space-y-6">
      {/* Tarjeta de Elegibilidad KYC / KYB */}
      <Card aria-labelledby="eligibility-title">
        <CardHeader>
          <div className="flex items-center gap-2">
            <ShieldCheck className="size-5 text-primary" aria-hidden="true" />
            <CardTitle id="eligibility-title">
              Elegibilidad de Verificación (KYC / KYB)
            </CardTitle>
          </div>
          <CardDescription>
            Estado de cumplimiento normativo exigido para operar y publicar
            espacios.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div
            role="note"
            className="flex items-start gap-2 rounded-md bg-blue-50 p-3 text-xs text-blue-900 border border-blue-200 dark:bg-blue-950/30 dark:border-blue-800 dark:text-blue-200"
          >
            <FlaskConical className="size-4 shrink-0 mt-0.5 text-blue-600" />
            <div>
              <span className="font-semibold">
                Entorno de verificación sintética:
              </span>{" "}
              Las verificaciones y pruebas de evidencia aceptan el estándar
              sintético &quot;synthetic-png-v1&quot;. Los documentos son
              validados en un entorno seguro sin requerir cédulas ni escrituras
              reales.
            </div>
          </div>

          {eligibility.length > 0 ? (
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              {eligibility.map((item) => (
                <div
                  key={item.type}
                  className="rounded-lg border p-4 flex flex-col justify-between gap-3 bg-muted/20"
                >
                  <div className="flex items-start justify-between gap-2">
                    <div>
                      <h3 className="font-medium capitalize text-sm">
                        Verificación {item.type.toUpperCase()}
                      </h3>
                      <p className="text-xs text-muted-foreground mt-1">
                        {item.eligible
                          ? "Cumples las condiciones para iniciar o mantener esta verificación."
                          : item.revocation_reason_code ||
                            "No elegible temporalmente."}
                      </p>
                    </div>
                    <Badge
                      variant={item.eligible ? "default" : "outline"}
                      className={
                        item.eligible
                          ? "bg-emerald-800 text-white font-medium"
                          : ""
                      }
                    >
                      {item.eligible ? "Elegible" : "No elegible"}
                    </Badge>
                  </div>

                  {item.eligible && (
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      className="self-start"
                      onClick={() =>
                        handleStartVerification(item.type as VerificationType)
                      }
                      disabled={
                        Boolean(loadingAction) ||
                        !accessToken ||
                        verifications.some(
                          (v) =>
                            v.type === item.type && v.state === "en_revision",
                        )
                      }
                    >
                      {loadingAction === `start-${item.type}` ? (
                        <Loader2 className="mr-2 size-3.5 animate-spin" />
                      ) : (
                        <FileCheck2 className="mr-2 size-3.5" />
                      )}
                      Iniciar {item.type.toUpperCase()}
                    </Button>
                  )}
                </div>
              ))}
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">
              Cargando condiciones de elegibilidad...
            </p>
          )}
        </CardContent>
      </Card>

      {/* Alertas globales */}
      {error && (
        <div
          role="alert"
          className="flex items-center gap-2 rounded-md bg-destructive/15 p-3 text-sm text-destructive border border-destructive/20"
        >
          <AlertCircle className="size-4 shrink-0" />
          <span>{error}</span>
        </div>
      )}

      {message && (
        <div
          role="status"
          className="flex items-center gap-2 rounded-md bg-emerald-50 p-3 text-sm text-emerald-800 border border-emerald-200 dark:bg-emerald-950/30 dark:border-emerald-800 dark:text-emerald-300"
        >
          <CheckCircle2 className="size-4 shrink-0" />
          <span>{message}</span>
        </div>
      )}

      {/* Lista de casos de verificación */}
      <Card aria-labelledby="cases-title">
        <CardHeader>
          <div className="flex items-center justify-between gap-2">
            <div className="flex items-center gap-2">
              <History className="size-5 text-primary" aria-hidden="true" />
              <CardTitle id="cases-title">
                Historial de Verificaciones
              </CardTitle>
            </div>
            <Button
              type="button"
              size="sm"
              variant="ghost"
              onClick={() => accessToken && loadOverview(accessToken)}
              disabled={snapshot.loading || !accessToken}
              aria-label="Actualizar listado de verificaciones"
            >
              <RefreshCw
                className={`size-4 ${snapshot.loading ? "animate-spin" : ""}`}
              />
            </Button>
          </div>
          <CardDescription>
            Revisa el estado actual y el historial de cambios de cada proceso.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {verifications.length > 0 ? (
            <div className="divide-y rounded-md border text-sm">
              {verifications.map((v) => (
                <div
                  key={v.id}
                  className={`p-4 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 cursor-pointer transition-colors hover:bg-muted/50 ${
                    selectedCaseId === v.id ? "bg-muted/70 font-medium" : ""
                  }`}
                  onClick={() => handleSelectCase(v.id)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      handleSelectCase(v.id);
                    }
                  }}
                  role="button"
                  tabIndex={0}
                  aria-pressed={selectedCaseId === v.id}
                >
                  <div className="space-y-1">
                    <div className="flex items-center gap-2">
                      <span className="font-semibold uppercase text-xs tracking-wider">
                        {v.type}
                      </span>
                      <span className="text-xs text-muted-foreground font-mono">
                        ID: {v.id.slice(0, 8)}...
                      </span>
                    </div>
                    <p className="text-xs text-muted-foreground">
                      Creada el{" "}
                      {v.created_at
                        ? new Date(v.created_at).toLocaleDateString("es-CL")
                        : "--"}
                    </p>
                  </div>
                  <div className="flex items-center gap-3">
                    {renderStateBadge(v.state)}
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      onClick={(e) => {
                        e.stopPropagation();
                        handleSelectCase(v.id);
                      }}
                    >
                      Ver detalle
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">
              No tienes procesos de verificación registrados.
            </p>
          )}
        </CardContent>
      </Card>

      {/* Detalle del caso seleccionado */}
      {currentCase && selectedCaseId === currentCase.id && (
        <Card aria-labelledby="case-detail-title">
          <CardHeader>
            <div className="flex items-center justify-between gap-2">
              <div className="flex items-center gap-2">
                <FileCheck2
                  className="size-5 text-primary"
                  aria-hidden="true"
                />
                <CardTitle id="case-detail-title">
                  Detalle de Verificación ({currentCase.type.toUpperCase()})
                </CardTitle>
              </div>
              {renderStateBadge(currentCase.state)}
            </div>
            <CardDescription className="font-mono text-xs">
              Identificador de caso: {currentCase.id}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            {/* Acciones para casos rechazados */}
            {currentCase.state === "rechazada" && (
              <div className="rounded-lg border border-destructive/30 bg-destructive/10 p-4 space-y-3">
                <div className="flex items-center gap-2 text-destructive font-medium text-sm">
                  <AlertCircle className="size-4" />
                  <span>Caso rechazado — Puedes reintentarlo</span>
                </div>
                <p className="text-xs text-muted-foreground">
                  Envía una corrección compatible con el código de ensayo
                  (&apos;fixture_vigente_actualizado&apos;) para reiniciar la
                  revisión.
                </p>
                <Button
                  type="button"
                  variant="destructive"
                  size="sm"
                  onClick={() =>
                    handleRetry(currentCase.id, "fixture_vigente_actualizado")
                  }
                  disabled={
                    loadingAction === `retry-${currentCase.id}` || !accessToken
                  }
                >
                  {loadingAction === `retry-${currentCase.id}` ? (
                    <Loader2 className="mr-2 size-3.5 animate-spin" />
                  ) : (
                    <RotateCcw className="mr-2 size-3.5" />
                  )}
                  Reintentar verificación
                </Button>
              </div>
            )}

            {/* Sección de Evidencias */}
            <div className="space-y-3">
              <div className="flex items-center justify-between gap-2">
                <h3 className="font-medium text-sm">Evidencias del caso</h3>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  onClick={() => handleUploadEvidence(currentCase.id)}
                  disabled={
                    loadingAction === `upload-${currentCase.id}` || !accessToken
                  }
                >
                  {loadingAction === `upload-${currentCase.id}` ? (
                    <Loader2 className="mr-2 size-3.5 animate-spin" />
                  ) : (
                    <UploadCloud className="mr-2 size-3.5" />
                  )}
                  Subir evidencia sintética
                </Button>
              </div>

              {snapshot.evidence.length > 0 ? (
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  {snapshot.evidence.map((ev) => (
                    <div
                      key={ev.id}
                      className="rounded-lg border p-3 flex flex-col justify-between gap-2 bg-muted/20 text-xs"
                    >
                      <div className="space-y-1">
                        <div className="flex items-center justify-between gap-1">
                          <span className="font-mono font-medium">
                            {ev.fixture_code ?? "synthetic-png-v1"}
                          </span>
                          <Badge variant="outline" className="text-[10px]">
                            ID: {ev.id.slice(0, 6)}
                          </Badge>
                        </div>
                        <p className="text-muted-foreground text-[11px]">
                          Cargado:{" "}
                          {ev.created_at
                            ? new Date(ev.created_at).toLocaleDateString(
                                "es-CL",
                              )
                            : "--"}
                        </p>
                      </div>

                      <div className="flex items-center gap-2 pt-2 border-t">
                        <Button
                          type="button"
                          size="sm"
                          variant="ghost"
                          className="h-7 px-2 text-xs"
                          onClick={() =>
                            handleViewEvidence(currentCase.id, ev.id)
                          }
                          disabled={Boolean(loadingAction) || !accessToken}
                        >
                          <Eye className="mr-1 size-3" />
                          Ver
                        </Button>
                        <Button
                          type="button"
                          size="sm"
                          variant="ghost"
                          className="h-7 px-2 text-xs"
                          onClick={() =>
                            handleDownloadEvidence(currentCase.id, ev.id)
                          }
                          disabled={Boolean(loadingAction) || !accessToken}
                        >
                          <Download className="mr-1 size-3" />
                          Descargar
                        </Button>
                        <Button
                          type="button"
                          size="sm"
                          variant="ghost"
                          className="h-7 px-2 text-xs text-destructive hover:bg-destructive/10"
                          onClick={() =>
                            handleDeleteEvidence(currentCase.id, ev.id)
                          }
                          disabled={Boolean(loadingAction) || !accessToken}
                        >
                          <Trash2 className="size-3" />
                        </Button>
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <p className="text-xs text-muted-foreground italic">
                  No se han registrado evidencias para este caso.
                </p>
              )}

              {/* Visor de evidencia activa */}
              {previewBlobUrl && (
                <div className="rounded-lg border p-4 bg-muted/30 space-y-2">
                  <div className="flex items-center justify-between">
                    <span className="text-xs font-semibold">
                      Visor de evidencia sintética (Blob privado)
                    </span>
                    <Button
                      type="button"
                      size="sm"
                      variant="ghost"
                      className="h-6 px-2 text-xs"
                      onClick={() => {
                        if (previewBlobUrl) {
                          privateBlobUrls.revoke(previewBlobUrl);
                          setPreviewBlobUrl(null);
                          setPreviewEvidenceId(null);
                        }
                      }}
                    >
                      Cerrar visor
                    </Button>
                  </div>
                  <div className="relative h-64 w-full overflow-hidden rounded-md border bg-black/5 flex items-center justify-center">
                    <Image
                      src={previewBlobUrl}
                      alt="Evidencia sintética de verificación"
                      fill
                      unoptimized
                      className="object-contain"
                    />
                  </div>
                </div>
              )}
            </div>

            {/* Historial del caso */}
            <div className="space-y-3">
              <h3 className="font-medium text-sm">Historial de auditoría</h3>
              {snapshot.history.length > 0 ? (
                <div className="space-y-2">
                  {snapshot.history.map((h, i) => (
                    <div
                      key={h.sequence ?? i}
                      className="rounded-md border p-3 text-xs flex flex-col sm:flex-row items-start sm:items-center justify-between gap-2"
                    >
                      <div>
                        <span className="font-semibold uppercase">
                          {h.action ?? "Transición"}
                        </span>
                        <p className="text-muted-foreground mt-0.5">
                          {h.reason_code ?? "Sin código de motivo adjunto"}
                        </p>
                      </div>
                      <div className="text-right shrink-0">
                        <Badge variant="secondary">{h.to_state}</Badge>
                        {h.occurred_at && (
                          <p className="text-[10px] text-muted-foreground mt-1">
                            {new Date(h.occurred_at).toLocaleString("es-CL")}
                          </p>
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <p className="text-xs text-muted-foreground italic">
                  No hay eventos históricos registrados.
                </p>
              )}
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
