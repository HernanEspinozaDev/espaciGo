"use client";

import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/workspace/ui/button";
import type { AccessToken } from "@/features/auth/application/auth.port";
import type { GalleryPhoto, GalleryUploadStatus } from "../domain/types";
import {
  GalleryUploadQueue,
  type QueueItem,
} from "../application/gallery-upload-queue";

const PHOTO_MIME_BY_EXTENSION: Readonly<Record<string, string>> = {
  ".jpg": "image/jpeg",
  ".jpeg": "image/jpeg",
  ".png": "image/png",
  ".webp": "image/webp",
};

interface GalleryUploadBatchProps {
  listingId: string;
  token: AccessToken;
  photoCount: number;
  disabled: boolean;
  onBusyChange(busy: boolean): void;
  uploadPhoto(
    token: AccessToken,
    listingId: string,
    input: { image: File },
    key?: string,
    signal?: AbortSignal,
  ): Promise<GalleryPhoto>;
  getPhotoUploadStatus(
    token: AccessToken,
    listingId: string,
    key: string,
  ): Promise<GalleryUploadStatus>;
  loadPhotos(
    token: AccessToken,
    listingId: string,
  ): Promise<readonly GalleryPhoto[]>;
  setInitialCover(photoId: string): Promise<void>;
  onCoverUncertain(): void;
  allowInitialCover: boolean;
}

const labels: Record<QueueItem["state"], string> = {
  queued: "En cola",
  uploading: "Subiendo…",
  pending: "Resultado pendiente de confirmar",
  ready: "Completada",
  failed:
    "Carga sin confirmar; consulta el resultado o reintenta explícitamente",
  rejected: "Imagen rechazada. Selecciona otro archivo.",
  cancelled: "Cancelada antes del envío",
};

