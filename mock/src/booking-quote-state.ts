export class BookingQuoteState {
  private revision = 0;
  private selectedSpaceID: string | null = null;
  private prepared: {id: string; spaceID: string; revision: number} | null = null;

  beginSearch(): void {
    this.revision++;
    this.selectedSpaceID = null;
    this.prepared = null;
  }

  beginSelection(spaceID: string): number {
    this.revision++;
    this.selectedSpaceID = spaceID;
    this.prepared = null;
    return this.revision;
  }

  selectionIsCurrent(token: number, spaceID: string): boolean {
    return token === this.revision && this.selectedSpaceID === spaceID;
  }

  beginQuote(spaceID: string): number | null {
    return this.selectedSpaceID === spaceID ? this.revision : null;
  }

  acceptQuote(token: number, spaceID: string, quoteID: string): boolean {
    if (!quoteID || !this.selectionIsCurrent(token, spaceID)) return false;
    this.prepared = {id: quoteID, spaceID, revision: token};
    return true;
  }

  canRequest(quoteID: string, selectedSpaceID: string | null): boolean {
    return Boolean(this.prepared && quoteID && this.prepared.id === quoteID &&
      this.prepared.spaceID === selectedSpaceID && this.prepared.revision === this.revision &&
      this.selectedSpaceID === selectedSpaceID);
  }
}
