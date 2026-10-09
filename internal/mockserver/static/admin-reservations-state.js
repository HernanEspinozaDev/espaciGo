export class AdminReservationsState {
    revision = 0;
    cursor = "";
    nextCursor = "";
    loading = false;
    criteria = "";
    updateCriteria(criteria) {
        if (criteria === this.criteria)
            return false;
        this.criteria = criteria;
        this.revision++;
        this.cursor = "";
        this.nextCursor = "";
        this.loading = false;
        return true;
    }
    begin(accountID, token, generation) {
        this.loading = true;
        return { revision: ++this.revision, accountID, token, generation, criteria: this.criteria };
    }
    current(context, accountID, token, generation, criteria = this.criteria) {
        return context.revision === this.revision && context.accountID === accountID && context.token === token && context.generation === generation && context.criteria === criteria && Boolean(token);
    }
    finish(context, accountID, token, generation, criteria = this.criteria) {
        if (!this.current(context, accountID, token, generation, criteria))
            return false;
        this.loading = false;
        return true;
    }
    clear() { this.revision++; this.cursor = ""; this.nextCursor = ""; this.loading = false; this.criteria = ""; }
}
