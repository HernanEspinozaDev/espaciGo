"use client";

import {
  type ChangeEvent,
  useEffect,
  useRef,
  useState,
  useTransition,
} from "react";
import { useRouter } from "next/navigation";
import Image from "next/image";
import Link from "next/link";
import {
  ArrowLeft,
  Save,
  Send,
  Archive,
  Trash2,
  Image as ImageIcon,
  Calendar,
  DollarSign,
  FileText,
  AlertCircle,
  CheckCircle2,
  Loader2,
  Star,
  ArrowUp,
  ArrowDown,
  Plus,
  History,
} from "lucide-react";
import { Button } from "@/components/workspace/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/workspace/ui/card";
import { Input } from "@/components/workspace/ui/input";
import { Label } from "@/components/workspace/ui/label";
import { Badge } from "@/components/ui/badge";
import { useAuth } from "@/features/auth/ui/use-auth";
import { ApiError } from "@/lib/api/errors";
import { useHostListings } from "./use-host-listings";
import { hostListingsFacade } from "../host-listings-runtime";
import { privateBlobUrls } from "@/lib/api/private-blob-urls";
import {
  canPublishListing,
  isValidIanaTimeZone,
  validateCategoryAttributes,
  validateListingDraftInput,
  validateScheduleSlots,
} from "../domain/validation";
import type {
  CategoryProfileSchema,
  GalleryPhoto,
  PricingHistoryItem,
  RateUnit,
  ScheduleSlot,
} from "../domain/types";

interface ListingEditorProps {
  readonly listingId: string;
}

type TabKey = "info" | "pricing" | "schedule" | "photos";

const MAX_CURRENT_GALLERY_PHOTOS = 30;
const MAX_PHOTO_INPUT_BYTES = 10 * 1024 * 1024;
const MAX_CONCURRENT_PHOTO_UPLOADS = 2;
type PhotoUploadStatus =
  "queued" | "uploading" | "pending" | "completed" | "failed" | "cancelled";
interface PhotoUploadItem {
  readonly id: number;
  readonly filename: string;
  readonly image: File;
  readonly isCover: boolean;
  readonly idempotencyKey: string;
  readonly status: PhotoUploadStatus;
  readonly cancelRequested?: boolean;
  readonly cancellable?: boolean;
  readonly error?: string;
}
interface PendingPhotoReplacement {
  readonly filename: string;
  readonly image: File;
  readonly targetPhotoId: string;
  readonly idempotencyKey: string;
  readonly ifMatch: string;
  readonly status: "pending" | "failed";
}
function shouldDiscardPendingPhotoReplacement(error: unknown): boolean {
  return (
    (error instanceof ApiError && error.status >= 400 && error.status < 500) ||
    (error instanceof DOMException && error.name === "AbortError")
  );
}
const PHOTO_MIME_BY_EXTENSION: Record<string, string> = {
  ".jpg": "image/jpeg",
  ".jpeg": "image/jpeg",
  ".png": "image/png",
  ".webp": "image/webp",
};

function mapDraftServerFieldErrors(
  error: unknown,
  schema: CategoryProfileSchema | null,
): Record<string, string> {
  if (
    !(error instanceof ApiError) ||
    error.status !== 422 ||
    error.code !== "validation_error"
  ) {
    return {};
  }

  const formFields = new Map<string, string>([
    ["title", "title"],
    ["description", "description"],
    ["area_m2", "area_m2"],
    ["capacity", "capacity"],
    ["usage_rules", "usage_rules"],
    ["address", "address"],
    ["region_code", "region_code"],
    ["comuna_code", "comuna_code"],
    ["base_price_clp", "base_price"],
  ]);
  const messages = Object.create(null) as Record<string, string>;

  for (const fieldError of error.fields) {
    let target = formFields.get(fieldError.field);
    const attributePrefix = "attributes.";
    if (!target && fieldError.field.startsWith(attributePrefix)) {
      const attributeCode = fieldError.field.slice(attributePrefix.length);
      if (
        schema?.attributes.some((attribute) => attribute.code === attributeCode)
      ) {
        target = `${attributePrefix}${attributeCode}`;
      }
    }
    if (!target || messages[target]) continue;

    if (fieldError.field.startsWith(attributePrefix)) {
      const attributeMessage = {
        required: "Completa este atributo.",
        invalid_value: "Ingresa un valor válido para este atributo.",
        exceeds_area: "La superficie no puede superar el área total.",
        invalid_combination: "Revisa la combinación de atributos relacionada.",
      }[fieldError.code];
      if (!attributeMessage) continue;
      messages[target] = attributeMessage;
      continue;
    }

    if (fieldError.field === "title" && fieldError.code === "max_length") {
      messages[target] = "El título no puede superar 70 caracteres.";
    } else if (
      fieldError.field === "description" &&
      fieldError.code === "min_length"
    ) {
      messages[target] = "La descripción debe tener al menos 100 caracteres.";
    } else if (
      fieldError.field === "usage_rules" &&
      fieldError.code === "max_length"
    ) {
      messages[target] = "Las reglas no pueden superar 250 caracteres.";
    } else if (
      fieldError.field === "address" &&
      fieldError.code === "max_length"
    ) {
      messages[target] = "La dirección no puede superar 500 caracteres.";
    } else if (
      (fieldError.field === "region_code" ||
        fieldError.field === "comuna_code") &&
      fieldError.code === "location_pair"
    ) {
      messages[target] = "Selecciona una región y comuna compatibles.";
    } else if (fieldError.code === "required") {
      messages[target] = "Este campo es obligatorio.";
    } else if (fieldError.code === "minimum") {
      messages[target] = "El valor está por debajo del mínimo permitido.";
    } else if (fieldError.code === "maximum") {
      messages[target] = "El valor supera el máximo permitido.";
    } else if (fieldError.code === "step") {
      messages[target] = "El valor no coincide con el intervalo permitido.";
    } else {
      messages[target] = "Revisa el valor de este campo.";
    }
  }

  return messages;
}

const DAYS_MAP = [
  { day: 1, name: "Lunes" },
  { day: 2, name: "Martes" },
  { day: 3, name: "Miércoles" },
  { day: 4, name: "Jueves" },
  { day: 5, name: "Viernes" },
  { day: 6, name: "Sábado" },
  { day: 7, name: "Domingo" },
];

interface PrivateGalleryThumbnailProps {
  readonly accountId: string | null;
  readonly listingId: string;
  readonly photo: GalleryPhoto;
  readonly index: number;
}

function PrivateGalleryThumbnail({
  accountId,
  listingId,
  photo,
  index,
}: PrivateGalleryThumbnailProps) {
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);
  const alt = photo.alt_text || `Imagen ${index + 1} del espacio`;
  const source = photo.variants?.find(
    (variant) => variant.name === "thumbnail",
  )?.url;

  useEffect(() => {
    const controller = new AbortController();
    const resourceKey = `space-gallery:${accountId ?? "anonymous"}:${listingId}:${photo.id}:thumbnail`;
    let resource: ReturnType<typeof privateBlobUrls.create> | null = null;
    let disposed = false;
    if (!source)
      return () => {
        disposed = true;
        controller.abort();
        resource?.revoke();
      };

    void fetch(source, {
      method: "GET",
      credentials: "same-origin",
      cache: "no-store",
      signal: controller.signal,
    })
      .then(async (response) => {
        if (!response.ok) throw new Error("No se pudo cargar la imagen");
        const mediaType = response.headers
          .get("content-type")
          ?.split(";", 1)[0]
          .trim()
          .toLowerCase();
        if (!mediaType?.startsWith("image/")) {
          throw new Error("Tipo de imagen no válido");
        }
        return response.blob();
      })
      .then((blob) => {
        if (disposed) return;
        resource = privateBlobUrls.create(resourceKey, blob);
        setPreviewUrl(resource.url);
      })
      .catch(() => {});

    return () => {
      disposed = true;
      controller.abort();
      resource?.revoke();
    };
  }, [accountId, listingId, photo.id, source]);

  return (
    <div className="relative h-32 overflow-hidden rounded-lg border bg-muted/60">
      {previewUrl ? (
        <Image
          src={previewUrl}
          alt={alt}
          fill
          sizes="(max-width: 640px) 100vw, (max-width: 768px) 50vw, 33vw"
          unoptimized
          className="object-cover"
        />
      ) : (
        <div className="flex h-full flex-col items-center justify-center p-2 text-center">
          <ImageIcon
            className="size-6 text-muted-foreground mb-1"
            aria-hidden="true"
          />
          <span className="text-xs font-medium text-foreground line-clamp-1">
            {alt}
          </span>
        </div>
      )}
    </div>
  );
}

