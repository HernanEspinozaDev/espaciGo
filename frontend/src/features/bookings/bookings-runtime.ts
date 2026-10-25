import "client-only";

import { BookingsFacade } from "./application/bookings-facade";
import { OpenApiBookingsAdapter } from "./infrastructure/bookings-openapi-adapter";

export const bookingsAdapter = new OpenApiBookingsAdapter();
export const bookingsFacade = new BookingsFacade(bookingsAdapter);
