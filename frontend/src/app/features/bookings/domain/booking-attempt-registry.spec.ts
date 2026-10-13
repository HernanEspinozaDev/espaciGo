import { describe, expect, it } from 'vitest';
import { BookingAttemptRegistry } from './booking-attempt-registry';

describe('pure booking retry ledger', () => {
  it('keeps intent exact, isolates accounts and does not settle a newer attempt with an old key', () => {
    let sequence = 0;
    const registry = new BookingAttemptRegistry(() => `key-${++sequence}`);
    const first = registry.get('a', 'r:payment', { outcome: 'exito' });
    expect(registry.get('a', 'r:payment', { outcome: 'exito' })).toBe(first);
    expect(() => registry.get('a', 'r:payment', { outcome: 'rechazo' })).toThrow();
    expect(registry.get('b', 'r:payment', { outcome: 'exito' }).key).not.toBe(first.key);
    registry.settle('a', 'r:payment', first.key);
    const current = registry.get('a', 'r:payment', { outcome: 'exito' });
    registry.settle('a', 'r:payment', first.key);
    expect(registry.find('a', 'r:payment')).toBe(current);
  });
  it('does not replace a newer confirmed refund with an old snapshot', () => {
    const registry = new BookingAttemptRegistry(() => 'key');
    registry.saveRefundSnapshot('a', 'r', {
      operationId: 'refund',
      amount: '9007199254740993',
      state: 'completada',
      revision: 2,
    });
    registry.saveRefundSnapshot('a', 'r', {
      operationId: 'refund',
      amount: '9007199254740993',
      state: 'pendiente',
      revision: 1,
    });
    expect(registry.refundSnapshot('a', 'r')?.state).toBe('completada');
    expect(registry.refundSnapshot('b', 'r')).toBeNull();
  });
});
