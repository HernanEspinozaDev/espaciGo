export class AdminReservationsState {
    revision = 0;
    cursor = "";
    nextCursor = "";
    loading = false;
    begin(accountID, token, generation) {
        this.loading = true;
        return { revision: ++this.revision, accountID, token, generation };
    }
    current(context, accountID, token, generation) {
        return context.revision === this.revision && context.accountID === accountID && context.token === token && context.generation === generation && Boolean(token);
    }
    finish(context, accountID, token, generation) {
        if (!this.current(context, accountID, token, generation))
            return false;
        this.loading = false;
        return true;
    }
    clear() { this.revision++; this.cursor = ""; this.nextCursor = ""; this.loading = false; }
}
