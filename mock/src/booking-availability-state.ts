export interface AvailabilityContext {
  spaceID: string;
  date: string;
  duration: string;
  accountID: string;
  sessionToken: string;
}

export interface SelectedAvailability {
  startAt: string;
  endAt: string;
  startLocal: string;
  endLocal: string;
}

export class BookingAvailabilityState {
  private generation = 0;
  private context: AvailabilityContext | null = null;
  private selected: SelectedAvailability | null = null;

  beginRequest(context: AvailabilityContext): number {
    this.generation++;
    this.context = { ...context };
    this.selected = null;
    return this.generation;
  }

  accepts(token: number, current: AvailabilityContext): boolean {
    return token === this.generation && this.sameContext(this.context, current);
  }

  select(token: number, current: AvailabilityContext, interval: SelectedAvailability): boolean {
    if (!this.accepts(token, current)) return false;
    this.selected = { ...interval };
    return true;
  }

  selectedFor(current: AvailabilityContext, startLocal: string, endLocal: string): SelectedAvailability | null {
    if (!this.sameContext(this.context, current) || !this.selected ||
        this.selected.startLocal !== startLocal || this.selected.endLocal !== endLocal) return null;
    return { ...this.selected };
  }

  invalidate(): void {
    this.generation++;
    this.context = null;
    this.selected = null;
  }

  private sameContext(left: AvailabilityContext | null, right: AvailabilityContext): boolean {
    return !!left && left.spaceID === right.spaceID && left.date === right.date &&
      left.duration === right.duration && left.accountID === right.accountID &&
      left.sessionToken === right.sessionToken;
  }
}
