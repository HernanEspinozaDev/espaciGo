import type {
  components,
  Readable,
  Writable,
} from "@/lib/api/generated/openapi";

export type ProfileResponse = Readable<components["schemas"]["Profile"]>;
export type ProfileUpdatePayload = Writable<
  components["schemas"]["ProfileUpdate"]
>;

export interface Profile {
  readonly displayName: string;
  readonly phone: string | null;
  readonly updatedAt: string;
}

const NAME_ALLOWED_REGEX = /^[\p{L}\s]+$/u;
const NINE_DIGITS_REGEX = /^[0-9]{9}$/;

export function isValidProfileName(name: string): boolean {
  const trimmed = name.trim();
  const chars = Array.from(trimmed);
  return (
    chars.length > 0 && chars.length <= 120 && NAME_ALLOWED_REGEX.test(trimmed)
  );
}

export function isValidChileanPhone(phone: string): boolean {
  const normalized = normalizeChileanPhone(phone);
  return normalized !== null;
}

export function normalizeChileanPhone(
  phone: string | null | undefined,
): string | null {
  if (!phone) return null;
  const trimmed = phone.trim();
  if (trimmed === "") return null;

  // Remove all formatting chars (spaces, hyphens, parens, plus)
  const cleaned = trimmed.replace(/[\s\-\(\)\+]/g, "");

  // Check if non-digits are present
  if (!/^[0-9]+$/.test(cleaned)) {
    return null;
  }

  // Handle country code: if starts with 56 and length is 11, strip 56
  if (cleaned.startsWith("56") && cleaned.length === 11) {
    const stripped = cleaned.slice(2);
    if (NINE_DIGITS_REGEX.test(stripped)) {
      return stripped;
    }
    return null;
  }

  // Exactly 9 digits (standard Chilean numbering)
  if (NINE_DIGITS_REGEX.test(cleaned)) {
    return cleaned;
  }

  return null;
}

export interface ProfileValidationResult {
  readonly valid: boolean;
  readonly errors: {
    readonly displayName?: string;
    readonly phone?: string;
  };
}

export function validateProfileInput(input: {
  displayName: string;
  phone?: string | null;
}): ProfileValidationResult {
  const errors: { displayName?: string; phone?: string } = {};

  if (!isValidProfileName(input.displayName)) {
    errors.displayName =
      "El nombre debe contener solo letras y espacios, y tener entre 1 y 120 caracteres.";
  }

  if (input.phone && input.phone.trim() !== "") {
    if (!isValidChileanPhone(input.phone)) {
      errors.phone =
        "El teléfono debe ser un número chileno válido de 9 dígitos.";
    }
  }

  return {
    valid: Object.keys(errors).length === 0,
    errors,
  };
}

export function normalizeProfileUpdatePayload(input: {
  displayName: string;
  phone?: string | null;
}): ProfileUpdatePayload {
  const trimmedName = input.displayName.trim();
  const normalizedPhone = normalizeChileanPhone(input.phone);

  const payload: ProfileUpdatePayload = {
    display_name: trimmedName,
  };

  if (normalizedPhone !== null) {
    payload.phone = normalizedPhone;
  }

  return payload;
}
