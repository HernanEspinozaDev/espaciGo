import type { components, Readable } from "@/lib/api/generated/openapi";

export type DocumentVerificationCase = Readable<
  components["schemas"]["DocumentVerificationOwnerCase"]
>;
export type DocumentVerificationFile = Readable<
  components["schemas"]["DocumentVerificationOwnerFile"]
>;
export type DocumentVerificationCreate =
  components["schemas"]["DocumentVerificationCaseCreate"];

export interface OwnerSpaceOption {
  readonly id: string;
  readonly title: string;
}

export function normalizeDocumentVerificationCreate(
  input: unknown,
): DocumentVerificationCreate {
  if (!input || typeof input !== "object" || Array.isArray(input))
    throw new TypeError("Invalid document verification create payload");
  const value = input as Record<string, unknown>;
  if (value.purpose === "identity") {
    if (
      Object.keys(value).some(
        (key) => key !== "purpose" && key !== "document_expires_at",
      ) ||
      typeof value.document_expires_at !== "string" ||
      !value.document_expires_at
    )
      throw new TypeError("Invalid identity document verification payload");
    return {
      purpose: "identity",
      document_expires_at: value.document_expires_at,
    };
  }
  if (value.purpose === "space") {
    if (
      Object.keys(value).some(
        (key) => !["purpose", "space_id", "document_expires_at"].includes(key),
      ) ||
      typeof value.space_id !== "string" ||
      !value.space_id ||
      (value.document_expires_at !== undefined &&
        (typeof value.document_expires_at !== "string" ||
          !value.document_expires_at))
    )
      throw new TypeError("Invalid space document verification payload");
    return value.document_expires_at === undefined
      ? { purpose: "space", space_id: value.space_id }
      : {
          purpose: "space",
          space_id: value.space_id,
          document_expires_at: value.document_expires_at as string,
        };
  }
  throw new TypeError("Unsupported document verification purpose");
}

export function isStrongETag(value: string | null): value is string {
  return value !== null && /^\"(?:0|[1-9][0-9]*)\"$/.test(value);
}
