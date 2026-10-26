import "client-only";

import { authFacade } from "@/features/auth/auth-runtime";
import { privateBlobUrls } from "@/lib/api/private-blob-urls";
import { HostListingsFacade } from "./application/host-listings-facade";
import { OpenApiHostListingsAdapter } from "./infrastructure/host-listings-openapi-adapter";

export const hostListingsAdapter = new OpenApiHostListingsAdapter();
export const hostListingsFacade = new HostListingsFacade(
  hostListingsAdapter,
  authFacade,
  () => privateBlobUrls.clear(),
);
