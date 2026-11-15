import "client-only";

import { createRentalAvailabilityService } from "../application/rental-availability.service";
import { createRentalAvailabilityAdapter } from "./rental-availability-openapi-adapter";

export const publicRentalAvailability = createRentalAvailabilityService(
  createRentalAvailabilityAdapter(),
);
