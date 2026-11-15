import "client-only";

import { createContractPreviewService } from "../application/contract-preview.service";
import { createContractPreviewAdapter } from "./contract-preview-openapi-adapter";

export const publicRentalContractPreview = createContractPreviewService(
  createContractPreviewAdapter(),
);
