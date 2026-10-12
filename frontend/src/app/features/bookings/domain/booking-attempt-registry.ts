export type BookingAttempt = { key: string; payload: string; uncertain: boolean };
export type ConfirmedRefundSnapshot = {
  operationId: string;
  amount: string;
  state: 'pendiente' | 'completada';
  revision: number;
};

/** Pure frontend retry ledger. It stores intent, never authentication or backend business policy. */
export class BookingAttemptRegistry {
  private readonly attempts = new Map<string, BookingAttempt>();
  private readonly refunds = new Map<string, ConfirmedRefundSnapshot>();
  private revision = 0;
  constructor(private readonly newKey: () => string) {}
  nextRevision(): number {
    return ++this.revision;
  }
  refundSnapshot(accountId: string, reservationId: string): ConfirmedRefundSnapshot | null {
    return this.refunds.get(`${accountId}:${reservationId}`) ?? null;
  }
  saveRefundSnapshot(
    accountId: string,
    reservationId: string,
    snapshot: ConfirmedRefundSnapshot,
  ): void {
    const scope = `${accountId}:${reservationId}`;
    if ((this.refunds.get(scope)?.revision ?? 0) <= snapshot.revision)
      this.refunds.set(scope, snapshot);
  }
  get(accountId: string, operation: string, payload: unknown): BookingAttempt {
    const scope = `${accountId}:${operation}`;
    const serialized = JSON.stringify(payload);
    const prior = this.attempts.get(scope);
    if (prior) {
      if (prior.payload !== serialized)
        throw new Error(
          'Hay un intento incierto. Reintenta con los mismos datos antes de cambiar la operación.',
        );
      return prior;
    }
    const attempt = { key: this.newKey(), payload: serialized, uncertain: true };
    this.attempts.set(scope, attempt);
    return attempt;
  }
  find(accountId: string, operation: string): BookingAttempt | null {
    return this.attempts.get(`${accountId}:${operation}`) ?? null;
  }
  markUncertain(accountId: string, operation: string, key: string): void {
    const scope = `${accountId}:${operation}`,
      attempt = this.attempts.get(scope);
    if (attempt?.key === key) this.attempts.set(scope, { ...attempt, uncertain: true });
  }
  settle(accountId: string, operation: string, key: string): void {
    const scope = `${accountId}:${operation}`;
    if (this.attempts.get(scope)?.key === key) this.attempts.delete(scope);
  }
}
