"use client";

import { useEffect, useState } from "react";
import Image from "next/image";
import {
  AlertCircle,
  Camera,
  CheckCircle2,
  FlaskConical,
  Loader2,
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
import { useAuth } from "@/features/auth/ui/use-auth";
import { useAccount } from "./use-account";
import { privateBlobUrls } from "@/lib/api/private-blob-urls";

export function SyntheticPhotoCard() {
  const { snapshot: authSnapshot } = useAuth();
  const { snapshot, generateSyntheticPhoto, removeProfilePhoto } = useAccount();
  const [loadingAction, setLoadingAction] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    return () => {
      // Limpieza preventiva de URLs blob de fotos
      privateBlobUrls.clear();
    };
  }, []);

  const handleGenerate = async () => {
    if (authSnapshot.status !== "authenticated") return;
    setError(null);
    setMessage(null);
    setLoadingAction("generate");
    try {
      const idempotencyKey = crypto.randomUUID();
      await generateSyntheticPhoto(idempotencyKey);
      setMessage("Foto sintética de ensayo generada con éxito.");
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Error al generar foto sintética",
      );
    } finally {
      setLoadingAction(null);
    }
  };

  const handleDelete = async () => {
    if (authSnapshot.status !== "authenticated") return;
    setError(null);
    setMessage(null);
    setLoadingAction("delete");
    try {
      const idempotencyKey = crypto.randomUUID();
      await removeProfilePhoto(idempotencyKey);
      setMessage("Foto de perfil eliminada.");
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Error al eliminar la foto",
      );
    } finally {
      setLoadingAction(null);
    }
  };

  const photoUrl = snapshot.photoUrl;
  const hasPhoto = Boolean(snapshot.photo && photoUrl);

  return (
    <Card aria-labelledby="photo-title">
      <CardHeader>
        <div className="flex items-center gap-2">
          <Camera className="size-5 text-primary" aria-hidden="true" />
          <CardTitle id="photo-title">Foto de perfil</CardTitle>
        </div>
        <CardDescription>
          Entorno de pruebas con generación de foto sintética privada.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div
          role="note"
          className="flex items-start gap-2 rounded-md bg-blue-50 p-3 text-xs text-blue-900 border border-blue-200 dark:bg-blue-950/30 dark:border-blue-800 dark:text-blue-200"
        >
          <FlaskConical className="size-4 shrink-0 mt-0.5 text-blue-600" />
          <div>
            <span className="font-semibold">Aviso de ensayo:</span> La foto se
            descarga como objeto Blob privado autenticado mediante encabezados
            estrictos. No se almacena ni transmite imagen personal real.
          </div>
        </div>

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

        <div className="flex flex-col sm:flex-row items-center gap-6 pt-2">
          <div className="relative size-28 overflow-hidden rounded-full border-2 border-dashed border-border bg-muted flex items-center justify-center shrink-0">
            {hasPhoto && photoUrl ? (
              <Image
                src={photoUrl}
                alt="Foto de perfil sintética del usuario"
                fill
                sizes="112px"
                unoptimized
                className="object-cover"
              />
            ) : (
              <Camera
                className="size-8 text-muted-foreground"
                aria-hidden="true"
              />
            )}
          </div>

          <div className="flex flex-wrap gap-3">
            <Button
              type="button"
              onClick={handleGenerate}
              disabled={
                Boolean(loadingAction) ||
                authSnapshot.status !== "authenticated"
              }
            >
              {loadingAction === "generate" ? (
                <Loader2
                  className="mr-2 size-4 animate-spin"
                  aria-hidden="true"
                />
              ) : (
                <FlaskConical className="mr-2 size-4" aria-hidden="true" />
              )}
              Generar foto sintética
            </Button>

            {hasPhoto && (
              <Button
                type="button"
                variant="destructive"
                onClick={handleDelete}
                disabled={
                  Boolean(loadingAction) ||
                  authSnapshot.status !== "authenticated"
                }
              >
                {loadingAction === "delete" ? (
                  <Loader2
                    className="mr-2 size-4 animate-spin"
                    aria-hidden="true"
                  />
                ) : (
                  <Trash2 className="mr-2 size-4" aria-hidden="true" />
                )}
                Retirar foto
              </Button>
            )}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
