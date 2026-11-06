import { apiClient } from "@/lib/api/client";
import type { AccessToken } from "@/features/auth/application/auth.port";
import type { ListingReturnsResult } from "../domain/listing-returns";
import type { ListingReturnsPort } from "../application/listing-returns.port";

export class ListingReturnsOpenApiAdapter implements ListingReturnsPort {
  async listReturns(
    token: AccessToken,
    spaceId: string,
  ): Promise<ListingReturnsResult> {
    const { data, error } = await apiClient.GET(
      "/local/booking-trial/spaces/{spaceID}/returns",
      {
        params: { path: { spaceID: spaceId } },
        headers: { Authorization: `Bearer ${token}` },
      },
    );
    if (error || !data) throw new Error("No fue posible cargar entregas.");
    return data.data;
  }
  async confirmVacancy(
    token: AccessToken,
    reservationId: string,
    idempotencyKey: string,
  ): Promise<unknown> {
    const { data, error } = await apiClient.POST(
      "/local/booking-trial/reservations/{reservationID}/reception",
      {
        params: {
          path: { reservationID: reservationId },
          header: { "Idempotency-Key": idempotencyKey },
        },
        headers: { Authorization: `Bearer ${token}` },
        // The existing operation derives the result from observations. It
        // rejects a client-selected result with DisallowUnknownFields.
        body: {},
      },
    );
    if (error || !data)
      throw new Error("No fue posible confirmar la desocupación.");
    return data;
  }
}
