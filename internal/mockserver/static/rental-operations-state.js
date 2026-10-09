export function rentalOperationControls(accountID, reservation, operations, hasClaim, claim, hasSession) {
    const kinds = new Set(operations.map(item => item.kind));
    return {
        canCheckIn: Boolean(hasSession && reservation && reservation.renter_id === accountID && reservation.state === "lista_para_checkin" && !kinds.has("checkin")),
        canCheckOut: Boolean(hasSession && reservation && reservation.renter_id === accountID && reservation.state === "en_curso" && !kinds.has("checkout")),
        canReceive: Boolean(hasSession && reservation && reservation.host_id === accountID && reservation.state === "finalizada" && !kinds.has("recepcion")),
        canOpenClaim: Boolean(hasSession && reservation && reservation.host_id === accountID && reservation.state === "finalizada" && operations.some(item => item.kind === "checkout") && !hasClaim),
        canDefend: Boolean(hasSession && reservation && reservation.renter_id === accountID && reservation.state === "en_disputa" && hasClaim && claim?.state === "abierto" && !claim.defense),
    };
}
export function captureRentalOperationContext(value) { return { ...value }; }
export function rentalOperationResponseIsCurrent(context, current) { return context.revision === current.revision && context.reservationID === current.reservationID && context.accountID === current.accountID && context.token === current.token && context.generation === current.generation && Boolean(context.token); }
export class RentalOperationIdempotencyKeys {
    values = new Map();
    get(reservationID, action, payload, generate) { const id = `${reservationID}:${action}`, prior = this.values.get(id); if (prior?.payload === payload)
        return prior.key; const next = { key: generate(), payload }; this.values.set(id, next); return next.key; }
    clear(reservationID, action) { this.values.delete(`${reservationID}:${action}`); }
    clearAll() { this.values.clear(); }
}
