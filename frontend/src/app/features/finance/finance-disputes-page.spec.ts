import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, provideRouter } from '@angular/router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ApiRuntimeConfig } from '../../core/api-runtime-config';
import { IdentitySession } from '../../core/identity/application/identity-session';
import { IdentityApi } from '../../core/identity/infrastructure/identity-api';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { provideFrontendPorts } from '../../frontend.providers';
import { FinanceDisputesPage } from './finance-disputes-page';

describe('FinanceDisputesPage', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('requires Go confirmed claim resolution before recording a financial decision', async () => {
    const api = {
      login: vi.fn().mockResolvedValue({
        account_id: 'admin-1',
        roles: ['administrador'],
        access_token: 'admin-memory-token',
        token_type: 'Bearer',
        expires_at: '2027-01-01T00:00:00Z',
        capabilities: [],
        restricted_mode: false,
      }),
      session: vi
        .fn()
        .mockResolvedValue({
          account_id: 'admin-1',
          roles: ['administrador'],
          capabilities: [],
          restricted_mode: false,
        }),
      logout: vi.fn(),
    };
    TestBed.configureTestingModule({
      imports: [FinanceDisputesPage],
      providers: [
        ...provideFrontendPorts(),
        provideHttpClient(),
        provideHttpClientTesting(),
        provideRouter([]),
        PrivateSessionState,
        {
          provide: ActivatedRoute,
          useValue: {
            snapshot: { data: { mode: 'admin' } },
            paramMap: { subscribe: () => ({ unsubscribe() {} }) },
          },
        },
        { provide: IdentityApi, useValue: api },
      ],
    });
    const identity = TestBed.inject(IdentitySession);
    TestBed.inject(ApiRuntimeConfig).apiBaseUrl.set('http://127.0.0.1:18180/api/v1');
    await identity.signIn('admin@example.invalid', 'Synthetic#123');
    const fixture = TestBed.createComponent(FinanceDisputesPage);
    fixture.detectChanges();
    const http = TestBed.inject(HttpTestingController);
    const page = fixture.componentInstance as unknown as {
      resolve(claim: unknown): Promise<void>;
      decideClaim(claim: unknown): Promise<void>;
      claimResolution(claim: unknown): string;
      selectedClaim(): unknown;
      financialDecision(): unknown;
      deductionAmount: { set(value: string): void };
      setDecisionEvidenceId(value: string): void;
    };
    await Promise.resolve();
    http.expectOne((request) => request.url.endsWith('/admin/disputes')).flush('{"items":[]}');
    http
      .expectOne((request) => request.url.endsWith('/admin/local/damage-claims'))
      .flush('{"items":[]}');
    const claim = { id: 'claim-1', reservation_id: 'reservation-1', state: 'open' };

    const resolution = page.resolve(claim);
    const resolutionRequest = http.expectOne((request) => request.url.endsWith('/resolution'));
    expect(resolutionRequest.request.headers.get('Idempotency-Key')).toMatch(/^[0-9a-f-]{36}$/);
    resolutionRequest.flush(
      '{"data":{"id":"claim-1","reservation_id":"reservation-1","resolution":{"outcome":"acogido","reason_code":"evidencia_suficiente"}}}',
    );
    await Promise.resolve();
    await Promise.resolve();
    http.expectOne((request) => request.url.endsWith('/admin/disputes')).flush('{"items":[]}');
    http
      .expectOne((request) => request.url.endsWith('/admin/local/damage-claims'))
      .flush('{"items":[]}');
    await resolution;
    const selected = page.selectedClaim();
    expect(page.claimResolution(selected)).toBe('acogido');

    page.deductionAmount.set('1000');
    page.setDecisionEvidenceId('evidence-1');
    const decision = page.decideClaim(selected);
    const decisionRequest = http.expectOne((request) =>
      request.url.endsWith('/financial-decision'),
    );
    expect(decisionRequest.request.body).toMatchObject({
      claim_id: 'claim-1',
      outcome: 'acogido',
      deduction_clp: 1000,
      reason_code: 'dano_acreditado',
      evidence_id: 'evidence-1',
    });
    decisionRequest.flush(
      '{"data":{"state":"pendiente","deduction_clp":1000,"evidence_id":"evidence-1"}}',
    );
    await Promise.resolve();
    await Promise.resolve();
    http.expectOne((request) => request.url.endsWith('/admin/disputes')).flush('{"items":[]}');
    http
      .expectOne((request) => request.url.endsWith('/admin/local/damage-claims'))
      .flush('{"items":[]}');
    await decision;
    expect(page.selectedClaim()).toEqual(selected);
    expect(page.financialDecision()).toEqual({
      state: 'pendiente',
      deduction_clp: '1000',
      evidence_id: 'evidence-1',
    });
    http.verify();
    fixture.destroy();
  });

  it('clears reservation administration context and discards out-of-order reads when the selected reservation changes', async () => {
    const api = {
      login: vi
        .fn()
        .mockResolvedValue({
          account_id: 'admin-2',
          roles: ['administrador'],
          access_token: 'admin-context-token',
          token_type: 'Bearer',
          expires_at: '2027-01-01T00:00:00Z',
          capabilities: [],
          restricted_mode: false,
        }),
      session: vi
        .fn()
        .mockResolvedValue({
          account_id: 'admin-2',
          roles: ['administrador'],
          capabilities: [],
          restricted_mode: false,
        }),
      logout: vi.fn(),
    };
    TestBed.configureTestingModule({
      imports: [FinanceDisputesPage],
      providers: [
        ...provideFrontendPorts(),
        provideHttpClient(),
        provideHttpClientTesting(),
        provideRouter([]),
        PrivateSessionState,
        {
          provide: ActivatedRoute,
          useValue: {
            snapshot: { data: { mode: 'admin' } },
            paramMap: { subscribe: () => ({ unsubscribe() {} }) },
          },
        },
        { provide: IdentityApi, useValue: api },
      ],
    });
    const identity = TestBed.inject(IdentitySession);
    TestBed.inject(ApiRuntimeConfig).apiBaseUrl.set('http://127.0.0.1:18180/api/v1');
    await identity.signIn('admin@example.invalid', 'Synthetic#123');
    const fixture = TestBed.createComponent(FinanceDisputesPage);
    fixture.detectChanges();
    const http = TestBed.inject(HttpTestingController);
    const page = fixture.componentInstance as unknown as {
      load(): Promise<void>;
      setAdminReservationId(value: string): void;
      adminReservationId(): string;
      guarantee(): unknown;
      settlement(): unknown;
      receipt(): unknown;
      selectedClaim(): unknown;
      financialDecision(): unknown;
    };
    http.expectOne((request) => request.url.endsWith('/admin/disputes')).flush('{"items":[]}');
    http
      .expectOne((request) => request.url.endsWith('/admin/local/damage-claims'))
      .flush('{"items":[]}');
    page.setAdminReservationId('reservation-a');
    const oldLoad = page.load();
    const oldRequests = http.match((request) =>
      request.url.includes('/reservations/reservation-a/'),
    );
    expect(oldRequests).toHaveLength(3);
    http.expectOne((request) => request.url.endsWith('/admin/disputes')).flush('{"items":[]}');
    http
      .expectOne((request) => request.url.endsWith('/admin/local/damage-claims'))
      .flush('{"items":[]}');
    page.setAdminReservationId('reservation-b');
    expect(page.guarantee()).toBeNull();
    expect(page.settlement()).toBeNull();
    expect(page.receipt()).toBeNull();
    expect(page.selectedClaim()).toBeNull();
    expect(page.financialDecision()).toBeNull();
    oldRequests.forEach((request) =>
      request.flush('{"data":{"reservation_id":"reservation-a","state":"old"}}'),
    );
    await oldLoad;
    expect(page.adminReservationId()).toBe('reservation-b');
    expect(page.guarantee()).toBeNull();
    expect(page.settlement()).toBeNull();
    expect(page.receipt()).toBeNull();

    const currentLoad = page.load();
    http
      .match((request) => request.url.includes('/reservations/reservation-b/'))
      .forEach((request) =>
        request.flush('{"data":{"reservation_id":"reservation-b","state":"actual"}}'),
      );
    http.expectOne((request) => request.url.endsWith('/admin/disputes')).flush('{"items":[]}');
    http
      .expectOne((request) => request.url.endsWith('/admin/local/damage-claims'))
      .flush('{"items":[]}');
    await currentLoad;
    expect(page.guarantee()).toEqual({ reservation_id: 'reservation-b', state: 'actual' });
    expect(page.settlement()).toEqual({ reservation_id: 'reservation-b', state: 'actual' });
    expect(page.receipt()).toEqual({ reservation_id: 'reservation-b', state: 'actual' });
    http.verify();
    fixture.destroy();
  });

  it('isolates uncertain admin writes when the account changes without clearing the old instance early', async () => {
    const api = {
      login: vi.fn().mockImplementation(async (email: string) => {
        const account = email.startsWith('admin-a') ? 'admin-a' : 'admin-b';
        return {
          account_id: account,
          roles: ['administrador'],
          access_token: `${account}-token`,
          token_type: 'Bearer',
          expires_at: '2027-01-01T00:00:00Z',
          capabilities: [],
          restricted_mode: false,
        };
      }),
      session: vi
        .fn()
        .mockImplementation(async (token: string) => ({
          account_id: token.startsWith('admin-a') ? 'admin-a' : 'admin-b',
          roles: ['administrador'],
          capabilities: [],
          restricted_mode: false,
        })),
      logout: vi.fn(),
    };
    TestBed.configureTestingModule({
      imports: [FinanceDisputesPage],
      providers: [
        ...provideFrontendPorts(),
        provideHttpClient(),
        provideHttpClientTesting(),
        provideRouter([]),
        PrivateSessionState,
        {
          provide: ActivatedRoute,
          useValue: {
            snapshot: { data: { mode: 'admin' } },
            paramMap: { subscribe: () => ({ unsubscribe() {} }) },
          },
        },
        { provide: IdentityApi, useValue: api },
      ],
    });
    const identity = TestBed.inject(IdentitySession);
    TestBed.inject(ApiRuntimeConfig).apiBaseUrl.set('http://127.0.0.1:18180/api/v1');
    await identity.signIn('admin-a@example.invalid', 'Synthetic#123');
    const fixture = TestBed.createComponent(FinanceDisputesPage);
    fixture.detectChanges();
    const http = TestBed.inject(HttpTestingController);
    const page = fixture.componentInstance as unknown as {
      resolve(claim: unknown): Promise<void>;
      setDecision(value: string): void;
    };
    http.expectOne((request) => request.url.endsWith('/admin/disputes')).flush('{"items":[]}');
    http
      .expectOne((request) => request.url.endsWith('/admin/local/damage-claims'))
      .flush('{"items":[]}');

    const claim = { id: 'shared-claim', reservation_id: 'shared-reservation' };
    const first = page.resolve(claim);
    const firstRequest = http.expectOne((request) => request.url.endsWith('/resolution'));
    const firstKey = firstRequest.request.headers.get('Idempotency-Key');
    firstRequest.flush('{"error":"timeout"}', { status: 504, statusText: 'Gateway Timeout' });
    await first;

    await identity.signIn('admin-b@example.invalid', 'Synthetic#123');
    fixture.detectChanges();
    await Promise.resolve();
    await Promise.resolve();
    http.expectOne((request) => request.url.endsWith('/admin/disputes')).flush('{"items":[]}');
    http
      .expectOne((request) => request.url.endsWith('/admin/local/damage-claims'))
      .flush('{"items":[]}');
    page.setDecision('acogido');
    const second = page.resolve(claim);
    const secondRequest = http.expectOne((request) => request.url.endsWith('/resolution'));
    expect(secondRequest.request.body).toMatchObject({ outcome: 'acogido' });
    expect(secondRequest.request.headers.get('Idempotency-Key')).not.toBe(firstKey);
    secondRequest.flush(
      '{"data":{"id":"shared-claim","reservation_id":"shared-reservation","resolution":{"outcome":"acogido"}}}',
    );
    await Promise.resolve();
    await Promise.resolve();
    http.expectOne((request) => request.url.endsWith('/admin/disputes')).flush('{"items":[]}');
    http
      .expectOne((request) => request.url.endsWith('/admin/local/damage-claims'))
      .flush('{"items":[]}');
    await second;
    http.verify();
    fixture.destroy();
  });
});
