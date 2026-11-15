import type { RentalAvailabilityPort } from "./rental-availability.port";

export function createRentalAvailabilityService(port: RentalAvailabilityPort) {
  return {
    getAvailability(
      spaceId: string,
      date: string,
      duration: number,
      signal?: AbortSignal,
    ) {
      return port.getAvailability(spaceId, date, duration, signal);
    },
  };
}
