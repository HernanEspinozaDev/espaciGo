import type { AccessToken } from "@/features/auth/application/auth.port";
import type { ListingReturnsResult } from "../domain/listing-returns";

export interface ListingReturnsPort {
  listReturns(
    token: AccessToken,
    spaceId: string,
  ): Promise<ListingReturnsResult>;
  confirmVacancy(
    token: AccessToken,
    reservationId: string,
    idempotencyKey: string,
  ): Promise<unknown>;
}
