"use client";

import {
  useRef,
  useEffect,
  useState,
  useTransition,
  useSyncExternalStore,
} from "react";
import { useRouter } from "next/navigation";
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
import { listingErrorMessage } from "../domain/listing-error-message";
import { useHostListings } from "./use-host-listings";
import {
  hostListingsFacade,
  listingReturnsFacade,
} from "../host-listings-runtime";
import {
  canPublishListing,
  isValidIanaTimeZone,
  validateCategoryAttributes,
  validateListingDraftInput,
  validateScheduleSlots,
} from "../domain/validation";
import type {
  CategoryProfileSchema,
  PricingHistoryItem,
  RateUnit,
  ScheduleSlot,
} from "../domain/types";
import { ListingGallery } from "./listing-gallery";
import { ListingReturnsPanel } from "./listing-returns-panel";
import { WeeklyScheduleSlots } from "./weekly-schedule-slots";

interface ListingEditorProps {
  readonly listingId: string;
}

type TabKey = "info" | "pricing" | "schedule" | "photos";

function mapDraftServerFieldErrors(
  error: unknown,
  schema: CategoryProfileSchema | null,
): Record<string, string> {
  if (
    !(error instanceof ApiError) ||
    error.status !== 422 ||
    error.code !== "validation_error"
  )
    return {};

  const formFields = new Set([
    "title",
    "description",
    "area_m2",
    "capacity",
    "usage_rules",
    "address",
    "region_code",
    "comuna_code",
    "base_price_clp",
  ]);
  const messages: Record<string, string> = Object.create(null);
  for (const detail of error.fields) {
    const attrPrefix = "attributes.";
    const attrCode = detail.field.startsWith(attrPrefix)
      ? detail.field.slice(attrPrefix.length)
      : "";
    const isAttribute = Boolean(
      attrCode && schema?.attributes.some((item) => item.code === attrCode),
    );
    if (!formFields.has(detail.field) && !isAttribute) continue;
    const target = isAttribute
      ? `${attrPrefix}${attrCode}`
      : detail.field === "base_price_clp"
        ? "base_price"
        : detail.field;
    if (messages[target]) continue;
    const acceptedCodes: Record<string, readonly string[]> = {
      title: ["required", "max_length"],
      description: ["min_length"],
      area_m2: ["invalid_value", "minimum", "maximum", "step"],
      capacity: ["minimum"],
      usage_rules: ["required", "max_length"],
      address: ["required", "max_length"],
      base_price_clp: ["minimum"],
      region_code: ["location_pair", "invalid_value"],
      comuna_code: ["location_pair", "invalid_value"],
    };
    if (!isAttribute && !acceptedCodes[detail.field]?.includes(detail.code))
      continue;
    const pairs: Record<string, Record<string, string>> = {
      title: { max_length: "El título no puede superar 70 caracteres." },
      description: {
        min_length: "La descripción debe tener al menos 100 caracteres.",
      },
      usage_rules: {
        max_length: "Las reglas no pueden superar 250 caracteres.",
      },
      address: { max_length: "La dirección no puede superar 500 caracteres." },
      region_code: {
        location_pair: "Selecciona una región y comuna compatibles.",
      },
      comuna_code: {
        location_pair: "Selecciona una región y comuna compatibles.",
      },
    };
    const attributeMessages: Record<string, string> = {
      required: "Completa este atributo.",
      invalid_value: "Ingresa un valor válido para este atributo.",
      exceeds_area: "La superficie no puede superar el área total.",
      invalid_combination: "Revisa la combinación de atributos relacionada.",
    };
    messages[target] = isAttribute
      ? (attributeMessages[detail.code] ?? "")
      : (pairs[detail.field]?.[detail.code] ??
        (detail.code === "required"
          ? "Este campo es obligatorio."
          : detail.code === "minimum"
            ? "El valor está por debajo del mínimo permitido."
            : detail.code === "maximum"
              ? "El valor supera el máximo permitido."
              : detail.code === "step"
                ? "El valor no coincide con el intervalo permitido."
                : detail.code === "invalid_value"
                  ? "Ingresa un valor válido para este campo."
                  : ""));
  }
  return Object.fromEntries(
    Object.entries(messages).filter(([, message]) => message),
  );
}

