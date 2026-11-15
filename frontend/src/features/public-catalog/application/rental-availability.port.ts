import type { PublicRentalAvailabilityResult } from "../domain/rental-availability";

export interface RentalAvailabilityPort {
  getAvailability(
    spaceId: string,
    date: string,
    duration: number,
    signal?: AbortSignal,
  ): Promise<PublicRentalAvailabilityResult>;
}
