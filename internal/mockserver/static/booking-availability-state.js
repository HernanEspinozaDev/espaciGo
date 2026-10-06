export class BookingAvailabilityState {
    generation = 0;
    context = null;
    selected = null;
    beginRequest(context) {
        this.generation++;
        this.context = { ...context };
        this.selected = null;
        return this.generation;
    }
    accepts(token, current) {
        return token === this.generation && this.sameContext(this.context, current);
    }
    select(token, current, interval) {
        if (!this.accepts(token, current))
            return false;
        this.selected = { ...interval };
        return true;
    }
    selectedFor(current, startLocal, endLocal) {
        if (!this.sameContext(this.context, current) || !this.selected ||
            this.selected.startLocal !== startLocal || this.selected.endLocal !== endLocal)
            return null;
        return { ...this.selected };
    }
    invalidate() {
        this.generation++;
        this.context = null;
        this.selected = null;
    }
    sameContext(left, right) {
        return !!left && left.spaceID === right.spaceID && left.date === right.date &&
            left.duration === right.duration && left.accountID === right.accountID &&
            left.sessionToken === right.sessionToken;
    }
}