export function ListingEditor({ listingId }: ListingEditorProps) {
  const router = useRouter();
  const { snapshot: authSnapshot } = useAuth();
  const {
    snapshot,
    loadPhotos,
    selectListing,
    loadCategorySchema,
    updateDraft,
    updateContent,
    updatePricing,
    loadPricingHistory,
    updateWeeklySchedule,
    configureTimeZone,
    uploadPhoto,
    getPhotoUploadStatus,
    replacePhoto,
    setCoverPhoto,
    removePhoto,
    movePhoto,
    publishListing,
    archiveListing,
    deleteListing,
  } = useHostListings();

  const [activeTab, setActiveTab] = useState<TabKey>("info");
  const [isPending, startTransition] = useTransition();
  const [feedbackSuccess, setFeedbackSuccess] = useState<string | null>(null);
  const [feedbackError, setFeedbackError] = useState<string | null>(null);
  const [photoUploads, setPhotoUploads] = useState<PhotoUploadItem[]>([]);
  const [uploadingPhotos, setUploadingPhotos] = useState(false);
  const [replacingPhotoId, setReplacingPhotoId] = useState<string | null>(null);
  const [pendingPhotoReplacement, setPendingPhotoReplacement] =
    useState<PendingPhotoReplacement | null>(null);
  const [checkingPhotoReplacement, setCheckingPhotoReplacement] =
    useState(false);
  const replacementInputRef = useRef<HTMLInputElement>(null);
  const replacementTargetPhotoId = useRef<string | null>(null);
  const photoUploadId = useRef(0);
  const cancelledPhotoUploadIds = useRef(new Set<number>());
  const photoUploadControllers = useRef(new Map<number, AbortController>());
  const reconciledCancelledUploads = useRef(new Set<number>());
  const [initialLoaded, setInitialLoaded] = useState(false);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const titleRef = useRef<HTMLInputElement>(null);
  const descriptionRef = useRef<HTMLTextAreaElement>(null);
  const addressRef = useRef<HTMLInputElement>(null);
  const rulesRef = useRef<HTMLTextAreaElement>(null);
  const capacityRef = useRef<HTMLInputElement>(null);
  const areaRef = useRef<HTMLInputElement>(null);

  // Pestaña 1: Información
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [capacity, setCapacity] = useState(1);
  const [areaM2, setAreaM2] = useState(10);
  const [address, setAddress] = useState("");
  const [comunaCode, setComunaCode] = useState("");
  const [regionCode, setRegionCode] = useState("13");
  const [usageRules, setUsageRules] = useState("");
  const [attributes, setAttributes] = useState<Record<string, unknown>>({});

  // Pestaña 2: Tarifas
  const [rateUnit, setRateUnit] = useState<RateUnit>("hora");
  const [basePrice, setBasePrice] = useState(10000);

  // Pestaña 3: Horarios
  const [timeZone, setTimeZone] = useState("America/Santiago");
  const [scheduleEnabled, setScheduleEnabled] = useState(true);
  const [scheduleSlots, setScheduleSlots] = useState<ScheduleSlot[]>([]);

  // Carga inicial de datos del espacio
  useEffect(() => {
    if (authSnapshot.status !== "authenticated" || !listingId) return;

    selectListing(listingId)
      .then(async (detail) => {
        if (detail) {
          setTitle(detail.title);
          setDescription(detail.description);
          setCapacity(detail.capacity);
          setAreaM2(detail.area_m2);
          setAddress(detail.address);
          setComunaCode(detail.comuna_code ?? "");
          setRegionCode(detail.region_code ?? "13");
          setUsageRules(detail.usage_rules);
          setRateUnit(detail.rate_unit);
          setBasePrice(detail.base_price_clp);
          setTimeZone(detail.time_zone ?? "America/Santiago");
          setAttributes(detail.attributes ?? {});

          if (detail.category_code) {
            loadCategorySchema(
              detail.category_code,
              detail.attribute_schema_version,
            ).catch(() => {});
          }

          const sched = hostListingsFacade.getSnapshot().weeklySchedule;
          if (sched) {
            setScheduleEnabled(sched.enabled);
            if (sched.time_zone) {
              setTimeZone(sched.time_zone);
            }
            const rebuiltSlots: ScheduleSlot[] = [];
            for (const day of sched.days) {
              for (const p of day.periods) {
                rebuiltSlots.push({
                  day_of_week: day.weekday,
                  start_time: p.open,
                  end_time: p.close,
                });
              }
            }
            if (rebuiltSlots.length > 0) {
              setScheduleSlots(rebuiltSlots);
            }
          }
        }
        setInitialLoaded(true);
      })
      .catch((err) => {
        const msg =
          err instanceof Error ? err.message : "Error al cargar espacio";
        setFeedbackError(msg);
      });

    loadPricingHistory(listingId).catch(() => {});
  }, [
    authSnapshot.status,
    listingId,
    selectListing,
    loadCategorySchema,
    loadPricingHistory,
  ]);

  const currentListing = snapshot.selectedListing;
  const currentSchema = currentListing
    ? snapshot.categorySchemas[
        `${currentListing.category_code}@${currentListing.attribute_schema_version}`
      ] || snapshot.categorySchemas[currentListing.category_code]
    : null;

  // Guardar Pestaña 1: Contenido y atributos
  const handleSaveContent = async (e: React.FormEvent) => {
    e.preventDefault();
    if (authSnapshot.status !== "authenticated" || !currentListing) return;
    setFeedbackError(null);
    setFeedbackSuccess(null);

    const validation = validateListingDraftInput({
      category_code: currentListing.category_code,
      title: title,
      description: description.trim(),
      area_m2: Number(areaM2),
      capacity: Number(capacity),
      usage_rules: usageRules,
      base_price_clp: Number(basePrice),
      rate_unit: rateUnit,
      address: address,
    });
    setFieldErrors(validation.errors);
    if (!validation.valid) {
      const first = Object.keys(validation.errors)[0];
      const focusTargets: Record<
        string,
        React.RefObject<HTMLInputElement | HTMLTextAreaElement | null>
      > = {
        title: titleRef,
        description: descriptionRef,
        address: addressRef,
        usage_rules: rulesRef,
        capacity: capacityRef,
        area_m2: areaRef,
      };
      focusTargets[first]?.current?.focus();
      return;
    }

    if (currentSchema) {
      const val = validateCategoryAttributes(
        attributes,
        currentSchema,
        Number(areaM2),
      );
      if (!val.valid) {
        const firstErr = Object.values(val.errors)[0];
        setFeedbackError(firstErr ?? "Atributos requeridos incompletos.");
        return;
      }
    }

    startTransition(async () => {
      try {
        if (currentListing.state === "borrador") {
          const ifMatch = currentListing.etag;
          if (!ifMatch) {
            throw new Error(
              "No se pudo verificar la versión del borrador. Recarga la página antes de guardar.",
            );
          }
          await updateDraft(
            listingId,
            {
              category_code: currentListing.category_code,
              title: title.trim(),
              description: description.trim(),
              area_m2: Number(areaM2),
              capacity: Number(capacity),
              usage_rules: usageRules.trim(),
              rate_unit: rateUnit,
              base_price_clp: Number(basePrice),
              address: address.trim(),
              region_code: regionCode || undefined,
              comuna_code: comunaCode.trim() || undefined,
              attribute_schema_version: currentListing.attribute_schema_version,
              attributes,
            },
            ifMatch,
          );
        } else {
          const ifMatch = currentListing.etag;
          if (!ifMatch) {
            throw new Error(
              "No se pudo verificar la versión del anuncio. Recarga la página antes de guardar.",
            );
          }
          await updateContent(
            listingId,
            {
              title: title.trim(),
              description: description.trim(),
              capacity: Number(capacity),
              usage_rules: usageRules.trim(),
              base_price_clp: Number(basePrice),
              region_code: regionCode || undefined,
              comuna_code: comunaCode.trim() || undefined,
            },
            ifMatch,
          );
        }
        setFeedbackSuccess("Información actualizada exitosamente.");
      } catch (err) {
        if (
          err instanceof ApiError &&
          err.status === 422 &&
          err.code === "validation_error"
        ) {
          const serverFieldErrors = mapDraftServerFieldErrors(
            err,
            currentSchema,
          );
          if (Object.keys(serverFieldErrors).length > 0) {
            setFieldErrors((errors) => ({ ...errors, ...serverFieldErrors }));
          }
          setFeedbackError(
            "No se pudo guardar. Corrige los campos indicados y vuelve a intentar.",
          );
          return;
        }
        const msg =
          err instanceof Error ? err.message : "Error al actualizar contenido";
        setFeedbackError(msg);
      }
    });
  };

  // Guardar Pestaña 2: Tarifas
  const handleSavePricing = async (e: React.FormEvent) => {
    e.preventDefault();
    if (authSnapshot.status !== "authenticated") return;
    setFeedbackError(null);
    setFeedbackSuccess(null);

    if (
      basePrice <= 5000 ||
      !Number.isSafeInteger(basePrice) ||
      !["hora", "dia", "mes"].includes(rateUnit)
    ) {
      setFieldErrors((errors) => ({
        ...errors,
        base_price:
          "El precio debe ser un entero mayor a $5.000 CLP por hora, día o mes.",
      }));
      setFeedbackError(
        "El precio base debe ser un entero mayor a $5.000 CLP y la unidad de cobro debe ser válida.",
      );
      return;
    }

    setFieldErrors((errors) => ({ ...errors, base_price: "" }));
    startTransition(async () => {
      try {
        if (!currentListing?.etag) {
          throw new Error(
            "No se pudo verificar la versión del espacio. Recarga la página antes de guardar la tarifa.",
          );
        }
        await updatePricing(
          listingId,
          {
            rate_unit: rateUnit,
            base_price: Number(basePrice),
          },
          currentListing.etag,
        );
        await loadPricingHistory(listingId);
        setFeedbackSuccess("Tarifa actualizada y registrada en el historial.");
      } catch (err) {
        const msg =
          err instanceof Error ? err.message : "Error al actualizar tarifas";
        setFeedbackError(msg);
      }
    });
  };

  // Guardar Pestaña 3: Horarios
  const handleSaveSchedule = async (e: React.FormEvent) => {
    e.preventDefault();
    if (authSnapshot.status !== "authenticated") return;
    setFeedbackError(null);
    setFeedbackSuccess(null);

    if (!isValidIanaTimeZone(timeZone)) {
      setFeedbackError(
        "La zona horaria debe ser un identificador IANA válido (ej: America/Santiago).",
      );
      return;
    }

    if (scheduleEnabled && scheduleSlots.length > 0) {
      const val = validateScheduleSlots(scheduleSlots);
      if (!val.valid) {
        setFeedbackError(
          val.errors[0] ?? "Horarios con solapamientos o tramos inválidos.",
        );
        return;
      }
    }

    const listingEtag = currentListing?.etag;
    if (!listingEtag) {
      setFeedbackError("No se pudo determinar la versión actual del espacio.");
      return;
    }

    startTransition(async () => {
      try {
        const configured = await configureTimeZone(
          listingId,
          timeZone,
          listingEtag,
        );
        await updateWeeklySchedule(
          listingId,
          {
            timezone: timeZone,
            enabled: scheduleEnabled,
            weekly_slots: scheduleSlots,
          },
          configured.etag,
        );
        setFeedbackSuccess(
          "Disponibilidad semanal y zona horaria guardadas con éxito.",
        );
      } catch (err) {
        const msg =
          err instanceof Error ? err.message : "Error al guardar horarios";
        setFeedbackError(msg);
      }
    });
  };

  // Pestaña 4: Fotografías reales; Go conserva la autoridad final del cupo.
  const handleUploadImages = (event: ChangeEvent<HTMLInputElement>) => {
    const input = event.currentTarget;
    const files = Array.from(input.files ?? []);
    input.value = "";
    if (authSnapshot.status !== "authenticated" || files.length === 0) return;

    setFeedbackError(null);
    setFeedbackSuccess(null);

    if (snapshot.photos.length + files.length > MAX_CURRENT_GALLERY_PHOTOS) {
      setFeedbackError(
        `El límite actual es de ${MAX_CURRENT_GALLERY_PHOTOS} imágenes por espacio.`,
      );
      return;
    }

    if (
      files.some((file) => {
        const extension = file.name.match(/\.[^.]+$/)?.[0].toLowerCase();
        return (
          PHOTO_MIME_BY_EXTENSION[extension ?? ""] !== file.type ||
          file.size > MAX_PHOTO_INPUT_BYTES
        );
      })
    ) {
      setFeedbackError(
        "Usa imágenes JPG, PNG o WebP de hasta 10 MiB cada una.",
      );
      return;
    }

    const shouldSetCover = snapshot.photos.length === 0;
    const ids = files.map(() => ++photoUploadId.current);
    const photoUploadsForBatch = files.map((file, index) => ({
      id: ids[index]!,
      filename: file.name,
      image: file,
      isCover: shouldSetCover && index === 0,
      idempotencyKey: crypto.randomUUID(),
      status: "queued" as const,
      cancellable: true,
    }));
    setPhotoUploads(photoUploadsForBatch);
    setUploadingPhotos(true);
    void (async () => {
      let nextIndex = 0;
      let failedCount = 0;
      let succeededCount = 0;
      let pendingCount = 0;
      const workers = Array.from(
        { length: Math.min(MAX_CONCURRENT_PHOTO_UPLOADS, files.length) },
        async () => {
          while (nextIndex < files.length) {
            const index = nextIndex++;
            const item = photoUploadsForBatch[index]!;
            const id = item.id;
            if (cancelledPhotoUploadIds.current.has(id)) continue;
            setPhotoUploads((items) =>
              items.map((item) =>
                item.id === id ? { ...item, status: "uploading" } : item,
              ),
            );
            const controller = new AbortController();
            photoUploadControllers.current.set(id, controller);
            try {
              const result = await uploadPhoto(
                listingId,
                { image: item.image, is_cover: item.isCover },
                item.idempotencyKey,
                controller.signal,
              );
              if (result.state === "pending") {
                pendingCount++;
                setPhotoUploads((items) =>
                  items.map((item) =>
                    item.id === id
                      ? {
                          ...item,
                          status: "pending",
                          cancellable: false,
                        }
                      : item,
                  ),
                );
              } else {
                succeededCount++;
                setPhotoUploads((items) =>
                  items.map((item) =>
                    item.id === id
                      ? { ...item, status: "completed", cancellable: false }
                      : item,
                  ),
                );
              }
            } catch (err) {
              if (cancelledPhotoUploadIds.current.has(id)) {
                if (!reconciledCancelledUploads.current.has(id)) {
                  reconciledCancelledUploads.current.add(id);
                  try {
                    const status = await getPhotoUploadStatus(
                      listingId,
                      item.idempotencyKey,
                    );
                    if (status.state === "ready") {
                      await loadPhotos(listingId);
                      if (item.isCover)
                        await setCoverPhoto(listingId, status.photo_id);
                      succeededCount++;
                      setPhotoUploads((items) =>
                        items.map((upload) =>
                          upload.id === id
                            ? {
                                ...upload,
                                status: "completed",
                                error: undefined,
                                cancelRequested: false,
                                cancellable: false,
                              }
                            : upload,
                        ),
                      );
                    } else if (
                      status.state === "failed" ||
                      status.state === "not_found"
                    ) {
                      failedCount++;
                      setPhotoUploads((items) =>
                        items.map((upload) =>
                          upload.id === id
                            ? {
                                ...upload,
                                status: "failed",
                                error:
                                  "La carga no se completó. Puedes reintentar explícitamente.",
                                cancellable: false,
                              }
                            : upload,
                        ),
                      );
                    } else {
                      pendingCount++;
                      setPhotoUploads((items) =>
                        items.map((upload) =>
                          upload.id === id
                            ? {
                                ...upload,
                                status: "pending",
                                cancelRequested: true,
                                cancellable: false,
                                error:
                                  "Cancelación solicitada; el estado remoto sigue pendiente.",
                              }
                            : upload,
                        ),
                      );
                    }
                  } catch {
                    pendingCount++;
                    setPhotoUploads((items) =>
                      items.map((upload) =>
                        upload.id === id
                          ? {
                              ...upload,
                              status: "pending",
                              cancelRequested: true,
                              cancellable: false,
                              error:
                                "No se pudo confirmar el estado remoto. Revisa el estado antes de reintentar.",
                            }
                          : upload,
                      ),
                    );
                  }
                }
              } else {
                failedCount++;
                const message =
                  err instanceof Error ? err.message : "Error al subir imagen";
                setPhotoUploads((items) =>
                  items.map((item) =>
                    item.id === id
                      ? {
                          ...item,
                          status: "failed",
                          error: message,
                          cancellable: false,
                        }
                      : item,
                  ),
                );
              }
            } finally {
              photoUploadControllers.current.delete(id);
            }
          }
        },
      );
      await Promise.all(workers);
      setUploadingPhotos(false);
      if (failedCount > 0) {
        setFeedbackError(
          failedCount === files.length
            ? `No se pudieron agregar ${failedCount} imágenes a la galería.`
            : `Se agregaron ${succeededCount} imágenes; ${failedCount} no se pudieron subir.`,
        );
      }
      if (succeededCount > 0 || pendingCount > 0) {
        const completionSummary = `${succeededCount} ${succeededCount === 1 ? "correcta" : "correctas"}`;
        const pendingSummary =
          pendingCount > 0
            ? `; ${pendingCount} pendiente${pendingCount === 1 ? "" : "s"} de confirmación`
            : "";
        setFeedbackSuccess(
          `Lote de galería finalizado: ${completionSummary}${pendingSummary}.`,
        );
      }
    })();
  };

  const handleReplaceImage = async (event: ChangeEvent<HTMLInputElement>) => {
    const input = event.currentTarget;
    const file = input.files?.[0];
    input.value = "";
    const photoId = replacementTargetPhotoId.current;
    replacementTargetPhotoId.current = null;
    if (
      !file ||
      !photoId ||
      authSnapshot.status !== "authenticated" ||
      pendingPhotoReplacement
    )
      return;

    setFeedbackError(null);
    setFeedbackSuccess(null);
    const extension = file.name.match(/\.[^.]+$/)?.[0].toLowerCase();
    if (
      PHOTO_MIME_BY_EXTENSION[extension ?? ""] !== file.type ||
      file.size > MAX_PHOTO_INPUT_BYTES
    ) {
      setFeedbackError(
        "Usa imágenes JPG, PNG o WebP de hasta 10 MiB cada una.",
      );
      return;
    }
    const ifMatch = currentListing?.etag;
    if (!ifMatch) {
      setFeedbackError("Recarga el espacio antes de reemplazar una imagen.");
      return;
    }

    const replacement: PendingPhotoReplacement = {
      filename: file.name,
      image: file,
      targetPhotoId: photoId,
      idempotencyKey: crypto.randomUUID(),
      ifMatch,
      status: "pending",
    };
    setReplacingPhotoId(photoId);
    try {
      const result = await replacePhoto(
        listingId,
        photoId,
        { image: file },
        replacement.idempotencyKey,
        ifMatch,
      );
      if (result.state === "pending") {
        setPendingPhotoReplacement(replacement);
        setFeedbackSuccess(
          "El reemplazo está pendiente de confirmación; la foto actual se conserva.",
        );
      } else {
        setPendingPhotoReplacement(null);
        setFeedbackSuccess("Foto reemplazada correctamente.");
      }
    } catch (err) {
      setPendingPhotoReplacement(
        shouldDiscardPendingPhotoReplacement(err) ? null : replacement,
      );
      const message =
        err instanceof Error ? err.message : "Error al reemplazar la imagen";
      setFeedbackError(message);
    } finally {
      setReplacingPhotoId(null);
    }
  };

  const handleReviewPendingPhotoReplacement = async () => {
    const replacement = pendingPhotoReplacement;
    if (!replacement || checkingPhotoReplacement) return;

    setCheckingPhotoReplacement(true);
    setFeedbackError(null);
    setFeedbackSuccess(null);
    try {
      const status = await getPhotoUploadStatus(
        listingId,
        replacement.idempotencyKey,
      );
      if (status.state === "deleting") {
        setPendingPhotoReplacement(null);
        setFeedbackError(
          "El reemplazo no se completó; el archivo reservado está en limpieza. La foto actual se conserva. Elige nuevamente una imagen para iniciar otro intento.",
        );
        return;
      }
      if (status.state === "pending") {
        setPendingPhotoReplacement({ ...replacement, status: "pending" });
        setFeedbackSuccess(
          "El reemplazo continúa pendiente; la foto actual se conserva.",
        );
        return;
      }
      if (status.state === "failed" || status.state === "not_found") {
        setPendingPhotoReplacement({ ...replacement, status: "failed" });
        setFeedbackError(
          "No se confirmó el reemplazo. Puedes reintentar con el mismo archivo.",
        );
        return;
      }

      const refreshed = await selectListing(listingId);
      if (!refreshed) throw new Error("No se pudo actualizar la galería.");
      setPendingPhotoReplacement(null);
      setFeedbackSuccess("Foto reemplazada correctamente.");
    } catch (err) {
      if (shouldDiscardPendingPhotoReplacement(err)) {
        setPendingPhotoReplacement(null);
      }
      const message =
        err instanceof Error
          ? err.message
          : "No se pudo consultar el estado del reemplazo.";
      setFeedbackError(message);
    } finally {
      setCheckingPhotoReplacement(false);
    }
  };

  const handleRetryPendingPhotoReplacement = async () => {
    const replacement = pendingPhotoReplacement;
    if (
      !replacement ||
      replacement.status !== "failed" ||
      replacingPhotoId !== null
    )
      return;

    setReplacingPhotoId(replacement.targetPhotoId);
    setFeedbackError(null);
    setFeedbackSuccess(null);
    try {
      const result = await replacePhoto(
        listingId,
        replacement.targetPhotoId,
        { image: replacement.image },
        replacement.idempotencyKey,
        replacement.ifMatch,
      );
      if (result.state === "pending") {
        setPendingPhotoReplacement({ ...replacement, status: "pending" });
        setFeedbackSuccess(
          "El reemplazo está pendiente de confirmación; la foto actual se conserva.",
        );
      } else {
        setPendingPhotoReplacement(null);
        setFeedbackSuccess("Foto reemplazada correctamente.");
      }
    } catch (err) {
      setPendingPhotoReplacement(
        shouldDiscardPendingPhotoReplacement(err)
          ? null
          : { ...replacement, status: "pending" },
      );
      const message =
        err instanceof Error ? err.message : "Error al reintentar el reemplazo";
      setFeedbackError(message);
    } finally {
      setReplacingPhotoId(null);
    }
  };

  const handleCancelQueuedPhoto = (id: number) => {
    const item = photoUploads.find((upload) => upload.id === id);
    if (!item || item.status !== "queued") return;
    cancelledPhotoUploadIds.current.add(id);
    setPhotoUploads((items) =>
      items.map((upload) =>
        upload.id === id ? { ...upload, status: "cancelled" } : upload,
      ),
    );
  };

  const handleCancelDispatchedPhoto = (id: number) => {
    const item = photoUploads.find((upload) => upload.id === id);
    const controller = photoUploadControllers.current.get(id);
    if (!item || item.status !== "uploading" || !controller) return;
    cancelledPhotoUploadIds.current.add(id);
    setPhotoUploads((items) =>
      items.map((upload) =>
        upload.id === id
          ? {
              ...upload,
              status: "pending",
              cancelRequested: true,
              cancellable: false,
              error: "Cancelación solicitada; verificando estado remoto…",
            }
          : upload,
      ),
    );
    controller.abort();
  };

  const handleRetryPhoto = async (id: number) => {
    if (uploadingPhotos || authSnapshot.status !== "authenticated") return;
    const item = photoUploads.find((upload) => upload.id === id);
    if (!item || (item.status !== "failed" && item.status !== "pending"))
      return;

    const cancelRequested = item.cancelRequested === true;
    let reconciling = item.status === "pending";
    setUploadingPhotos(true);
    setPhotoUploads((items) =>
      items.map((upload) =>
        upload.id === id
          ? {
              ...upload,
              status: "uploading",
              cancellable: false,
              error: undefined,
            }
          : upload,
      ),
    );
    try {
      if (reconciling) {
        const status = await getPhotoUploadStatus(
          listingId,
          item.idempotencyKey,
        );
        if (status.state === "ready") {
          await loadPhotos(listingId);
          if (item.isCover) {
            await setCoverPhoto(listingId, status.photo_id);
          }
          setPhotoUploads((items) =>
            items.map((upload) =>
              upload.id === id
                ? {
                    ...upload,
                    status: "completed",
                    cancelRequested: false,
                    cancellable: false,
                    error: undefined,
                  }
                : upload,
            ),
          );
          return;
        }
        if (status.state === "pending" || status.state === "deleting") {
          setPhotoUploads((items) =>
            items.map((upload) =>
              upload.id === id
                ? {
                    ...upload,
                    status: "pending",
                    cancelRequested,
                    cancellable: false,
                    error: cancelRequested
                      ? "La cancelación local no detuvo el servidor; la carga sigue pendiente."
                      : undefined,
                  }
                : upload,
            ),
          );
          return;
        }
        if (cancelRequested) {
          setPhotoUploads((items) =>
            items.map((upload) =>
              upload.id === id
                ? {
                    ...upload,
                    status: "failed",
                    cancellable: false,
                    error:
                      "No se encontró una reserva. Reintentar enviará el mismo archivo con la misma clave.",
                  }
                : upload,
            ),
          );
          return;
        }
        reconciling = false;
      }

      const result = await uploadPhoto(
        listingId,
        { image: item.image, is_cover: item.isCover },
        item.idempotencyKey,
      );
      setPhotoUploads((items) =>
        items.map((upload) =>
          upload.id === id
            ? {
                ...upload,
                status: result.state === "pending" ? "pending" : "completed",
                cancelRequested: false,
                cancellable: false,
                error: undefined,
              }
            : upload,
        ),
      );
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Error al subir imagen";
      setPhotoUploads((items) =>
        items.map((upload) =>
          upload.id === id
            ? {
                ...upload,
                status: reconciling ? "pending" : "failed",
                cancelRequested: reconciling && cancelRequested,
                cancellable: false,
                error: message,
              }
            : upload,
        ),
      );
    } finally {
      setUploadingPhotos(false);
    }
  };

  const handleSetCover = async (photoId: string) => {
    if (authSnapshot.status !== "authenticated") return;
    startTransition(async () => {
      try {
        await setCoverPhoto(listingId, photoId);
        setFeedbackSuccess("Foto de portada actualizada.");
      } catch (err) {
        const msg =
          err instanceof Error ? err.message : "Error al definir portada";
        setFeedbackError(msg);
      }
    });
  };

  const handleRemovePhoto = async (photoId: string) => {
    if (authSnapshot.status !== "authenticated") return;
    startTransition(async () => {
      try {
        const ifMatch =
          currentListing?.id === listingId ? currentListing.etag : undefined;
        if (!ifMatch)
          throw new Error("No se pudo validar la versión del espacio");
        await removePhoto(listingId, photoId, ifMatch);
        setFeedbackSuccess("Foto eliminada de la galería.");
      } catch (err) {
        const msg =
          err instanceof Error ? err.message : "Error al eliminar foto";
        setFeedbackError(msg);
      }
    });
  };

  const handleMovePhoto = async (index: number, direction: "up" | "down") => {
    if (authSnapshot.status !== "authenticated") return;
    const targetIndex = direction === "up" ? index - 1 : index + 1;
    if (targetIndex < 0 || targetIndex >= snapshot.photos.length) return;

    const photoId = snapshot.photos[index]!.id;
    const ifMatch = currentListing?.etag;
    if (!ifMatch) {
      setFeedbackError(
        "Recarga el espacio antes de reordenar las fotografías.",
      );
      return;
    }
    startTransition(async () => {
      try {
        await movePhoto(listingId, photoId, targetIndex + 1, ifMatch);
        setFeedbackSuccess("Orden de fotografías actualizado.");
      } catch (err) {
        const msg =
          err instanceof Error ? err.message : "Error al reordenar fotos";
        setFeedbackError(msg);
      }
    });
  };

  // Acciones Globales: Publicar / Archivar / Retirar
  const handleGlobalPublish = async () => {
    if (authSnapshot.status !== "authenticated" || !currentListing) return;
    setFeedbackError(null);
    setFeedbackSuccess(null);

    const pubCheck = canPublishListing(currentListing);
    if (!pubCheck.eligible) {
      setFeedbackError(
        `Requisitos pendientes: ${pubCheck.reasons.join(" · ")}`,
      );
      return;
    }

    startTransition(async () => {
      try {
        await publishListing(listingId, currentListing.etag);
        setFeedbackSuccess(
          "¡El espacio ha sido publicado exitosamente y está activo en el catálogo!",
        );
      } catch (err) {
        const msg =
          err instanceof Error ? err.message : "Error al publicar espacio";
        setFeedbackError(msg);
      }
    });
  };

  const handleGlobalArchive = async () => {
    if (authSnapshot.status !== "authenticated") return;
    startTransition(async () => {
      try {
        await archiveListing(listingId, currentListing?.etag);
        setFeedbackSuccess(
          "El espacio ha sido archivado (oculto del catálogo público).",
        );
      } catch (err) {
        const msg =
          err instanceof Error ? err.message : "Error al archivar espacio";
        setFeedbackError(msg);
      }
    });
  };

  const handleGlobalRetire = async () => {
    if (
      authSnapshot.status !== "authenticated" ||
      !currentListing ||
      (currentListing.state !== "activa" && currentListing.state !== "oculta")
    ) {
      return;
    }
    const confirmed = window.confirm(
      `¿Retirar la publicación "${currentListing.title}" del catálogo? Las reservas e historiales se conservarán; esto no elimina el espacio físicamente.`,
    );
    if (!confirmed) return;

    startTransition(async () => {
      try {
        await deleteListing(listingId);
        router.push("/anfitrion/espacios");
      } catch (err) {
        const msg =
          err instanceof Error ? err.message : "Error al retirar publicación";
        setFeedbackError(msg);
      }
    });
  };

  if (!initialLoaded && snapshot.loading) {
    return (
      <div className="py-20 flex flex-col items-center justify-center space-y-3">
        <Loader2
          className="size-8 animate-spin text-primary"
          aria-hidden="true"
        />
        <p className="text-sm text-muted-foreground">
          Cargando detalles del espacio...
        </p>
      </div>
    );
  }

  if (!currentListing) {
    return (
      <div className="space-y-4 py-8 max-w-xl mx-auto text-center">
        <h1 className="text-xl font-bold text-foreground">
          Espacio no encontrado
        </h1>
        <p className="text-sm text-muted-foreground">
          No fue posible encontrar el espacio solicitado o no tienes permisos de
          anfitrión sobre él.
        </p>
        <Button asChild>
          <Link href="/anfitrion/espacios">Volver a mis espacios</Link>
        </Button>
      </div>
    );
  }

  const publishCheck = canPublishListing(currentListing);

  return (
    <div className="space-y-8 max-w-5xl mx-auto pb-12">
      {/* Cabecera Principal */}
      <div className="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-4 pb-4 border-b border-border">
        <div className="space-y-1.5">
          <div className="flex items-center gap-2 flex-wrap">
            <Button
              asChild
              variant="ghost"
              size="sm"
              className="h-7 px-2 -ml-2 text-xs"
            >
              <Link href="/anfitrion/espacios">
                <ArrowLeft className="size-3.5 mr-1" aria-hidden="true" />
                Mis Espacios
              </Link>
            </Button>
            <span className="text-muted-foreground">/</span>
            <Badge variant="outline" className="text-xs font-normal">
              {currentListing.category_name || currentListing.category_code}
            </Badge>
            <Badge
              variant={
                currentListing.state === "activa" ? "default" : "secondary"
              }
              className={`text-xs ${
                currentListing.state === "activa"
                  ? "bg-emerald-600 text-white"
                  : currentListing.state === "oculta"
                    ? "bg-amber-100 text-amber-800"
                    : "bg-slate-100 text-slate-700"
              }`}
            >
              {currentListing.state === "activa"
                ? "Publicado"
                : currentListing.state === "oculta"
                  ? "Archivado"
                  : "Borrador"}
            </Badge>
          </div>
          <h1 className="text-2xl sm:text-3xl font-bold tracking-tight text-foreground line-clamp-1">
            {currentListing.title}
          </h1>
          <p className="text-xs text-muted-foreground flex items-center gap-2">
            <span>
              ID: <code className="font-mono">{currentListing.id}</code>
            </span>
            <span>·</span>
            <span>Zona: {currentListing.time_zone || "No configurada"}</span>
          </p>
        </div>

        {/* Acciones Globales */}
        <div className="flex items-center gap-2 flex-wrap shrink-0">
          {currentListing.state === "borrador" && (
            <>
              <Button
                variant="default"
                size="sm"
                disabled={isPending || !publishCheck.eligible}
                onClick={handleGlobalPublish}
                className="gap-1.5 font-semibold bg-emerald-600 hover:bg-emerald-700 text-white shadow-xs"
                title={
                  publishCheck.eligible
                    ? "Publicar espacio"
                    : publishCheck.reasons.join(", ")
                }
              >
                <Send className="size-4" aria-hidden="true" />
                Publicar
              </Button>
            </>
          )}

          {currentListing.state === "activa" && (
            <Button
              variant="outline"
              size="sm"
              disabled={isPending}
              onClick={handleGlobalArchive}
              className="gap-1.5 text-amber-700 hover:bg-amber-50"
            >
              <Archive className="size-4" aria-hidden="true" />
              Archivar
            </Button>
          )}

          {currentListing.state === "oculta" && (
            <Button
              variant="default"
              size="sm"
              disabled={isPending || !publishCheck.eligible}
              onClick={handleGlobalPublish}
              className="gap-1.5 bg-emerald-600 hover:bg-emerald-700 text-white"
            >
              <Send className="size-4" aria-hidden="true" />
              Reactivar
            </Button>
          )}

          {(currentListing.state === "activa" ||
            currentListing.state === "oculta") && (
            <Button
              variant="outline"
              size="sm"
              disabled={isPending}
              onClick={handleGlobalRetire}
              className="gap-1.5 text-destructive hover:bg-destructive/10"
            >
              <Trash2 className="size-4" aria-hidden="true" />
              Retirar publicación
            </Button>
          )}
        </div>
      </div>

      {/* Requisitos para publicar si está en borrador */}
      {currentListing.state === "borrador" && !publishCheck.eligible && (
        <div
          role="alert"
          className="rounded-lg border border-amber-300 bg-amber-50 p-4 text-xs text-amber-900 space-y-1.5"
        >
          <div className="font-semibold flex items-center gap-2">
            <AlertCircle className="size-4 text-amber-700" aria-hidden="true" />
            Requisitos pendientes para poder publicar tu espacio:
          </div>
          <ul className="list-disc list-inside space-y-0.5 text-amber-800 pl-2">
            {publishCheck.reasons.map((r, i) => (
              <li key={i}>{r}</li>
            ))}
          </ul>
        </div>
      )}

      {/* Mensajes de Éxito / Error */}
      {feedbackSuccess && (
        <div
          role="status"
          className="rounded-lg border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-800 flex items-start gap-3"
        >
          <CheckCircle2
            className="size-5 shrink-0 text-emerald-600 mt-0.5"
            aria-hidden="true"
          />
          <div className="flex-1">{feedbackSuccess}</div>
          <button
            type="button"
            onClick={() => setFeedbackSuccess(null)}
            className="text-emerald-700 hover:text-emerald-900 text-xs font-semibold"
          >
            ✕
          </button>
        </div>
      )}

      {feedbackError && (
        <div
          role="alert"
          className="rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive flex items-start gap-3"
        >
          <AlertCircle className="size-5 shrink-0 mt-0.5" aria-hidden="true" />
          <div className="flex-1">{feedbackError}</div>
          <button
            type="button"
            onClick={() => setFeedbackError(null)}
            className="text-destructive hover:underline text-xs font-semibold"
          >
            ✕
          </button>
        </div>
      )}

      {/* Navegación por Pestañas (Tablist accesible) */}
      <div
        role="tablist"
        aria-label="Secciones de edición del espacio"
        className="flex items-center gap-2 border-b border-border overflow-x-auto pb-px"
      >
        <button
          type="button"
          role="tab"
          id="tab-info"
          aria-controls="panel-info"
          aria-selected={activeTab === "info"}
          onClick={() => setActiveTab("info")}
          className={`inline-flex items-center gap-2 px-4 py-2.5 text-sm font-medium border-b-2 whitespace-nowrap transition-colors ${
            activeTab === "info"
              ? "border-primary text-primary font-semibold"
              : "border-transparent text-muted-foreground hover:text-foreground"
          }`}
        >
          <FileText className="size-4" aria-hidden="true" />
          Información y atributos
        </button>

        <button
          type="button"
          role="tab"
          id="tab-pricing"
          aria-controls="panel-pricing"
          aria-selected={activeTab === "pricing"}
          onClick={() => setActiveTab("pricing")}
          className={`inline-flex items-center gap-2 px-4 py-2.5 text-sm font-medium border-b-2 whitespace-nowrap transition-colors ${
            activeTab === "pricing"
              ? "border-primary text-primary font-semibold"
              : "border-transparent text-muted-foreground hover:text-foreground"
          }`}
        >
          <DollarSign className="size-4" aria-hidden="true" />
          Tarifas e historial
        </button>

        <button
          type="button"
          role="tab"
          id="tab-schedule"
          aria-controls="panel-schedule"
          aria-selected={activeTab === "schedule"}
          onClick={() => setActiveTab("schedule")}
          className={`inline-flex items-center gap-2 px-4 py-2.5 text-sm font-medium border-b-2 whitespace-nowrap transition-colors ${
            activeTab === "schedule"
              ? "border-primary text-primary font-semibold"
              : "border-transparent text-muted-foreground hover:text-foreground"
          }`}
        >
          <Calendar className="size-4" aria-hidden="true" />
          Calendario y horarios
        </button>

        <button
          type="button"
          role="tab"
          id="tab-photos"
          aria-controls="panel-photos"
          aria-selected={activeTab === "photos"}
          onClick={() => setActiveTab("photos")}
          className={`inline-flex items-center gap-2 px-4 py-2.5 text-sm font-medium border-b-2 whitespace-nowrap transition-colors ${
            activeTab === "photos"
              ? "border-primary text-primary font-semibold"
              : "border-transparent text-muted-foreground hover:text-foreground"
          }`}
        >
          <ImageIcon className="size-4" aria-hidden="true" />
          Fotografías ({snapshot.photos.length})
        </button>
      </div>

      {/* PESTAÑA 1: Información y atributos */}
      <div
        role="tabpanel"
        id="panel-info"
        aria-labelledby="tab-info"
        hidden={activeTab !== "info"}
      >
        {activeTab === "info" && (
          <form onSubmit={handleSaveContent}>
            <Card>
              <CardHeader>
                <CardTitle className="text-lg">
                  Detalles de la propiedad
                </CardTitle>
                <CardDescription>
                  Edita el título, descripción general y características del
                  espacio.
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-6">
                <div className="space-y-2">
                  <Label htmlFor="editor-title">Título del anuncio *</Label>
                  <Input
                    id="editor-title"
                    ref={titleRef}
                    aria-invalid={Boolean(fieldErrors.title)}
                    aria-describedby={
                      fieldErrors.title
                        ? "editor-title-error"
                        : "editor-title-count"
                    }
                    value={title}
                    onChange={(e) => {
                      setTitle(e.target.value);
                      setFieldErrors((errors) => ({ ...errors, title: "" }));
                    }}
                    required
                  />
                  <p
                    id="editor-title-count"
                    className="text-xs text-muted-foreground"
                  >
                    {Array.from(title).length}/70 caracteres
                  </p>
                  {fieldErrors.title && (
                    <p
                      id="editor-title-error"
                      className="text-sm text-destructive"
                    >
                      {fieldErrors.title}
                    </p>
                  )}
                </div>

                <div className="space-y-2">
                  <Label htmlFor="editor-desc">Descripción detallada *</Label>
                  <textarea
                    id="editor-desc"
                    ref={descriptionRef}
                    aria-invalid={Boolean(fieldErrors.description)}
                    aria-describedby={
                      fieldErrors.description
                        ? "editor-desc-error"
                        : "editor-desc-count"
                    }
                    rows={4}
                    value={description}
                    onChange={(e) => {
                      setDescription(e.target.value);
                      setFieldErrors((errors) => ({
                        ...errors,
                        description: "",
                      }));
                    }}
                    className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
                    required
                  />
                  <p
                    id="editor-desc-count"
                    className="text-xs text-muted-foreground"
                  >
                    {Array.from(description).length}/100 caracteres mínimos
                  </p>
                  {fieldErrors.description && (
                    <p
                      id="editor-desc-error"
                      className="text-sm text-destructive"
                    >
                      {fieldErrors.description}
                    </p>
                  )}
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                  <div className="space-y-2 sm:col-span-2">
                    <Label htmlFor="editor-address">Dirección *</Label>
                    <Input
                      id="editor-address"
                      ref={addressRef}
                      aria-invalid={Boolean(fieldErrors.address)}
                      aria-describedby={
                        fieldErrors.address
                          ? "editor-address-error"
                          : "editor-address-count"
                      }
                      value={address}
                      onChange={(e) => {
                        setAddress(e.target.value);
                        setFieldErrors((errors) => ({
                          ...errors,
                          address: "",
                        }));
                      }}
                      required
                    />
                    <p
                      id="editor-address-count"
                      className="text-xs text-muted-foreground"
                    >
                      {Array.from(address).length}/500 caracteres
                    </p>
                    {fieldErrors.address && (
                      <p
                        id="editor-address-error"
                        className="text-sm text-destructive"
                      >
                        {fieldErrors.address}
                      </p>
                    )}
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="editor-comuna">Comuna</Label>
                    <Input
                      id="editor-comuna"
                      aria-invalid={Boolean(fieldErrors.comuna_code)}
                      aria-describedby={
                        fieldErrors.comuna_code
                          ? "editor-comuna-error"
                          : undefined
                      }
                      value={comunaCode}
                      onChange={(e) => {
                        setComunaCode(e.target.value);
                        setFieldErrors((errors) => ({
                          ...errors,
                          comuna_code: "",
                        }));
                      }}
                      placeholder="Ej: Santiago o 13101"
                    />
                    {fieldErrors.comuna_code && (
                      <p
                        id="editor-comuna-error"
                        className="text-sm text-destructive"
                      >
                        {fieldErrors.comuna_code}
                      </p>
                    )}
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="editor-region">Región</Label>
                    <select
                      id="editor-region"
                      aria-invalid={Boolean(fieldErrors.region_code)}
                      aria-describedby={
                        fieldErrors.region_code
                          ? "editor-region-error"
                          : undefined
                      }
                      value={regionCode}
                      onChange={(e) => {
                        setRegionCode(e.target.value);
                        setFieldErrors((errors) => ({
                          ...errors,
                          region_code: "",
                        }));
                      }}
                      className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
                    >
                      <option value="13">Región Metropolitana (13)</option>
                      <option value="05">Valparaíso (05)</option>
                      <option value="08">Biobío (08)</option>
                      <option value="04">Coquimbo (04)</option>
                      <option value="09">La Araucanía (09)</option>
                      <option value="10">Los Lagos (10)</option>
                    </select>
                    {fieldErrors.region_code && (
                      <p
                        id="editor-region-error"
                        className="text-sm text-destructive"
                      >
                        {fieldErrors.region_code}
                      </p>
                    )}
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="editor-capacity">
                      Capacidad máxima (personas) *
                    </Label>
                    <Input
                      id="editor-capacity"
                      ref={capacityRef}
                      aria-invalid={Boolean(fieldErrors.capacity)}
                      aria-describedby={
                        fieldErrors.capacity
                          ? "editor-capacity-error"
                          : undefined
                      }
                      type="number"
                      step="1"
                      value={capacity}
                      onChange={(e) => {
                        setCapacity(Number(e.target.value));
                        setFieldErrors((errors) => ({
                          ...errors,
                          capacity: "",
                        }));
                      }}
                      required
                    />
                    {fieldErrors.capacity && (
                      <p
                        id="editor-capacity-error"
                        className="text-sm text-destructive"
                      >
                        {fieldErrors.capacity}
                      </p>
                    )}
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="editor-area">Superficie (m²) *</Label>
                    <Input
                      id="editor-area"
                      ref={areaRef}
                      aria-invalid={Boolean(fieldErrors.area_m2)}
                      aria-describedby={
                        fieldErrors.area_m2 ? "editor-area-error" : undefined
                      }
                      type="number"
                      step="any"
                      value={areaM2}
                      onChange={(e) => {
                        setAreaM2(Number(e.target.value));
                        setFieldErrors((errors) => ({
                          ...errors,
                          area_m2: "",
                        }));
                      }}
                      required
                    />
                    {fieldErrors.area_m2 && (
                      <p
                        id="editor-area-error"
                        className="text-sm text-destructive"
                      >
                        {fieldErrors.area_m2}
                      </p>
                    )}
                  </div>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="editor-rules">
                    Reglas de uso y convivencia
                  </Label>
                  <textarea
                    id="editor-rules"
                    ref={rulesRef}
                    aria-invalid={Boolean(fieldErrors.usage_rules)}
                    aria-describedby={
                      fieldErrors.usage_rules
                        ? "editor-rules-error"
                        : "editor-rules-count"
                    }
                    rows={3}
                    value={usageRules}
                    onChange={(e) => {
                      setUsageRules(e.target.value);
                      setFieldErrors((errors) => ({
                        ...errors,
                        usage_rules: "",
                      }));
                    }}
                    className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
                  />
                  <p
                    id="editor-rules-count"
                    className="text-xs text-muted-foreground"
                  >
                    {Array.from(usageRules).length}/250 caracteres
                  </p>
                  {fieldErrors.usage_rules && (
                    <p
                      id="editor-rules-error"
                      className="text-sm text-destructive"
                    >
                      {fieldErrors.usage_rules}
                    </p>
                  )}
                </div>

                {/* Atributos dinámicos de categoría */}
                {currentSchema && currentSchema.attributes.length > 0 && (
                  <div className="border-t pt-4 space-y-3">
                    <div className="flex items-center justify-between">
                      <h2 className="text-sm font-semibold text-foreground">
                        Atributos específicos (
                        {currentListing.category_name ||
                          currentListing.category_code}
                        )
                      </h2>
                      <Badge variant="outline" className="text-xs font-mono">
                        versión {currentListing.attribute_schema_version}
                      </Badge>
                    </div>

                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 bg-muted/20 p-4 rounded-lg border">
                      {currentSchema.attributes.map((def) => {
                        const val = attributes[def.code];
                        const inputId = `attr-editor-${encodeURIComponent(def.code)}`;
                        const attributeError =
                          fieldErrors[`attributes.${def.code}`] ?? "";
                        const attributeErrorId = `${inputId}-error`;
                        const clearAttributeError = () =>
                          setFieldErrors((errors) => ({
                            ...errors,
                            [`attributes.${def.code}`]: "",
                          }));

                        if (def.type === "boolean") {
                          return (
                            <div key={def.code} className="space-y-1">
                              <div className="flex items-center justify-between p-2.5 rounded bg-background border">
                                <Label
                                  htmlFor={inputId}
                                  className="text-xs cursor-pointer"
                                >
                                  {def.label}
                                </Label>
                                <input
                                  id={inputId}
                                  type="checkbox"
                                  aria-invalid={Boolean(attributeError)}
                                  aria-describedby={
                                    attributeError
                                      ? attributeErrorId
                                      : undefined
                                  }
                                  checked={Boolean(val)}
                                  onChange={(e) => {
                                    setAttributes((prev) => ({
                                      ...prev,
                                      [def.code]: e.target.checked,
                                    }));
                                    clearAttributeError();
                                  }}
                                  className="size-4 rounded text-primary focus:ring-primary"
                                />
                              </div>
                              {attributeError && (
                                <p
                                  id={attributeErrorId}
                                  className="text-sm text-destructive"
                                >
                                  {attributeError}
                                </p>
                              )}
                            </div>
                          );
                        }

                        if (def.type === "integer" || def.type === "number") {
                          return (
                            <div key={def.code} className="space-y-1">
                              <Label
                                htmlFor={inputId}
                                className="text-xs font-semibold"
                              >
                                {def.label} {def.unit ? `(${def.unit})` : ""}
                              </Label>
                              <Input
                                id={inputId}
                                aria-invalid={Boolean(attributeError)}
                                aria-describedby={
                                  attributeError ? attributeErrorId : undefined
                                }
                                type="number"
                                min={def.minimum}
                                max={def.maximum}
                                step={def.step ?? 1}
                                value={val !== undefined ? String(val) : ""}
                                onChange={(e) => {
                                  setAttributes((prev) => ({
                                    ...prev,
                                    [def.code]:
                                      e.target.value === ""
                                        ? undefined
                                        : Number(e.target.value),
                                  }));
                                  clearAttributeError();
                                }}
                              />
                              {attributeError && (
                                <p
                                  id={attributeErrorId}
                                  className="text-sm text-destructive"
                                >
                                  {attributeError}
                                </p>
                              )}
                            </div>
                          );
                        }

                        if (def.type === "enum_list" && def.options) {
                          const options = def.options;
                          const selected = Array.isArray(val)
                            ? options.filter(
                                (option) =>
                                  typeof option === "string" &&
                                  val.includes(option),
                              )
                            : [];
                          const fieldsetId = `attr-editor-${encodeURIComponent(def.code)}`;
                          return (
                            <fieldset key={def.code} className="space-y-2">
                              <legend className="text-xs font-semibold">
                                {def.label}
                              </legend>
                              <div className="space-y-2">
                                {def.options.map((option, index) => {
                                  const optionId = `${fieldsetId}-option-${index}`;
                                  return (
                                    <div
                                      key={`${index}-${option}`}
                                      className="flex items-center gap-2"
                                    >
                                      <input
                                        id={optionId}
                                        type="checkbox"
                                        aria-invalid={Boolean(attributeError)}
                                        aria-describedby={
                                          attributeError
                                            ? attributeErrorId
                                            : undefined
                                        }
                                        checked={selected.includes(option)}
                                        onChange={(event) => {
                                          setAttributes((previous) => {
                                            const currentValue =
                                              previous[def.code];
                                            const current: string[] =
                                              Array.isArray(currentValue)
                                                ? (
                                                    currentValue as unknown[]
                                                  ).filter(
                                                    (item): item is string =>
                                                      typeof item ===
                                                        "string" &&
                                                      options.includes(item),
                                                  )
                                                : [];
                                            const next = event.target.checked
                                              ? [...current, option]
                                              : current.filter(
                                                  (item) => item !== option,
                                                );
                                            return {
                                              ...previous,
                                              [def.code]: def.options!.filter(
                                                (item) => next.includes(item),
                                              ),
                                            };
                                          });
                                          clearAttributeError();
                                        }}
                                        className="size-4 rounded text-primary focus:ring-primary"
                                      />
                                      <Label
                                        htmlFor={optionId}
                                        className="text-xs cursor-pointer"
                                      >
                                        {option}
                                      </Label>
                                    </div>
                                  );
                                })}
                              </div>
                              {attributeError && (
                                <p
                                  id={attributeErrorId}
                                  className="text-sm text-destructive"
                                >
                                  {attributeError}
                                </p>
                              )}
                            </fieldset>
                          );
                        }

                        if (def.type === "enum" && def.options) {
                          return (
                            <div key={def.code} className="space-y-1">
                              <Label
                                htmlFor={inputId}
                                className="text-xs font-semibold"
                              >
                                {def.label}
                              </Label>
                              <select
                                id={inputId}
                                aria-required={def.required || undefined}
                                aria-invalid={Boolean(attributeError)}
                                aria-describedby={
                                  attributeError ? attributeErrorId : undefined
                                }
                                value={typeof val === "string" ? val : ""}
                                onChange={(e) => {
                                  setAttributes((prev) => ({
                                    ...prev,
                                    [def.code]: e.target.value,
                                  }));
                                  clearAttributeError();
                                }}
                                className="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm"
                              >
                                <option value="">-- Selecciona --</option>
                                {def.options.map((opt) => (
                                  <option key={opt} value={opt}>
                                    {opt}
                                  </option>
                                ))}
                              </select>
                              {attributeError && (
                                <p
                                  id={attributeErrorId}
                                  className="text-sm text-destructive"
                                >
                                  {attributeError}
                                </p>
                              )}
                            </div>
                          );
                        }

                        return null;
                      })}
                    </div>
                  </div>
                )}
              </CardContent>
              <CardFooter className="border-t pt-4 flex justify-end">
                <Button type="submit" disabled={isPending} className="gap-2">
                  {isPending ? (
                    <Loader2
                      className="size-4 animate-spin"
                      aria-hidden="true"
                    />
                  ) : (
                    <Save className="size-4" aria-hidden="true" />
                  )}
                  Guardar contenido
                </Button>
              </CardFooter>
            </Card>
          </form>
        )}
      </div>

      {/* PESTAÑA 2: Tarifas e Historial */}
      <div
        role="tabpanel"
        id="panel-pricing"
        aria-labelledby="tab-pricing"
        hidden={activeTab !== "pricing"}
      >
        {activeTab === "pricing" && (
          <div className="space-y-6">
            <form onSubmit={handleSavePricing}>
              <Card>
                <CardHeader>
                  <CardTitle className="text-lg">Tarifas del espacio</CardTitle>
                  <CardDescription>
                    Actualiza el precio base por hora, día o mes. Cada
                    actualización generará una nueva versión auditada.
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                    <div className="space-y-2">
                      <Label htmlFor="editor-rate-unit">
                        Unidad de cobro *
                      </Label>
                      <select
                        id="editor-rate-unit"
                        value={rateUnit}
                        onChange={(e) =>
                          setRateUnit(e.target.value as RateUnit)
                        }
                        className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
                      >
                        <option value="hora">Por Hora</option>
                        <option value="dia">Por Día</option>
                        <option value="mes">Por Mes</option>
                      </select>
                    </div>

                    <div className="space-y-2">
                      <Label htmlFor="editor-price">Precio base (CLP) *</Label>
                      <Input
                        id="editor-price"
                        type="number"
                        step="any"
                        aria-invalid={Boolean(fieldErrors.base_price)}
                        aria-describedby={
                          fieldErrors.base_price
                            ? "editor-price-error"
                            : undefined
                        }
                        value={basePrice}
                        onChange={(e) => {
                          setBasePrice(Number(e.target.value));
                          setFieldErrors((errors) => ({
                            ...errors,
                            base_price: "",
                          }));
                        }}
                        required
                      />
                      {fieldErrors.base_price && (
                        <p
                          id="editor-price-error"
                          className="text-sm text-destructive"
                        >
                          {fieldErrors.base_price}
                        </p>
                      )}
                      <p className="text-xs text-muted-foreground">
                        Precio actual configurado: $
                        {new Intl.NumberFormat("es-CL").format(basePrice)} CLP /
                        {rateUnit}
                      </p>
                    </div>
                  </div>
                </CardContent>
                <CardFooter className="border-t pt-4 flex justify-end">
                  <Button type="submit" disabled={isPending} className="gap-2">
                    {isPending ? (
                      <Loader2
                        className="size-4 animate-spin"
                        aria-hidden="true"
                      />
                    ) : (
                      <Save className="size-4" aria-hidden="true" />
                    )}
                    Actualizar tarifa
                  </Button>
                </CardFooter>
              </Card>
            </form>

            {/* Historial de tarifas */}
            <Card>
              <CardHeader>
                <div className="flex items-center gap-2">
                  <History
                    className="size-5 text-muted-foreground"
                    aria-hidden="true"
                  />
                  <div>
                    <CardTitle className="text-lg">
                      Historial de cambios de tarifa
                    </CardTitle>
                    <CardDescription>
                      Registro inmutable de versiones de precios y fechas de
                      vigencia.
                    </CardDescription>
                  </div>
                </div>
              </CardHeader>
              <CardContent>
                {snapshot.tariffHistory.length === 0 ? (
                  <p className="text-xs text-muted-foreground italic py-3">
                    No hay registros históricos previos para este espacio.
                  </p>
                ) : (
                  <div className="overflow-x-auto">
                    <table className="w-full text-left text-xs">
                      <thead>
                        <tr className="border-b text-muted-foreground font-semibold">
                          <th className="py-2 pr-4">Versión</th>
                          <th className="py-2 px-4">Precio Base</th>
                          <th className="py-2 px-4">Unidad</th>
                          <th className="py-2 pl-4">Fecha Registro</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-border">
                        {snapshot.tariffHistory.map(
                          (item: PricingHistoryItem) => (
                            <tr
                              key={item.version}
                              className="hover:bg-muted/30"
                            >
                              <td className="py-2.5 pr-4 font-mono font-medium">
                                v{item.version}
                              </td>
                              <td className="py-2.5 px-4 font-bold text-foreground">
                                $
                                {new Intl.NumberFormat("es-CL").format(
                                  item.base_price,
                                )}{" "}
                                {item.currency}
                              </td>
                              <td className="py-2.5 px-4 capitalize">
                                por {item.rate_unit}
                              </td>
                              <td className="py-2.5 pl-4 text-muted-foreground">
                                {new Date(item.created_at).toLocaleDateString(
                                  "es-CL",
                                  {
                                    year: "numeric",
                                    month: "short",
                                    day: "numeric",
                                    hour: "2-digit",
                                    minute: "2-digit",
                                  },
                                )}
                              </td>
                            </tr>
                          ),
                        )}
                      </tbody>
                    </table>
                  </div>
                )}
              </CardContent>
            </Card>
          </div>
        )}
      </div>

      {/* PESTAÑA 3: Calendario y Horarios */}
      <div
        role="tabpanel"
        id="panel-schedule"
        aria-labelledby="tab-schedule"
        hidden={activeTab !== "schedule"}
      >
        {activeTab === "schedule" && (
          <form onSubmit={handleSaveSchedule}>
            <Card>
              <CardHeader>
                <CardTitle className="text-lg">
                  Disponibilidad semanal
                </CardTitle>
                <CardDescription>
                  Define los tramos en los que el espacio está abierto para
                  reservas directas.
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-6">
                <div className="flex items-center justify-between p-4 rounded-xl bg-muted/40 border">
                  <div>
                    <Label
                      htmlFor="editor-sched-enable"
                      className="text-sm font-semibold text-foreground cursor-pointer"
                    >
                      Calendario habilitado
                    </Label>
                    <p className="text-xs text-muted-foreground mt-0.5">
                      Permite a los usuarios seleccionar turnos según este
                      horario.
                    </p>
                  </div>
                  <input
                    type="checkbox"
                    id="editor-sched-enable"
                    aria-label="Calendario habilitado"
                    checked={scheduleEnabled}
                    onChange={(e) => setScheduleEnabled(e.target.checked)}
                    className="size-5 rounded text-primary focus:ring-primary"
                  />
                </div>

                <div className="space-y-2">
                  <Label htmlFor="editor-timezone">Zona horaria IANA *</Label>
                  <Input
                    id="editor-timezone"
                    value={timeZone}
                    onChange={(e) => setTimeZone(e.target.value)}
                    placeholder="America/Santiago"
                    required
                  />
                  <p className="text-xs text-muted-foreground">
                    Requerido para activar la publicación. Ejemplo:{" "}
                    <code>America/Santiago</code>.
                  </p>
                </div>

                <div className="space-y-3 pt-2">
                  <h2 className="text-sm font-semibold text-foreground">
                    Horarios por día
                  </h2>

                  <div className="space-y-3">
                    {DAYS_MAP.map((day) => {
                      const daySlots = scheduleSlots
                        .map((s, idx) => ({ ...s, originalIndex: idx }))
                        .filter((s) => s.day_of_week === day.day);

                      return (
                        <div
                          key={day.day}
                          className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 p-3 rounded-lg border bg-background"
                        >
                          <div className="w-28 font-medium text-sm text-foreground">
                            {day.name}
                          </div>

                          <div className="flex-1 flex flex-col gap-2">
                            {daySlots.length === 0 ? (
                              <span className="text-xs text-muted-foreground italic">
                                Cerrado
                              </span>
                            ) : (
                              daySlots.map((slot) => (
                                <div
                                  key={slot.originalIndex}
                                  className="flex items-center gap-2 flex-wrap text-xs"
                                >
                                  <input
                                    type="time"
                                    value={slot.start_time}
                                    onChange={(e) =>
                                      setScheduleSlots((prev) =>
                                        prev.map((s, i) =>
                                          i === slot.originalIndex
                                            ? {
                                                ...s,
                                                start_time: e.target.value,
                                              }
                                            : s,
                                        ),
                                      )
                                    }
                                    aria-label={`Hora apertura ${day.name}`}
                                    className="rounded border border-input px-2 py-1 bg-background text-xs"
                                  />
                                  <span>a</span>
                                  <input
                                    type="time"
                                    value={slot.end_time}
                                    onChange={(e) =>
                                      setScheduleSlots((prev) =>
                                        prev.map((s, i) =>
                                          i === slot.originalIndex
                                            ? { ...s, end_time: e.target.value }
                                            : s,
                                        ),
                                      )
                                    }
                                    aria-label={`Hora cierre ${day.name}`}
                                    className="rounded border border-input px-2 py-1 bg-background text-xs"
                                  />
                                  <button
                                    type="button"
                                    onClick={() =>
                                      setScheduleSlots((prev) =>
                                        prev.filter(
                                          (_, i) => i !== slot.originalIndex,
                                        ),
                                      )
                                    }
                                    className="text-destructive hover:bg-destructive/10 p-1 rounded"
                                    aria-label={`Eliminar tramo ${day.name}`}
                                  >
                                    <Trash2
                                      className="size-3.5"
                                      aria-hidden="true"
                                    />
                                  </button>
                                </div>
                              ))
                            )}
                          </div>

                          {daySlots.length < 2 && (
                            <Button
                              type="button"
                              variant="ghost"
                              size="sm"
                              onClick={() => {
                                setScheduleSlots((prev) => [
                                  ...prev,
                                  {
                                    day_of_week: day.day,
                                    start_time: "09:00",
                                    end_time: "18:00",
                                  },
                                ]);
                              }}
                              className="gap-1 text-xs self-start sm:self-auto"
                            >
                              <Plus className="size-3.5" aria-hidden="true" />
                              Agregar tramo
                            </Button>
                          )}
                        </div>
                      );
                    })}
                  </div>
                </div>
              </CardContent>
              <CardFooter className="border-t pt-4 flex justify-end">
                <Button type="submit" disabled={isPending} className="gap-2">
                  {isPending ? (
                    <Loader2
                      className="size-4 animate-spin"
                      aria-hidden="true"
                    />
                  ) : (
                    <Save className="size-4" aria-hidden="true" />
                  )}
                  Guardar horarios
                </Button>
              </CardFooter>
            </Card>
          </form>
        )}
      </div>

      {/* PESTAÑA 4: Fotografías y Portada */}
      <div
        role="tabpanel"
        id="panel-photos"
        aria-labelledby="tab-photos"
        hidden={activeTab !== "photos"}
      >
        {activeTab === "photos" && (
          <div className="space-y-6">
            <Card>
              <CardHeader className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
                <div>
                  <CardTitle className="text-lg">Galería de imágenes</CardTitle>
                  <CardDescription>
                    Agrega imágenes del espacio. Puedes elegir una portada y
                    organizar su orden.
                  </CardDescription>
                </div>
                <div className="w-full sm:max-w-xs space-y-2">
                  <Label htmlFor="gallery-upload">Subir imágenes</Label>
                  <Input
                    id="gallery-upload"
                    type="file"
                    accept="image/jpeg,image/png,image/webp"
                    multiple
                    onChange={handleUploadImages}
                    disabled={
                      isPending ||
                      uploadingPhotos ||
                      replacingPhotoId !== null ||
                      pendingPhotoReplacement !== null ||
                      snapshot.photos.length >= MAX_CURRENT_GALLERY_PHOTOS
                    }
                    aria-describedby="gallery-upload-help"
                  />
                  <p
                    id="gallery-upload-help"
                    className="text-xs text-muted-foreground"
                  >
                    JPG, PNG o WebP; hasta 10 MiB por imagen y un máximo actual
                    de 30 por espacio.
                  </p>
                  <Input
                    ref={replacementInputRef}
                    id="gallery-replace-input"
                    type="file"
                    accept="image/jpeg,image/png,image/webp"
                    className="sr-only"
                    tabIndex={-1}
                    aria-hidden="true"
                    onChange={handleReplaceImage}
                  />
                  {photoUploads.length > 0 && (
                    <ul aria-live="polite" className="space-y-1 text-xs">
                      {photoUploads.map((item) => (
                        <li key={item.id} className="flex items-center gap-2">
                          <span
                            role="status"
                            className="flex items-center gap-2"
                          >
                            {item.status === "uploading" && (
                              <Loader2
                                className="size-3 animate-spin"
                                aria-hidden="true"
                              />
                            )}
                            {item.status === "queued"
                              ? `En cola ${item.filename}`
                              : item.status === "uploading"
                                ? `Subiendo ${item.filename}`
                                : item.status === "completed"
                                  ? `Carga completada: ${item.filename}`
                                  : item.status === "pending"
                                    ? `Carga pendiente de confirmación: ${item.filename}`
                                    : item.status === "cancelled"
                                      ? `Carga cancelada: ${item.filename}`
                                      : `Error al subir ${item.filename}: ${item.error}`}
                          </span>
                          {item.error && (
                            <span className="text-amber-800 dark:text-amber-300">
                              {item.error}
                            </span>
                          )}
                          {(item.status === "queued" ||
                            (item.status === "uploading" &&
                              item.cancellable !== false)) && (
                            <Button
                              type="button"
                              variant="outline"
                              size="sm"
                              aria-label={`Cancelar carga ${item.filename}`}
                              onClick={() =>
                                item.status === "queued"
                                  ? handleCancelQueuedPhoto(item.id)
                                  : handleCancelDispatchedPhoto(item.id)
                              }
                            >
                              Cancelar
                            </Button>
                          )}
                          {(item.status === "failed" ||
                            item.status === "pending") && (
                            <Button
                              type="button"
                              variant="outline"
                              size="sm"
                              disabled={uploadingPhotos}
                              aria-label={`${item.status === "pending" ? "Revisar estado" : "Reintentar"} ${item.filename}`}
                              onClick={() => void handleRetryPhoto(item.id)}
                            >
                              {item.status === "pending"
                                ? "Revisar estado"
                                : "Reintentar"}
                            </Button>
                          )}
                        </li>
                      ))}
                    </ul>
                  )}
                  {pendingPhotoReplacement && (
                    <div className="flex flex-wrap items-center gap-2 rounded-md border border-amber-300 p-2 text-xs">
                      <p role="status" aria-live="polite">
                        {pendingPhotoReplacement.status === "pending"
                          ? `Reemplazo pendiente: ${pendingPhotoReplacement.filename}. La foto actual se conserva.`
                          : `Reemplazo no confirmado: ${pendingPhotoReplacement.filename}. La foto actual se conserva.`}
                      </p>
                      {pendingPhotoReplacement.status === "pending" ? (
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          disabled={checkingPhotoReplacement}
                          aria-label={`Revisar estado del reemplazo ${pendingPhotoReplacement.filename}`}
                          onClick={() =>
                            void handleReviewPendingPhotoReplacement()
                          }
                        >
                          {checkingPhotoReplacement
                            ? "Consultando estado…"
                            : "Revisar estado del reemplazo"}
                        </Button>
                      ) : (
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          disabled={replacingPhotoId !== null}
                          aria-label={`Reintentar reemplazo ${pendingPhotoReplacement.filename}`}
                          onClick={() =>
                            void handleRetryPendingPhotoReplacement()
                          }
                        >
                          Reintentar reemplazo
                        </Button>
                      )}
                    </div>
                  )}
                </div>
              </CardHeader>
              <CardContent>
                {snapshot.photos.length === 0 ? (
                  <div className="border-2 border-dashed rounded-xl p-10 text-center space-y-3">
                    <ImageIcon
                      className="size-10 text-muted-foreground mx-auto"
                      aria-hidden="true"
                    />
                    <p className="text-sm font-medium text-foreground">
                      Aún no hay imágenes
                    </p>
                    <p className="text-xs text-muted-foreground max-w-sm mx-auto">
                      Selecciona imágenes desde el control superior para
                      comenzar tu galería.
                    </p>
                  </div>
                ) : (
                  <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-4">
                    {snapshot.photos.map((photo: GalleryPhoto, idx: number) => {
                      const isCover =
                        photo.is_cover ||
                        currentListing.cover_photo_id === photo.id;

                      return (
                        <div
                          key={photo.id}
                          className={`relative rounded-xl border overflow-hidden p-3 flex flex-col justify-between space-y-3 transition-all ${
                            isCover
                              ? "border-primary ring-2 ring-primary/20 bg-primary/5"
                              : "bg-card hover:border-border"
                          }`}
                        >
                          {/* Cabecera miniatura */}
                          <div className="flex items-center justify-between">
                            <span className="text-xs font-mono text-muted-foreground">
                              #{idx + 1}
                            </span>
                            {isCover ? (
                              <Badge
                                variant="default"
                                className="gap-1 bg-primary text-xs"
                              >
                                <Star
                                  className="size-3 fill-current"
                                  aria-hidden="true"
                                />
                                Portada
                              </Badge>
                            ) : (
                              <Button
                                type="button"
                                variant="ghost"
                                size="sm"
                                onClick={() => handleSetCover(photo.id)}
                                disabled={
                                  isPending ||
                                  replacingPhotoId !== null ||
                                  pendingPhotoReplacement !== null
                                }
                                className="h-6 px-2 text-xs text-muted-foreground hover:text-foreground"
                              >
                                Marcar portada
                              </Button>
                            )}
                          </div>

                          {/* Variante privada descargada vía BFF y revocada al salir */}
                          {photo.is_synthetic ? (
                            <div className="relative h-32 overflow-hidden rounded-lg border bg-muted/60">
                              <div className="flex h-full flex-col items-center justify-center p-2 text-center">
                                <ImageIcon
                                  className="size-6 text-muted-foreground mb-1"
                                  aria-hidden="true"
                                />
                                <span className="text-xs font-medium text-foreground line-clamp-1">
                                  {photo.alt_text || `Imagen ${idx + 1}`}
                                </span>
                              </div>
                            </div>
                          ) : photo.variants?.some(
                              (variant) => variant.name === "thumbnail",
                            ) ? (
                            <PrivateGalleryThumbnail
                              key={`${authSnapshot.accountId ?? "anonymous"}:${listingId}:${photo.id}`}
                              accountId={authSnapshot.accountId}
                              listingId={listingId}
                              photo={photo}
                              index={idx}
                            />
                          ) : (
                            <div className="relative h-32 overflow-hidden rounded-lg border bg-muted/60">
                              <div className="flex h-full flex-col items-center justify-center p-2 text-center">
                                <ImageIcon
                                  className="size-6 text-muted-foreground mb-1"
                                  aria-hidden="true"
                                />
                                <span className="text-xs font-medium text-foreground line-clamp-1">
                                  {photo.alt_text || `Imagen ${idx + 1}`}
                                </span>
                              </div>
                            </div>
                          )}

                          {/* Botones de acción: Mover arriba/abajo y eliminar */}
                          <div className="flex items-center justify-between border-t pt-2">
                            <div className="flex items-center gap-1">
                              <Button
                                type="button"
                                variant="outline"
                                size="sm"
                                disabled={
                                  idx === 0 ||
                                  isPending ||
                                  replacingPhotoId !== null ||
                                  pendingPhotoReplacement !== null
                                }
                                onClick={() => handleMovePhoto(idx, "up")}
                                className="size-7 p-0"
                                aria-label="Mover foto antes"
                              >
                                <ArrowUp
                                  className="size-3.5"
                                  aria-hidden="true"
                                />
                              </Button>
                              <Button
                                type="button"
                                variant="outline"
                                size="sm"
                                disabled={
                                  idx === snapshot.photos.length - 1 ||
                                  isPending ||
                                  replacingPhotoId !== null ||
                                  pendingPhotoReplacement !== null
                                }
                                onClick={() => handleMovePhoto(idx, "down")}
                                className="size-7 p-0"
                                aria-label="Mover foto después"
                              >
                                <ArrowDown
                                  className="size-3.5"
                                  aria-hidden="true"
                                />
                              </Button>
                            </div>

                            <Button
                              type="button"
                              variant="outline"
                              size="sm"
                              disabled={
                                isPending ||
                                uploadingPhotos ||
                                replacingPhotoId !== null ||
                                pendingPhotoReplacement !== null ||
                                photo.is_synthetic ||
                                !currentListing.etag
                              }
                              aria-label={`Reemplazar imagen ${idx + 1}`}
                              onClick={() => {
                                replacementTargetPhotoId.current = photo.id;
                                replacementInputRef.current?.click();
                              }}
                              className="h-7 px-2 text-xs"
                            >
                              {replacingPhotoId === photo.id
                                ? "Reemplazando…"
                                : "Reemplazar"}
                            </Button>
                            <Button
                              type="button"
                              variant="ghost"
                              size="sm"
                              onClick={() => handleRemovePhoto(photo.id)}
                              disabled={
                                isPending ||
                                uploadingPhotos ||
                                replacingPhotoId !== null ||
                                pendingPhotoReplacement !== null
                              }
                              className="h-7 px-2 text-destructive hover:bg-destructive/10 text-xs"
                              aria-label="Eliminar foto"
                            >
                              <Trash2
                                className="size-3.5 mr-1"
                                aria-hidden="true"
                              />
                              Quitar
                            </Button>
                          </div>
                        </div>
                      );
                    })}
                  </div>
                )}
              </CardContent>
            </Card>
          </div>
        )}
      </div>
    </div>
  );
}
