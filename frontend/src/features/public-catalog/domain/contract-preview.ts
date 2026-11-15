import type { components } from "@/lib/api/generated/openapi";
import type { ContractPreviewErrorCode } from "./contract-preview-error";

export type RentalContractPreview =
  components["schemas"]["RentalContractPreview"];

export type RentalContractPreviewResult =
  | { ok: true; value: RentalContractPreview }
  | {
      ok: false;
      error: { kind: ContractPreviewErrorCode; status: number };
    };
