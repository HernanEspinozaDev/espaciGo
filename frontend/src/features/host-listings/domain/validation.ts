import type {
  CategoryProfileSchema,
  ListingDetail,
  ListingDraftInput,
  LocalWeeklyDay,
  ScheduleSlot,
} from "./types";

const UUID_REGEX =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

const TIME_REGEX = /^(?:[01][0-9]|2[0-3]):[0-5][0-9]$/;

export function isValidUUID(id: unknown): boolean {
  return typeof id === "string" && UUID_REGEX.test(id.trim());
}

export function generateIdempotencyKey(): string {
  if (
    typeof crypto !== "undefined" &&
    typeof crypto.randomUUID === "function"
  ) {
    return crypto.randomUUID();
  }
  // Fallback RFC 4122 v4 UUID generator
  return "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    const v = c === "x" ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
}

export function isValidPriceCLP(price: unknown): boolean {
  if (typeof price !== "number") return false;
  return Number.isSafeInteger(price) && price > 0;
}

export function isValidAreaM2(area: unknown): boolean {
  if (typeof area !== "number") return false;
  return Number.isFinite(area) && area > 0;
}

export function isValidCapacity(capacity: unknown): boolean {
  if (typeof capacity !== "number") return false;
  return Number.isSafeInteger(capacity) && capacity >= 1;
}

export function isValidTimeFormat(
  time: string,
  allowEndOfDay = false,
): boolean {
  if (typeof time !== "string") return false;
  const trimmed = time.trim();
  if (allowEndOfDay && trimmed === "24:00") return true;
  return TIME_REGEX.test(trimmed);
}

/**
 * Normaliza día de semana a ISO 1..7 (Lunes=1, Domingo=7).
 * Acepta 0..6 (Domingo=0, Lunes=1..Sábado=6) o 1..7.
 */
export function normalizeDayOfWeek(day: number): number {
  if (!Number.isInteger(day)) {
    throw new RangeError("Day of week must be an integer");
  }
  if (day === 0) return 7;
  if (day >= 1 && day <= 7) return day;
  throw new RangeError(`Invalid day of week: ${day}. Expected 0..6 or 1..7`);
}

/**
 * Convierte día ISO 1..7 a convención 0..6 (Domingo=0, Lunes=1..Sábado=6).
 */
export function denormalizeDayOfWeek(isoDay: number): number {
  if (!Number.isInteger(isoDay) || isoDay < 1 || isoDay > 7) {
    throw new RangeError(`Invalid ISO day of week: ${isoDay}. Expected 1..7`);
  }
  return isoDay === 7 ? 0 : isoDay;
}

export function isValidIanaTimeZone(tz: unknown): boolean {
  if (typeof tz !== "string" || tz.trim() === "") return false;
  try {
    new Intl.DateTimeFormat("en-US", { timeZone: tz.trim() }).format(0);
    return true;
  } catch {
    return false;
  }
}

export interface ListingDraftValidationResult {
  readonly valid: boolean;
  readonly errors: Readonly<Record<string, string>>;
}

export function validateListingDraftInput(
  input: Partial<ListingDraftInput>,
): ListingDraftValidationResult {
  const errors: Record<string, string> = {};

  if (!input.title || input.title.trim().length === 0) {
    errors.title = "El título es obligatorio.";
  } else if (input.title.trim().length > 120) {
    errors.title = "El título no puede superar los 120 caracteres.";
  }

  if (!input.description || input.description.trim().length === 0) {
    errors.description = "La descripción es obligatoria.";
  }

  if (!input.category_code || input.category_code.trim().length === 0) {
    errors.category_code = "La categoría es obligatoria.";
  }

  if (input.area_m2 === undefined || !isValidAreaM2(input.area_m2)) {
    errors.area_m2 = "La superficie debe ser un número mayor a 0.";
  }

  if (input.capacity === undefined || !isValidCapacity(input.capacity)) {
    errors.capacity =
      "La capacidad debe ser un número entero mayor o igual a 1.";
  }

  if (
    input.base_price_clp === undefined ||
    !isValidPriceCLP(input.base_price_clp)
  ) {
    errors.base_price_clp =
      "El precio base debe ser un monto entero positivo en CLP.";
  }

  if (!input.rate_unit || !["hora", "dia", "mes"].includes(input.rate_unit)) {
    errors.rate_unit = "La unidad de cobro debe ser 'hora', 'dia' o 'mes'.";
  }

  if (!input.address || input.address.trim().length === 0) {
    errors.address = "La dirección es obligatoria.";
  }

  return {
    valid: Object.keys(errors).length === 0,
    errors: Object.freeze(errors),
  };
}

