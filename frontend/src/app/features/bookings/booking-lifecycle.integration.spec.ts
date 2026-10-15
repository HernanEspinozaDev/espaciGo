import {
  createComponent,
  createEnvironmentInjector,
  EnvironmentInjector,
  runInInjectionContext,
} from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { ActivatedRoute, convertToParamMap, provideRouter } from '@angular/router';
import { BehaviorSubject } from 'rxjs';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type {
  LocalBookingDetail,
  LocalRentalOperation,
  LocalSyntheticContract,
} from '../../core/contracts/models';
import { IdentitySession } from '../../core/identity/application/identity-session';
import { IdentityApi } from '../../core/identity/infrastructure/identity-api';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { provideFrontendPorts } from '../../frontend.providers';
import { BookingLifecycleAttempts } from './application/booking-lifecycle-attempts';
import { BookingDetailPage } from './booking-detail-page';
import { BookingLifecyclePanel } from './booking-lifecycle-panel';
import { BookingApi, BookingApiError } from './infrastructure/booking-api';
import { BookingLifecycleApi } from './infrastructure/booking-lifecycle-api';

const participant = (account_id: string, access_token: string) => ({
  account_id,
  roles: ['arrendatario'],
  access_token,
  token_type: 'Bearer',
  expires_at: '2027-01-01T00:00:00Z',
  capabilities: [],
  restricted_mode: false,
});
const detail = (state: string) =>
  ({
    id: 'reservation-1',
    quote_id: 'quote-1',
    space_id: 'space-1',
    renter_id: 'renter-a',
    host_id: 'host-a',
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
  }) as unknown as LocalBookingDetail;
const contractFor = (state: string, signerStates: Record<string, string>) =>
  ({
    id: 'contract-1',
    reservation_id: 'reservation-1',
    version: 1,
    state,
    snapshot: '{"terms":"synthetic"}',
    sha256: 'a'.repeat(64),
    signatures: Object.entries(signerStates).map(([signer_id, signature]) => ({
      signer_id,
      role: signer_id === 'host-a' ? 'anfitrion' : 'arrendatario',
      state: signature,
      updated_at: '2026-10-12T00:00:00Z',
    })),
    history: [],
  }) as unknown as LocalSyntheticContract;
const operation = (kind: string, sequence: number) =>
  ({
    id: `operation-${sequence}`,
    reservation_id: 'reservation-1',
    kind,
    actor_id: 'renter-a',
    occurred_at: '2026-10-12T00:00:00Z',
    time_zone: 'America/Santiago',
    synthetic_location: {
      source: 'synthetic-fixture-v1',
      location_code: 'santiago-demo-center-v1',
      latitude: -33.456,
      longitude: -70.6693,
    },
    result: 'registrada',
    sequence,
    evidence: [],
  }) as unknown as LocalRentalOperation;
const envelope = <T>(data: T) => ({ safety_notice: 'ENSAYO LOCAL — SIN COBRO REAL', data });
const deferred = <T>() => {
  let resolve!: (value: T) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<T>((ok, fail) => {
    resolve = ok;
    reject = fail;
  });
  return { promise, resolve, reject };
};
const settle = async (fixture: ReturnType<typeof TestBed.createComponent>) => {
  for (let i = 0; i < 4; i++) await Promise.resolve();
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
};
const settleRef = async (ref: { changeDetectorRef: { detectChanges(): void } }) => {
  for (let i = 0; i < 4; i++) await Promise.resolve();
  ref.changeDetectorRef.detectChanges();
};
type PanelAccess = {
  openContract(): Promise<void>;
  refreshContract(): Promise<void>;
  sign(): Promise<void>;
  reject(): Promise<void>;
  record(action: 'check-in' | 'check-out' | 'reception'): Promise<void>;
  comments: { set(value: string): void; (): string };
  observations: { set(value: string): void; (): string };
  attempts?: unknown;
};
type PageAccess = { reservation(): LocalBookingDetail | null; error(): string };

