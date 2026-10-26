import "client-only";

import { authFacade } from "@/features/auth/auth-runtime";
import { privateBlobUrls } from "@/lib/api/private-blob-urls";
import { BookingsFacade } from "./application/bookings-facade";
import { OpenApiBookingsAdapter } from "./infrastructure/bookings-openapi-adapter";

export const bookingsAdapter = new OpenApiBookingsAdapter();
export const bookingsFacade = new BookingsFacade(
  bookingsAdapter,
  authFacade,
  () => privateBlobUrls.clear(),
);
