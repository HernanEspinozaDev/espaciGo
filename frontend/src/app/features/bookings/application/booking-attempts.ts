import { Injectable } from '@angular/core';
import { BookingAttemptRegistry } from '../domain/booking-attempt-registry';
export type { BookingAttempt, ConfirmedRefundSnapshot } from '../domain/booking-attempt-registry';

/** Angular lifetime/UUID binding for the pure in-memory ledger. */
@Injectable({ providedIn: 'root' })
export class BookingAttempts extends BookingAttemptRegistry {
  constructor() {
    super(() =>
      globalThis.crypto?.randomUUID?.() ??
      `booking-${Date.now()}-${Math.random().toString(36).slice(2)}`,
    );
  }
}
