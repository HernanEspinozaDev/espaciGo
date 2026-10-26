import "client-only";

import { authFacade } from "@/features/auth/auth-runtime";
import { privateBlobUrls } from "@/lib/api/private-blob-urls";
import { AdminFacade } from "./application/admin-facade";
import { OpenApiAdminAdapter } from "./infrastructure/admin-openapi-adapter";

export const adminAdapter = new OpenApiAdminAdapter();
export const adminFacade = new AdminFacade(adminAdapter, authFacade, () =>
  privateBlobUrls.clear(),
);