export function GalleryUploadBatch({
  listingId,
  token,
  photoCount,
  disabled,
  onBusyChange,
  uploadPhoto,
  getPhotoUploadStatus,
  loadPhotos,
  setInitialCover,
  onCoverUncertain,
  allowInitialCover,
}: GalleryUploadBatchProps) {
  const queue = useRef<GalleryUploadQueue | null>(null);
  const fileInput = useRef<HTMLInputElement>(null);
  const [items, setItems] = useState<readonly QueueItem[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [dragActive, setDragActive] = useState(false);
  const initialCoverTask = useRef<string | null>(null);
  const coverAttempted = useRef(false);
  const unresolved = items.some(
    (item) =>
      item.state !== "ready" &&
      item.state !== "rejected" &&
      item.state !== "cancelled",
  );
  const queueDisabled = disabled || unresolved || photoCount >= 30;
  const completed = items.filter((item) => item.state === "ready").length;
  useEffect(() => {
    let alive = true;
    initialCoverTask.current = null;
    coverAttempted.current = false;
    const current = new GalleryUploadQueue({
      upload: (task, signal) =>
        uploadPhoto(token, listingId, { image: task.file }, task.key, signal),
      lookup: (task) => getPhotoUploadStatus(token, listingId, task.key),
      confirm: async (task, photoId) => {
        const photos = await loadPhotos(token, listingId);
        if (!alive) return false;
        const ready = photos.some(
          (photo) => photo.id === photoId && photo.state === "ready",
        );
        if (
          ready &&
          task.id === initialCoverTask.current &&
          !coverAttempted.current
        ) {
          coverAttempted.current = true;
          try {
            await setInitialCover(photoId);
          } catch {
            if (alive) onCoverUncertain();
          }
        }
        return alive && ready;
      },
      changed: (snapshot) => {
        if (!alive) return;
        setItems(snapshot);
        onBusyChange(
          snapshot.some(
            (item) =>
              item.state !== "ready" &&
              item.state !== "rejected" &&
              item.state !== "cancelled",
          ),
        );
      },
    });
    queue.current = current;
    return () => {
      alive = false;
      current.dispose();
      queue.current = null;
      onBusyChange(false);
    };
  }, [
    listingId,
    token,
    uploadPhoto,
    getPhotoUploadStatus,
    loadPhotos,
    onBusyChange,
    setInitialCover,
    onCoverUncertain,
  ]);

  function enqueueFiles(files: readonly File[]): void {
    const currentQueue = queue.current;
    if (queueDisabled || !currentQueue || files.length === 0) return;
    setError(null);
    if (photoCount + files.length > 30) {
      setError(
        "La galería admite hasta 30 fotografías, incluidas las cargas en curso.",
      );
      return;
    }
    if (
      files.some(
        (file) =>
          PHOTO_MIME_BY_EXTENSION[
            file.name.match(/\.[^.]+$/)?.[0].toLowerCase() ?? ""
          ] !== file.type || file.size > 10 * 1024 * 1024,
      )
    ) {
      setError("Usa imágenes JPG, PNG o WebP de hasta 10 MiB.");
      return;
    }
    currentQueue.enqueue(
      files.map((file, index) => {
        const task = {
          id: crypto.randomUUID(),
          file,
          key: crypto.randomUUID(),
        };
        if (allowInitialCover && index === 0) {
          initialCoverTask.current = task.id;
          coverAttempted.current = false;
        }
        return task;
      }),
    );
  }

  return (
    <section aria-label="Cola de carga de fotografías" className="space-y-3">
      <label className="text-sm font-medium">
        Subir fotografías
        <input
          ref={fileInput}
          id="gallery-photo-upload"
          aria-label="Subir fotografías"
          aria-describedby="gallery-upload-help"
          type="file"
          multiple
          accept="image/jpeg,image/png,image/webp,.jpg,.jpeg,.png,.webp"
          disabled={queueDisabled}
          className="mt-2 block min-h-11 max-w-full text-sm"
          onChange={(event) => {
            const files = Array.from(event.currentTarget.files ?? []);
            event.currentTarget.value = "";
            enqueueFiles(files);
          }}
        />
      </label>
      <div
        data-testid="gallery-dropzone-surface"
        className="w-full"
        onDragEnter={(event) => {
          if (
            Array.from(event.dataTransfer.types).includes("Files") &&
            !queueDisabled
          )
            setDragActive(true);
        }}
        onDragOver={(event) => {
          if (!Array.from(event.dataTransfer.types).includes("Files")) {
            setDragActive(false);
            return;
          }
          event.preventDefault();
          if (queueDisabled) {
            event.dataTransfer.dropEffect = "none";
            setDragActive(false);
            return;
          }
          event.dataTransfer.dropEffect = "copy";
          setDragActive(true);
        }}
        onDragLeave={(event) => {
          if (!event.currentTarget.contains(event.relatedTarget as Node | null))
            setDragActive(false);
        }}
        onDrop={(event) => {
          setDragActive(false);
          // Prevent dropped content from navigating away, including non-files.
          event.preventDefault();
          if (!Array.from(event.dataTransfer.types).includes("Files")) return;
          enqueueFiles(Array.from(event.dataTransfer.files));
        }}
      >
        <button
          type="button"
          data-testid="gallery-dropzone"
          data-drag-active={dragActive}
          aria-describedby="gallery-upload-help"
          aria-controls="gallery-photo-upload"
          disabled={queueDisabled}
          onClick={() => fileInput.current?.click()}
          className={`block w-full rounded-lg border-2 border-dashed p-4 text-center text-sm text-muted-foreground ${queueDisabled ? "pointer-events-none" : ""} ${dragActive ? "border-primary bg-primary/5" : "border-border"}`}
        >
          Arrastra fotografías aquí o utiliza el selector de archivos.
        </button>
      </div>
      <p id="gallery-upload-help" className="text-xs text-muted-foreground">
        La galería admite hasta 30 fotografías. JPG, PNG o WebP; hasta 10 MiB
        por imagen.
      </p>
      <p className="text-xs text-muted-foreground">
        Hasta dos cargas simultáneas. Cancelar el envío no revierte una
        operación ya recibida por el servidor.
      </p>
      {error && (
        <p role="alert" className="text-sm text-red-800">
          {error}
        </p>
      )}
      {items.length > 0 && (
        <>
          <p role="status" className="text-sm text-emerald-800">
            {completed === 0
              ? "Todavía no hay cargas confirmadas."
              : completed === 1
                ? "Fotografía agregada a la galería."
                : `${completed} fotografías agregadas a la galería.`}
          </p>
          <ol aria-label="Estado de las cargas" className="space-y-2">
            {items.map((item, index) => (
              <li
                key={item.id}
                data-testid="gallery-upload-item"
                data-upload-state={item.state}
                className="flex flex-wrap items-center gap-2 rounded border p-2 text-sm"
              >
                <span>
                  Fotografía {index + 1}:{" "}
                  <span role="status">{labels[item.state]}</span>
                </span>
                {(item.state === "queued" || item.state === "uploading") && (
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => queue.current?.cancel(item.id)}
                  >
                    {item.state === "queued"
                      ? "Cancelar en cola"
                      : "Detener envío y consultar"}
                  </Button>
                )}
                {(item.state === "pending" || item.state === "failed") && (
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => {
                      void queue.current?.check(item.id);
                    }}
                  >
                    Consultar estado de carga
                  </Button>
                )}
                {item.retryAllowed && (
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => queue.current?.retry(item.id)}
                  >
                    Reintentar carga con el mismo archivo
                  </Button>
                )}
                {item.discardAllowed && (
                  <Button
                    type="button"
                    variant="outline"
                    className="min-h-11"
                    onClick={() => queue.current?.discard(item.id)}
                  >
                    {item.state === "rejected"
                      ? "Quitar archivo rechazado"
                      : "Descartar carga fallida"}
                  </Button>
                )}
              </li>
            ))}
          </ol>
        </>
      )}
    </section>
  );
}
