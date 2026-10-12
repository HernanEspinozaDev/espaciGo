import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting, HttpTestingController } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, provideRouter } from '@angular/router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { IdentityApi } from '../../core/identity/identity-api';
import { IdentitySession } from '../../core/identity/identity-session';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { ApiRuntimeConfig } from '../../core/api-runtime-config';
import { FinanceDisputesPage } from './finance-disputes-page';

describe('FinanceDisputesPage', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('requires Go confirmed claim resolution before recording a financial decision', async () => {
    const api = {
      login: vi.fn().mockResolvedValue({
        account_id: 'admin-1', roles: ['administrador'], access_token: 'admin-memory-token',
        token_type: 'Bearer', expires_at: '2027-01-01T00:00:00Z', restricted_mode: false,
      }),
      session: vi.fn().mockResolvedValue({ account_id: 'admin-1', roles: ['administrador'], restricted_mode: false }),
      logout: vi.fn(),
    };
    TestBed.configureTestingModule({
      imports: [FinanceDisputesPage],
      providers: [
        provideHttpClient(), provideHttpClientTesting(), provideRouter([]), PrivateSessionState,
        { provide: ActivatedRoute, useValue: { snapshot: { data: { mode: 'admin' } }, paramMap: { subscribe: () => ({ unsubscribe() {} }) } } },
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
    http.expectOne((request) => request.url.endsWith('/admin/local/damage-claims')).flush('{"items":[]}');
    const claim = { id: 'claim-1', reservation_id: 'reservation-1', state: 'open' };

    const resolution = page.resolve(claim);
    http.expectOne((request) => request.url.endsWith('/resolution'))
      .flush('{"data":{"id":"claim-1","reservation_id":"reservation-1","resolution":{"outcome":"acogido","reason_code":"evidencia_suficiente"}}}');
    await Promise.resolve(); await Promise.resolve();
    http.expectOne((request) => request.url.endsWith('/admin/disputes')).flush('{"items":[]}');
    http.expectOne((request) => request.url.endsWith('/admin/local/damage-claims')).flush('{"items":[]}');
    await resolution;
    const selected = page.selectedClaim();
    expect(page.claimResolution(selected)).toBe('acogido');

    page.deductionAmount.set('1000');
    page.setDecisionEvidenceId('evidence-1');
    const decision = page.decideClaim(selected);
    const decisionRequest = http.expectOne((request) => request.url.endsWith('/financial-decision'));
    expect(decisionRequest.request.body).toMatchObject({ claim_id: 'claim-1', outcome: 'acogido', deduction_clp: 1000, reason_code: 'dano_acreditado', evidence_id: 'evidence-1' });
    decisionRequest.flush('{"data":{"state":"pendiente","deduction_clp":1000,"evidence_id":"evidence-1"}}');
    await Promise.resolve(); await Promise.resolve();
    http.expectOne((request) => request.url.endsWith('/admin/disputes')).flush('{"items":[]}');
    http.expectOne((request) => request.url.endsWith('/admin/local/damage-claims')).flush('{"items":[]}');
    await decision;
    expect(page.selectedClaim()).toEqual(selected);
    expect(page.financialDecision()).toEqual({ state: 'pendiente', deduction_clp: '1000', evidence_id: 'evidence-1' });
    http.verify();
    fixture.destroy();
  });
});
