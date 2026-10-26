import type {
  components,
  Readable,
  Writable,
} from "@/lib/api/generated/openapi";

export type VerificationType = Writable<
  components["schemas"]["VerificationCase"]["type"]
>; // "kyc" | "kyb"

export type VerificationState = Writable<
  components["schemas"]["VerificationCase"]["state"]
>; // "en_revision" | "aprobada" | "rechazada" | "revocada"

export type VerificationCorrectionCode = Writable<
  NonNullable<components["schemas"]["VerificationCase"]["correction_code"]>
>;

export type VerificationHistoryAction = Writable<
  components["schemas"]["VerificationHistoryList"]["items"][number]["action"]
>;

export type VerificationCase = Readable<
  components["schemas"]["VerificationCase"]
>;
export type VerificationEligibilityItem = Readable<
  components["schemas"]["VerificationEligibilityList"]["items"][number]
>;
export type VerificationEligibilityList = Readable<
  components["schemas"]["VerificationEligibilityList"]
>;
export type VerificationHistoryEntry = Readable<
  components["schemas"]["VerificationHistoryList"]["items"][number]
>;
export type SyntheticEvidence = Readable<
  components["schemas"]["SyntheticEvidence"]
>;

export const SYNTHETIC_EVIDENCE_FIXTURE_CODE = "synthetic-png-v1" as const;

export const VERIFICATION_STATES: readonly VerificationState[] = [
  "en_revision",
  "aprobada",
  "rechazada",
  "revocada",
] as const;

export const VERIFICATION_CORRECTION_CODES: readonly VerificationCorrectionCode[] =
  [
    "fixture_vigente_actualizado",
    "antecedentes_fixture_actualizados",
    "inicio_actividades_fixture_actualizadas",
    "legado_pre_v28_sin_codigo",
  ] as const;

export function isVerificationEligible(
  eligibilityList: readonly VerificationEligibilityItem[] | null | undefined,
  type: VerificationType,
): boolean {
  if (!eligibilityList || !Array.isArray(eligibilityList)) return false;
  return eligibilityList.some(
    (item) => item.type === type && item.eligible === true,
  );
}

export function canRetryVerification(
  verif: VerificationCase | null | undefined,
): boolean {
  return verif?.state === "rechazada";
}

export function normalizeVerificationCreatePayload(type: VerificationType): {
  type: VerificationType;
} {
  if (type !== "kyc" && type !== "kyb") {
    throw new Error(`Tipo de verificación no soportado: ${type}`);
  }
  return { type };
}

export function normalizeVerificationRetryPayload(
  correctionCode: VerificationCorrectionCode,
): {
  correction_code:
    | "fixture_vigente_actualizado"
    | "antecedentes_fixture_actualizados"
    | "inicio_actividades_fixture_actualizadas";
} {
  if (
    correctionCode !== "fixture_vigente_actualizado" &&
    correctionCode !== "antecedentes_fixture_actualizados" &&
    correctionCode !== "inicio_actividades_fixture_actualizadas"
  ) {
    throw new Error(`Código de corrección no válido: ${correctionCode}`);
  }
  return { correction_code: correctionCode };
}

export function normalizeSyntheticEvidenceCreatePayload(): {
  fixture_code: typeof SYNTHETIC_EVIDENCE_FIXTURE_CODE;
} {
  return { fixture_code: SYNTHETIC_EVIDENCE_FIXTURE_CODE };
}
