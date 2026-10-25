"use client";

import { useState } from "react";
import {
  AlertCircle,
  Archive,
  CheckCircle2,
  Download,
  FileJson,
  FileText,
  Loader2,
  Shield,
  Trash2,
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
import { useAccount } from "./use-account";
import type { RightsRequestKind } from "../domain/privacy";
import { privateBlobUrls } from "@/lib/api/private-blob-urls";

export function PrivacyCard() {
  const { accessToken } = useAuth();
  const {
    snapshot,
    requestDataRight,
    exportIdentityBlob,
    downloadIdentityArchive,
  } = useAccount();
  const [loadingAction, setLoadingAction] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const handleRequest = async (kind: RightsRequestKind) => {
    if (!accessToken) return;
    setError(null);
    setMessage(null);
    setLoadingAction(kind);
    try {
      await requestDataRight(accessToken, kind);
      setMessage(
        kind === "supresion"
          ? "Solicitud de supresión de datos registrada exitosamente."
          : "Solicitud de acceso a datos registrada exitosamente.",
      );
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Error al enviar la solicitud",
      );
    } finally {
      setLoadingAction(null);
    }
  };

  const downloadBlobInBrowser = (
    blob: Blob,
    filename: string,
    resourceKey: string,
  ) => {
    const registered = privateBlobUrls.create(resourceKey, blob);
    const link = document.createElement("a");
    link.href = registered.url;
    link.download = filename;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    setTimeout(() => {
      registered.revoke();
    }, 2000);
  };

  const handleExportJson = async () => {
    if (!accessToken) return;
    setError(null);
    setMessage(null);
    setLoadingAction("export-json");
    try {
      const blob = await exportIdentityBlob(accessToken);
      if (!blob) {
        setError("No se pudo obtener el archivo de exportación JSON.");
        return;
      }
      downloadBlobInBrowser(
        blob,
        "espacigo-identity-export.json",
        "export-json",
      );
      setMessage("Exportación JSON descargada correctamente.");
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Error al descargar la exportación JSON",
      );
    } finally {
      setLoadingAction(null);
    }
  };

  const handleDownloadArchive = async () => {
    if (!accessToken) return;
    setError(null);
    setMessage(null);
    setLoadingAction("export-archive");
    try {
      const blob = await downloadIdentityArchive(accessToken);
      if (!blob) {
        setError("No se pudo obtener el archivo comprimido ZIP.");
        return;
      }
      downloadBlobInBrowser(
        blob,
        "espacigo-identity-archive.zip",
        "export-zip",
      );
      setMessage("Archivo ZIP descargado correctamente.");
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Error al descargar el archivo ZIP",
      );
    } finally {
      setLoadingAction(null);
    }
  };

  const requests = snapshot.rightsRequests;

  return (
    <div className="space-y-6">
      <Card aria-labelledby="privacy-rights-title">
        <CardHeader>
          <div className="flex items-center gap-2">
            <Shield className="size-5 text-primary" aria-hidden="true" />
            <CardTitle id="privacy-rights-title">
              Derechos ARCO y Privacidad de Datos
            </CardTitle>
          </div>
          <CardDescription>
            Ejerce tus derechos legales de acceso o supresión de datos
            personales conforme a la normativa chilena (Ley 19.628).
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
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

          <div className="flex flex-wrap gap-3">
            <Button
              type="button"
              variant="outline"
              onClick={() => handleRequest("acceso")}
              disabled={Boolean(loadingAction) || !accessToken}
            >
              {loadingAction === "acceso" ? (
                <Loader2
                  className="mr-2 size-4 animate-spin"
                  aria-hidden="true"
                />
              ) : (
                <FileText className="mr-2 size-4" aria-hidden="true" />
              )}
              Solicitar derecho de acceso
            </Button>

            <Button
              type="button"
              variant="outline"
              className="border-destructive/40 text-destructive hover:bg-destructive/10"
              onClick={() => handleRequest("supresion")}
              disabled={Boolean(loadingAction) || !accessToken}
            >
              {loadingAction === "supresion" ? (
                <Loader2
                  className="mr-2 size-4 animate-spin"
                  aria-hidden="true"
                />
              ) : (
                <Trash2 className="mr-2 size-4" aria-hidden="true" />
              )}
              Solicitar derecho de supresión
            </Button>
          </div>

          {requests.length > 0 && (
            <div className="pt-4 space-y-2">
              <h3 className="text-sm font-medium">Solicitudes registradas</h3>
              <div className="divide-y rounded-md border text-sm">
                {requests.map((r, i) => (
                  <div
                    key={r.id ?? i}
                    className="flex items-center justify-between p-3"
                  >
                    <div>
                      <span className="font-medium capitalize">{r.kind}</span>
                      {r.created_at && (
                        <span className="text-xs text-muted-foreground ml-2">
                          {new Date(r.created_at).toLocaleDateString("es-CL")}
                        </span>
                      )}
                    </div>
                    <Badge variant="secondary">{r.state}</Badge>
                  </div>
                ))}
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      <Card aria-labelledby="privacy-export-title">
        <CardHeader>
          <div className="flex items-center gap-2">
            <Download className="size-5 text-primary" aria-hidden="true" />
            <CardTitle id="privacy-export-title">
              Exportación de Datos Personales
            </CardTitle>
          </div>
          <CardDescription>
            Descarga directa de tu copia de datos en formato JSON estructurado o
            archivo ZIP completo.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex flex-wrap gap-3">
            <Button
              type="button"
              variant="outline"
              onClick={handleExportJson}
              disabled={Boolean(loadingAction) || !accessToken}
            >
              {loadingAction === "export-json" ? (
                <Loader2
                  className="mr-2 size-4 animate-spin"
                  aria-hidden="true"
                />
              ) : (
                <FileJson className="mr-2 size-4" aria-hidden="true" />
              )}
              Descargar exportación JSON
            </Button>

            <Button
              type="button"
              variant="outline"
              onClick={handleDownloadArchive}
              disabled={Boolean(loadingAction) || !accessToken}
            >
              {loadingAction === "export-archive" ? (
                <Loader2
                  className="mr-2 size-4 animate-spin"
                  aria-hidden="true"
                />
              ) : (
                <Archive className="mr-2 size-4" aria-hidden="true" />
              )}
              Descargar archivo ZIP
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
