import { provideHttpClient } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { IdentityApi } from '../../core/identity/identity-api';
import { IdentitySession } from '../../core/identity/identity-session';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { AdminApi, AdminApiError } from './admin-api';
import { FinanceApi } from '../finance/finance-api';
import { AdminPage } from './admin-page';

const adminSession = (account_id: string) => ({ account_id, roles: ['administrador'], restricted_mode: false });
const login = (account_id: string, access_token: string) => ({ account_id, roles: ['administrador'], access_token, token_type: 'Bearer', expires_at: '2027-01-01T00:00:00Z', restricted_mode: false });

function setup(api: { get: ReturnType<typeof vi.fn>; post: ReturnType<typeof vi.fn> }) {
  TestBed.configureTestingModule({
    imports: [AdminPage],
    providers: [provideHttpClient(), provideRouter([]), PrivateSessionState,
      { provide: IdentityApi, useValue: {
        login: vi.fn().mockResolvedValueOnce(login('admin-a', 'token-a')).mockResolvedValueOnce(login('admin-b', 'token-b')).mockResolvedValueOnce(login('member', 'token-member')),
        session: vi.fn().mockImplementation((token: string) => Promise.resolve(adminSession(token === 'token-a' ? 'admin-a' : token === 'token-b' ? 'admin-b' : 'member'))),
        logout: vi.fn().mockResolvedValue(undefined),
      } },
      { provide: AdminApi, useValue: api },
      { provide: FinanceApi, useValue: { call: vi.fn().mockResolvedValue({ data: { confirmed_movements: [{ amount_clp: '9007199254740993' }] } }) } },
    ],
  });
  return TestBed;
}

function page(fixture: ReturnType<typeof TestBed.createComponent>) {
  return fixture.componentInstance as unknown as {
    items: () => Array<Record<string, unknown>>;
    result: () => unknown;
    error: () => string;
    success: () => string;
    active: () => string;
    filter: { (): string; set(value: string): void };
    nextCursor: () => string;
    selectSection(value: string): Promise<void>;
    applyFilter(): Promise<void>;
    nextPage(): Promise<void>;
    submitAction(): Promise<void>;
    select(item: Record<string, unknown>, action: unknown): void;
    actionsForTemplate(): Array<{ label: string }>;
    loadSingle(path: string, query?: Record<string, string | number | undefined>): Promise<void>;
  };
}

