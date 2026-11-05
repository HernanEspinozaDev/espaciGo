export interface ListingReturn {
  readonly reservation_id: string;
  readonly state: "finalizada";
  readonly start_at: string;
  readonly end_at: string;
  readonly check_out_recorded: boolean;
  readonly vacancy_confirmed: boolean;
}

export interface ListingReturnsResult {
  readonly space_id: string;
  readonly items: readonly ListingReturn[];
}

export interface ListingReturnsSnapshot {
  readonly spaceId: string | null;
  readonly sessionScope: string | null;
  readonly items: readonly ListingReturn[];
  readonly loading: boolean;
  readonly pendingReservationId: string | null;
  readonly uncertain: boolean;
  readonly error: string | null;
}

export const initialListingReturnsSnapshot: ListingReturnsSnapshot =
  Object.freeze({
    spaceId: null,
    sessionScope: null,
    items: Object.freeze([]),
    loading: false,
    pendingReservationId: null,
    uncertain: false,
    error: null,
  });

export const uncertainReturnMessage =
  "No pudimos confirmar el resultado. Actualiza las entregas de este espacio antes de continuar.";

export function eligibleReturns(
  items: readonly ListingReturn[],
): ListingReturn[] {
  return items.filter(
    (item) => item.state === "finalizada" && item.check_out_recorded,
  );
}
