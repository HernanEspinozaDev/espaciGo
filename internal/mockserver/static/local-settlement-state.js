export class LocalSettlementState {
    attempts = new Map();
    inFlight = new Set();
    revision = 0;
    begin(reservationID, outcome, createKey) {
        if (!reservationID || this.inFlight.has(reservationID))
            return null;
        let attempt = this.attempts.get(reservationID);
        if (!attempt) {
            attempt = { reservationID, outcome, idempotencyKey: createKey() };
            this.attempts.set(reservationID, attempt);
        }
        this.inFlight.add(reservationID);
        return { ...attempt };
    }
    finish(reservationID, terminal) {
        this.inFlight.delete(reservationID);
        if (terminal)
            this.attempts.delete(reservationID);
    }
    capture(reservationID, accountID, token, generation) {
        return { reservationID, accountID, token, generation, revision: this.revision };
    }
    accepts(context, current) {
        return Boolean(context.reservationID && context.reservationID === current.reservationID && context.accountID === current.accountID && context.token === current.token && context.generation === current.generation && context.revision === this.revision);
    }
    clear() {
        this.revision++;
        this.attempts.clear();
        this.inFlight.clear();
    }
    get(reservationID) {
        const value = this.attempts.get(reservationID);
        return value ? { ...value } : null;
    }
    isInFlight(reservationID) { return this.inFlight.has(reservationID); }
}
