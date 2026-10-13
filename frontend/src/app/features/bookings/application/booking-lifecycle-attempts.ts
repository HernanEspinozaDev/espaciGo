import { Injectable } from '@angular/core';
import type { LocalRentalOperationInput } from '../../../core/contracts/models';
import { Attempt } from '../domain/ports/booking-lifecycle-port';
@Injectable({ providedIn: 'root' })
export class BookingLifecycleAttempts {
  private readonly attempts = new Map<string, Attempt>();

  get(
    accountId: string,
    reservationId: string,
    action: string,
    payload: LocalRentalOperationInput,
  ): Attempt {
    const id = this.id(accountId, reservationId, action);
    const serialized = JSON.stringify(payload);
    const prior = this.attempts.get(id);
    if (prior?.payload === serialized) return prior;
    if (prior)
      throw new Error(
        'Hay un intento incierto con otro contenido. Espera la confirmación de Go antes de cambiarlo.',
      );
    const attempt = { key: crypto.randomUUID(), payload: serialized };
    this.attempts.set(id, attempt);
    return attempt;
  }
  settle(accountId: string, reservationId: string, action: string, key: string): void {
    const id = this.id(accountId, reservationId, action);
    if (this.attempts.get(id)?.key === key) this.attempts.delete(id);
  }
  markUncertain(accountId: string, reservationId: string, action: string, key: string): void {
    if (this.attempts.get(this.id(accountId, reservationId, action))?.key !== key) return;
    // Presence alone denotes an uncertain attempt; payload/key remain unchanged.
  }
  find(accountId: string, reservationId: string, action: string): Attempt | null {
    return this.attempts.get(this.id(accountId, reservationId, action)) ?? null;
  }
  private id(accountId: string, reservationId: string, action: string): string {
    return `${accountId}:${reservationId}:${action}`;
  }
}
