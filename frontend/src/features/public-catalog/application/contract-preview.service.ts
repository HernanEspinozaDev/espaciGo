import type { ContractPreviewPort } from "./contract-preview.port";

export function createContractPreviewService(port: ContractPreviewPort) {
  return {
    getPreview(quoteId: string, signal?: AbortSignal) {
      return port.getPreview(quoteId, signal);
    },
  };
}
