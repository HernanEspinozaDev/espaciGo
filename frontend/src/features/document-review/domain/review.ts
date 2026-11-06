import type { components, Readable } from "@/lib/api/generated/openapi";

export type ReviewCase = Readable<
  components["schemas"]["DocumentVerificationOwnerCase"]
>;
export type ReviewFile = Readable<
  components["schemas"]["DocumentVerificationOwnerFile"]
>;
export type ReviewDecision = "approve" | "reject" | "needs_evidence";
export type ReviewCapability = "review_identity" | "review_space";
export interface ReviewInput {
  capability: ReviewCapability;
  decision: ReviewDecision;
  expected_revision: number;
  reason_code?: string;
  verified_until?: string;
}
export interface RevokeInput {
  purpose: "identity" | "space";
  reason_code: string;
  expected_revision: number;
}
export function revisionNumber(value: number | string): number {
  const parsed = Number(value);
  if (!Number.isSafeInteger(parsed) || parsed < 0)
    throw new TypeError("Invalid case revision");
  return parsed;
}
export function validExpiry(
  value: string,
  now = Date.now(),
  documentExpiry?: string,
  maxMonths = 24,
): string | null {
  const parsed = Date.parse(value);
  const deadline = new Date(now);
  deadline.setUTCMonth(deadline.getUTCMonth() + maxMonths);
  if (!Number.isFinite(parsed) || parsed <= now || parsed > deadline.getTime())
    return null;
  if (documentExpiry && parsed > Date.parse(documentExpiry)) return null;
  return new Date(parsed).toISOString();
}
export const reviewReasonCodes = [
  "documento_ilegible",
  "documento_invalido",
  "documento_vencido",
  "cambio_titularidad",
  "antecedentes_incompletos",
] as const;
export const revokeReasons = [
  "documento_invalido",
  "documento_vencido",
  "cambio_titularidad",
  "conflicto_interes",
  "solicitud_titular",
] as const;
export function reasonsFor(decision: ReviewDecision): readonly string[] {
  if (decision === "reject") return reviewReasonCodes.slice(0, 4);
  if (decision === "needs_evidence") return ["antecedentes_incompletos"];
  return [];
}
export function canReview(value: ReviewCase): boolean {
  return (
    value.state === "en_revision" ||
    value.state === "enviada" ||
    value.state === "requiere_antecedentes"
  );
}
