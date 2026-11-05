"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import {
  ArrowDown,
  ArrowUp,
  Image as ImageIcon,
  Loader2,
  Star,
  Trash2,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/workspace/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/workspace/ui/card";
import { privateBlobUrls } from "@/lib/api/private-blob-urls";
import type { AccessToken } from "@/features/auth/application/auth.port";
import type { GalleryPhoto, GalleryUploadStatus } from "../domain/types";

const MAX_PHOTOS = 30;
const MAX_BYTES = 10 * 1024 * 1024;
const ERROR_TEXT =
  "No se pudo completar la operación. Actualiza la galería y vuelve a intentarlo.";
const FORMATS = new Set(["image/jpeg", "image/png", "image/webp"]);

interface ListingGalleryProps {
  listingId: string;
  token: AccessToken;
  photos: readonly GalleryPhoto[];
  selectedListing: {
    readonly cover_photo_id?: string | null;
    readonly etag?: string;
  } | null;
  uploadPhoto(
    token: AccessToken,
    listingId: string,
    input: { image: File },
    key?: string,
  ): Promise<GalleryPhoto>;
  replacePhoto(
    token: AccessToken,
    listingId: string,
    photoId: string,
    input: { image: File },
    key: string,
    ifMatch?: string,
  ): Promise<GalleryPhoto>;
  setCoverPhoto(
    token: AccessToken,
    listingId: string,
    photoId: string,
  ): Promise<void>;
  reorderPhotos(
    token: AccessToken,
    listingId: string,
    photoIds: string[],
  ): Promise<void>;
  removePhoto(
    token: AccessToken,
    listingId: string,
    photoId: string,
  ): Promise<void>;
  loadPhotos(
    token: AccessToken,
    listingId: string,
  ): Promise<readonly GalleryPhoto[]>;
  loadPhotoVariant(
    token: AccessToken,
    listingId: string,
    photoId: string,
    variant: "thumbnail" | "display",
    signal?: AbortSignal,
  ): Promise<Blob>;
  getPhotoUploadStatus(
    token: AccessToken,
    listingId: string,
    key: string,
  ): Promise<GalleryUploadStatus>;
}

interface RetryTask {
  kind: "upload" | "replace";
  file: File;
  key: string;
  photoId?: string;
  etag?: string;
}

function newKey(): string {
  return crypto.randomUUID();
}

