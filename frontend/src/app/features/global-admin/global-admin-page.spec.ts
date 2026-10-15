import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { afterEach, expect, it, vi } from 'vitest';
import { IdentitySession } from '../../core/identity/application/identity-session';
import { IdentityPort } from '../../core/identity/domain/ports/identity-port';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { GlobalAdminPort } from './domain/ports/global-admin-port';
import { GlobalAdminPage } from './global-admin-page';

const globalSession = {
  account_id: 'master',
  roles: ['administrador_global'],
  capabilities: ['accounts.list', 'accounts.read'],
  restricted_mode: false,
};
const overview = {
  total_accounts: 2,
  administrative_blocked: 0,
  by_state: { activo: 2 },
  by_role: {},
  generated_at: '2026-10-14T00:00:00Z',
  definition: 'Conteo de cuentas persistidas.',
};
const account = {
  account_id: 'customer',
  email: 'private@example.invalid',
  display_name: 'Private customer',
  state: 'activo',
  created_at: '2026-10-14T00:00:00Z',
  administrative_blocked: false,
  roles: ['arrendatario'],
  publication_count: 1,
  reservation_count: 0,
  incident_count: 0,
};

function login(accountId: string, token: string) {
  return {
    ...globalSession,
    account_id: accountId,
    access_token: token,
    token_type: 'Bearer',
    expires_at: '2027-01-01T00:00:00Z',
  };
}

async function setup() {
  let confirmed = globalSession;
  const identityApi = {
    login: vi.fn().mockResolvedValueOnce(login('master', 'token-a')).mockResolvedValueOnce(login('master', 'token-b')),
    session: vi.fn().mockImplementation(async () => confirmed),
    logout: vi.fn().mockResolvedValue(undefined),
  };
  const api = {
    overview: vi.fn().mockResolvedValue(overview),
    list: vi.fn().mockResolvedValue({ items: [account], next_cursor: 'cursor-initial', matching_total: 2, generated_at: overview.generated_at }),
    account: vi.fn().mockResolvedValue(account),
  };
  TestBed.configureTestingModule({
    providers: [
      provideRouter([]),
      PrivateSessionState,
      { provide: IdentityPort, useValue: identityApi },
      { provide: GlobalAdminPort, useValue: api },
    ],
  });
  const identity = TestBed.inject(IdentitySession);
  await identity.signIn('master@example.invalid', 'synthetic-test-only');
  const fixture = TestBed.createComponent(GlobalAdminPage);
  fixture.detectChanges();
  const facade = fixture.componentInstance.vm;
  await vi.waitFor(() => expect(facade.page()?.items).toHaveLength(1));
  return {
    identity,
    identityApi,
    api,
    facade,
    component: fixture.componentInstance,
    setConfirmed(value: typeof globalSession) { confirmed = value; },
  };
}

afterEach(() => TestBed.resetTestingModule());

it('clears directory, selection and form controls on real logout', async () => {
  const { identity, facade, component } = await setup();
  await facade.open('customer');
  component.filters.controls.emailPrefix.setValue('private@example.invalid');
  expect(facade.selected()?.email).toBe(account.email);

  await identity.signOut();
  TestBed.tick();

  expect(facade.page()).toBeNull();
  expect(facade.overview()).toBeNull();
  expect(facade.selected()).toBeNull();
  expect(component.filters.controls.emailPrefix.value).toBe('');
});

it('clears private state when the same bearer loses accounts.list on session revalidation', async () => {
  const { identity, identityApi, facade, component, setConfirmed } = await setup();
  component.filters.controls.emailPrefix.setValue('private@example.invalid');
  setConfirmed({ ...globalSession, roles: ['arrendatario'], capabilities: [] });

  await identity.refreshSessionIfCurrent('token-a');
  TestBed.tick();

  expect(identity.accessToken()).toBe('token-a');
  expect(facade.page()).toBeNull();
  expect(facade.overview()).toBeNull();
  expect(component.filters.controls.emailPrefix.value).toBe('');
  expect(identityApi.logout).not.toHaveBeenCalled();
});

it('clears controls and revalidates capabilities after a current 403 without closing the login', async () => {
  const { identity, api, facade, component } = await setup();
  component.filters.controls.emailPrefix.setValue('private@example.invalid');
  api.list.mockRejectedValueOnce({ status: 403 });

  await facade.load({ email_prefix: 'private@example.invalid' });
  TestBed.tick();

  expect(identity.accessToken()).toBe('token-a');
  expect(identity.session()?.capabilities).toContain('accounts.list');
  expect(component.filters.controls.emailPrefix.value).toBe('');
  expect(facade.page()).toBeNull();
  expect(facade.error()).toContain('Go rechazó el acceso');
});

it('paginates using applied filters even when the form contains an unsubmitted draft', async () => {
  const { component, api } = await setup();
  component.search();
  await vi.waitFor(() => expect(api.list).toHaveBeenCalledTimes(2));
  component.filters.controls.emailPrefix.setValue('draft-not-applied');

  component.next();

  expect(api.list).toHaveBeenLastCalledWith('token-a', expect.objectContaining({
    cursor: 'cursor-initial',
    email_prefix: undefined,
  }));
});

it('does not let a stale error from a previous login clear or announce over the current login', async () => {
  let rejectOld!: (reason: unknown) => void;
  const stale = new Promise<never>((_resolve, reject) => { rejectOld = reject; });
  const { identity, api, facade } = await setup();
  api.list.mockImplementationOnce((token: string) => token === 'token-a' ? stale : Promise.resolve({
    items: [account], next_cursor: '', matching_total: 1, generated_at: overview.generated_at,
  }));
  const oldRequest = facade.load({ email_prefix: 'old-session' });
  await identity.signOut();
  await identity.signIn('master@example.invalid', 'synthetic-test-only');
  await vi.waitFor(() => expect(facade.page()?.items[0].email).toBe(account.email));
  rejectOld({ status: 403, message: 'stale denial' });
  await oldRequest;

  expect(facade.page()?.items[0].email).toBe(account.email);
  expect(facade.error()).toBe('');
});
