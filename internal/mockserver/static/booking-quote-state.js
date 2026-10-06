export class BookingQuoteState {
    revision = 0;
    selectedSpaceID = null;
    prepared = null;
    beginSearch() {
        this.revision++;
        this.selectedSpaceID = null;
        this.prepared = null;
    }
    beginSelection(spaceID) {
        this.revision++;
        this.selectedSpaceID = spaceID;
        this.prepared = null;
        return this.revision;
    }
    selectionIsCurrent(token, spaceID) {
        return token === this.revision && this.selectedSpaceID === spaceID;
    }
    beginQuote(spaceID) {
        return this.selectedSpaceID === spaceID ? this.revision : null;
    }
    acceptQuote(token, spaceID, quoteID) {
        if (!quoteID || !this.selectionIsCurrent(token, spaceID))
            return false;
        this.prepared = { id: quoteID, spaceID, revision: token };
        return true;
    }
    canRequest(quoteID, selectedSpaceID) {
        return Boolean(this.prepared && quoteID && this.prepared.id === quoteID &&
            this.prepared.spaceID === selectedSpaceID && this.prepared.revision === this.revision &&
            this.selectedSpaceID === selectedSpaceID);
    }
}
