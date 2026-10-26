import "client-only";

import { HostListingsFacade } from "./application/host-listings-facade";
import { OpenApiHostListingsAdapter } from "./infrastructure/host-listings-openapi-adapter";

export const hostListingsAdapter = new OpenApiHostListingsAdapter();
export const hostListingsFacade = new HostListingsFacade(hostListingsAdapter);
