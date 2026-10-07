export type PaymentAttemptResult = "completed" | "uncertain";

export interface PaymentAttempt {
  reservationID: string;
  idempotencyKey: string;
  outcome: string;
}

export type PaymentExecution<T> =
  | {status: "busy"}
  | {status: "completed"; attempt: PaymentAttempt; value: T}
  | {status: "uncertain"; attempt: PaymentAttempt; error: unknown};

export interface BookingRequestContext {
  reservationID: string;
  accountID: string;
  sessionToken: string;
  selectionRevision: number;
}

export class BookingPaymentState {
  private readonly attempts = new Map<string, PaymentAttempt>();
  private readonly inFlight = new Set<string>();

  begin(reservationID: string, outcome: string, createKey: () => string): PaymentAttempt | null {
    if (!reservationID || this.inFlight.has(reservationID)) return null;
    let attempt = this.attempts.get(reservationID);
    if (!attempt) {
      attempt = {reservationID, idempotencyKey: createKey(), outcome};
      this.attempts.set(reservationID, attempt);
    }
    this.inFlight.add(reservationID);
    return {...attempt};
  }

  finish(reservationID: string, result: PaymentAttemptResult): void {
    this.inFlight.delete(reservationID);
    if (result === "completed") this.attempts.delete(reservationID);
  }

  get(reservationID: string): PaymentAttempt | null {
    const attempt = this.attempts.get(reservationID);
    return attempt ? {...attempt} : null;
  }

  isInFlight(reservationID: string): boolean {
    return this.inFlight.has(reservationID);
  }

  clearCompleted(reservationID: string): void {
    if (!this.inFlight.has(reservationID)) this.attempts.delete(reservationID);
  }
}

export async function executePaymentAttempt<T>(
  state: BookingPaymentState,
  reservationID: string,
  outcome: string,
  createKey: () => string,
  send: (attempt: PaymentAttempt) => Promise<T>,
): Promise<PaymentExecution<T>> {
  const attempt = state.begin(reservationID, outcome, createKey);
  if (!attempt) return {status: "busy"};
  try {
    const value = await send(attempt);
    state.finish(reservationID, "completed");
    return {status: "completed", attempt, value};
  } catch (error) {
    state.finish(reservationID, "uncertain");
    return {status: "uncertain", attempt, error};
  }
}

export class BookingRequestState {
  private selectedReservationID = "";
  private selectionRevision = 0;

  select(reservationID: string): void {
    if (this.selectedReservationID === reservationID) return;
    this.selectedReservationID = reservationID;
    this.selectionRevision++;
  }

  invalidate(): void {
    this.selectedReservationID = "";
    this.selectionRevision++;
  }

  capture(accountID: string, sessionToken: string): BookingRequestContext {
    return {
      reservationID: this.selectedReservationID,
      accountID,
      sessionToken,
      selectionRevision: this.selectionRevision,
    };
  }

  accepts(context: BookingRequestContext, currentAccountID: string, currentSessionToken: string): boolean {
    return Boolean(
      context.reservationID &&
      context.reservationID === this.selectedReservationID &&
      context.selectionRevision === this.selectionRevision &&
      context.accountID === currentAccountID &&
      context.sessionToken === currentSessionToken,
    );
  }
}