export function ListingGallery(props: ListingGalleryProps) {
  const { listingId, token, photos, selectedListing, loadPhotoVariant } = props;
  const [pending, setPending] = useState<string | null>(null);
  const [uncertain, setUncertain] = useState<RetryTask | null>(null);
  const [retryAllowed, setRetryAllowed] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [urls, setUrls] = useState<Record<string, string>>({});
  const galleryKey = useMemo(() => `host-gallery:${listingId}`, [listingId]);
  const lifetime = useRef(0);
  useEffect(() => {
    lifetime.current++;
    return () => {
      lifetime.current++;
    };
  }, [listingId, token]);

  useEffect(() => {
    let alive = true;
    const controllers = new Map<string, AbortController>();
    for (const photo of photos) {
      const controller = new AbortController();
      controllers.set(photo.id, controller);
      void loadPhotoVariant(
        token,
        listingId,
        photo.id,
        "thumbnail",
        controller.signal,
      )
        .then((blob) => {
          if (!alive) return;
          const key = `${galleryKey}:${photo.id}`;
          const resource = privateBlobUrls.create(key, blob);
          setUrls((current) => ({ ...current, [photo.id]: resource.url }));
        })
        .catch(() => {
          if (alive && !controller.signal.aborted) setError(ERROR_TEXT);
        });
    }
    return () => {
      alive = false;
      controllers.forEach((controller) => controller.abort());
      for (const photo of photos)
        privateBlobUrls.revoke(`${galleryKey}:${photo.id}`);
    };
  }, [galleryKey, listingId, photos, loadPhotoVariant, token]);

  async function run(task: RetryTask): Promise<void> {
    if (pending !== null || (uncertain !== null && !retryAllowed)) return;
    const generation = lifetime.current;
    const marker = task.photoId ?? "new";
    setPending(marker);
    setError(null);
    setNotice(null);
    try {
      const photo =
        task.kind === "upload"
          ? await props.uploadPhoto(
              token,
              listingId,
              { image: task.file },
              task.key,
            )
          : await props.replacePhoto(
              token,
              listingId,
              task.photoId!,
              { image: task.file },
              task.key,
              task.etag,
            );
      if (generation !== lifetime.current) return;
      if (photo.state === "pending") {
        setUncertain(task);
        setRetryAllowed(false);
        setNotice("Fotografía pendiente de procesamiento.");
        await checkUpload(task);
        return;
      }
      const refreshed = await props.loadPhotos(token, listingId);
      if (generation !== lifetime.current) return;
      if (
        !refreshed.some(
          (item) => item.id === photo.id && item.state !== "pending",
        )
      ) {
        setUncertain(task);
        setRetryAllowed(false);
        setNotice("Resultado pendiente de confirmar en la galería.");
        return;
      }
      setUncertain(null);
      setRetryAllowed(false);
      setNotice(
        task.kind === "replace"
          ? "Fotografía reemplazada."
          : "Fotografía agregada a la galería.",
      );
    } catch (failure) {
      if (generation !== lifetime.current) return;
      // The media handler rejects invalid bytes before committing a photo.
      if (
        failure instanceof Error &&
        "status" in failure &&
        failure.status === 422 &&
        "code" in failure &&
        failure.code === "validation_error"
      ) {
        setUncertain(null);
        setRetryAllowed(false);
        setError(
          "La imagen fue rechazada. Selecciona otro archivo; la fotografía anterior se conserva.",
        );
        return;
      }
      setUncertain(task);
      setRetryAllowed(false);
      setError(
        "La solicitud quedó sin confirmar. No se repetirá automáticamente; actualiza la galería o reintenta explícitamente.",
      );
    } finally {
      if (generation === lifetime.current) setPending(null);
    }
  }

  async function action(
    id: string,
    operation: () => Promise<unknown>,
    success: string,
  ) {
    if (pending !== null || uncertain !== null) return;
    const generation = lifetime.current;
    setPending(id);
    setError(null);
    setNotice(null);
    try {
      await operation();
      if (generation !== lifetime.current) return;
      setNotice(success);
    } catch {
      if (generation !== lifetime.current) return;
      setError(ERROR_TEXT);
    } finally {
      if (generation === lifetime.current) setPending(null);
    }
  }

  async function refreshGallery(): Promise<void> {
    if (pending !== null) return;
    const generation = lifetime.current;
    if (uncertain) {
      await checkUpload(uncertain);
      return;
    }
    try {
      await props.loadPhotos(token, listingId);
      if (generation !== lifetime.current) return;
      setNotice("Galería actualizada.");
      setError(null);
    } catch {
      if (generation !== lifetime.current) return;
      setError(ERROR_TEXT);
    }
  }

  async function checkUpload(
    task: RetryTask | null = uncertain,
  ): Promise<void> {
    if (!task || pending !== null) return;
    const generation = lifetime.current;
    setPending(task.photoId ?? "new");
    try {
      const status = await props.getPhotoUploadStatus(
        token,
        listingId,
        task.key,
      );
      if (generation !== lifetime.current) return;
      if (status.state === "ready") {
        const current = await props.loadPhotos(token, listingId);
        if (generation !== lifetime.current) return;
        if (
          current.some((p) => p.id === status.photo_id && p.state !== "pending")
        ) {
          setUncertain(null);
          setNotice(
            task.kind === "replace"
              ? "Fotografía reemplazada."
              : "Fotografía agregada a la galería.",
          );
        } else setNotice("La galería aún no confirma el resultado.");
        setRetryAllowed(false);
      } else if (status.state === "failed" || status.state === "not_found") {
        setRetryAllowed(true);
        setNotice(
          "La operación no se completó. Puedes reintentar explícitamente.",
        );
      } else {
        setRetryAllowed(false);
        setNotice("Fotografía pendiente de procesamiento.");
      }
      setError(null);
    } catch {
      if (generation !== lifetime.current) return;
      setRetryAllowed(false);
      setError(
        "No se pudo consultar el resultado; la operación sigue incierta.",
      );
    } finally {
      if (generation === lifetime.current) setPending(null);
    }
  }

  function valid(file: File): boolean {
    if (!FORMATS.has(file.type) || file.size > MAX_BYTES) {
      setError("Usa imágenes JPG, PNG o WebP de hasta 10 MiB.");
      return false;
    }
    return true;
  }

  return (
    <div className="space-y-6">
      <div
        role="note"
        className="rounded-lg border border-amber-300 bg-amber-50 p-4 text-xs text-amber-900"
      >
        Entorno de ensayo de galería privada. Las fotografías de este entorno
        son para ensayos locales.
      </div>
      <Card>
        <CardHeader className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <CardTitle className="text-lg">Galería de imágenes</CardTitle>
            <CardDescription>
              Selecciona la portada y organiza las fotografías del espacio.
            </CardDescription>
          </div>
          <label className="text-sm font-medium">
            Subir fotografías
            <input
              id="gallery-photo-upload"
              aria-label="Subir fotografías"
              type="file"
              accept="image/jpeg,image/png,image/webp,.jpg,.jpeg,.png,.webp"
              disabled={
                pending !== null ||
                uncertain !== null ||
                photos.length >= MAX_PHOTOS
              }
              className="mt-2 block min-h-11 max-w-full text-sm"
              onChange={(event) => {
                const files = Array.from(event.currentTarget.files ?? []);
                event.currentTarget.value = "";
                if (photos.length + files.length > MAX_PHOTOS) {
                  setError("La galería admite hasta 30 fotografías.");
                  return;
                }
                const file = files[0];
                if (file && valid(file))
                  void run({ kind: "upload", file, key: newKey() });
              }}
            />
          </label>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between text-sm">
            <span>Fotografías</span>
            <span aria-live="polite">
              {photos.length} / {MAX_PHOTOS}
            </span>
            <Button
              type="button"
              variant="outline"
              disabled={pending !== null}
              onClick={() => void refreshGallery()}
            >
              Actualizar galería
            </Button>
          </div>
          {error && (
            <p role="alert" className="text-sm text-red-800">
              {error}
            </p>
          )}
          {notice && (
            <p role="status" className="text-sm text-emerald-700">
              {notice}
            </p>
          )}
          {uncertain && (
            <div className="rounded-md border border-amber-300 bg-amber-50 p-3 text-sm">
              <p>
                Operación pendiente de confirmación; no se reenvía
                automáticamente.
              </p>
              <Button
                type="button"
                className="mt-2 min-h-11"
                disabled={pending !== null}
                onClick={() => void checkUpload()}
              >
                Consultar estado
              </Button>
              <Button
                type="button"
                variant="outline"
                className="ml-2 min-h-11"
                disabled={pending !== null || !retryAllowed}
                onClick={() => void run(uncertain)}
              >
                Reintentar con el mismo archivo
              </Button>
            </div>
          )}
          {photos.length === 0 ? (
            <div className="rounded-xl border-2 border-dashed p-10 text-center">
              <ImageIcon
                className="mx-auto size-10 text-muted-foreground"
                aria-hidden="true"
              />
              <p className="mt-3 text-sm">No hay imágenes en la galería</p>
              <p className="mt-1 text-xs text-muted-foreground">
                Agrega una fotografía JPG, PNG o WebP.
              </p>
            </div>
          ) : (
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 md:grid-cols-3">
              {photos.map((photo, idx) => {
                const cover =
                  photo.is_cover ||
                  selectedListing?.cover_photo_id === photo.id;
                return (
                  <article
                    key={photo.id}
                    data-testid="gallery-photo"
                    data-photo-id={photo.id}
                    className={`space-y-3 rounded-xl border p-3 ${cover ? "border-primary bg-primary/5 ring-2 ring-primary/20" : "bg-card"}`}
                  >
                    <div className="flex items-center justify-between">
                      <span className="text-xs">#{idx + 1}</span>
                      {cover ? (
                        <Badge>
                          <Star className="mr-1 size-3" aria-hidden="true" />
                          Portada
                        </Badge>
                      ) : (
                        <Button
                          type="button"
                          variant="ghost"
                          className="min-h-11"
                          disabled={pending !== null || uncertain !== null}
                          onClick={() =>
                            void action(
                              photo.id,
                              () =>
                                props.setCoverPhoto(token, listingId, photo.id),
                              "Portada actualizada.",
                            )
                          }
                        >
                          Marcar portada
                        </Button>
                      )}
                    </div>
                    <div className="flex h-36 items-center justify-center overflow-hidden rounded-lg border bg-muted/40">
                      <img
                        src={urls[photo.id] ?? ""}
                        alt={photo.alt_text || "Fotografía del espacio"}
                        className="h-full w-full object-cover"
                      />
                    </div>
                    <div className="flex flex-wrap items-center justify-between gap-2 border-t pt-2">
                      <div className="flex gap-1">
                        <Button
                          type="button"
                          variant="outline"
                          className="size-11 p-0"
                          disabled={
                            pending !== null || uncertain !== null || idx === 0
                          }
                          aria-label="Mover foto antes"
                          onClick={() => {
                            const ids = photos.map((item) => item.id);
                            [ids[idx - 1], ids[idx]] = [
                              ids[idx]!,
                              ids[idx - 1]!,
                            ];
                            void action(
                              photo.id,
                              () => props.reorderPhotos(token, listingId, ids),
                              "Orden actualizado.",
                            );
                          }}
                        >
                          <ArrowUp aria-hidden="true" />
                        </Button>
                        <Button
                          type="button"
                          variant="outline"
                          className="size-11 p-0"
                          disabled={
                            pending !== null ||
                            uncertain !== null ||
                            idx === photos.length - 1
                          }
                          aria-label="Mover foto después"
                          onClick={() => {
                            const ids = photos.map((item) => item.id);
                            [ids[idx], ids[idx + 1]] = [
                              ids[idx + 1]!,
                              ids[idx]!,
                            ];
                            void action(
                              photo.id,
                              () => props.reorderPhotos(token, listingId, ids),
                              "Orden actualizado.",
                            );
                          }}
                        >
                          <ArrowDown aria-hidden="true" />
                        </Button>
                      </div>
                      <label className="inline-flex min-h-11 cursor-pointer items-center rounded-md border px-3 text-sm">
                        Reemplazar foto
                        <input
                          type="file"
                          aria-label="Reemplazar fotografía"
                          accept="image/jpeg,image/png,image/webp,.jpg,.jpeg,.png,.webp"
                          disabled={pending !== null || uncertain !== null}
                          className="sr-only"
                          onChange={(event) => {
                            const file = event.currentTarget.files?.[0];
                            event.currentTarget.value = "";
                            if (file && valid(file))
                              void run({
                                kind: "replace",
                                file,
                                key: newKey(),
                                photoId: photo.id,
                                etag: selectedListing?.etag,
                              });
                          }}
                        />
                      </label>
                      <Button
                        type="button"
                        variant="ghost"
                        className="min-h-11 text-red-800"
                        disabled={pending !== null || uncertain !== null}
                        aria-label="Eliminar foto"
                        onClick={() => {
                          if (
                            window.confirm(
                              "¿Eliminar esta fotografía de la galería?",
                            )
                          )
                            void action(
                              photo.id,
                              () =>
                                props.removePhoto(token, listingId, photo.id),
                              "Fotografía eliminada.",
                            );
                        }}
                      >
                        <Trash2 className="mr-1 size-4" aria-hidden="true" />
                        Eliminar
                      </Button>
                    </div>
                    {pending === photo.id && (
                      <p role="status" className="text-xs">
                        {uncertain ? (
                          "Resultado pendiente de confirmar"
                        ) : (
                          <>
                            <Loader2 className="mr-1 inline size-3 animate-spin" />
                            Procesando…
                          </>
                        )}
                      </p>
                    )}
                  </article>
                );
              })}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
