/** Tracks API-confirmed time zones and invalidates replies when draft selection changes. */
export class CalendarRequestState {
    sequence = 0;
    confirmed = new Map();
    beginSelection() {
        this.sequence += 1;
        return this.sequence;
    }
    snapshot() { return this.sequence; }
    accepts(request, requestedSpaceID, selectedSpaceID) {
        return request === this.sequence && requestedSpaceID !== "" && requestedSpaceID === selectedSpaceID;
    }
    confirm(spaceID, timeZone, request, selectedSpaceID) {
        if (!this.accepts(request, spaceID, selectedSpaceID))
            return false;
        this.confirmed.set(spaceID, timeZone);
        return true;
    }
    zoneFor(spaceID, editableZone) {
        const confirmed = this.confirmed.get(spaceID);
        if (!confirmed)
            throw new Error("Consulta la zona horaria guardada por la API.");
        if (editableZone !== confirmed)
            throw new Error("Guarda la zona horaria antes de consultar o crear bloqueos.");
        return confirmed;
    }
    invalidate(spaceID) {
        this.confirmed.delete(spaceID);
    }
}