describe('AdminPage', () => {
  afterEach(() => { vi.restoreAllMocks(); TestBed.resetTestingModule(); });

  it('does not let an old queue response replace the data from a new admin session', async () => {
    let finishA!: (value: unknown) => void;
    const pendingA = new Promise<unknown>((resolve) => { finishA = resolve; });
    const api = { get: vi.fn().mockImplementation((token: string) => token === 'token-a' ? pendingA : Promise.resolve({ items: [{ id: 'case-b', state: 'en_revision' }] })), post: vi.fn() };
    setup(api);
    const identity = TestBed.inject(IdentitySession);
    await identity.signIn('admin-a@example.invalid', 'Synthetic#123');
    const fixture = TestBed.createComponent(AdminPage); fixture.detectChanges();
    await vi.waitFor(() => expect(api.get).toHaveBeenCalledWith('token-a', '/admin/verifications', undefined));
    await identity.signOut(); await identity.signIn('admin-b@example.invalid', 'Synthetic#456');
    await vi.waitFor(() => expect(page(fixture).items()[0]?.['id']).toBe('case-b'));
    finishA({ items: [{ id: 'case-a', state: 'en_revision' }] });
    await Promise.resolve();
    expect(page(fixture).items()[0]?.['id']).toBe('case-b');
    fixture.destroy();
  });

  it('resets reservation cursor on filter changes and follows only Go next_cursor', async () => {
    const api = { get: vi.fn()
      .mockResolvedValueOnce({ data: { items: [], next_cursor: 'cursor-1' } })
      .mockResolvedValueOnce({ data: { items: [{ id: 'r-1', state: 'pagada' }], next_cursor: 'cursor-2' } })
      .mockResolvedValueOnce({ data: { items: [{ id: 'r-2', state: 'finalizada' }] } })
      .mockResolvedValueOnce({ data: { items: [{ id: 'r-filtered', state: 'pagada' }] } }), post: vi.fn() };
    setup(api);
    const identity = TestBed.inject(IdentitySession); await identity.signIn('admin-a@example.invalid', 'Synthetic#123');
    const fixture = TestBed.createComponent(AdminPage); fixture.detectChanges();
    await vi.waitFor(() => expect(api.get).toHaveBeenCalled());
    await page(fixture).selectSection('reservations');
    expect(page(fixture).nextCursor()).toBe('cursor-2');
    await page(fixture).nextPage();
    expect(api.get.mock.calls.at(-1)?.[2]).toMatchObject({ cursor: 'cursor-2' });
    page(fixture).filter.set('r-filtered'); await page(fixture).applyFilter();
    expect(api.get.mock.calls.at(-1)?.[2]).toMatchObject({ reservation_id: 'r-filtered', cursor: undefined });
    expect(page(fixture).items()[0]?.['id']).toBe('r-filtered');
    fixture.destroy();
  });

  it('shows Go 403 as a permission failure and preserves identity', async () => {
    const api = { get: vi.fn().mockRejectedValue(new AdminApiError('Go confirmó que esta cuenta no tiene permiso de administración.', 403)), post: vi.fn() };
    setup(api);
    const identity = TestBed.inject(IdentitySession); await identity.signIn('admin-a@example.invalid', 'Synthetic#123');
    const fixture = TestBed.createComponent(AdminPage); fixture.detectChanges();
    await vi.waitFor(() => expect(page(fixture).error()).toContain('no tiene permiso'));
    expect(identity.accessToken()).toBe('token-a');
    expect(page(fixture).items()).toEqual([]);
    fixture.destroy();
  });

  it('submits the selected moderation decision with its reason and idempotency scope', async () => {
    const report = { id: 'report-1', state: 'pendiente' };
    const api = { get: vi.fn().mockResolvedValue({ items: [report] }), post: vi.fn().mockResolvedValue({ data: { ...report, state: 'oculta' } }) };
    setup(api);
    const identity = TestBed.inject(IdentitySession); await identity.signIn('admin-a@example.invalid', 'Synthetic#123');
    const fixture = TestBed.createComponent(AdminPage); fixture.detectChanges();
    await vi.waitFor(() => expect(api.get).toHaveBeenCalled());
    await page(fixture).selectSection('reviews');
    const component = page(fixture);
    const hide = component.actionsForTemplate().find((action) => action.label === 'Ocultar');
    component.select(report, hide);
    const internal = fixture.componentInstance as unknown as { reason: { set(value: string): void } };
    internal.reason.set('contenido_inadecuado');
    await component.submitAction();
    expect(api.post.mock.calls[0]?.slice(0, 4)).toEqual(['token-a', 'admin-a', '/admin/local/review-reports/report-1/hide', { reason_code: 'contenido_inadecuado', correlation_id: expect.any(String) }]);
    expect(api.post.mock.calls[0]?.[4]).toBe('review-report:report-1:hide');
    expect(component.success()).toContain('Go confirmó');
    fixture.destroy();
  });

  it('keeps large CLP report values exact while using the raw-text finance transport', async () => {
    const api = { get: vi.fn().mockResolvedValue({ data: { items: [] } }), post: vi.fn() };
    setup(api);
    const identity = TestBed.inject(IdentitySession); await identity.signIn('admin-a@example.invalid', 'Synthetic#123');
    const fixture = TestBed.createComponent(AdminPage); fixture.detectChanges();
    await vi.waitFor(() => expect(api.get).toHaveBeenCalled());
    const finance = TestBed.inject(FinanceApi);
    await page(fixture).loadSingle('/admin/local/reports/finance', { from: '2026-10-01T00:00:00-03:00', until: '2026-10-02T00:00:00-03:00', timezone: 'America/Santiago' });
    expect(vi.mocked(finance.call)).toHaveBeenCalledWith('token-a', expect.stringContaining('timezone=America%2FSantiago'), 'GET');
    expect(JSON.stringify(page(fixture).result())).toContain('9007199254740993');
    fixture.destroy();
  });
});
