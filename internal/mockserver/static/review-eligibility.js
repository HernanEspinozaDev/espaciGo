export function reviewableReservationState(state) {
    return state === "finalizada" || state === "cerrada";
}
