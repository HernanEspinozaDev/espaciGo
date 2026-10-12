import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap, provideRouter } from '@angular/router';
import { BehaviorSubject } from 'rxjs';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { LocalBookingDetail } from '../../../generated/api/types.gen';
import { IdentityApi } from '../../core/identity/identity-api';
import { IdentitySession } from '../../core/identity/identity-session';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { BookingApi, BookingApiError } from './booking-api';
import { BookingAttempts } from './booking-attempts';
import { BookingDetailPage } from './booking-detail-page';

const makeLogin = (account_id: string, access_token: string) => ({
  account_id, roles: ['arrendatario'], access_token, token_type: 'Bearer',
  expires_at: '2027-01-01T00:00:00Z', restricted_mode: false,
});
const makeReservation = (id: string, renter = 'renter-a', state = 'pendiente_de_pago', extra: Record<string, unknown> = {}) => ({
  id, quote_id: 'quote-1', space_id: 'space-1', renter_id: renter, host_id: 'host-1', state,
  rate_unit: 'hora', unit_price_clp: 12500, units: 1, subtotal_clp: 12500,
  start_at: '2026-10-13T12:00:00Z', end_at: '2026-10-13T13:00:00Z', time_zone: 'America/Santiago',
  conditions: '', cancellation_policy_version: 'local_flexible_v1', pay_expires_at: '2026-10-13T11:00:00Z',
  created_at: '2026-10-12T00:00:00Z', updated_at: '2026-10-12T00:00:00Z', unread_count: 0,
  history: [], ...extra,
}) as unknown as LocalBookingDetail;
const envelope = <T>(data: T) => ({ safety_notice: 'ENSAYO LOCAL — SIN COBRO REAL', data });
const pending = <T>() => {
  let resolve!: (value: T) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej; });
  return { promise, resolve, reject };
};

function setup(api: Record<string, ReturnType<typeof vi.fn>>) {
  const params = new BehaviorSubject(convertToParamMap({ id: 'r1' }));
  const identityApi = {
    login: vi.fn(async (email: string) => email.startsWith('b@') ? makeLogin('renter-b', 'token-b') : makeLogin('renter-a', 'token-a')),
    session: vi.fn(async (token: string) => ({ account_id: token === 'token-b' ? 'renter-b' : 'renter-a', roles: ['arrendatario'], restricted_mode: false })),
    logout: vi.fn(async () => undefined),
  };
  TestBed.configureTestingModule({
    imports: [BookingDetailPage],
    providers: [provideRouter([]), PrivateSessionState, { provide: ActivatedRoute, useValue: { paramMap: params } },
      { provide: IdentityApi, useValue: identityApi }, { provide: BookingApi, useValue: api }],
  });
  const fixture = TestBed.createComponent(BookingDetailPage);
  fixture.detectChanges();
  return { fixture, params, identity: TestBed.inject(IdentitySession), attempts: TestBed.inject(BookingAttempts) };
}
type PageAccess = {
  load(): Promise<void>; pay(): Promise<void>; cancel(): Promise<void>; refund(): Promise<void>;
  reservation(): LocalBookingDetail | null; uncertainNotice(): string; error(): string;
  paymentOutcome: { set(value: 'exito' | 'rechazo' | 'sin_respuesta'): void; (): string };
  refundOutcome: { set(value: 'exito' | 'fallo' | 'sin_respuesta'): void; (): string };
  reason: { set(value: string): void; (): string };
  decisionReason: { set(value: string): void; (): string };
  confirmCancel: { set(value: boolean): void; (): boolean };
  preview: { set(value: object): void };
};
const page = (fixture: ReturnType<typeof TestBed.createComponent>) => fixture.componentInstance as unknown as PageAccess;
async function settle(fixture: ReturnType<typeof TestBed.createComponent>) {
  await Promise.resolve(); await Promise.resolve(); fixture.detectChanges(); await fixture.whenStable(); fixture.detectChanges();
}