export interface AttributeValidationResult {
  readonly valid: boolean;
  readonly errors: Readonly<Record<string, string>>;
}

export function validateCategoryAttributes(
  attributes: Record<string, unknown> | undefined,
  schema: CategoryProfileSchema,
): AttributeValidationResult {
  const errors: Record<string, string> = {};
  const attrs = attributes ?? {};

  for (const def of schema.attributes) {
    const val = attrs[def.code];
    const isMissing =
      val === undefined ||
      val === null ||
      (typeof val === "string" && val.trim() === "") ||
      (Array.isArray(val) && val.length === 0);

    if (def.required && isMissing) {
      errors[def.code] = `El atributo '${def.label}' es obligatorio.`;
      continue;
    }

    if (isMissing) continue;

    switch (def.type) {
      case "boolean": {
        if (typeof val !== "boolean") {
          errors[def.code] =
            `El atributo '${def.label}' debe ser verdadero o falso.`;
        }
        break;
      }
      case "integer": {
        if (typeof val !== "number" || !Number.isSafeInteger(val)) {
          errors[def.code] =
            `El atributo '${def.label}' debe ser un número entero.`;
        } else {
          if (def.minimum !== undefined && val < def.minimum) {
            errors[def.code] =
              `El atributo '${def.label}' debe ser al menos ${def.minimum}.`;
          }
          if (def.maximum !== undefined && val > def.maximum) {
            errors[def.code] =
              `El atributo '${def.label}' no puede ser mayor a ${def.maximum}.`;
          }
        }
        break;
      }
      case "number": {
        if (typeof val !== "number" || !Number.isFinite(val)) {
          errors[def.code] =
            `El atributo '${def.label}' debe ser un número válido.`;
        } else {
          if (def.minimum !== undefined && val < def.minimum) {
            errors[def.code] =
              `El atributo '${def.label}' debe ser al menos ${def.minimum}.`;
          }
          if (def.maximum !== undefined && val > def.maximum) {
            errors[def.code] =
              `El atributo '${def.label}' no puede ser mayor a ${def.maximum}.`;
          }
        }
        break;
      }
      case "enum": {
        if (typeof val !== "string") {
          errors[def.code] =
            `El atributo '${def.label}' debe ser una opción válida.`;
        } else if (def.options && !def.options.includes(val)) {
          errors[def.code] =
            `La opción '${val}' no es válida para '${def.label}'.`;
        }
        break;
      }
      case "enum_list": {
        if (!Array.isArray(val)) {
          errors[def.code] =
            `El atributo '${def.label}' debe ser una lista de opciones.`;
        } else if (
          def.options &&
          !val.every(
            (item) => typeof item === "string" && def.options!.includes(item),
          )
        ) {
          errors[def.code] =
            `Contiene opciones no válidas para '${def.label}'.`;
        }
        break;
      }
    }
  }

  return {
    valid: Object.keys(errors).length === 0,
    errors: Object.freeze(errors),
  };
}

function timeToMinutes(time: string): number {
  const [hours, minutes] = time.split(":").map(Number);
  return hours * 60 + minutes;
}

export interface ScheduleValidationResult {
  readonly valid: boolean;
  readonly errors: readonly string[];
}

