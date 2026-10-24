import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap, provideRouter } from '@angular/router';
import { BehaviorSubject } from 'rxjs';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { LocalBookingDetail } from '../../core/contracts/models';
import { IdentitySession } from '../../core/identity/application/identity-session';
import { IdentityApi } from '../../core/identity/infrastructure/identity-api';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { provideFrontendPorts } from '../../frontend.providers';
import { BookingAttempts } from './application/booking-attempts';
import { BookingDetailPage } from './booking-detail-page';
import { BookingApi, BookingApiError } from './infrastructure/booking-api';

const makeLogin = (account_id: string, access_token: string) => ({
  account_id,
  roles: ['arrendatario'],
  access_token,
  token_type: 'Bearer',
  expires_at: '2027-01-01T00:00:00Z',
  capabilities: [],
  restricted_mode: false,
});
const makeReservation = (
  id: string,
  renter = 'renter-a',
  state = 'pendiente_de_pago',
  extra: Record<string, unknown> = {},
) =>
  ({
    id,
    quote_id: 'quote-1',
    space_id: 'space-1',
    renter_id: renter,
    host_id: 'host-1',
    state,
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
    ...extra,
  }) as unknown as LocalBookingDetail;
const envelope = <T>(data: T) => ({ safety_notice: 'ENSAYO LOCAL — SIN COBRO REAL', data });
const pending = <T>() => {
  let resolve!: (value: T) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
};

function setup(
  api: Record<string, ReturnType<typeof vi.fn>>,
  identityOverrides: Record<string, unknown> = {},
) {
  const params = new BehaviorSubject(convertToParamMap({ id: 'r1' }));
  const identityApi = {
    login: vi.fn(async (email: string) =>
      email.startsWith('b@') ? makeLogin('renter-b', 'token-b') : makeLogin('renter-a', 'token-a'),
    ),
    session: vi.fn(async (token: string) => ({
      account_id: token === 'token-b' ? 'renter-b' : 'renter-a',
      roles: ['arrendatario'],
      capabilities: [],
      restricted_mode: false,
    })),
    logout: vi.fn(async () => undefined),
    ...identityOverrides,
  };
  TestBed.configureTestingModule({
    imports: [BookingDetailPage],
    providers: [
      ...provideFrontendPorts(),
      provideRouter([]),
      PrivateSessionState,
      { provide: ActivatedRoute, useValue: { paramMap: params } },
      { provide: IdentityApi, useValue: identityApi },
      { provide: BookingApi, useValue: api },
    ],
  });
  const fixture = TestBed.createComponent(BookingDetailPage);
  fixture.detectChanges();
  return {
    fixture,
    params,
    identity: TestBed.inject(IdentitySession),
    attempts: TestBed.inject(BookingAttempts),
  };
}
type PageAccess = {
  load(): Promise<void>;
  pay(): Promise<void>;
  cancel(): Promise<void>;
  refund(): Promise<void>;
  decide(value: 'aprobar' | 'rechazar'): Promise<void>;
  refreshAfterLifecycle(): void;
  paymentConfirmed(item: LocalBookingDetail): boolean;
  reservation(): LocalBookingDetail | null;
  uncertainNotice(): string;
  error(): string;
  paymentOutcome: { set(value: 'exito' | 'rechazo' | 'sin_respuesta'): void; (): string };
  refundOutcome: { set(value: 'exito' | 'fallo' | 'sin_respuesta'): void; (): string };
  reason: { set(value: string): void; (): string };
  decisionReason: { set(value: string): void; (): string };
  confirmCancel: { set(value: boolean): void; (): boolean };
  preview: { set(value: object): void };
};
const page = (fixture: ReturnType<typeof TestBed.createComponent>) =>
  fixture.componentInstance as unknown as PageAccess;
async function settle(fixture: ReturnType<typeof TestBed.createComponent>) {
  await Promise.resolve();
  await Promise.resolve();
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
}

