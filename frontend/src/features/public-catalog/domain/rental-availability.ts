import type { components } from "@/lib/api/generated/openapi";
import type { CatalogResult } from "./types";

export type PublicRentalAvailability =
  components["schemas"]["PublicRentalAvailability"];
export type PublicRentalAvailabilityResult =
  CatalogResult<PublicRentalAvailability>;
