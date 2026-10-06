export class CatalogPaginationState {
    generation = 0;
    cursor = "";
    loading = false;
    beginRequest() { this.generation++; this.loading = true; return this.generation; }
    accepts(generation, sessionAtStart, sessionNow) { return generation === this.generation && sessionAtStart === sessionNow; }
    finish(generation, nextCursor) { if (generation !== this.generation)
        return; this.loading = false; this.cursor = nextCursor; }
    invalidate() { this.generation++; this.loading = false; this.cursor = ""; }
    beginNext() { if (this.loading || !this.cursor)
        return null; return this.cursor; }
    get canNext() { return !this.loading && Boolean(this.cursor); }
}