export function validateScheduleSlots(
  slots: readonly ScheduleSlot[],
): ScheduleValidationResult {
  const errors: string[] = [];
  const periodsByDay = new Map<
    number,
    Array<{ open: number; close: number }>
  >();

  for (let i = 0; i < slots.length; i += 1) {
    const slot = slots[i];
    let isoDay: number;
    try {
      isoDay = normalizeDayOfWeek(slot.day_of_week);
    } catch {
      errors.push(
        `Ranura ${i + 1}: día de semana inválido (${slot.day_of_week}).`,
      );
      continue;
    }

    if (!isValidTimeFormat(slot.start_time, false)) {
      errors.push(
        `Ranura ${i + 1}: hora de inicio inválida (${slot.start_time}). Debe ser HH:MM entre 00:00 y 23:59.`,
      );
      continue;
    }

    if (!isValidTimeFormat(slot.end_time, true)) {
      errors.push(
        `Ranura ${i + 1}: hora de fin inválida (${slot.end_time}). Debe ser HH:MM o 24:00.`,
      );
      continue;
    }

    const openMin = timeToMinutes(slot.start_time);
    const closeMin = timeToMinutes(slot.end_time);

    if (openMin >= closeMin) {
      errors.push(
        `Ranura ${i + 1}: la hora de apertura (${slot.start_time}) debe ser anterior al cierre (${slot.end_time}).`,
      );
      continue;
    }

    const existing = periodsByDay.get(isoDay) ?? [];
    if (existing.length >= 2) {
      errors.push(
        `El día ISO ${isoDay} supera el límite de 2 tramos horarios permitidos.`,
      );
      continue;
    }

    // Comprobar solapamiento
    for (const p of existing) {
      if (openMin < p.close && closeMin > p.open) {
        errors.push(
          `El día ISO ${isoDay} tiene tramos horarios solapados entre sí.`,
        );
        break;
      }
    }

    existing.push({ open: openMin, close: closeMin });
    periodsByDay.set(isoDay, existing);
  }

  return {
    valid: errors.length === 0,
    errors: Object.freeze(errors),
  };
}

export function convertSlotsToWeeklyDays(
  slots: readonly ScheduleSlot[],
): LocalWeeklyDay[] {
  const daysMap = new Map<number, Array<{ open: string; close: string }>>();
  for (let d = 1; d <= 7; d += 1) {
    daysMap.set(d, []);
  }

  for (const slot of slots) {
    const isoDay = normalizeDayOfWeek(slot.day_of_week);
    const periods = daysMap.get(isoDay)!;
    periods.push({
      open: slot.start_time.trim(),
      close: slot.end_time.trim(),
    });
  }

  const result: LocalWeeklyDay[] = [];
  for (let d = 1; d <= 7; d += 1) {
    const periods = daysMap.get(d)!;
    periods.sort((a, b) => timeToMinutes(a.open) - timeToMinutes(b.open));
    result.push({
      weekday: d,
      periods,
    });
  }
  return result;
}

export function canPublishListing(listing: ListingDetail): {
  eligible: boolean;
  reasons: string[];
} {
  const reasons: string[] = [];

  if (!listing.title || listing.title.trim().length === 0) {
    reasons.push("Falta el título de la publicación.");
  }
  if (!listing.description || listing.description.trim().length === 0) {
    reasons.push("Falta la descripción de la publicación.");
  }
  if (!listing.address || listing.address.trim().length === 0) {
    reasons.push("Falta la dirección del espacio.");
  }
  if (!isValidPriceCLP(listing.base_price_clp)) {
    reasons.push("El precio base debe ser mayor a 0 CLP.");
  }
  if (!isValidAreaM2(listing.area_m2)) {
    reasons.push("La superficie debe ser mayor a 0 m².");
  }
  if (!isValidCapacity(listing.capacity)) {
    reasons.push("La capacidad debe ser al menos 1 persona.");
  }
  // REGLA CRÍTICA BACKEND: Publicar exige zona horaria IANA válida.
  if (!listing.time_zone || !isValidIanaTimeZone(listing.time_zone)) {
    reasons.push(
      "Publicar exige una zona horaria IANA válida (ej. 'America/Santiago').",
    );
  }

  return {
    eligible: reasons.length === 0,
    reasons,
  };
}

export function preserveAttributeSchemaVersion(
  input: ListingDraftInput,
  schema: CategoryProfileSchema,
): ListingDraftInput {
  return {
    ...input,
    category_code: schema.category_code,
    attribute_schema_version: schema.schema_version,
  };
}
