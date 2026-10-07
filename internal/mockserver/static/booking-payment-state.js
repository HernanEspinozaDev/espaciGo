/** Prefer a freshly loaded API state to the error from the preceding payment request. */
export function paymentPanelAfterError(refreshedState, hasAttempt) {
    if (refreshedState === "pagada") {
        return { message: "Pago fake confirmado por la API. La reserva espera la decisión del anfitrión.", buttonLabel: "Enviar pago de ensayo", clearAttempt: true };
    }
    if (refreshedState === "cancelada_por_pago") {
        return { message: "La API confirmó el rechazo del pago fake; la reserva quedó cancelada.", buttonLabel: "Enviar pago de ensayo", clearAttempt: true };
    }
    if (refreshedState === "vencida_pago") {
        return { message: "La API confirmó que venció el plazo de pago. No se iniciará otro intento.", buttonLabel: "Enviar pago de ensayo", clearAttempt: true };
    }
    if (refreshedState && refreshedState !== "pendiente_de_pago") {
        const labels = {
            cancelada_arrendatario: "cancelada por el arrendatario",
            rechazada_arrendador: "rechazada por el anfitrión",
            vencida_host: "vencida por falta de decisión del anfitrión",
            aprobada_host: "aprobada por el anfitrión",
        };
        return { message: `La API confirmó que la reserva está ${labels[refreshedState] ?? refreshedState}; no corresponde reintentar el pago.`, buttonLabel: "Enviar pago de ensayo", clearAttempt: true };
    }
    if (refreshedState === "pendiente_de_pago" && hasAttempt) {
        return { message: "La reserva continúa pendiente y el resultado no está confirmado. Consulta/reintenta con la misma clave y el mismo resultado.", buttonLabel: "Consultar / reintentar pago (misma clave)", clearAttempt: false };
    }
    return { message: hasAttempt ? "No se pudo actualizar el estado de la reserva. Conservamos la misma clave y solicitud para consultar/reintentar el mismo intento." : "No se pudo actualizar el estado de la reserva; vuelve a consultar tu bandeja.", buttonLabel: hasAttempt ? "Consultar / reintentar pago (misma clave)" : "Enviar pago de ensayo", clearAttempt: false };
}
export class BookingPaymentState {
    attempts = new Map();
    inFlight = new Set();
    begin(reservationID, outcome, createKey) {
        if (!reservationID || this.inFlight.has(reservationID))
            return null;
        let attempt = this.attempts.get(reservationID);
        if (!attempt) {
            attempt = { reservationID, idempotencyKey: createKey(), outcome };
            this.attempts.set(reservationID, attempt);
        }
        this.inFlight.add(reservationID);
        return { ...attempt };
    }
    finish(reservationID, result) {
        this.inFlight.delete(reservationID);
        if (result === "completed")
            this.attempts.delete(reservationID);
    }
    get(reservationID) {
        const attempt = this.attempts.get(reservationID);
        return attempt ? { ...attempt } : null;
    }
    isInFlight(reservationID) {
        return this.inFlight.has(reservationID);
    }
    clearCompleted(reservationID) {
        if (!this.inFlight.has(reservationID))
            this.attempts.delete(reservationID);
    }
}
export async function executePaymentAttempt(state, reservationID, outcome, createKey, send) {
    const attempt = state.begin(reservationID, outcome, createKey);
    if (!attempt)
        return { status: "busy" };
    try {
        const value = await send(attempt);
        state.finish(reservationID, "completed");
        return { status: "completed", attempt, value };
    }
    catch (error) {
        state.finish(reservationID, "uncertain");
        return { status: "uncertain", attempt, error };
    }
}
export class BookingRequestState {
    selectedReservationID = "";
    selectionRevision = 0;
    select(reservationID) {
        if (this.selectedReservationID === reservationID)
            return;
        this.selectedReservationID = reservationID;
        this.selectionRevision++;
    }
    invalidate() {
        this.selectedReservationID = "";
        this.selectionRevision++;
    }
    capture(accountID, sessionToken) {
        return {
            reservationID: this.selectedReservationID,
            accountID,
            sessionToken,
            selectionRevision: this.selectionRevision,
        };
    }
    accepts(context, currentAccountID, currentSessionToken) {
        return Boolean(context.reservationID &&
            context.reservationID === this.selectedReservationID &&
            context.selectionRevision === this.selectionRevision &&
            context.accountID === currentAccountID &&
            context.sessionToken === currentSessionToken);
    }
}