function integratedSetup(
  initialState: string,
  lifecycleOverrides: Record<string, ReturnType<typeof vi.fn>> = {},
) {
  const routeParams = new BehaviorSubject(convertToParamMap({ id: 'reservation-1' }));
  let goState = initialState;
  let contract = contractFor('generado', { 'renter-a': 'pendiente', 'host-a': 'pendiente' });
  let sequence = 0;
  const identityApi = {
    login: vi.fn(async (email: string) =>
      email.startsWith('host')
        ? participant('host-a', 'token-host')
        : email.startsWith('third')
          ? participant('third-party', 'token-third')
          : participant('renter-a', 'token-renter'),
    ),
    session: vi.fn(async (token: string) => ({
      account_id:
        token === 'token-host' ? 'host-a' : token === 'token-third' ? 'third-party' : 'renter-a',
      roles: ['arrendatario'],
      capabilities: [],
      restricted_mode: false,
    })),
    logout: vi.fn(async () => undefined),
  };
  const bookingApi = {
    detail: vi.fn(async (_token: string, _id: string) => envelope(detail(goState))),
    list: vi.fn(),
    requestReservation: vi.fn(),
    payment: vi.fn(),
    decision: vi.fn(),
    cancellationPreview: vi.fn(),
    cancel: vi.fn(),
    refund: vi.fn(),
  };
  const lifecycleApi = {
    createOrGetContract: vi.fn(async () => envelope(contract)),
    getContract: vi.fn(async () => envelope(contract)),
    signContract: vi.fn(async (token: string) => {
      const account = token === 'token-host' ? 'host-a' : 'renter-a';
      const signatures = Object.fromEntries(
        contract.signatures.map((entry) => [entry.signer_id, entry.state]),
      );
      signatures[account] = 'firmada';
      const signed = Object.values(signatures).every((value) => value === 'firmada');
      contract = contractFor(signed ? 'firmado' : 'firma_parcial', signatures);
      goState = signed ? 'lista_para_checkin' : 'firma_parcial';
      return envelope(contract);
    }),
    rejectContract: vi.fn(async () => {
      contract = contractFor('anulado', { 'renter-a': 'rechazada', 'host-a': 'pendiente' });
      goState = 'cancelada_por_firma';
      return envelope(contract);
    }),
    contractPdf: vi.fn(async () => new Blob(['%PDF synthetic'], { type: 'application/pdf' })),
    operations: vi.fn(async () => ({ data: { items: [] }, notice: 'EVIDENCIA SINTÉTICA' })),
    record: vi.fn(
      async (_token: string, _id: string, action: string, _body: unknown, _key: string) => {
        goState =
          action === 'check-in' ? 'en_curso' : action === 'check-out' ? 'finalizada' : 'cerrada';
        return envelope(
          operation(
            action === 'check-in' ? 'checkin' : action === 'check-out' ? 'checkout' : 'recepcion',
            ++sequence,
          ),
        );
      },
    ),
    evidence: vi.fn(async () => new Blob(['PNG'], { type: 'image/png' })),
    ...lifecycleOverrides,
  };
  TestBed.configureTestingModule({
    imports: [BookingDetailPage],
    providers: [
      ...provideFrontendPorts(),
      provideRouter([]),
      PrivateSessionState,
      { provide: ActivatedRoute, useValue: { paramMap: routeParams } },
      { provide: IdentityApi, useValue: identityApi },
      { provide: BookingApi, useValue: bookingApi },
      { provide: BookingLifecycleApi, useValue: lifecycleApi },
    ],
  });
  const fixture = TestBed.createComponent(BookingDetailPage);
  fixture.detectChanges();
  const panel = () =>
    fixture.debugElement.query(By.directive(BookingLifecyclePanel))
      .componentInstance as unknown as PanelAccess;
  return {
    fixture,
    identity: TestBed.inject(IdentitySession),
    attempts: TestBed.inject(BookingLifecycleAttempts),
    bookingApi,
    lifecycleApi,
    panel,
    page: () => fixture.componentInstance as unknown as PageAccess,
    state: () => goState,
    setGoState: (state: string) => {
      goState = state;
    },
  };
}

