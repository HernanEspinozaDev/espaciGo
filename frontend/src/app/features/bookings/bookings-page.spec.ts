import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { IdentitySession } from '../../core/identity/application/identity-session';
import { IdentityPort } from '../../core/identity/domain/ports/identity-port';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { BookingLifecyclePort } from './domain/ports/booking-lifecycle-port';
import { BookingPort } from './domain/ports/booking-port';
import { BookingsPage } from './bookings-page';

describe('BookingsPage recovery', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('retries the reservation read after an error and then renders the confirmed empty state', async () => {
    const login = {
      account_id: 'synthetic-renter',
      roles: ['arrendatario'],
      capabilities: [],
      access_token: 'synthetic-booking-token',
      token_type: 'Bearer',
      expires_at: '2027-01-01T00:00:00Z',
      restricted_mode: false,
    };
    const identityApi = {
      login: vi.fn().mockResolvedValue(login),
      session: vi.fn().mockResolvedValue({
        account_id: login.account_id,
        roles: login.roles,
        capabilities: [],
        restricted_mode: false,
      }),
      logout: vi.fn().mockResolvedValue(undefined),
    };
    const bookingApi = {
      list: vi
        .fn()
        .mockRejectedValueOnce(new Error('Lectura temporal no disponible'))
        .mockResolvedValueOnce({ safety_notice: 'ENSAYO LOCAL — SIN COBRO REAL', data: { items: [] } }),
    };
    TestBed.configureTestingModule({
      providers: [
        provideRouter([{ path: 'reservas', component: BookingsPage }]),
        { provide: IdentityPort, useValue: identityApi },
        { provide: BookingPort, useValue: bookingApi },
        { provide: BookingLifecyclePort, useValue: { getContract: vi.fn() } },
        PrivateSessionState,
      ],
    });

    const harness = await RouterTestingHarness.create('/reservas');
    const identity = await TestBed.inject(IdentitySession).signIn('renter@example.invalid', 'Synthetic#123');
    expect(identity.account_id).toBe(login.account_id);
    await vi.waitFor(() => expect(bookingApi.list).toHaveBeenCalledOnce());
    await vi.waitFor(() =>
      expect(harness.routeNativeElement?.querySelector('[role="alert"]')?.textContent).toContain(
        'Lectura temporal',
      ),
    );

    const retry = harness.routeNativeElement?.querySelector('.error button') as HTMLButtonElement;
    expect(retry.textContent).toContain('Volver a consultar reservas');
    retry.click();
    await vi.waitFor(() => expect(bookingApi.list).toHaveBeenCalledTimes(2));
    await vi.waitFor(() => expect(harness.routeNativeElement?.querySelector('.empty')).not.toBeNull());
    expect(harness.routeNativeElement?.querySelector('[role="alert"]')).toBeNull();
  });
});
