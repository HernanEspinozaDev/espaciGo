import type { RentalContractPreviewResult } from "../domain/contract-preview";

export interface ContractPreviewPort {
  getPreview(
    quoteId: string,
    signal?: AbortSignal,
  ): Promise<RentalContractPreviewResult>;
}