export function ListingEditor({ listingId }: ListingEditorProps) {
  const router = useRouter();
  const { accessToken, snapshot: authSnapshot } = useAuth();
  const {
    snapshot,
    selectListing,
    loadCategorySchema,
    updateDraft,
    updateContent,
    updatePricing,
    loadPricingHistory,
    updateWeeklySchedule,
    configureTimeZone,
    uploadPhoto,
    setCoverPhoto,
    removePhoto,
    reorderPhotos,
    replacePhoto,
    loadPhotoVariant,
    getPhotoUploadStatus,
    loadPhotos,
    publishListing,
    archiveListing,
    deleteListing,
  } = useHostListings();

  const [activeTab, setActiveTab] = useState<TabKey>("info");
  const [galleryBusy, setGalleryBusy] = useState(false);
  const [isPending, startTransition] = useTransition();
  const [feedbackSuccess, setFeedbackSuccess] = useState<string | null>(null);
  const [feedbackError, setFeedbackError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const titleRef = useRef<HTMLInputElement>(null);
  const descriptionRef = useRef<HTMLTextAreaElement>(null);
  const addressRef = useRef<HTMLInputElement>(null);
  const rulesRef = useRef<HTMLTextAreaElement>(null);
  const capacityRef = useRef<HTMLInputElement>(null);
  const areaRef = useRef<HTMLInputElement>(null);
  const priceRef = useRef<HTMLInputElement>(null);
  const [initialLoaded, setInitialLoaded] = useState(false);
  const [returnsUncertainBusy, setReturnsUncertainBusy] = useState(false);
  const returnsSnapshot = useSyncExternalStore(
    listingReturnsFacade.subscribe,
    listingReturnsFacade.getSnapshot,
    listingReturnsFacade.getServerSnapshot,
  );
  const sessionScope = JSON.stringify([
    authSnapshot.status,
    authSnapshot.accountId,
    authSnapshot.roles,
    authSnapshot.capabilities,
    authSnapshot.restrictedMode,
  ]);
  const returnsBusy =
    returnsSnapshot.uncertain || returnsSnapshot.pendingReservationId !== null;

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
    if (!accessToken || !listingId) return;

    selectListing(accessToken, listingId)
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
              accessToken,
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
        setFeedbackError(listingErrorMessage(err));
      });

    loadPricingHistory(accessToken, listingId).catch(() => {});
  }, [
    accessToken,
    listingId,
    selectListing,
    loadCategorySchema,
    loadPricingHistory,
  ]);

  const currentListing = snapshot.selectedListing;
  useEffect(() => {
    if (
      !accessToken ||
      !listingId ||
      !["activa", "oculta"].includes(currentListing?.state ?? "")
    )
      return;
    void listingReturnsFacade.load(accessToken, listingId, sessionScope);
    return () => listingReturnsFacade.clear();
  }, [accessToken, listingId, sessionScope, currentListing?.state]);
  const currentSchema = currentListing
    ? snapshot.categorySchemas[
        `${currentListing.category_code}@${currentListing.attribute_schema_version}`
      ] || snapshot.categorySchemas[currentListing.category_code]
    : null;

  // Guardar Pestaña 1: Contenido y atributos
  const handleSaveContent = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!accessToken || !currentListing || returnsBusy || galleryBusy) return;
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
        setFieldErrors((errors) => ({
          ...errors,
          ...Object.fromEntries(
            Object.entries(val.errors).map(([code, message]) => [
              `attributes.${code}`,
              message,
            ]),
          ),
        }));
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
            accessToken,
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
            accessToken,
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
        setFeedbackError(listingErrorMessage(err));
      }
    });
  };

  // Guardar Pestaña 2: Tarifas
  const handleSavePricing = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!accessToken || returnsBusy || galleryBusy) return;
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
      priceRef.current?.focus();
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
          accessToken,
          listingId,
          {
            rate_unit: rateUnit,
            base_price: Number(basePrice),
          },
          currentListing.etag,
        );
        await loadPricingHistory(accessToken, listingId);
        setFeedbackSuccess("Tarifa actualizada y registrada en el historial.");
      } catch (err) {
        setFeedbackError(listingErrorMessage(err));
      }
    });
  };

  // Guardar Pestaña 3: Horarios
  const handleSaveSchedule = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!accessToken || returnsBusy || galleryBusy) return;
    setFeedbackError(null);
    setFeedbackSuccess(null);

    if (!isValidIanaTimeZone(timeZone)) {
      setFeedbackError(
        "La zona horaria debe ser un identificador IANA válido (ej: America/Santiago).",
      );
      return;
    }

    const hourlySchedule = currentListing?.rate_unit === "hora";

    if (hourlySchedule && scheduleEnabled && scheduleSlots.length > 0) {
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
          accessToken,
          listingId,
          timeZone,
          listingEtag,
        );
        if (hourlySchedule) {
          await updateWeeklySchedule(
            accessToken,
            listingId,
            {
              timezone: timeZone,
              enabled: scheduleEnabled,
              weekly_slots: scheduleSlots,
            },
            configured.etag,
          );
          setFeedbackSuccess(
            "Horario semanal habitual y zona horaria guardados.",
          );
        } else {
          setFeedbackSuccess(
            "Zona horaria guardada. El horario semanal no aplica a esta tarifa.",
          );
        }
      } catch (err) {
        setFeedbackError(listingErrorMessage(err));
      }
    });
  };

  // Acciones Globales: Publicar / Archivar / Eliminar
  const handleGlobalPublish = async () => {
    if (!accessToken || !currentListing || returnsBusy) return;
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
        await publishListing(accessToken, listingId);
        setFeedbackSuccess(
          "¡El espacio ha sido publicado exitosamente y está activo en el catálogo!",
        );
      } catch (err) {
        setFeedbackError(listingErrorMessage(err));
      }
    });
  };

  const handleGlobalArchive = async () => {
    if (!accessToken || returnsBusy) return;
    startTransition(async () => {
      try {
        await archiveListing(accessToken, listingId);
        setFeedbackSuccess(
          "El espacio ha sido archivado (oculto del catálogo público).",
        );
      } catch (err) {
        setFeedbackError(listingErrorMessage(err));
      }
    });
  };

  const handleGlobalDelete = async () => {
    if (!accessToken || !currentListing || returnsBusy) return;
    const confirmed = window.confirm(
      `¿Eliminar definitivamente el borrador "${currentListing.title}"?`,
    );
    if (!confirmed) return;

    startTransition(async () => {
      try {
        await deleteListing(accessToken, listingId);
        router.push("/anfitrion/espacios");
      } catch (err) {
        setFeedbackError(listingErrorMessage(err));
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
          <Link href="/mi-cuenta/espacios">Volver a mis espacios</Link>
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
                  ? "bg-emerald-700 text-white"
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
                disabled={
                  galleryBusy ||
                  returnsUncertainBusy ||
                  isPending ||
                  !publishCheck.eligible
                }
                onClick={handleGlobalPublish}
                className="gap-1.5 font-semibold bg-emerald-700 hover:bg-emerald-800 text-white shadow-xs"
                title={
                  publishCheck.eligible
                    ? "Publicar espacio"
                    : publishCheck.reasons.join(", ")
                }
              >
                <Send className="size-4" aria-hidden="true" />
                Publicar
              </Button>
              <Button
                variant="outline"
                size="sm"
                disabled={galleryBusy || returnsUncertainBusy || isPending}
                onClick={handleGlobalDelete}
                className="text-destructive hover:bg-destructive/10"
              >
                <Trash2 className="size-4" aria-hidden="true" />
                Eliminar
              </Button>
            </>
          )}

          {currentListing.state === "activa" && (
            <Button
              variant="outline"
              size="sm"
              disabled={galleryBusy || returnsUncertainBusy || isPending}
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
              disabled={
                galleryBusy ||
                returnsUncertainBusy ||
                isPending ||
                returnsBusy ||
                !publishCheck.eligible
              }
              onClick={handleGlobalPublish}
              className="gap-1.5 bg-emerald-700 hover:bg-emerald-800 text-white"
            >
              <Send className="size-4" aria-hidden="true" />
              Republicar
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

      {(currentListing.state === "activa" ||
        currentListing.state === "oculta") &&
        accessToken && (
          <ListingReturnsPanel
            key={`${sessionScope}:${listingId}`}
            spaceId={listingId}
            token={accessToken}
            sessionScope={sessionScope}
            snapshot={returnsSnapshot}
            onBusy={setReturnsUncertainBusy}
          />
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
          disabled={galleryBusy || returnsUncertainBusy}
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
          disabled={galleryBusy || returnsUncertainBusy}
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
          disabled={galleryBusy || returnsUncertainBusy}
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
          disabled={returnsUncertainBusy}
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
            <form onSubmit={handleSavePricing} noValidate>
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
                        ref={priceRef}
                        type="number"
                        min={5001}
                        step={1}
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
                  Define el horario semanal habitual del espacio.
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-6">
                {currentListing?.rate_unit === "hora" && (
                  <>
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
                          horario habitual.
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
                    <WeeklyScheduleSlots
                      slots={scheduleSlots}
                      onChange={setScheduleSlots}
                      disabled={
                        isPending ||
                        returnsBusy ||
                        returnsUncertainBusy ||
                        galleryBusy ||
                        !accessToken ||
                        !currentListing
                      }
                      timeZone={timeZone}
                    />
                  </>
                )}
                <div className="space-y-2">
                  <Label htmlFor="editor-timezone">Zona horaria IANA *</Label>
                  <Input
                    id="editor-timezone"
                    value={timeZone}
                    onChange={(e) => setTimeZone(e.target.value)}
                    placeholder="America/Santiago"
                    required
                    disabled={
                      isPending ||
                      returnsBusy ||
                      returnsUncertainBusy ||
                      galleryBusy ||
                      !accessToken ||
                      !currentListing
                    }
                  />
                  <p className="text-xs text-muted-foreground">
                    Requerido para activar la publicación. Ejemplo:{" "}
                    <code>America/Santiago</code>.
                  </p>
                </div>
                {currentListing?.rate_unit !== "hora" && (
                  <p className="text-sm text-muted-foreground" role="note">
                    El horario semanal solo aplica a tarifas por hora. Puedes
                    guardar la zona horaria sin modificar los horarios
                    existentes.
                  </p>
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
                  {currentListing?.rate_unit === "hora"
                    ? "Guardar horarios"
                    : "Guardar zona horaria"}
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
          <ListingGallery
            needsRefresh={snapshot.galleryNeedsRefresh}
            key={`${authSnapshot.accountId}:${listingId}`}
            listingId={listingId}
            token={accessToken!}
            photos={snapshot.photos}
            selectedListing={currentListing}
            onBusyChange={(busy) =>
              setGalleryBusy(busy || returnsUncertainBusy)
            }
            uploadPhoto={uploadPhoto}
            replacePhoto={replacePhoto}
            setCoverPhoto={setCoverPhoto}
            reorderPhotos={reorderPhotos}
            removePhoto={removePhoto}
            loadPhotos={loadPhotos}
            loadPhotoVariant={loadPhotoVariant}
            getPhotoUploadStatus={getPhotoUploadStatus}
          />
        )}
      </div>
    </div>
  );
}
