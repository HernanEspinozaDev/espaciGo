import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap, provideRouter } from '@angular/router';
import { BehaviorSubject } from 'rxjs';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { LocalBookingDetail, SessionResponse } from '../../../core/contracts/models';
import { IdentitySession } from '../../../core/identity/application/identity-session';
import { BookingApiError, BookingPort } from '../domain/ports/booking-port';
import { BookingAttempts } from './booking-attempts';
import { BookingDetailFacade } from './booking-detail-facade';

// A pure port implementation: no SDK, HttpClient, adapter provider or Angular component.
const reservation: LocalBookingDetail = {
  id: 'r1',
  quote_id: 'quote-a',
  space_id: 'space-a',
  renter_id: 'renter-a',
  host_id: 'host-a',
  state: 'pendiente_de_pago',
  currency: 'CLP',
  rate_unit: 'hora',
  unit_price_clp: 12500,
  units: 1,
  subtotal_clp: 12500,
  start_at: '2026-10-13T12:00:00Z',
  end_at: '2026-10-13T13:00:00Z',
  time_zone: 'America/Santiago',
  conditions: '',
  cancellation_policy_version: 'local_flexible_v1',
  pay_expires_at: '2026-10-13T11:00:00Z',
  created_at: '2026-10-12T00:00:00Z',
  updated_at: '2026-10-12T00:00:00Z',
  unread_count: 0,
  history: [],
};
const envelope = <T>(data: T) => ({ safety_notice: 'ENSAYO LOCAL — SIN COBRO REAL', data });
describe('booking application through its port', () => {
  afterEach(() => TestBed.resetTestingModule());
  it('preserves a payment key and payload through a transport-independent uncertain retry', async () => {
    const port = {
      detail: vi.fn().mockResolvedValue(envelope(reservation)),
      payment: vi
        .fn()
        .mockRejectedValueOnce(new BookingApiError('uncertain', 504))
        .mockResolvedValueOnce(envelope({ ...reservation, state: 'pagada' })),
    } satisfies Pick<BookingPort, 'detail' | 'payment'>;
    const identity = {
      session: signal<SessionResponse>({
        account_id: 'renter-a',
        roles: ['arrendatario'],
        capabilities: [],
        restricted_mode: false,
      }),
      accessToken: signal('in-memory-token'),
      expireIfCurrent: vi.fn(),
    };
    TestBed.configureTestingModule({
      providers: [
        BookingDetailFacade,
        provideRouter([]),
        { provide: BookingPort, useValue: port },
        { provide: IdentitySession, useValue: identity },
        {
          provide: ActivatedRoute,
          useValue: { paramMap: new BehaviorSubject(convertToParamMap({ id: 'r1' })) },
        },
      ],
    });
    const facade = TestBed.inject(BookingDetailFacade);
    await vi.waitFor(() => expect(facade.reservation()?.id).toBe('r1'));
    await facade.pay();
    expect(
      JSON.stringify(TestBed.inject(BookingAttempts).find('renter-a', 'r1:payment')),
    ).not.toContain('in-memory-token');
    facade.paymentOutcome.set('rechazo');
    await facade.pay();
    expect(port.payment).toHaveBeenCalledTimes(2);
    expect(port.payment.mock.calls[1]).toEqual(port.payment.mock.calls[0]);
    expect(
      JSON.stringify(TestBed.inject(BookingAttempts).find('renter-a', 'r1:payment')),
    ).not.toContain('in-memory-token');
  });
});
