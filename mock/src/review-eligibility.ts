export function reviewableReservationState(state: string | undefined): boolean {
  return state === "finalizada" || state === "cerrada";
}
