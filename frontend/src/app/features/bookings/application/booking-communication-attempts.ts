import { Injectable } from '@angular/core';
import { Attempt } from '../domain/ports/booking-communication-port';
@Injectable({ providedIn: 'root' })
export class BookingCommunicationAttempts {
  private readonly attempts = new Map<string, Attempt>();
  get(scope: string, payload: unknown): Attempt {
    const serialized = JSON.stringify(payload);
    const current = this.attempts.get(scope);
    if (current) {
      if (current.payload !== serialized)
        throw new Error(
          'Hay una solicitud incierta con otro contenido; reintenta primero el contenido original.',
        );
      return current;
    }
    const attempt = { key: crypto.randomUUID(), payload: serialized };
    this.attempts.set(scope, attempt);
    return attempt;
  }
  find(scope: string): Attempt | null {
    return this.attempts.get(scope) ?? null;
  }
  settle(scope: string, key: string): void {
    if (this.attempts.get(scope)?.key === key) this.attempts.delete(scope);
  }
}
