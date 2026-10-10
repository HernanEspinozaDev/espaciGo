export class AdminAuditState {
    revision = 0;
    cursor = "";
    nextCursor = "";
    loading = false;
    criteria = "";
    updateCriteria(value) { if (value === this.criteria)
        return false; this.criteria = value; this.revision++; this.cursor = ""; this.nextCursor = ""; this.loading = false; return true; }
    begin(accountID, token, generation) { this.loading = true; return { revision: ++this.revision, accountID, token, generation, criteria: this.criteria }; }
    current(c, accountID, token, generation, criteria = this.criteria) { return Boolean(token) && c.revision === this.revision && c.accountID === accountID && c.token === token && c.generation === generation && c.criteria === criteria; }
    finish(c, accountID, token, generation, criteria = this.criteria) { if (!this.current(c, accountID, token, generation, criteria))
        return false; this.loading = false; return true; }
    clear() { this.revision++; this.cursor = ""; this.nextCursor = ""; this.loading = false; this.criteria = ""; }
}
