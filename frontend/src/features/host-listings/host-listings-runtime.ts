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

import { createHostListingsFeatureBridge } from "./application/host-listings-feature-bridge";
import { ListingReturnsFacade } from "./application/listing-returns-facade";
import { ListingReturnsOpenApiAdapter } from "./infrastructure/listing-returns-openapi-adapter";

export const hostListingsFeatureBridge = createHostListingsFeatureBridge(
  hostListingsFacade,
  authFacade,
);
export const listingReturnsFacade = new ListingReturnsFacade(
  new ListingReturnsOpenApiAdapter(),
);
let returnScope = authFacade.getAccessToken();
authFacade.subscribe(() => {
  const next = authFacade.getAccessToken();
  if (next !== returnScope) {
    returnScope = next;
    listingReturnsFacade.clear();
  }
});
