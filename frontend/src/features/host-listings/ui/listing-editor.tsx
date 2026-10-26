"use client";

import { useEffect, useState, useTransition } from "react";
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
  Star,
  ArrowUp,
  ArrowDown,
  Plus,
  Info,
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
import { useHostListings } from "./use-host-listings";
import { hostListingsFacade } from "../host-listings-runtime";
import {
  canPublishListing,
  isValidIanaTimeZone,
  validateCategoryAttributes,
  validateScheduleSlots,
} from "../domain/validation";
import type {
  GalleryPhoto,
  PricingHistoryItem,
  RateUnit,
  ScheduleSlot,
} from "../domain/types";

interface ListingEditorProps {
  readonly listingId: string;
}

type TabKey = "info" | "pricing" | "schedule" | "photos";

const DAYS_MAP = [
  { day: 1, name: "Lunes" },
  { day: 2, name: "Martes" },
  { day: 3, name: "Miércoles" },
  { day: 4, name: "Jueves" },
  { day: 5, name: "Viernes" },
  { day: 6, name: "Sábado" },
  { day: 7, name: "Domingo" },
];

export function ListingEditor({ listingId }: ListingEditorProps) {
  const router = useRouter();
  const { snapshot: authSnapshot } = useAuth();
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
    publishListing,
    archiveListing,
    deleteListing,
  } = useHostListings();

  const [activeTab, setActiveTab] = useState<TabKey>("info");
  const [isPending, startTransition] = useTransition();
  const [feedbackSuccess, setFeedbackSuccess] = useState<string | null>(null);
  const [feedbackError, setFeedbackError] = useState<string | null>(null);
  const [initialLoaded, setInitialLoaded] = useState(false);

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

    if (!title.trim() || !description.trim() || !address.trim()) {
      setFeedbackError("Título, descripción y dirección son obligatorios.");
      return;
    }

    if (currentSchema) {
      const val = validateCategoryAttributes(attributes, currentSchema);
      if (!val.valid) {
        const firstErr = Object.values(val.errors)[0];
        setFeedbackError(firstErr ?? "Atributos requeridos incompletos.");
        return;
      }
    }

    startTransition(async () => {
      try {
        if (currentListing.state === "borrador") {
          await updateDraft(listingId, {
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
          });
        } else {
          await updateContent(listingId, {
            title: title.trim(),
            description: description.trim(),
            capacity: Number(capacity),
            usage_rules: usageRules.trim(),
            base_price_clp: Number(basePrice),
            region_code: regionCode || undefined,
            comuna_code: comunaCode.trim() || undefined,
          });
        }
        setFeedbackSuccess("Información actualizada exitosamente.");
      } catch (err) {
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

    if (basePrice <= 0 || !Number.isSafeInteger(basePrice)) {
      setFeedbackError(
        "El precio base debe ser un monto entero positivo en CLP.",
      );
      return;
    }

    startTransition(async () => {
      try {
        await updatePricing(listingId, {
          rate_unit: rateUnit,
          base_price: Number(basePrice),
        });
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

    startTransition(async () => {
      try {
        await configureTimeZone(listingId, timeZone);
        await updateWeeklySchedule(listingId, {
          timezone: timeZone,
          enabled: scheduleEnabled,
          weekly_slots: scheduleSlots,
        });
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

  // Pestaña 4: Fotos
  const handleUploadSyntheticPhoto = async () => {
    if (authSnapshot.status !== "authenticated") return;
    setFeedbackError(null);
    setFeedbackSuccess(null);

    startTransition(async () => {
      try {
        await uploadPhoto(listingId, {
          caption: `Foto de ensayo para ${currentListing?.title || "espacio"}`,
          is_cover: snapshot.photos.length === 0,
        });
        setFeedbackSuccess(
          "Fotografía sintética de ensayo agregada a la galería.",
        );
      } catch (err) {
        const msg = err instanceof Error ? err.message : "Error al subir foto";
        setFeedbackError(msg);
      }
    });
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
        await removePhoto(listingId, photoId);
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

    const newOrder = [...snapshot.photos];
    const temp = newOrder[index]!;
    newOrder[index] = newOrder[targetIndex]!;
    newOrder[targetIndex] = temp;

    const photoIds = newOrder.map((p) => p.id);
    startTransition(async () => {
      try {
        await reorderPhotos(listingId, photoIds);
        setFeedbackSuccess("Orden de fotografías actualizado.");
      } catch (err) {
        const msg =
          err instanceof Error ? err.message : "Error al reordenar fotos";
        setFeedbackError(msg);
      }
    });
  };

  // Acciones Globales: Publicar / Archivar / Eliminar
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
        await publishListing(listingId);
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
        await archiveListing(listingId);
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

  const handleGlobalDelete = async () => {
    if (authSnapshot.status !== "authenticated" || !currentListing) return;
    const confirmed = window.confirm(
      `¿Eliminar definitivamente el borrador "${currentListing.title}"?`,
    );
    if (!confirmed) return;

    startTransition(async () => {
      try {
        await deleteListing(listingId);
        router.push("/anfitrion/espacios");
      } catch (err) {
        const msg =
          err instanceof Error ? err.message : "Error al eliminar borrador";
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
              <Button
                variant="outline"
                size="sm"
                disabled={isPending}
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
                    value={title}
                    onChange={(e) => setTitle(e.target.value)}
                    maxLength={120}
                    required
                  />
                </div>

                <div className="space-y-2">
                  <Label htmlFor="editor-desc">Descripción detallada *</Label>
                  <textarea
                    id="editor-desc"
                    rows={4}
                    value={description}
                    onChange={(e) => setDescription(e.target.value)}
                    className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
                    required
                  />
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                  <div className="space-y-2 sm:col-span-2">
                    <Label htmlFor="editor-address">Dirección *</Label>
                    <Input
                      id="editor-address"
                      value={address}
                      onChange={(e) => setAddress(e.target.value)}
                      required
                    />
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="editor-comuna">Comuna</Label>
                    <Input
                      id="editor-comuna"
                      value={comunaCode}
                      onChange={(e) => setComunaCode(e.target.value)}
                      placeholder="Ej: Santiago o 13101"
                    />
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="editor-region">Región</Label>
                    <select
                      id="editor-region"
                      value={regionCode}
                      onChange={(e) => setRegionCode(e.target.value)}
                      className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
                    >
                      <option value="13">Región Metropolitana (13)</option>
                      <option value="05">Valparaíso (05)</option>
                      <option value="08">Biobío (08)</option>
                      <option value="04">Coquimbo (04)</option>
                      <option value="09">La Araucanía (09)</option>
                      <option value="10">Los Lagos (10)</option>
                    </select>
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="editor-capacity">
                      Capacidad máxima (personas) *
                    </Label>
                    <Input
                      id="editor-capacity"
                      type="number"
                      min={1}
                      value={capacity}
                      onChange={(e) => setCapacity(Number(e.target.value))}
                      required
                    />
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="editor-area">Superficie (m²) *</Label>
                    <Input
                      id="editor-area"
                      type="number"
                      min={1}
                      value={areaM2}
                      onChange={(e) => setAreaM2(Number(e.target.value))}
                      required
                    />
                  </div>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="editor-rules">
                    Reglas de uso y convivencia
                  </Label>
                  <textarea
                    id="editor-rules"
                    rows={3}
                    value={usageRules}
                    onChange={(e) => setUsageRules(e.target.value)}
                    className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
                  />
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
                        const inputId = `attr-editor-${def.code}`;

                        if (def.type === "boolean") {
                          return (
                            <div
                              key={def.code}
                              className="flex items-center justify-between p-2.5 rounded bg-background border"
                            >
                              <Label
                                htmlFor={inputId}
                                className="text-xs cursor-pointer"
                              >
                                {def.label}
                              </Label>
                              <input
                                id={inputId}
                                type="checkbox"
                                checked={Boolean(val)}
                                onChange={(e) =>
                                  setAttributes((prev) => ({
                                    ...prev,
                                    [def.code]: e.target.checked,
                                  }))
                                }
                                className="size-4 rounded text-primary focus:ring-primary"
                              />
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
                                type="number"
                                min={def.minimum}
                                max={def.maximum}
                                step={def.step ?? 1}
                                value={val !== undefined ? String(val) : ""}
                                onChange={(e) =>
                                  setAttributes((prev) => ({
                                    ...prev,
                                    [def.code]:
                                      e.target.value === ""
                                        ? undefined
                                        : Number(e.target.value),
                                  }))
                                }
                              />
                            </div>
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
                                value={typeof val === "string" ? val : ""}
                                onChange={(e) =>
                                  setAttributes((prev) => ({
                                    ...prev,
                                    [def.code]: e.target.value,
                                  }))
                                }
                                className="w-full rounded-md border border-input bg-background px-3 py-1.5 text-sm"
                              >
                                <option value="">-- Selecciona --</option>
                                {def.options.map((opt) => (
                                  <option key={opt} value={opt}>
                                    {opt}
                                  </option>
                                ))}
                              </select>
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
                        min={100}
                        step={100}
                        value={basePrice}
                        onChange={(e) => setBasePrice(Number(e.target.value))}
                        required
                      />
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
            {/* Aviso Permanente de Ensayo / Fotos Sintéticas */}
            <div
              role="note"
              className="rounded-lg border border-amber-300 bg-amber-50 p-4 text-xs text-amber-900 flex items-start gap-3"
            >
              <Info
                className="size-4 text-amber-700 shrink-0 mt-0.5"
                aria-hidden="true"
              />
              <div className="space-y-0.5">
                <div className="font-semibold">
                  Entorno de ensayo de galería privada
                </div>
                <p className="text-amber-800">
                  Aviso permanente: Las fotografías cargadas en este entorno de
                  desarrollo son simuladas y sintéticas para pruebas y ensayos
                  locales de layout y concurrencia. No se almacenan en
                  servidores productivos externos.
                </p>
              </div>
            </div>

            <Card>
              <CardHeader className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
                <div>
                  <CardTitle className="text-lg">Galería de imágenes</CardTitle>
                  <CardDescription>
                    Selecciona la imagen principal de portada y reorganiza el
                    orden de visualización.
                  </CardDescription>
                </div>
                <Button
                  type="button"
                  onClick={handleUploadSyntheticPhoto}
                  disabled={isPending}
                  className="gap-1.5 shrink-0"
                >
                  <Plus className="size-4" aria-hidden="true" />
                  Agregar foto de ensayo
                </Button>
              </CardHeader>
              <CardContent>
                {snapshot.photos.length === 0 ? (
                  <div className="border-2 border-dashed rounded-xl p-10 text-center space-y-3">
                    <ImageIcon
                      className="size-10 text-muted-foreground mx-auto"
                      aria-hidden="true"
                    />
                    <p className="text-sm font-medium text-foreground">
                      No hay imágenes en la galería
                    </p>
                    <p className="text-xs text-muted-foreground max-w-sm mx-auto">
                      Agrega al menos una fotografía para ilustrar el espacio a
                      tus futuros clientes.
                    </p>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={handleUploadSyntheticPhoto}
                      disabled={isPending}
                      className="mt-2"
                    >
                      Generar primera fotografía
                    </Button>
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
                                disabled={isPending}
                                className="h-6 px-2 text-xs text-muted-foreground hover:text-foreground"
                              >
                                Marcar portada
                              </Button>
                            )}
                          </div>

                          {/* Ilustración visual sintética */}
                          <div className="h-32 rounded-lg bg-muted/60 flex flex-col items-center justify-center p-2 text-center border">
                            <ImageIcon
                              className="size-6 text-muted-foreground mb-1"
                              aria-hidden="true"
                            />
                            <span className="text-xs font-medium text-foreground line-clamp-1">
                              {photo.alt_text || "Foto de ensayo"}
                            </span>
                            <span className="text-[10px] text-muted-foreground font-mono truncate max-w-full">
                              {photo.sha256.slice(0, 12)}...
                            </span>
                          </div>

                          {/* Botones de acción: Mover arriba/abajo y eliminar */}
                          <div className="flex items-center justify-between border-t pt-2">
                            <div className="flex items-center gap-1">
                              <Button
                                type="button"
                                variant="outline"
                                size="sm"
                                disabled={idx === 0 || isPending}
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
                                  isPending
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
                              variant="ghost"
                              size="sm"
                              onClick={() => handleRemovePhoto(photo.id)}
                              disabled={isPending}
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
