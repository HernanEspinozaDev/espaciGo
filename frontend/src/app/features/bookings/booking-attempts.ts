import { Injectable, signal } from '@angular/core';

export type BookingAttempt = { key: string; payload: string; uncertain: boolean };
export type ConfirmedRefundSnapshot = { operationId: string; amount: string; state: 'pendiente' | 'completada'; revision: number };

/** In-memory only. Every unsettled write keeps its exact key and payload for a retry. */
@Injectable({ providedIn: 'root' })
export class BookingAttempts {
  private readonly attempts = signal<Record<string, BookingAttempt>>({});
  private readonly refunds = signal<Record<string, ConfirmedRefundSnapshot>>({});
  private revision = 0;

  nextRevision(): number { return ++this.revision; }

  refundSnapshot(accountId: string, reservationId: string): ConfirmedRefundSnapshot | null {
    return this.refunds()[accountId + ':' + reservationId] ?? null;
  }

  refundRevision(accountId: string, reservationId: string): number {
    return this.refundSnapshot(accountId, reservationId)?.revision ?? 0;
  }

  saveRefundSnapshot(accountId: string, reservationId: string, snapshot: ConfirmedRefundSnapshot): void {
    const id = accountId + ':' + reservationId;
    this.refunds.update((items) => (items[id]?.revision ?? 0) > snapshot.revision
      ? items
      : { ...items, [id]: snapshot });
  }

  get(accountId: string, operation: string, payload: unknown): BookingAttempt {
    const id = `${accountId}:${operation}`;
    const serialized = JSON.stringify(payload);
    const prior = this.attempts()[id];
    if (prior) {
      if (prior.payload !== serialized) throw new Error('Hay un intento incierto. Reintenta con los mismos datos antes de cambiar la operación.');
      return prior;
    }
    const attempt = { key: crypto.randomUUID(), payload: serialized, uncertain: true };
    this.attempts.update((items) => ({ ...items, [id]: attempt }));
    return attempt;
  }

  find(accountId: string, operation: string): BookingAttempt | null {
    return this.attempts()[accountId + ':' + operation] ?? null;
  }

  markUncertain(accountId: string, operation: string, key: string): void {
    const id = accountId + ':' + operation;
    this.attempts.update((items) => items[id]?.key === key
      ? { ...items, [id]: { ...items[id], uncertain: true } }
      : items);
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
}