describe('BookingDetailPage with real IdentitySession', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    TestBed.resetTestingModule();
  });

  it('keeps timeout notice through a failed and pending refresh, and clears only after Go confirms a terminal state', async () => {
    let confirmed = false;
    const api = {
      detail: vi.fn(async () =>
        envelope(makeReservation('r1', 'renter-a', confirmed ? 'pagada' : 'pendiente_de_pago')),
      ),
      payment: vi.fn().mockRejectedValue(new BookingApiError('Timeout', 504)),
      decision: vi.fn(),
      cancellationPreview: vi.fn(),
      cancel: vi.fn(),
      refund: vi.fn(),
    };
    const { fixture, identity, attempts } = setup(api);
    await identity.signIn('a@example.invalid', 'secret');
    await settle(fixture);
    const view = page(fixture);
    view.paymentOutcome.set('sin_respuesta');
    await view.pay();
    await settle(fixture);
    expect(view.uncertainNotice()).toContain('pendiente');
    expect(fixture.nativeElement.textContent).toContain('pendiente');
    const prior = attempts.find('renter-a', 'r1:payment');
    expect(prior?.uncertain).toBe(true);
    api.detail.mockRejectedValueOnce(new Error('lectura temporal caída'));
    await view.load();
    await settle(fixture);
    expect(view.uncertainNotice()).toContain('pendiente');
    api.detail.mockResolvedValue(envelope(makeReservation('r1', 'renter-a', 'pagada')));
    await view.load();
    await settle(fixture);
    expect(view.uncertainNotice()).toBe('');
    expect(attempts.find('renter-a', 'r1:payment')).toBeNull();
    fixture.destroy();
  });

  it('reuses same-account uncertain payload and key after logout/login while keeping controls reset', async () => {
    const api = {
      detail: vi.fn(async () => envelope(makeReservation('r1'))),
      payment: vi.fn().mockRejectedValue(new BookingApiError('Timeout', 504)),
      decision: vi.fn(),
      cancellationPreview: vi.fn(),
      cancel: vi.fn(),
      refund: vi.fn(),
    };
    const { fixture, identity, attempts } = setup(api);
    await identity.signIn('a@example.invalid', 'secret');
    await settle(fixture);
    const view = page(fixture);
    view.paymentOutcome.set('sin_respuesta');
    await view.pay();
    await settle(fixture);
    const first = attempts.find('renter-a', 'r1:payment');
    expect(first?.uncertain).toBe(true);
    await identity.signOut();
    await identity.signIn('a@example.invalid', 'secret');
    await settle(fixture);
    expect(view.uncertainNotice()).toContain('pago como pendiente');
    await view.pay();
    await settle(fixture);
    expect(api.payment).toHaveBeenNthCalledWith(2, 'token-a', 'r1', 'sin_respuesta', first?.key);
    expect(attempts.find('renter-a', 'r1:payment')?.key).toBe(first?.key);
    expect(view.paymentOutcome()).toBe('exito');
    fixture.destroy();
  });

  it('keeps Go-confirmed payment visible after host approval and later contract lifecycle transitions', async () => {
    const paid = makeReservation('r1', 'renter-a', 'pagada', {
      history: [
        {
          sequence: 2,
          from: 'pendiente_de_pago',
          to: 'pagada',
          reason: '',
          at: '2026-10-12T12:00:00Z',
        },
      ],
    });
    let refreshed = paid;
    const api = {
      detail: vi.fn(async () => envelope(refreshed)),
      payment: vi.fn(),
      decision: vi.fn(async () => {
        refreshed = makeReservation('r1', 'renter-a', 'aprobada_host', {
          history: [
            ...paid.history,
            {
              sequence: 3,
              from: 'pagada',
              to: 'aprobada_host',
              reason: '',
              at: '2026-10-12T12:01:00Z',
            },
          ],
        });
        return envelope(refreshed);
      }),
      cancellationPreview: vi.fn(),
      cancel: vi.fn(),
      refund: vi.fn(),
    };
    const { fixture, identity } = setup(api, {
      login: vi.fn(async () => makeLogin('host-1', 'token-host')),
      session: vi.fn(async () => ({
        account_id: 'host-1',
        roles: ['arrendador'],
        capabilities: [],
        restricted_mode: false,
      })),
    });
    await identity.signIn('host@example.invalid', 'secret');
    await settle(fixture);
    const view = page(fixture);
    expect(fixture.nativeElement.textContent).toContain('Confirmado por Go');

    await view.decide('aprobar');
    await settle(fixture);
    expect(view.reservation()?.state).toBe('aprobada_host');
    expect(fixture.nativeElement.textContent).toContain('Confirmado por Go');

    refreshed = makeReservation('r1', 'renter-a', 'firma_parcial', {
      history: [
        ...paid.history,
        {
          sequence: 3,
          from: 'pagada',
          to: 'aprobada_host',
          reason: '',
          at: '2026-10-12T12:01:00Z',
        },
        {
          sequence: 4,
          from: 'aprobada_host',
          to: 'firma_parcial',
          reason: '',
          at: '2026-10-12T12:02:00Z',
        },
      ],
    });
    view.refreshAfterLifecycle();
    await settle(fixture);
    expect(view.reservation()?.state).toBe('firma_parcial');
    expect(fixture.nativeElement.textContent).toContain('Confirmado por Go');

    refreshed = makeReservation('r1', 'renter-a', 'lista_para_checkin', {
      history: [
        ...paid.history,
        {
          sequence: 3,
          from: 'pagada',
          to: 'aprobada_host',
          reason: '',
          at: '2026-10-12T12:01:00Z',
        },
        {
          sequence: 4,
          from: 'aprobada_host',
          to: 'firma_parcial',
          reason: '',
          at: '2026-10-12T12:02:00Z',
        },
        {
          sequence: 5,
          from: 'firma_parcial',
          to: 'lista_para_checkin',
          reason: '',
          at: '2026-10-12T12:03:00Z',
        },
      ],
    });
    view.refreshAfterLifecycle();
    await settle(fixture);
    expect(view.reservation()?.state).toBe('lista_para_checkin');
    expect(fixture.nativeElement.textContent).toContain('Confirmado por Go');
    fixture.destroy();
  });

  it('clears private fields on a reservation change and ignores the old write completion', async () => {
    const oldWrite = pending<ReturnType<typeof envelope<unknown>>>();
    const api = {
      detail: vi.fn(async (_token: string, id: string) =>
        envelope(
          makeReservation(id, 'renter-a', id === 'r1' ? 'aprobada_host' : 'pendiente_de_pago'),
        ),
      ),
      payment: vi.fn(),
      decision: vi.fn(),
      cancellationPreview: vi.fn(async () => envelope({ eligible: true })),
      cancel: vi.fn(() => oldWrite.promise),
      refund: vi.fn(),
    };
    const { fixture, params, identity } = setup(api);
    await identity.signIn('a@example.invalid', 'secret');
    await settle(fixture);
    const view = page(fixture);
    view.reason.set('motivo privado A');
    view.decisionReason.set('rechazo privado A');
    view.confirmCancel.set(true);
    view.preview.set({ eligible: true });
    const operation = view.cancel();
    params.next(convertToParamMap({ id: 'r2' }));
    await settle(fixture);
    expect(view.reason()).toBe('');
    expect(view.decisionReason()).toBe('');
    expect(view.confirmCancel()).toBe(false);
    oldWrite.resolve(
      envelope({
        reservation: makeReservation('r1', 'renter-a', 'cancelada_arrendatario'),
        refund_state: 'no_aplica',
      }),
    );
    await operation;
    await settle(fixture);
    expect(view.reservation()?.id).toBe('r2');
    expect(view.reason()).toBe('');
    expect(view.confirmCancel()).toBe(false);
    fixture.destroy();
  });

  it('runs the Go-confirmed cancellation refund from the component and handles current versus stale 401', async () => {
    const canceled = makeReservation('r1', 'renter-a', 'cancelada_arrendatario', {
      refund_operation_id: 'refund-op',
      refund_amount_clp: 12500,
      refund_state: 'pendiente',
    });
    let state = makeReservation('r1', 'renter-a', 'aprobada_host');
    const api = {
      detail: vi.fn(async () => envelope(state)),
      payment: vi.fn(),
      decision: vi.fn(),
      cancellationPreview: vi.fn(),
      cancel: vi.fn(async () => {
        state = makeReservation('r1', 'renter-a', 'cancelada_arrendatario');
        return envelope({
          reservation: canceled,
          refund_amount_clp: 12500,
          refund_state: 'pendiente',
          notice_status: 'no_disponible_no_durable',
        });
      }),
      refund: vi.fn(async (_token: string, id: string, _outcome: string, operationId: string) =>
        envelope({
          reservation_id: id,
          operation_id: operationId,
          amount_clp: '12500',
          currency: 'CLP',
          state: 'completada',
          last_result: 'exito_simulado',
          updated_at: '2026-10-12T01:00:00Z',
          notice_status: 'no_disponible_no_durable',
        }),
      ),
    };
    const { fixture, identity, attempts } = setup(api);
    await identity.signIn('a@example.invalid', 'secret');
    await settle(fixture);
    const view = page(fixture);
    view.preview.set({ eligible: true });
    view.confirmCancel.set(true);
    view.reason.set('motivo local');
    await view.cancel();
    await settle(fixture);
    expect(view.reservation()?.refund_state).toBe('pendiente');
    expect(attempts.refundSnapshot('renter-a', 'r1')?.state).toBe('pendiente');
    expect(fixture.nativeElement.textContent).toContain('Devolución simulada pendiente');
    await identity.signOut();
    await identity.signIn('a@example.invalid', 'secret');
    await settle(fixture);
    expect(view.reservation()?.refund_state).toBe('pendiente');
    expect(fixture.nativeElement.textContent).toContain('Devolución simulada pendiente');
    await view.refund();
    await settle(fixture);
    expect(api.refund).toHaveBeenCalledWith('token-a', 'r1', 'exito', 'refund-op');
    expect(view.reservation()?.refund_state).toBe('completada');
    expect(attempts.refundSnapshot('renter-a', 'r1')?.state).toBe('completada');
    expect(fixture.nativeElement.textContent).toContain('Go confirmó la devolución fake');
    fixture.destroy();
  });

  it('keeps cancel and refund uncertainty across successful refreshes until Go confirms each terminal result', async () => {
    let reservation = makeReservation('r1', 'renter-a', 'aprobada_host');
    const api = {
      detail: vi.fn(async () => envelope(reservation)),
      payment: vi.fn(),
      decision: vi.fn(),
      cancellationPreview: vi.fn(async () => envelope({ eligible: true })),
      cancel: vi.fn().mockRejectedValue(new BookingApiError('cancel timeout', 504)),
      refund: vi.fn().mockRejectedValue(new BookingApiError('refund timeout', 504)),
    };
    const { fixture, identity, attempts } = setup(api);
    await identity.signIn('a@example.invalid', 'secret');
    await settle(fixture);
    const view = page(fixture);
    view.preview.set({ eligible: true });
    view.confirmCancel.set(true);
    await view.cancel();
    await settle(fixture);
    expect(view.uncertainNotice()).toContain('cancelación con el mismo intento');
    expect(attempts.find('renter-a', 'r1:cancel')?.uncertain).toBe(true);
    reservation = makeReservation('r1', 'renter-a', 'cancelada_arrendatario', {
      refund_operation_id: 'refund-r1',
      refund_amount_clp: 12500,
      refund_state: 'pendiente',
    });
    await view.load();
    await settle(fixture);
    expect(view.uncertainNotice()).toBe('');
    expect(attempts.find('renter-a', 'r1:cancel')).toBeNull();
    await view.refund();
    await settle(fixture);
    expect(view.uncertainNotice()).toContain('devolución pendiente');
    expect(attempts.find('renter-a', 'r1:refund')?.uncertain).toBe(true);
    reservation = makeReservation('r1', 'renter-a', 'cancelada_arrendatario', {
      refund_operation_id: 'refund-r1',
      refund_amount_clp: 12500,
      refund_state: 'completada',
    });
    await view.load();
    await settle(fixture);
    expect(view.uncertainNotice()).toBe('');
    expect(attempts.find('renter-a', 'r1:refund')).toBeNull();
    fixture.destroy();
  });

  it('expires a current 401 but a late 401 from the previous account cannot expire the new session', async () => {
    const oldPayment = pending<ReturnType<typeof envelope<unknown>>>();
    const api = {
      detail: vi.fn(async (token: string) =>
        envelope(makeReservation('r1', token === 'token-b' ? 'renter-b' : 'renter-a')),
      ),
      payment: vi.fn((token: string) =>
        token === 'token-a'
          ? oldPayment.promise
          : Promise.reject(new BookingApiError('unauthorized', 401)),
      ),
      decision: vi.fn(),
      cancellationPreview: vi.fn(),
      cancel: vi.fn(),
      refund: vi.fn(),
    };
    const { fixture, identity } = setup(api);
    await identity.signIn('a@example.invalid', 'secret');
    await settle(fixture);
    const view = page(fixture);
    view.reason.set('private reason A');
    view.decisionReason.set('private decision A');
    view.confirmCancel.set(true);
    const stale = view.pay();
    await identity.signOut();
    await identity.signIn('b@example.invalid', 'secret');
    await settle(fixture);
    expect(view.reason()).toBe('');
    expect(view.decisionReason()).toBe('');
    expect(view.confirmCancel()).toBe(false);
    oldPayment.reject(new BookingApiError('old bearer', 401));
    await stale;
    await settle(fixture);
    expect(identity.accessToken()).toBe('token-b');
    expect(identity.session()?.account_id).toBe('renter-b');
    await view.pay();
    await settle(fixture);
    expect(identity.accessToken()).toBeNull();
    expect(identity.session()).toBeNull();
    fixture.destroy();
  });

  it('does not let a definitive 4xx from a prior login clear the new same-account operation notice', async () => {
    const oldPayment = pending<ReturnType<typeof envelope<unknown>>>();
    const api = {
      detail: vi.fn(async () => envelope(makeReservation('r1'))),
      payment: vi
        .fn()
        .mockImplementationOnce(() => oldPayment.promise)
        .mockRejectedValueOnce(new BookingApiError('timeout', 504)),
      decision: vi.fn(),
      cancellationPreview: vi.fn(),
      cancel: vi.fn(),
      refund: vi.fn(),
    };
    let loginCount = 0;
    const { fixture, identity } = setup(api, {
      login: vi.fn(async () => makeLogin('renter-a', `token-a${++loginCount}`)),
    });
    await identity.signIn('a@example.invalid', 'secret');
    await settle(fixture);
    const view = page(fixture);
    const oldRequest = view.pay();
    await identity.signOut();
    await identity.signIn('a@example.invalid', 'secret');
    await settle(fixture);
    await view.pay();
    await settle(fixture);
    expect(api.payment.mock.calls.map(([token]) => token)).toEqual(['token-a1', 'token-a2']);
    const latestNotice = view.uncertainNotice();
    expect(latestNotice).toContain('pago como pendiente');
    oldPayment.reject(new BookingApiError('idempotency conflict', 409));
    await oldRequest;
    await settle(fixture);
    expect(view.uncertainNotice()).toBe(latestNotice);
    expect(identity.accessToken()).toBe('token-a2');
    fixture.destroy();
  });

  it('does not apply an older failed-refund confirmation after a newer success and delayed pending reload', async () => {
    const oldReload = pending<ReturnType<typeof envelope<LocalBookingDetail>>>();
    const completed = makeReservation('r1', 'renter-a', 'cancelada_arrendatario', {
      refund_operation_id: 'refund-op',
      refund_amount_clp: '12500',
      refund_state: 'completada',
    });
    let detailCall = 0;
    const api = {
      detail: vi.fn(() => {
        detailCall++;
        if (detailCall === 1)
          return Promise.resolve(
            envelope(
              makeReservation('r1', 'renter-a', 'cancelada_arrendatario', {
                refund_operation_id: 'refund-op',
                refund_amount_clp: '12500',
                refund_state: 'pendiente',
              }),
            ),
          );
        if (detailCall === 2) return oldReload.promise;
        return Promise.resolve(envelope(completed));
      }),
      payment: vi.fn(),
      decision: vi.fn(),
      cancellationPreview: vi.fn(),
      cancel: vi.fn(),
      refund: vi
        .fn()
        .mockResolvedValueOnce(
          envelope({
            reservation_id: 'r1',
            operation_id: 'refund-op',
            amount_clp: '12500',
            currency: 'CLP',
            state: 'pendiente',
            last_result: 'fallo_simulado',
            updated_at: '2026-10-12T01:00:00Z',
            notice_status: 'no_disponible_no_durable',
          }),
        )
        .mockResolvedValueOnce(
          envelope({
            reservation_id: 'r1',
            operation_id: 'refund-op',
            amount_clp: '12500',
            currency: 'CLP',
            state: 'completada',
            last_result: 'exito_simulado',
            updated_at: '2026-10-12T02:00:00Z',
            notice_status: 'no_disponible_no_durable',
          }),
        ),
    };
    const { fixture, identity, attempts } = setup(api);
    await identity.signIn('a@example.invalid', 'secret');
    await settle(fixture);
    const view = page(fixture);
    view.refundOutcome.set('fallo');
    const failedRefund = view.refund();
    await Promise.resolve();
    await Promise.resolve();
    expect(detailCall).toBe(2);
    view.refundOutcome.set('exito');
    await view.refund();
    await settle(fixture);
    expect(view.reservation()?.refund_state).toBe('completada');
    expect(attempts.refundSnapshot('renter-a', 'r1')?.state).toBe('completada');
    oldReload.resolve(
      envelope(
        makeReservation('r1', 'renter-a', 'cancelada_arrendatario', {
          refund_operation_id: 'refund-op',
          refund_amount_clp: '12500',
          refund_state: 'pendiente',
        }),
      ),
    );
    await failedRefund;
    await settle(fixture);
    expect(view.reservation()?.refund_state).toBe('completada');
    expect(attempts.refundSnapshot('renter-a', 'r1')?.state).toBe('completada');
    fixture.destroy();
  });

  it('keeps a newer refund view and snapshot when a cancellation response arrives out of order', async () => {
    const oldCancel = pending<ReturnType<typeof envelope<unknown>>>();
    const pendingRefund = makeReservation('r1', 'renter-a', 'cancelada_arrendatario', {
      refund_operation_id: 'refund-op',
      refund_amount_clp: '12500',
      refund_state: 'pendiente',
    });
    const completed = makeReservation('r1', 'renter-a', 'cancelada_arrendatario', {
      refund_operation_id: 'refund-op',
      refund_amount_clp: '12500',
      refund_state: 'completada',
    });
    let detailState = makeReservation('r1', 'renter-a', 'aprobada_host');
    const api = {
      detail: vi.fn(async () => envelope(detailState)),
      payment: vi.fn(),
      decision: vi.fn(),
      cancellationPreview: vi.fn(async () => envelope({ eligible: true })),
      cancel: vi.fn(() => oldCancel.promise),
      refund: vi.fn(async (_token: string, id: string, _outcome: string, operationId: string) =>
        envelope({
          reservation_id: id,
          operation_id: operationId,
          amount_clp: '12500',
          currency: 'CLP',
          state: 'completada',
          last_result: 'exito_simulado',
          updated_at: '2026-10-12T03:00:00Z',
          notice_status: 'no_disponible_no_durable',
        }),
      ),
    };
    const { fixture, identity, attempts } = setup(api);
    await identity.signIn('a@example.invalid', 'secret');
    await settle(fixture);
    const view = page(fixture);
    view.preview.set({ eligible: true });
    view.confirmCancel.set(true);
    const cancelRequest = view.cancel();
    detailState = pendingRefund;
    await view.load();
    await settle(fixture);
    expect(view.reservation()?.refund_state).toBe('pendiente');
    detailState = completed;
    await view.refund();
    await settle(fixture);
    expect(view.reservation()?.refund_state).toBe('completada');
    oldCancel.resolve(
      envelope({
        reservation: makeReservation('r1', 'renter-a', 'cancelada_arrendatario'),
        refund_amount_clp: '12500',
        refund_state: 'pendiente',
        notice_status: 'no_disponible_no_durable',
      }),
    );
    await cancelRequest;
    await settle(fixture);
    expect(view.reservation()?.refund_state).toBe('completada');
    expect(attempts.refundSnapshot('renter-a', 'r1')?.state).toBe('completada');
    fixture.destroy();
  });

  it('applies a pending refund POST before reload and keeps the later completed Go detail in the view and snapshot', async () => {
    let detailState = makeReservation('r1', 'renter-a', 'cancelada_arrendatario', {
      refund_operation_id: 'refund-op',
      refund_amount_clp: '12500',
      refund_state: 'pendiente',
    });
    const api = {
      detail: vi.fn(async () => envelope(detailState)),
      payment: vi.fn(),
      decision: vi.fn(),
      cancellationPreview: vi.fn(),
      cancel: vi.fn(),
      refund: vi.fn(async (_token: string, id: string, _outcome: string, operationId: string) => {
        detailState = makeReservation('r1', 'renter-a', 'cancelada_arrendatario', {
          refund_operation_id: operationId,
          refund_amount_clp: '12500',
          refund_state: 'completada',
        });
        return envelope({
          reservation_id: id,
          operation_id: operationId,
          amount_clp: '12500',
          currency: 'CLP',
          state: 'pendiente',
          last_result: 'fallo_simulado',
          updated_at: '2026-10-12T04:00:00Z',
          notice_status: 'no_disponible_no_durable',
        });
      }),
    };
    const { fixture, identity, attempts } = setup(api);
    await identity.signIn('a@example.invalid', 'secret');
    await settle(fixture);
    const view = page(fixture);
    view.refundOutcome.set('fallo');
    await view.refund();
    await settle(fixture);
    expect(view.reservation()?.refund_state).toBe('completada');
    expect(attempts.refundSnapshot('renter-a', 'r1')?.state).toBe('completada');
    fixture.destroy();
  });

  it('uses a completed Go detail over an older pending cancellation POST and preserves it in the snapshot', async () => {
    let detailCall = 0;
    const api = {
      detail: vi.fn(async () => {
        detailCall++;
        return envelope(
          makeReservation(
            'r1',
            'renter-a',
            detailCall === 1 ? 'aprobada_host' : 'cancelada_arrendatario',
            {
              refund_operation_id: 'refund-op',
              refund_amount_clp: '12500',
              refund_state: 'completada',
            },
          ),
        );
      }),
      payment: vi.fn(),
      decision: vi.fn(),
      cancellationPreview: vi.fn(async () => envelope({ eligible: true })),
      cancel: vi.fn(async () =>
        envelope({
          reservation: makeReservation('r1', 'renter-a', 'cancelada_arrendatario', {
            refund_operation_id: 'refund-op',
            refund_amount_clp: '12500',
          }),
          refund_amount_clp: '12500',
          refund_state: 'pendiente',
          notice_status: 'no_disponible_no_durable',
        }),
      ),
      refund: vi.fn(),
    };
    const { fixture, identity, attempts } = setup(api);
    await identity.signIn('a@example.invalid', 'secret');
    await settle(fixture);
    const view = page(fixture);
    view.preview.set({ eligible: true });
    view.confirmCancel.set(true);
    await view.cancel();
    await settle(fixture);
    expect(view.reservation()?.refund_state).toBe('completada');
    expect(attempts.refundSnapshot('renter-a', 'r1')?.state).toBe('completada');
    fixture.destroy();
  });

  it('keeps a confirmed refund snapshot when cancellation reload fails or omits refund fields', async () => {
    let detailCall = 0;
    const api = {
      detail: vi.fn(async () => {
        detailCall++;
        if (detailCall === 1) return envelope(makeReservation('r1', 'renter-a', 'aprobada_host'));
        if (detailCall === 2) throw new Error('GET temporalmente no disponible');
        return envelope(makeReservation('r1', 'renter-a', 'cancelada_arrendatario'));
      }),
      payment: vi.fn(),
      decision: vi.fn(),
      cancellationPreview: vi.fn(async () => envelope({ eligible: true })),
      cancel: vi.fn(async () =>
        envelope({
          reservation: makeReservation('r1', 'renter-a', 'cancelada_arrendatario', {
            refund_operation_id: 'refund-op',
            refund_amount_clp: '12500',
          }),
          refund_amount_clp: '12500',
          refund_state: 'pendiente',
          notice_status: 'no_disponible_no_durable',
        }),
      ),
      refund: vi.fn(),
    };
    const { fixture, identity, attempts } = setup(api);
    await identity.signIn('a@example.invalid', 'secret');
    await settle(fixture);
    const view = page(fixture);
    view.preview.set({ eligible: true });
    view.confirmCancel.set(true);
    await view.cancel();
    await settle(fixture);
    expect(view.reservation()?.refund_state).toBe('pendiente');
    expect(attempts.refundSnapshot('renter-a', 'r1')?.state).toBe('pendiente');
    await view.load();
    await settle(fixture);
    expect(view.reservation()?.refund_operation_id).toBe('refund-op');
    expect(view.reservation()?.refund_state).toBe('pendiente');
    expect(attempts.refundSnapshot('renter-a', 'r1')?.state).toBe('pendiente');
    fixture.destroy();
  });
});
