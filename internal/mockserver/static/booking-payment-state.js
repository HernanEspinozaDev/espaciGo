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