describe('BookingDetailPage with real IdentitySession', () => {
  afterEach(() => { vi.restoreAllMocks(); TestBed.resetTestingModule(); });

  it('keeps timeout notice through a failed and pending refresh, and clears only after Go confirms a terminal state', async () => {
    let confirmed = false;
    const api = {
      detail: vi.fn(async () => envelope(makeReservation('r1', 'renter-a', confirmed ? 'pagada' : 'pendiente_de_pago'))),
      payment: vi.fn().mockRejectedValue(new BookingApiError('Timeout', 504)),
      decision: vi.fn(), cancellationPreview: vi.fn(), cancel: vi.fn(), refund: vi.fn(),
    };
    const { fixture, identity, attempts } = setup(api);
    await identity.signIn('a@example.invalid', 'secret'); await settle(fixture);
    const view = page(fixture); view.paymentOutcome.set('sin_respuesta'); await view.pay(); await settle(fixture);
    expect(view.uncertainNotice()).toContain('pendiente');
    expect(fixture.nativeElement.textContent).toContain('pendiente');
    const prior = attempts.find('renter-a', 'r1:payment'); expect(prior?.uncertain).toBe(true);
    api.detail.mockRejectedValueOnce(new Error('lectura temporal caída'));
    await view.load(); await settle(fixture);
    expect(view.uncertainNotice()).toContain('pendiente');
    api.detail.mockResolvedValue(envelope(makeReservation('r1', 'renter-a', 'pagada')));
    await view.load(); await settle(fixture);
    expect(view.uncertainNotice()).toBe('');
    expect(attempts.find('renter-a', 'r1:payment')).toBeNull();
    fixture.destroy();
  });

  it('reuses same-account uncertain payload and key after logout/login while keeping controls reset', async () => {
    const api = {
      detail: vi.fn(async () => envelope(makeReservation('r1'))),
      payment: vi.fn().mockRejectedValue(new BookingApiError('Timeout', 504)),
      decision: vi.fn(), cancellationPreview: vi.fn(), cancel: vi.fn(), refund: vi.fn(),
    };
    const { fixture, identity, attempts } = setup(api);
    await identity.signIn('a@example.invalid', 'secret'); await settle(fixture);
    const view = page(fixture); view.paymentOutcome.set('sin_respuesta'); await view.pay(); await settle(fixture);
    const first = attempts.find('renter-a', 'r1:payment'); expect(first?.uncertain).toBe(true);
    await identity.signOut(); await identity.signIn('a@example.invalid', 'secret'); await settle(fixture);
    expect(view.uncertainNotice()).toContain('pago como pendiente');
    await view.pay(); await settle(fixture);
    expect(api.payment).toHaveBeenNthCalledWith(2, 'token-a', 'r1', 'sin_respuesta', first?.key);
    expect(attempts.find('renter-a', 'r1:payment')?.key).toBe(first?.key);
    expect(view.paymentOutcome()).toBe('exito');
    fixture.destroy();
  });

  it('clears private fields on a reservation change and ignores the old write completion', async () => {
    const oldWrite = pending<ReturnType<typeof envelope<unknown>>>();
    const api = {
      detail: vi.fn(async (_token: string, id: string) => envelope(makeReservation(id, 'renter-a', id === 'r1' ? 'aprobada_host' : 'pendiente_de_pago'))),
      payment: vi.fn(), decision: vi.fn(), cancellationPreview: vi.fn(async () => envelope({ eligible: true })),
      cancel: vi.fn(() => oldWrite.promise), refund: vi.fn(),
    };
    const { fixture, params, identity } = setup(api);
    await identity.signIn('a@example.invalid', 'secret'); await settle(fixture);
    const view = page(fixture); view.reason.set('motivo privado A'); view.decisionReason.set('rechazo privado A'); view.confirmCancel.set(true);
    view.preview.set({ eligible: true });
    const operation = view.cancel();
    params.next(convertToParamMap({ id: 'r2' })); await settle(fixture);
    expect(view.reason()).toBe(''); expect(view.decisionReason()).toBe(''); expect(view.confirmCancel()).toBe(false);
    oldWrite.resolve(envelope({ reservation: makeReservation('r1', 'renter-a', 'cancelada_arrendatario'), refund_state: 'no_aplica' }));
    await operation; await settle(fixture);
    expect(view.reservation()?.id).toBe('r2');
    expect(view.reason()).toBe(''); expect(view.confirmCancel()).toBe(false);
    fixture.destroy();
  });

  it('runs the Go-confirmed cancellation refund from the component and handles current versus stale 401', async () => {
    const canceled = makeReservation('r1', 'renter-a', 'cancelada_arrendatario', { refund_operation_id: 'refund-op', refund_amount_clp: 12500, refund_state: 'pendiente' });
    let state = makeReservation('r1', 'renter-a', 'aprobada_host');
    const api = {
      detail: vi.fn(async () => envelope(state)),
      payment: vi.fn(), decision: vi.fn(), cancellationPreview: vi.fn(),
      cancel: vi.fn(async () => {
        state = makeReservation('r1', 'renter-a', 'cancelada_arrendatario');
        return envelope({ reservation: canceled, refund_amount_clp: 12500, refund_state: 'pendiente', notice_status: 'no_disponible_no_durable' });
      }),
      refund: vi.fn(async (_token: string, id: string, _outcome: string, operationId: string) => envelope({ reservation_id: id, operation_id: operationId, amount_clp: '12500', currency: 'CLP', state: 'completada', last_result: 'exito_simulado', updated_at: '2026-10-12T01:00:00Z', notice_status: 'no_disponible_no_durable' })),
    };
    const { fixture, identity } = setup(api);
    await identity.signIn('a@example.invalid', 'secret'); await settle(fixture);
    const view = page(fixture); view.preview.set({ eligible: true }); view.confirmCancel.set(true); view.reason.set('motivo local');
    await view.cancel(); await settle(fixture);
    expect(view.reservation()?.refund_state).toBe('pendiente');
    expect(fixture.nativeElement.textContent).toContain('Devolución simulada pendiente');
    await identity.signOut(); await identity.signIn('a@example.invalid', 'secret'); await settle(fixture);
    expect(view.reservation()?.refund_state).toBe('pendiente');
    expect(fixture.nativeElement.textContent).toContain('Devolución simulada pendiente');
    await view.refund(); await settle(fixture);
    expect(api.refund).toHaveBeenCalledWith('token-a', 'r1', 'exito', 'refund-op');
    expect(view.reservation()?.refund_state).toBe('completada');
    expect(fixture.nativeElement.textContent).toContain('Go confirmó la devolución fake');
    fixture.destroy();
  });

  it('keeps cancel and refund uncertainty across successful refreshes until Go confirms each terminal result', async () => {
    let reservation = makeReservation('r1', 'renter-a', 'aprobada_host');
    const api = {
      detail: vi.fn(async () => envelope(reservation)),
      payment: vi.fn(), decision: vi.fn(), cancellationPreview: vi.fn(async () => envelope({ eligible: true })),
      cancel: vi.fn().mockRejectedValue(new BookingApiError('cancel timeout', 504)),
      refund: vi.fn().mockRejectedValue(new BookingApiError('refund timeout', 504)),
    };
    const { fixture, identity, attempts } = setup(api);
    await identity.signIn('a@example.invalid', 'secret'); await settle(fixture);
    const view = page(fixture); view.preview.set({ eligible: true }); view.confirmCancel.set(true); await view.cancel(); await settle(fixture);
    expect(view.uncertainNotice()).toContain('cancelación con el mismo intento');
    expect(attempts.find('renter-a', 'r1:cancel')?.uncertain).toBe(true);
    reservation = makeReservation('r1', 'renter-a', 'cancelada_arrendatario', { refund_operation_id: 'refund-r1', refund_amount_clp: 12500, refund_state: 'pendiente' });
    await view.load(); await settle(fixture);
    expect(view.uncertainNotice()).toBe('');
    expect(attempts.find('renter-a', 'r1:cancel')).toBeNull();
    await view.refund(); await settle(fixture);
    expect(view.uncertainNotice()).toContain('devolución pendiente');
    expect(attempts.find('renter-a', 'r1:refund')?.uncertain).toBe(true);
    reservation = makeReservation('r1', 'renter-a', 'cancelada_arrendatario', { refund_operation_id: 'refund-r1', refund_amount_clp: 12500, refund_state: 'completada' });
    await view.load(); await settle(fixture);
    expect(view.uncertainNotice()).toBe('');
    expect(attempts.find('renter-a', 'r1:refund')).toBeNull();
    fixture.destroy();
  });

  it('expires a current 401 but a late 401 from the previous account cannot expire the new session', async () => {
    const oldPayment = pending<ReturnType<typeof envelope<unknown>>>();
    const api = {
      detail: vi.fn(async (token: string) => envelope(makeReservation('r1', token === 'token-b' ? 'renter-b' : 'renter-a'))),
      payment: vi.fn((token: string) => token === 'token-a' ? oldPayment.promise : Promise.reject(new BookingApiError('unauthorized', 401))),
      decision: vi.fn(), cancellationPreview: vi.fn(), cancel: vi.fn(), refund: vi.fn(),
    };
    const { fixture, identity } = setup(api);
    await identity.signIn('a@example.invalid', 'secret'); await settle(fixture);
    const view = page(fixture); view.reason.set('private reason A'); view.decisionReason.set('private decision A'); view.confirmCancel.set(true);
    const stale = view.pay();
    await identity.signOut(); await identity.signIn('b@example.invalid', 'secret'); await settle(fixture);
    expect(view.reason()).toBe(''); expect(view.decisionReason()).toBe(''); expect(view.confirmCancel()).toBe(false);
    oldPayment.reject(new BookingApiError('old bearer', 401)); await stale; await settle(fixture);
    expect(identity.accessToken()).toBe('token-b'); expect(identity.session()?.account_id).toBe('renter-b');
    await view.pay(); await settle(fixture);
    expect(identity.accessToken()).toBeNull(); expect(identity.session()).toBeNull();
    fixture.destroy();
  });
});
