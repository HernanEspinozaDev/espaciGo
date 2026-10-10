export interface SettlementAttempt {
  reservationID: string;
  idempotencyKey: string;
  outcome: string;
}

export interface SettlementRequestContext {
  reservationID: string;
  accountID: string;
  token: string;
  generation: number;
  revision: number;
}

export class LocalSettlementState {
  private readonly attempts = new Map<string, SettlementAttempt>();
  private readonly inFlight = new Set<string>();
  private revision = 0;

  begin(reservationID: string, outcome: string, createKey: () => string): SettlementAttempt | null {
    if (!reservationID || this.inFlight.has(reservationID)) return null;
    let attempt = this.attempts.get(reservationID);
    if (!attempt) {
      attempt = { reservationID, outcome, idempotencyKey: createKey() };
      this.attempts.set(reservationID, attempt);
    }
    this.inFlight.add(reservationID);
    return { ...attempt };
  }

  finish(reservationID: string, terminal: boolean): void {
    this.inFlight.delete(reservationID);
    if (terminal) this.attempts.delete(reservationID);
  }

  capture(reservationID: string, accountID: string, token: string, generation: number): SettlementRequestContext {
    return { reservationID, accountID, token, generation, revision: this.revision };
  }

  accepts(context: SettlementRequestContext, current: {reservationID: string; accountID: string; token: string; generation: number}): boolean {
    return Boolean(context.reservationID && context.reservationID === current.reservationID && context.accountID === current.accountID && context.token === current.token && context.generation === current.generation && context.revision === this.revision);
  }

  clear(): void {
    this.revision++;
    this.attempts.clear();
    this.inFlight.clear();
  }

  get(reservationID: string): SettlementAttempt | null {
    const value = this.attempts.get(reservationID);
    return value ? { ...value } : null;
  }

  isInFlight(reservationID: string): boolean { return this.inFlight.has(reservationID); }
}
