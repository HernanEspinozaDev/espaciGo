function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export type ContractPreviewErrorCode =
  "identity_required" | "conflict" | "not_found" | "other";

export function classifyContractPreviewError(
  status: number,
  body: unknown,
): ContractPreviewErrorCode {
  if (status === 404) return "not_found";
  if (status !== 409) return "other";

  const error = isRecord(body) ? body.error : undefined;
  const code = isRecord(error) ? error.code : undefined;
  return code === "identity_required" ? "identity_required" : "conflict";
}
