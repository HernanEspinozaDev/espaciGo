import { Injectable, inject, signal } from '@angular/core';
import { PrivateSessionState } from '../../core/identity/private-session-state';

export type BookingAttempt = { key: string; payload: string };

/** In-memory only. An uncertain request keeps its exact key and payload for a retry. */
@Injectable({ providedIn: 'root' })
export class BookingAttempts {
  private readonly privateState = inject(PrivateSessionState);
  private readonly attempts = signal<Record<string, BookingAttempt>>({});

  constructor() { this.privateState.register(() => this.reset()); }

  get(accountId: string, operation: string, payload: unknown): BookingAttempt {
    const id = `${accountId}:${operation}`;
    const serialized = JSON.stringify(payload);
    const prior = this.attempts()[id];
    if (prior) {
      if (prior.payload !== serialized) throw new Error('Hay un intento incierto. Reintenta con los mismos datos antes de cambiar la operación.');
      return prior;
    }
    const attempt = { key: crypto.randomUUID(), payload: serialized };
    this.attempts.update((items) => ({ ...items, [id]: attempt }));
    return attempt;
  }

  settle(accountId: string, operation: string, key: string): void {
    const id = `${accountId}:${operation}`;
    this.attempts.update((items) => {
      if (items[id]?.key !== key) return items;
      const next = { ...items };
      delete next[id];
      return next;
    });
  }

  reset(): void { this.attempts.set({}); }
}
