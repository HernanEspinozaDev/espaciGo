/** Tracks API-confirmed time zones and invalidates replies when draft selection changes. */
export class CalendarRequestState {
  private sequence = 0;
  private confirmed = new Map<string, string>();

  beginSelection(): number {
    this.sequence += 1;
    return this.sequence;
  }

  snapshot(): number { return this.sequence; }

  accepts(request: number, requestedSpaceID: string, selectedSpaceID: string): boolean {
    return request === this.sequence && requestedSpaceID !== "" && requestedSpaceID === selectedSpaceID;
  }

  confirm(spaceID: string, timeZone: string, request: number, selectedSpaceID: string): boolean {
    if (!this.accepts(request, spaceID, selectedSpaceID)) return false;
    this.confirmed.set(spaceID, timeZone);
    return true;
  }

  zoneFor(spaceID: string, editableZone: string): string {
    const confirmed = this.confirmed.get(spaceID);
    if (!confirmed) throw new Error("Consulta la zona horaria guardada por la API.");
    if (editableZone !== confirmed) throw new Error("Guarda la zona horaria antes de consultar o crear bloqueos.");
    return confirmed;
  }

  invalidate(spaceID: string): void {
    this.confirmed.delete(spaceID);
  }
}
