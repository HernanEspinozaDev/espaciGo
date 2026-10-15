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
  capabilities: ['accounts.list', 'accounts.read','accounts.update','accounts.roles','accounts.block','publications.list','publications.read','publications.update','publications.moderate'],
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
  phone: null,
  profile_updated_at: '2026-10-14T00:00:00Z',
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
    updateProfile:vi.fn().mockResolvedValue({account_id:'customer',email:account.email,display_name:'Updated customer',phone:'987654321',updated_at:'2026-10-14T01:00:00Z'}),
    updateRole:vi.fn().mockResolvedValue(undefined),
    revokeSessions:vi.fn().mockResolvedValue(2),
    setBlocked:vi.fn().mockResolvedValue(undefined),
    listPublications:vi.fn().mockResolvedValue({items:[{publication_id:'space-1',owner_id:'customer',owner_name:'Private customer',owner_email:account.email,category_code:'oficina',category_name:'Oficina',title:'Test space',description:'d'.repeat(120),area_m2:'20.00',capacity:4,usage_rules:'Reglas',address:'Privada',state:'activa',rate_unit:'hora',base_price_clp:'9007199254740999',updated_at:'2026-10-14T00:00:00Z',publicly_visible:false,public_cover_available:false}],next_cursor:'',matching_total:1,generated_at:overview.generated_at}),
    setPublicationState:vi.fn().mockResolvedValue(undefined),
    updatePublication:vi.fn().mockResolvedValue(undefined),
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
  const search = component.search();
  await vi.waitFor(() => expect(api.list).toHaveBeenCalledTimes(2));
  await search;
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

it('keeps a pending filter search and its cursor when opening an account', async () => {
  const { component, api, facade } = await setup();
  let resolveSearch!: (value: Awaited<ReturnType<typeof api.list>>) => void;
  api.list.mockImplementationOnce(() => new Promise((resolve) => { resolveSearch = resolve; }));
  component.filters.controls.emailPrefix.setValue('new-filter');
  const search = component.search();

  await component.open('customer');
  expect(facade.page()).toBeNull();
  component.next();
  expect(api.list).toHaveBeenCalledTimes(2);

  resolveSearch({
    items: [{ ...account, account_id: 'new-customer', email: 'new-filter@example.invalid' }],
    next_cursor: 'cursor-new-filter',
    matching_total: 2,
    generated_at: overview.generated_at,
  });
  await search;
  expect(facade.page()?.items[0].account_id).toBe('new-customer');

  component.next();
  expect(api.list).toHaveBeenLastCalledWith('token-a', expect.objectContaining({
    email_prefix: 'new-filter',
    cursor: 'cursor-new-filter',
  }));
});

it('keeps a pending detail response when a new filtered page finishes first', async () => {
  const { component, api, facade } = await setup();
  let resolveDetail!: (value: typeof account) => void;
  api.account.mockImplementationOnce(() => new Promise((resolve) => { resolveDetail = resolve; }));
  const detail = component.open('customer');
  component.filters.controls.emailPrefix.setValue('second-filter');
  await component.search();
  expect(facade.page()?.items[0].account_id).toBe('customer');

  resolveDetail({ ...account, display_name: 'Details finished later' });
  await detail;

  expect(facade.page()?.items[0].account_id).toBe('customer');
  expect(facade.selected()?.display_name).toBe('Details finished later');
});

it('sends allowlisted account edits, ordinary roles, blocks, and session revocation through the port', async () => {
  const { component, api, facade } = await setup();
  await component.open('customer');
  await facade.updateProfile({display_name:'Updated customer',phone:'987654321',expected_updated_at:account.profile_updated_at,reason:'correccion_perfil'});
  await facade.updateRole('arrendador',{granted:true,reason:'revision_administrativa'});
  await facade.setBlocked({blocked:true,reason:'revision_administrativa'});
  const revoked=await facade.revokeSessions({reason:'gestion_operativa'});

  expect(api.updateProfile).toHaveBeenCalledWith('token-a','customer',expect.objectContaining({expected_updated_at:account.profile_updated_at}));
  expect(api.updateRole).toHaveBeenCalledWith('token-a','customer','arrendador',expect.objectContaining({granted:true}));
  expect(api.setBlocked).toHaveBeenCalledWith('token-a','customer',expect.objectContaining({blocked:true}));
  expect(revoked).toBe(2);
  expect(facade.selected()?.display_name).toBe('Updated customer');
  expect(facade.selected()?.administrative_blocked).toBe(true);
});

it('loads global publications by owner and preserves exact CLP strings while editing', async () => {
  const { component, api, facade } = await setup();
  component.showTab('publications');
  await vi.waitFor(()=>expect(facade.publications()?.items).toHaveLength(1));
  const publication=facade.publications()!.items[0];
  expect(publication.base_price_clp).toBe('9007199254740999');
  expect(api.listPublications).toHaveBeenCalledWith('token-a',expect.objectContaining({page_size:25}));

  component.editPublication(publication);
  await component.savePublication();
  expect(api.updatePublication).toHaveBeenCalledWith('token-a','space-1',expect.objectContaining({base_price_clp:'9007199254740999',expected_updated_at:publication.updated_at}));
});