function concurrentParticipantPanel(
  lifecycleApi: ReturnType<typeof vi.fn> | object,
  identityApi: object,
  reservationValue: LocalBookingDetail,
) {
  let injector!: EnvironmentInjector;
  injector = createEnvironmentInjector(
    [
      { provide: IdentityApi, useValue: identityApi },
      { provide: PrivateSessionState, useFactory: () => new PrivateSessionState() },
      { provide: BookingLifecycleApi, useValue: lifecycleApi },
      { provide: BookingLifecycleAttempts, useValue: TestBed.inject(BookingLifecycleAttempts) },
      {
        provide: IdentitySession,
        useFactory: () => runInInjectionContext(injector, () => new IdentitySession()),
      },
    ],
    TestBed.inject(EnvironmentInjector),
  );
  const ref = createComponent(BookingLifecyclePanel, { environmentInjector: injector });
  ref.setInput('reservation', reservationValue);
  ref.changeDetectorRef.detectChanges();
  return {
    ref,
    panel: ref.instance as unknown as PanelAccess,
    identity: injector.get(IdentitySession),
    destroy: () => {
      ref.destroy();
      injector.destroy();
    },
  };
}

describe('BookingDetailPage + BookingLifecyclePanel integration with real IdentitySession', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    TestBed.resetTestingModule();
  });

  it('re-reads Go state after both signatures, check-in, check-out and host reception', async () => {
    const test = integratedSetup('aprobada_host');
    await test.identity.signIn('renter@example.invalid', 'synthetic');
    await settle(test.fixture);
    const panel = test.panel();
    await panel.openContract();
    await settle(test.fixture);
    await panel.sign();
    await settle(test.fixture);
    expect(test.page().reservation()?.state).toBe('firma_parcial');

    await test.identity.signOut();
    await test.identity.signIn('host@example.invalid', 'synthetic');
    await settle(test.fixture);
    await test.panel().openContract();
    await settle(test.fixture);
    await test.panel().sign();
    await settle(test.fixture);
    expect(test.page().reservation()?.state).toBe('lista_para_checkin');

    await test.identity.signOut();
    await test.identity.signIn('renter@example.invalid', 'synthetic');
    await settle(test.fixture);
    await test.panel().record('check-in');
    await settle(test.fixture);
    expect(test.page().reservation()?.state).toBe('en_curso');
    expect(test.fixture.nativeElement.textContent).toContain('Registrar check-out');

    await test.panel().record('check-out');
    await settle(test.fixture);
    expect(test.page().reservation()?.state).toBe('finalizada');
    await test.identity.signOut();
    await test.identity.signIn('host@example.invalid', 'synthetic');
    await settle(test.fixture);
    await test.panel().record('reception');
    await settle(test.fixture);
    expect(test.page().reservation()?.state).toBe('cerrada');
    expect(test.bookingApi.detail.mock.calls.length).toBeGreaterThanOrEqual(7);
    test.fixture.destroy();
  });

  it('refreshes the parent after contract rejection', async () => {
    const test = integratedSetup('aprobada_host');
    await test.identity.signIn('renter@example.invalid', 'synthetic');
    await settle(test.fixture);
    await test.panel().openContract();
    await settle(test.fixture);
    await test.panel().reject();
    await settle(test.fixture);
    expect(test.page().reservation()?.state).toBe('cancelada_por_firma');
    expect(test.lifecycleApi.rejectContract).toHaveBeenCalledWith('token-renter', 'contract-1');
    test.fixture.destroy();
  });

  it('refreshes renter detail after the host signs concurrently, without logout or remount', async () => {
    const test = integratedSetup('aprobada_host');
    await test.identity.signIn('renter@example.invalid', 'synthetic');
    await settle(test.fixture);
    const renterView = test.fixture;
    await test.panel().openContract();
    await settle(test.fixture);
    await test.panel().sign();
    await settle(test.fixture);
    expect(test.page().reservation()?.state).toBe('firma_parcial');

    const host = concurrentParticipantPanel(
      test.lifecycleApi,
      TestBed.inject(IdentityApi),
      test.page().reservation()!,
    );
    await host.identity.signIn('host@example.invalid', 'synthetic');
    await settleRef(host.ref);
    await host.panel.openContract();
    await settleRef(host.ref);
    await host.panel.sign();
    await settleRef(host.ref);
    expect(test.identity.accessToken()).toBe('token-renter');
    expect(test.fixture).toBe(renterView);

    await test.panel().refreshContract();
    await settle(test.fixture);
    expect(test.page().reservation()?.state).toBe('lista_para_checkin');
    expect(test.fixture.nativeElement.textContent).toContain('Registrar check-in');
    host.destroy();
    test.fixture.destroy();
  });

  it('refreshes renter detail after an external host rejection and removes stale check-in capability', async () => {
    const test = integratedSetup('aprobada_host');
    await test.identity.signIn('renter@example.invalid', 'synthetic');
    await settle(test.fixture);
    await test.panel().openContract();
    await settle(test.fixture);
    const host = concurrentParticipantPanel(
      test.lifecycleApi,
      TestBed.inject(IdentityApi),
      test.page().reservation()!,
    );
    await host.identity.signIn('host@example.invalid', 'synthetic');
    await settleRef(host.ref);
    await host.panel.openContract();
    await settleRef(host.ref);
    await host.panel.reject();
    await settleRef(host.ref);

    await test.panel().refreshContract();
    await settle(test.fixture);
    expect(test.page().reservation()?.state).toBe('cancelada_por_firma');
    expect(test.fixture.nativeElement.textContent).toContain('Anulado por Go');
    expect(test.fixture.nativeElement.textContent).not.toContain('Registrar check-in');
    host.destroy();
    test.fixture.destroy();
  });

  it('retries an uncertain check-in with the original non-empty payload and key after reauthentication, while keeping fields clean', async () => {
    const first = deferred<ReturnType<typeof envelope<LocalRentalOperation>>>();
    let test!: ReturnType<typeof integratedSetup>;
    const record = vi
      .fn()
      .mockReturnValueOnce(first.promise)
      .mockImplementationOnce(async () => {
        test.setGoState('en_curso');
        return envelope(operation('checkin', 1));
      });
    test = integratedSetup('lista_para_checkin', { record });
    await test.identity.signIn('renter@example.invalid', 'synthetic');
    await settle(test.fixture);
    const panel = test.panel();
    panel.comments.set('synthetic comment A');
    const pendingWrite = panel.record('check-in');
    await test.identity.signOut();
    await test.identity.signIn('renter@example.invalid', 'synthetic');
    await settle(test.fixture);
    expect(test.panel().comments()).toBe('');
    expect(test.fixture.nativeElement.textContent).toContain('Reintentar check-in original');
    await test.panel().record('check-in');
    await settle(test.fixture);
    const key = test.lifecycleApi.record.mock.calls[0]?.[4];
    expect(test.lifecycleApi.record).toHaveBeenNthCalledWith(
      2,
      'token-renter',
      'reservation-1',
      'check-in',
      { comments: 'synthetic comment A' },
      key,
    );
    first.reject(new Error('timeout de la primera respuesta'));
    await pendingWrite;
    await settle(test.fixture);
    expect(test.page().reservation()?.state).toBe('en_curso');
    test.fixture.destroy();
  });

  it('settles only the originating rejected key when definitive responses arrive late', async () => {
    const oldOne = deferred<ReturnType<typeof envelope<LocalRentalOperation>>>();
    const oldRetry = deferred<ReturnType<typeof envelope<LocalRentalOperation>>>();
    const test = integratedSetup('lista_para_checkin', {
      record: vi
        .fn()
        .mockReturnValueOnce(oldOne.promise)
        .mockReturnValueOnce(oldRetry.promise)
        .mockResolvedValueOnce(envelope(operation('checkin', 1))),
    });
    await test.identity.signIn('renter@example.invalid', 'synthetic');
    await settle(test.fixture);
    test.panel().comments.set('comment A');
    const first = test.panel().record('check-in');
    await test.identity.signOut();
    await test.identity.signIn('renter@example.invalid', 'synthetic');
    await settle(test.fixture);
    const retry = test.panel().record('check-in');
    const original = test.attempts.find('renter-a', 'reservation-1', 'checkin');
    oldOne.reject(new BookingApiError('definitive validation', 422));
    await first;
    await settle(test.fixture);
    expect(test.attempts.find('renter-a', 'reservation-1', 'checkin')).toBeNull();
    test.panel().comments.set('corrected payload B');
    await test.panel().record('check-in');
    await settle(test.fixture);
    const replacement = test.attempts.find('renter-a', 'reservation-1', 'checkin');
    expect(replacement?.key).not.toBe(original?.key);
    oldRetry.reject(new BookingApiError('late old validation', 422));
    await retry;
    await settle(test.fixture);
    expect(test.attempts.find('renter-a', 'reservation-1', 'checkin')?.key).toBe(replacement?.key);
    expect(test.identity.accessToken()).toBe('token-renter');
    test.fixture.destroy();
  });
});
