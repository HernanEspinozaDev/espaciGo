import type { BookingState, RateUnit } from "./types";

export const TERMINAL_BOOKING_STATES: ReadonlySet<BookingState> = new Set([
  "cerrada",
  "cancelada_por_firma",
  "cancelada_por_pago",
  "rechazada_arrendador",
  "vencida_pago",
  "vencida_host",
  "cancelada_arrendatario",
]);

export function isTerminalBookingState(state: BookingState): boolean {
  return TERMINAL_BOOKING_STATES.has(state);
}

export function isTerminalState(state: BookingState): boolean {
  return isTerminalBookingState(state);
}

export function isStateReadOnly(state: BookingState): boolean {
  return isTerminalBookingState(state);
}

export const ALLOWED_TRANSITIONS: Readonly<
  Record<BookingState, readonly BookingState[]>
> = {
  pendiente_de_pago: ["pagada", "vencida_pago", "cancelada_por_pago"],
  pagada: [
    "aprobada_host",
    "rechazada_arrendador",
    "vencida_host",
    "cancelada_arrendatario",
  ],
  aprobada_host: [
    "firma_parcial",
    "lista_para_checkin",
    "cancelada_arrendatario",
    "cancelada_por_firma",
  ],
  firma_parcial: [
    "lista_para_checkin",
    "cancelada_por_firma",
    "cancelada_arrendatario",
  ],
  lista_para_checkin: ["en_curso", "cancelada_arrendatario", "en_disputa"],
  en_curso: ["finalizada", "en_disputa"],
  finalizada: ["cerrada", "en_disputa"],
  en_disputa: ["cerrada"],
  cerrada: [],
  cancelada_por_firma: [],
  cancelada_por_pago: [],
  rechazada_arrendador: [],
  vencida_pago: [],
  vencida_host: [],
  cancelada_arrendatario: [],
};

export function canTransitionBooking(
  from: BookingState,
  to: BookingState,
): boolean {
  const allowed = ALLOWED_TRANSITIONS[from];
  return Boolean(allowed && allowed.includes(to));
}

export function canTransitionState(
  from: BookingState,
  to: BookingState,
): boolean {
  return canTransitionBooking(from, to);
}

export const ALLOWED_DURATION_UNITS: Readonly<
  Record<RateUnit, readonly number[]>
> = Object.freeze({
  hora: [1, 2, 4],
  dia: [1, 2, 3],
  mes: [1],
});

export function validateClpAmount(amount: unknown): boolean {
  if (typeof amount !== "number" || !Number.isFinite(amount)) {
    return false;
  }
  if (Math.floor(amount) !== amount) {
    return false;
  }
  return amount >= 0 && amount <= 100_000_000_000;
}

export function isValidCLPAmount(amount: unknown): boolean {
  return (
    typeof amount === "number" && Number.isSafeInteger(amount) && amount > 0
  );
}

export function isValidNonNegativeCLPAmount(amount: unknown): boolean {
  return (
    typeof amount === "number" && Number.isSafeInteger(amount) && amount >= 0
  );
}

export function isValidRateUnit(unit: unknown): unit is RateUnit {
  return unit === "hora" || unit === "dia" || unit === "mes";
}

export function validateRateUnits(unit: RateUnit, units: number): boolean {
  if (!Number.isInteger(units) || units <= 0) return false;
  switch (unit) {
    case "hora":
      return units >= 1 && units <= 24;
    case "dia":
      return units >= 1 && units <= 365;
    case "mes":
      return units >= 1 && units <= 60;
    default:
      return false;
  }
}

export function isAllowedDurationUnits(unit: RateUnit, units: number): boolean {
  const allowed = ALLOWED_DURATION_UNITS[unit];
  if (!allowed) {
    return false;
  }
  return allowed.includes(units);
}

export function sanitizeMessageBody(raw: unknown): {
  valid: boolean;
  error?: string;
  sanitized?: string;
} {
  if (typeof raw !== "string") {
    return { valid: false, error: "El mensaje debe ser texto." };
  }
  const trimmed = raw.trim();
  if (trimmed.length === 0) {
    return { valid: false, error: "El mensaje no puede estar vacío." };
  }
  if (trimmed.length > 2000) {
    return {
      valid: false,
      error: "El mensaje no puede superar los 2000 caracteres.",
    };
  }
  // Control characters except newline and tab are rejected
  if (/[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]/.test(trimmed)) {
    return {
      valid: false,
      error: "El mensaje contiene caracteres de control no permitidos.",
    };
  }
  return { valid: true, sanitized: trimmed };
}

export function validateReviewRating(rating: unknown): boolean {
  if (typeof rating !== "number" || !Number.isInteger(rating)) return false;
  return rating >= 1 && rating <= 5;
}

export function validateReviewComment(comment?: string): {
  valid: boolean;
  error?: string;
  sanitized?: string;
} {
  if (comment === undefined || comment === null || comment === "") {
    return { valid: true, sanitized: undefined };
  }
  const trimmed = comment.trim();
  if (trimmed.length > 1000) {
    return {
      valid: false,
      error: "El comentario no puede exceder 1000 caracteres.",
    };
  }
  return { valid: true, sanitized: trimmed };
}

export function validateDisputeReasonCode(code: string): boolean {
  const validCodes = [
    "espacio_no_disponible",
    "condiciones_diferentes",
    "dano_inmueble",
    "problemas_acceso",
    "incumplimiento_normas",
    "otro",
  ];
  return validCodes.includes(code.trim());
}

export function deduplicateById<T extends { readonly id: string }>(
  items: readonly T[],
): T[] {
  const seen = new Set<string>();
  const result: T[] = [];
  for (const item of items) {
    if (!seen.has(item.id)) {
      seen.add(item.id);
      result.push(item);
    }
  }
  return result;
}

export function isValidUUID(val: unknown): boolean {
  if (typeof val !== "string") return false;
  return /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(
    val,
  );
}

export function generateIdempotencyKey(): string {
  if (
    typeof crypto !== "undefined" &&
    typeof crypto.randomUUID === "function"
  ) {
    return crypto.randomUUID();
  }
  return "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    const v = c === "x" ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
}
