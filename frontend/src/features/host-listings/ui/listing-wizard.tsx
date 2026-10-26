"use client";

import { useEffect, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import {
  ArrowLeft,
  ArrowRight,
  Check,
  Building2,
  MapPin,
  DollarSign,
  Calendar,
  Eye,
  Loader2,
  AlertCircle,
  Info,
  Plus,
  Trash2,
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
import {
  generateIdempotencyKey,
  validateCategoryAttributes,
  validateListingDraftInput,
  validateScheduleSlots,
  isValidIanaTimeZone,
} from "../domain/validation";
import type {
  Category,
  ListingDraftInput,
  RateUnit,
  ScheduleSlot,
} from "../domain/types";

const STEPS = [
  { number: 1, title: "Tipo y Resumen", icon: Building2 },
  { number: 2, title: "Ubicación y Atributos", icon: MapPin },
  { number: 3, title: "Tarifas y Reglas", icon: DollarSign },
  { number: 4, title: "Disponibilidad", icon: Calendar },
  { number: 5, title: "Confirmación", icon: Eye },
];

const DAYS_MAP = [
  { day: 1, name: "Lunes" },
  { day: 2, name: "Martes" },
  { day: 3, name: "Miércoles" },
  { day: 4, name: "Jueves" },
  { day: 5, name: "Viernes" },
  { day: 6, name: "Sábado" },
  { day: 7, name: "Domingo" },
];

export function ListingWizard() {
  const router = useRouter();
  const { snapshot: authSnapshot } = useAuth();
  const {
    snapshot,
    loadCategories,
    loadCategorySchema,
    createDraft,
    updateWeeklySchedule,
    configureTimeZone,
  } = useHostListings();

  const [currentStep, setCurrentStep] = useState<number>(1);
  const [stepError, setStepError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [, startTransition] = useTransition();

  // Paso 1: Tipo y resumen
  const [categoryCode, setCategoryCode] = useState<string>("");
  const [title, setTitle] = useState<string>("");
  const [description, setDescription] = useState<string>("");

  // Paso 2: Ubicación, dimensiones y atributos dinámicos
  const [address, setAddress] = useState<string>("");
  const [comunaCode, setComunaCode] = useState<string>("");
  const [regionCode, setRegionCode] = useState<string>("13"); // Metropolitana por defecto
  const [areaM2, setAreaM2] = useState<number>(25);
  const [capacity, setCapacity] = useState<number>(4);
  const [dynamicAttributes, setDynamicAttributes] = useState<
    Record<string, unknown>
  >({});
  const currentSchema = categoryCode
    ? (snapshot.categorySchemas[categoryCode] ?? null)
    : null;
  const loadingSchema = Boolean(
    categoryCode && !currentSchema && snapshot.loading,
  );

  // Paso 3: Tarifas
  const [rateUnit, setRateUnit] = useState<RateUnit>("hora");
  const [basePriceCLP, setBasePriceCLP] = useState<number>(15000);
  const [depositCLP, setDepositCLP] = useState<string>("");
  const [cancellationPolicy, setCancellationPolicy] =
    useState<string>("flexible");
  const [usageRules, setUsageRules] = useState<string>(
    "No fumar. Cuidar el orden y limpieza de las instalaciones.",
  );

  // Paso 4: Disponibilidad semanal
  const [timeZone, setTimeZone] = useState<string>("America/Santiago");
  const [scheduleEnabled, setScheduleEnabled] = useState<boolean>(true);
  const [slots, setSlots] = useState<ScheduleSlot[]>([
    { day_of_week: 1, start_time: "09:00", end_time: "18:00" },
    { day_of_week: 2, start_time: "09:00", end_time: "18:00" },
    { day_of_week: 3, start_time: "09:00", end_time: "18:00" },
    { day_of_week: 4, start_time: "09:00", end_time: "18:00" },
    { day_of_week: 5, start_time: "09:00", end_time: "18:00" },
  ]);

  // Cargar categorías dinámicamente de Go
  useEffect(() => {
    loadCategories().catch(() => {});
  }, [loadCategories]);

  // Si se selecciona o cambia categoría, cargar esquema de Go
  useEffect(() => {
    if (!categoryCode) return;
    loadCategorySchema(categoryCode).catch(() => {});
  }, [categoryCode, loadCategorySchema]);

  const handleAttributeChange = (code: string, value: unknown) => {
    setDynamicAttributes((prev) => ({
      ...prev,
      [code]: value,
    }));
  };

  const handleAddSlot = (dayOfWeek: number) => {
    const currentDaySlots = slots.filter((s) => s.day_of_week === dayOfWeek);
    if (currentDaySlots.length >= 2) {
      setStepError("Máximo 2 tramos horarios por día permitido.");
      return;
    }
    setSlots((prev) => [
      ...prev,
      { day_of_week: dayOfWeek, start_time: "09:00", end_time: "18:00" },
    ]);
  };

  const handleRemoveSlot = (index: number) => {
    setSlots((prev) => prev.filter((_, i) => i !== index));
  };

  const handleSlotChange = (
    index: number,
    field: "start_time" | "end_time",
    val: string,
  ) => {
    setSlots((prev) =>
      prev.map((slot, i) => (i === index ? { ...slot, [field]: val } : slot)),
    );
  };

  const validateStep = (step: number): boolean => {
    setStepError(null);

    if (step === 1) {
      if (!categoryCode) {
        setStepError("Debes seleccionar un tipo de espacio.");
        return false;
      }
      if (!title.trim()) {
        setStepError("El título es obligatorio.");
        return false;
      }
      if (title.length > 120) {
        setStepError("El título no puede superar los 120 caracteres.");
        return false;
      }
      if (!description.trim()) {
        setStepError("El resumen o descripción es obligatorio.");
        return false;
      }
      return true;
    }

    if (step === 2) {
      if (!address.trim()) {
        setStepError("La dirección es obligatoria.");
        return false;
      }
      if (areaM2 <= 0) {
        setStepError("La superficie debe ser mayor a 0 m².");
        return false;
      }
      if (capacity < 1) {
        setStepError("La capacidad debe ser de al menos 1 persona.");
        return false;
      }
      if (currentSchema) {
        const attrValidation = validateCategoryAttributes(
          dynamicAttributes,
          currentSchema,
        );
        if (!attrValidation.valid) {
          const firstErr = Object.values(attrValidation.errors)[0];
          setStepError(firstErr ?? "Atributos requeridos incompletos.");
          return false;
        }
      }
      return true;
    }

    if (step === 3) {
      if (basePriceCLP <= 0 || !Number.isSafeInteger(basePriceCLP)) {
        setStepError(
          "El precio base debe ser un monto entero positivo en CLP.",
        );
        return false;
      }
      if (!["hora", "dia", "mes"].includes(rateUnit)) {
        setStepError("Selecciona una unidad de cobro válida.");
        return false;
      }
      return true;
    }

    if (step === 4) {
      if (!isValidIanaTimeZone(timeZone)) {
        setStepError(
          "La zona horaria debe ser un identificador IANA válido (ej: America/Santiago).",
        );
        return false;
      }
      if (scheduleEnabled && slots.length > 0) {
        const schedVal = validateScheduleSlots(slots);
        if (!schedVal.valid) {
          setStepError(schedVal.errors[0] ?? "Horarios con tramos inválidos.");
          return false;
        }
      }
      return true;
    }

    return true;
  };

  const handleNext = () => {
    if (validateStep(currentStep)) {
      setCurrentStep((prev) => Math.min(5, prev + 1));
    }
  };

  const handleBack = () => {
    setStepError(null);
    setCurrentStep((prev) => Math.max(1, prev - 1));
  };

  const handleCreateDraft = async () => {
    if (authSnapshot.status !== "authenticated") {
      setStepError("Sesión no disponible. Inicia sesión nuevamente.");
      return;
    }

    // Validación final completa
    const draftPayload: ListingDraftInput = {
      category_code: categoryCode,
      title: title.trim(),
      description: description.trim(),
      area_m2: Number(areaM2),
      capacity: Number(capacity),
      usage_rules: `Política de cancelación: ${cancellationPolicy}. ${usageRules.trim()}`,
      rate_unit: rateUnit,
      base_price_clp: Number(basePriceCLP),
      address: address.trim(),
      region_code: regionCode || undefined,
      comuna_code: comunaCode.trim() || undefined,
      attribute_schema_version: currentSchema?.schema_version ?? 1,
      attributes: {
        ...dynamicAttributes,
        cancellation_policy: cancellationPolicy,
        ...(depositCLP ? { security_deposit_clp: Number(depositCLP) } : {}),
      },
    };

    const overallVal = validateListingDraftInput(draftPayload);
    if (!overallVal.valid) {
      const firstErr = Object.values(overallVal.errors)[0];
      setStepError(
        firstErr ?? "Por favor completa todos los campos requeridos.",
      );
      return;
    }

    setIsSubmitting(true);
    setStepError(null);

    try {
      const idempotencyKey = generateIdempotencyKey();
      const created = await createDraft(draftPayload, idempotencyKey);

      // Configurar zona horaria y horario semanal si se definió
      if (timeZone) {
        await configureTimeZone(created.id, timeZone).catch(() => {});
      }

      if (scheduleEnabled && slots.length > 0) {
        await updateWeeklySchedule(created.id, {
          timezone: timeZone,
          enabled: true,
          weekly_slots: slots,
        }).catch(() => {});
      }

      // Redirigir al editor tras el alta
      startTransition(() => {
        router.push(`/anfitrion/espacios/${created.id}/editar`);
      });
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "No fue posible crear el borrador";
      setStepError(message);
      setIsSubmitting(false);
    }
  };

  const selectedCatObj = snapshot.categories.find(
    (c: Category) => c.code === categoryCode,
  );

  return (
    <div className="max-w-4xl mx-auto space-y-8 pb-12">
      {/* Cabecera del Asistente */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 pb-2 border-b border-border">
        <div>
          <h1 className="text-2xl lg:text-3xl font-bold tracking-tight text-foreground">
            Publicar un nuevo espacio
          </h1>
          <p className="text-sm text-muted-foreground mt-1">
            Completa los 5 pasos para dar de alta tu propiedad en EspaciGo.
          </p>
        </div>
        <Button asChild variant="outline" size="sm">
          <Link href="/anfitrion/espacios" className="gap-1.5">
            <ArrowLeft className="size-4" aria-hidden="true" />
            Volver al listado
          </Link>
        </Button>
      </div>

      {/* Indicador de progreso de 5 pasos accesibles */}
      <nav aria-label="Pasos del asistente de publicación">
        <ol className="grid grid-cols-2 sm:grid-cols-5 gap-2 text-xs font-medium">
          {STEPS.map((s) => {
            const isCompleted = s.number < currentStep;
            const isCurrent = s.number === currentStep;
            const StepIcon = s.icon;

            return (
              <li
                key={s.number}
                aria-current={isCurrent ? "step" : undefined}
                className={`flex items-center gap-2 p-2.5 rounded-lg border transition-colors ${
                  isCurrent
                    ? "border-primary bg-primary/10 text-primary font-semibold shadow-xs"
                    : isCompleted
                      ? "border-border bg-muted/60 text-foreground"
                      : "border-transparent bg-muted/20 text-muted-foreground"
                }`}
              >
                <div
                  className={`flex size-6 items-center justify-center rounded-full text-xs font-bold ${
                    isCurrent
                      ? "bg-primary text-primary-foreground"
                      : isCompleted
                        ? "bg-emerald-600 text-white"
                        : "bg-muted text-muted-foreground"
                  }`}
                >
                  {isCompleted ? (
                    <Check className="size-3.5" aria-hidden="true" />
                  ) : (
                    <StepIcon className="size-3.5" aria-hidden="true" />
                  )}
                </div>
                <span className="truncate">{s.title}</span>
              </li>
            );
          })}
        </ol>
      </nav>

      {/* Mensaje de alerta / error */}
      {stepError && (
        <div
          role="alert"
          className="rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive flex items-start gap-3"
        >
          <AlertCircle className="size-5 shrink-0 mt-0.5" aria-hidden="true" />
          <div className="flex-1">{stepError}</div>
        </div>
      )}

      {/* Contenido del paso actual */}
      <Card className="shadow-sm">
        {/* PASO 1: Tipo de espacio, título y resumen */}
        {currentStep === 1 && (
          <>
            <CardHeader>
              <CardTitle className="text-xl">
                Paso 1: Tipo de espacio y resumen
              </CardTitle>
              <CardDescription>
                Selecciona la categoría que mejor describa tu inmueble y define
                un título atractivo.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
              <div className="space-y-2">
                <Label htmlFor="wizard-category" className="font-semibold">
                  Categoría del espacio *
                </Label>
                <select
                  id="wizard-category"
                  value={categoryCode}
                  onChange={(e) => setCategoryCode(e.target.value)}
                  className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
                  required
                >
                  <option value="">-- Selecciona una categoría --</option>
                  {snapshot.categories.map((cat: Category) => (
                    <option key={cat.code} value={cat.code}>
                      {cat.name} ({cat.code})
                    </option>
                  ))}
                </select>
                <p className="text-xs text-muted-foreground">
                  Las categorías se obtienen en tiempo real desde el catálogo
                  oficial.
                </p>
              </div>

              <div className="space-y-2">
                <Label htmlFor="wizard-title" className="font-semibold">
                  Título de la publicación *
                </Label>
                <Input
                  id="wizard-title"
                  value={title}
                  onChange={(e) => setTitle(e.target.value)}
                  placeholder="Ej. Oficina privada ejecutiva en Las Condes"
                  maxLength={120}
                  required
                />
                <p className="text-xs text-muted-foreground flex justify-between">
                  <span>Sé específico e incluye detalles atractivos.</span>
                  <span>{title.length}/120</span>
                </p>
              </div>

              <div className="space-y-2">
                <Label htmlFor="wizard-description" className="font-semibold">
                  Resumen o descripción *
                </Label>
                <textarea
                  id="wizard-description"
                  rows={4}
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  placeholder="Describe las comodidades, accesibilidad, iluminación y entorno del espacio..."
                  className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
                  required
                />
              </div>
            </CardContent>
          </>
        )}

        {/* PASO 2: Ubicación y atributos dinámicos */}
        {currentStep === 2 && (
          <>
            <CardHeader>
              <CardTitle className="text-xl">
                Paso 2: Ubicación y características
              </CardTitle>
              <CardDescription>
                Indica la dirección y completa los atributos específicos de{" "}
                {selectedCatObj?.name || "la categoría"}.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div className="space-y-2 sm:col-span-2">
                  <Label htmlFor="wizard-address" className="font-semibold">
                    Dirección física *
                  </Label>
                  <Input
                    id="wizard-address"
                    value={address}
                    onChange={(e) => setAddress(e.target.value)}
                    placeholder="Ej: Av. Apoquindo 4500, Oficina 801"
                    required
                  />
                </div>

                <div className="space-y-2">
                  <Label htmlFor="wizard-comuna" className="font-semibold">
                    Comuna (nombre o código)
                  </Label>
                  <Input
                    id="wizard-comuna"
                    value={comunaCode}
                    onChange={(e) => setComunaCode(e.target.value)}
                    placeholder="Ej: Las Condes o 13114"
                  />
                </div>

                <div className="space-y-2">
                  <Label htmlFor="wizard-region" className="font-semibold">
                    Región (código)
                  </Label>
                  <select
                    id="wizard-region"
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
                    <option value="06">O&apos;Higgins (06)</option>
                    <option value="07">Maule (07)</option>
                    <option value="01">Tarapacá (01)</option>
                    <option value="02">Antofagasta (02)</option>
                    <option value="03">Atacama (03)</option>
                    <option value="14">Los Ríos (14)</option>
                    <option value="15">Arica y Parinacota (15)</option>
                    <option value="16">Ñuble (16)</option>
                    <option value="11">Aysén (11)</option>
                    <option value="12">Magallanes (12)</option>
                  </select>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="wizard-area" className="font-semibold">
                    Superficie (m²) *
                  </Label>
                  <Input
                    id="wizard-area"
                    type="number"
                    min={1}
                    step={0.5}
                    value={areaM2}
                    onChange={(e) => setAreaM2(Number(e.target.value))}
                    required
                  />
                </div>

                <div className="space-y-2">
                  <Label htmlFor="wizard-capacity" className="font-semibold">
                    Capacidad (personas) *
                  </Label>
                  <Input
                    id="wizard-capacity"
                    type="number"
                    min={1}
                    step={1}
                    value={capacity}
                    onChange={(e) => setCapacity(Number(e.target.value))}
                    required
                  />
                </div>
              </div>

              {/* Atributos dinámicos del CategoryProfileSchema */}
              <div className="pt-4 border-t border-border space-y-4">
                <div className="flex items-center justify-between">
                  <h2 className="text-base font-semibold text-foreground">
                    Atributos del perfil ({selectedCatObj?.name || categoryCode}
                    )
                  </h2>
                  {currentSchema && (
                    <Badge variant="outline" className="text-xs font-mono">
                      v{currentSchema.schema_version}
                    </Badge>
                  )}
                </div>

                {loadingSchema && (
                  <div className="flex items-center gap-2 text-xs text-muted-foreground py-2">
                    <Loader2
                      className="size-4 animate-spin"
                      aria-hidden="true"
                    />
                    <span>
                      Cargando esquema de atributos desde el backend...
                    </span>
                  </div>
                )}

                {!loadingSchema &&
                currentSchema?.attributes &&
                currentSchema.attributes.length > 0 ? (
                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 bg-muted/20 p-4 rounded-xl border">
                    {currentSchema.attributes.map((def) => {
                      const attrVal = dynamicAttributes[def.code];
                      const fieldId = `attr-${def.code}`;

                      if (def.type === "boolean") {
                        return (
                          <div
                            key={def.code}
                            className="flex items-center justify-between p-3 rounded-lg bg-background border"
                          >
                            <Label
                              htmlFor={fieldId}
                              className="text-sm cursor-pointer"
                            >
                              {def.label} {def.required && "*"}
                            </Label>
                            <input
                              id={fieldId}
                              type="checkbox"
                              checked={Boolean(attrVal)}
                              onChange={(e) =>
                                handleAttributeChange(
                                  def.code,
                                  e.target.checked,
                                )
                              }
                              className="size-4 rounded text-primary focus:ring-primary"
                            />
                          </div>
                        );
                      }

                      if (def.type === "integer" || def.type === "number") {
                        return (
                          <div key={def.code} className="space-y-1.5">
                            <Label
                              htmlFor={fieldId}
                              className="text-xs font-semibold"
                            >
                              {def.label} {def.unit ? `(${def.unit})` : ""}{" "}
                              {def.required && "*"}
                            </Label>
                            <Input
                              id={fieldId}
                              type="number"
                              min={def.minimum}
                              max={def.maximum}
                              step={
                                def.step ?? (def.type === "integer" ? 1 : 0.1)
                              }
                              value={
                                attrVal !== undefined ? String(attrVal) : ""
                              }
                              onChange={(e) =>
                                handleAttributeChange(
                                  def.code,
                                  e.target.value === ""
                                    ? undefined
                                    : Number(e.target.value),
                                )
                              }
                              placeholder={
                                def.description || `Ingrese ${def.label}`
                              }
                            />
                          </div>
                        );
                      }

                      if (def.type === "enum" && def.options) {
                        return (
                          <div key={def.code} className="space-y-1.5">
                            <Label
                              htmlFor={fieldId}
                              className="text-xs font-semibold"
                            >
                              {def.label} {def.required && "*"}
                            </Label>
                            <select
                              id={fieldId}
                              value={typeof attrVal === "string" ? attrVal : ""}
                              onChange={(e) =>
                                handleAttributeChange(def.code, e.target.value)
                              }
                              className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
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
                ) : (
                  !loadingSchema && (
                    <p className="text-xs text-muted-foreground italic">
                      No hay atributos dinámicos adicionales requeridos para
                      esta categoría.
                    </p>
                  )
                )}
              </div>
            </CardContent>
          </>
        )}

        {/* PASO 3: Tarifas iniciales y reglas */}
        {currentStep === 3 && (
          <>
            <CardHeader>
              <CardTitle className="text-xl">
                Paso 3: Tarifas iniciales y condiciones
              </CardTitle>
              <CardDescription>
                Establece el precio base en pesos chilenos (CLP) y las políticas
                de cancelación.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label htmlFor="wizard-rate-unit" className="font-semibold">
                    Unidad de cobro *
                  </Label>
                  <select
                    id="wizard-rate-unit"
                    value={rateUnit}
                    onChange={(e) => setRateUnit(e.target.value as RateUnit)}
                    className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
                  >
                    <option value="hora">Por Hora</option>
                    <option value="dia">Por Día</option>
                    <option value="mes">Por Mes</option>
                  </select>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="wizard-price" className="font-semibold">
                    Precio base (CLP) *
                  </Label>
                  <Input
                    id="wizard-price"
                    type="number"
                    min={100}
                    step={100}
                    value={basePriceCLP}
                    onChange={(e) => setBasePriceCLP(Number(e.target.value))}
                    required
                  />
                  <p className="text-xs text-muted-foreground">
                    Monto en pesos chilenos por cada {rateUnit}.
                  </p>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="wizard-deposit" className="font-semibold">
                    Depósito de garantía (opcional)
                  </Label>
                  <Input
                    id="wizard-deposit"
                    type="number"
                    min={0}
                    step={1000}
                    value={depositCLP}
                    onChange={(e) => setDepositCLP(e.target.value)}
                    placeholder="Ej: 50000"
                  />
                </div>

                <div className="space-y-2">
                  <Label
                    htmlFor="wizard-cancellation"
                    className="font-semibold"
                  >
                    Política de cancelación
                  </Label>
                  <select
                    id="wizard-cancellation"
                    value={cancellationPolicy}
                    onChange={(e) => setCancellationPolicy(e.target.value)}
                    className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
                  >
                    <option value="flexible">
                      Flexible (reembolso total hasta 24h antes)
                    </option>
                    <option value="moderada">
                      Moderada (reembolso 50% hasta 48h antes)
                    </option>
                    <option value="estricta">
                      Estricta (sin reembolso tras confirmación)
                    </option>
                  </select>
                </div>
              </div>

              <div className="space-y-2">
                <Label htmlFor="wizard-usage-rules" className="font-semibold">
                  Reglas y normas de uso
                </Label>
                <textarea
                  id="wizard-usage-rules"
                  rows={3}
                  value={usageRules}
                  onChange={(e) => setUsageRules(e.target.value)}
                  className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
                  placeholder="Indicaciones sobre ruido, acceso de mascotas, horarios de cierre..."
                />
              </div>
            </CardContent>
          </>
        )}

        {/* PASO 4: Disponibilidad semanal */}
        {currentStep === 4 && (
          <>
            <CardHeader>
              <CardTitle className="text-xl">
                Paso 4: Disponibilidad semanal y zona horaria
              </CardTitle>
              <CardDescription>
                Configura los días y horarios en los que tu espacio puede ser
                reservado.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
              <div className="flex items-center justify-between p-4 rounded-xl bg-muted/40 border">
                <div>
                  <Label
                    htmlFor="schedule-enabled"
                    className="text-sm font-semibold text-foreground cursor-pointer"
                  >
                    Habilitar calendario de reservas semanal
                  </Label>
                  <p className="text-xs text-muted-foreground mt-0.5">
                    Si está deshabilitado, el espacio requerirá coordinación
                    manual de disponibilidad.
                  </p>
                </div>
                <input
                  type="checkbox"
                  id="schedule-enabled"
                  aria-label="Habilitar calendario de reservas semanal"
                  checked={scheduleEnabled}
                  onChange={(e) => setScheduleEnabled(e.target.checked)}
                  className="size-5 rounded text-primary focus:ring-primary"
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="wizard-timezone" className="font-semibold">
                  Zona horaria oficial (IANA) *
                </Label>
                <Input
                  id="wizard-timezone"
                  value={timeZone}
                  onChange={(e) => setTimeZone(e.target.value)}
                  placeholder="America/Santiago"
                  required
                />
                <p className="text-xs text-muted-foreground">
                  Por defecto para Chile: <code>America/Santiago</code> o{" "}
                  <code>America/Punta_Arenas</code>.
                </p>
              </div>

              {scheduleEnabled && (
                <div className="space-y-4 pt-2">
                  <h2 className="text-sm font-semibold text-foreground">
                    Tramos horarios por día de la semana
                  </h2>

                  <div className="space-y-3">
                    {DAYS_MAP.map((day) => {
                      const daySlots = slots
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
                              <span className="text-xs text-muted-foreground italic py-1">
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
                                      handleSlotChange(
                                        slot.originalIndex,
                                        "start_time",
                                        e.target.value,
                                      )
                                    }
                                    aria-label={`Hora inicio ${day.name}`}
                                    className="rounded border border-input px-2 py-1 bg-background text-xs"
                                  />
                                  <span>a</span>
                                  <input
                                    type="time"
                                    value={slot.end_time}
                                    onChange={(e) =>
                                      handleSlotChange(
                                        slot.originalIndex,
                                        "end_time",
                                        e.target.value,
                                      )
                                    }
                                    aria-label={`Hora fin ${day.name}`}
                                    className="rounded border border-input px-2 py-1 bg-background text-xs"
                                  />
                                  <button
                                    type="button"
                                    onClick={() =>
                                      handleRemoveSlot(slot.originalIndex)
                                    }
                                    className="text-destructive hover:bg-destructive/10 p-1 rounded"
                                    aria-label={`Eliminar tramo de ${day.name}`}
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
                              onClick={() => handleAddSlot(day.day)}
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
              )}
            </CardContent>
          </>
        )}

        {/* PASO 5: Vista previa y confirmación */}
        {currentStep === 5 && (
          <>
            <CardHeader>
              <CardTitle className="text-xl">
                Paso 5: Resumen y confirmación de alta
              </CardTitle>
              <CardDescription>
                Revisa todos los datos ingresados antes de crear el borrador
                seguro con clave de idempotencia.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
              <div className="rounded-xl border bg-muted/20 p-5 space-y-4 text-sm">
                <div className="flex items-start justify-between gap-4 border-b pb-3">
                  <div>
                    <Badge variant="outline" className="mb-1">
                      {selectedCatObj?.name || categoryCode}
                    </Badge>
                    <h2 className="text-lg font-bold text-foreground">
                      {title}
                    </h2>
                    <p className="text-xs text-muted-foreground mt-1">
                      {description}
                    </p>
                  </div>
                  <div className="text-right shrink-0">
                    <div className="text-xs text-muted-foreground">Tarifa</div>
                    <div className="text-lg font-bold text-primary">
                      ${new Intl.NumberFormat("es-CL").format(basePriceCLP)} CLP
                      <span className="text-xs font-normal text-muted-foreground">
                        /{rateUnit}
                      </span>
                    </div>
                  </div>
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 text-xs">
                  <div>
                    <span className="text-muted-foreground font-semibold">
                      Ubicación:
                    </span>
                    <p className="text-foreground mt-0.5">
                      {address}, {comunaCode || "Chile"} (Región {regionCode})
                    </p>
                  </div>
                  <div>
                    <span className="text-muted-foreground font-semibold">
                      Capacidad y Área:
                    </span>
                    <p className="text-foreground mt-0.5">
                      {areaM2} m² · Hasta {capacity} personas
                    </p>
                  </div>
                  <div>
                    <span className="text-muted-foreground font-semibold">
                      Cancelación y Normas:
                    </span>
                    <p className="text-foreground mt-0.5">
                      Política {cancellationPolicy}. {usageRules}
                    </p>
                  </div>
                  <div>
                    <span className="text-muted-foreground font-semibold">
                      Disponibilidad:
                    </span>
                    <p className="text-foreground mt-0.5">
                      {scheduleEnabled
                        ? `${slots.length} tramos semanales`
                        : "Sin horario activo"}{" "}
                      ({timeZone})
                    </p>
                  </div>
                </div>

                {Object.keys(dynamicAttributes).length > 0 && (
                  <div className="border-t pt-3 text-xs">
                    <span className="text-muted-foreground font-semibold">
                      Atributos configurados:
                    </span>
                    <ul className="mt-1 grid grid-cols-2 gap-2 text-foreground">
                      {Object.entries(dynamicAttributes).map(([k, v]) => (
                        <li key={k} className="truncate">
                          <span className="font-medium">{k}:</span> {String(v)}
                        </li>
                      ))}
                    </ul>
                  </div>
                )}
              </div>

              <div className="rounded-lg border border-blue-200 bg-blue-50 p-4 text-xs text-blue-900 flex items-start gap-2.5">
                <Info
                  className="size-4 shrink-0 text-blue-600 mt-0.5"
                  aria-hidden="true"
                />
                <div>
                  Al confirmar, se generará un borrador oficial con clave de
                  idempotencia única UUIDv4. Serás redirigido al editor donde
                  podrás cargar fotografías y gestionar la publicación.
                </div>
              </div>
            </CardContent>
          </>
        )}

        {/* Navegación y botones de pie de tarjeta */}
        <CardFooter className="flex items-center justify-between gap-4 border-t border-border pt-4">
          {currentStep > 1 ? (
            <Button
              type="button"
              variant="outline"
              onClick={handleBack}
              disabled={isSubmitting}
              className="gap-1.5"
            >
              <ArrowLeft className="size-4" aria-hidden="true" />
              Atrás
            </Button>
          ) : (
            <div />
          )}

          {currentStep < 5 ? (
            <Button
              type="button"
              onClick={handleNext}
              className="gap-1.5 font-semibold"
            >
              Siguiente
              <ArrowRight className="size-4" aria-hidden="true" />
            </Button>
          ) : (
            <Button
              type="button"
              onClick={handleCreateDraft}
              disabled={isSubmitting}
              className="gap-2 font-bold bg-primary text-primary-foreground shadow-sm"
            >
              {isSubmitting ? (
                <>
                  <Loader2 className="size-4 animate-spin" aria-hidden="true" />
                  Creando borrador...
                </>
              ) : (
                <>
                  <Check className="size-4" aria-hidden="true" />
                  Crear espacio y continuar a edición
                </>
              )}
            </Button>
          )}
        </CardFooter>
      </Card>
    </div>
  );
}
