export interface SyntheticPayoutAccount {
  readonly referenceId: string;
  readonly status: "activa_fake";
  readonly notice: string;
  readonly createdAt?: string;
}

export const SYNTHETIC_PAYOUT_STATUS = "activa_fake" as const;
export const SYNTHETIC_PAYOUT_NOTICE_PREFIX =
  "Referencia local ficticia" as const;

export function isSyntheticPayoutAccountActive(
  account: SyntheticPayoutAccount | null | undefined,
): boolean {
  return (
    Boolean(account) &&
    account?.status === SYNTHETIC_PAYOUT_STATUS &&
    typeof account?.referenceId === "string" &&
    account.referenceId.trim().length > 0
  );
}

export function normalizeSyntheticPayoutAccount(
  raw: unknown,
): SyntheticPayoutAccount | null {
  if (!raw || typeof raw !== "object") return null;
  const candidate = raw as Record<string, unknown>;
  const referenceId =
    typeof candidate.reference_id === "string"
      ? candidate.reference_id
      : typeof candidate.referenceId === "string"
        ? candidate.referenceId
        : "";
  const status =
    candidate.status === "activa_fake" ? "activa_fake" : "activa_fake";
  const notice =
    typeof candidate.notice === "string"
      ? candidate.notice
      : "Referencia local ficticia; no contiene datos bancarios.";
  const createdAt =
    typeof candidate.created_at === "string"
      ? candidate.created_at
      : typeof candidate.createdAt === "string"
        ? candidate.createdAt
        : undefined;

  if (!referenceId) return null;

  return {
    referenceId,
    status,
    notice,
    ...(createdAt ? { createdAt } : {}),
  };
}
