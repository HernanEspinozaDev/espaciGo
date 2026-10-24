import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import axe from 'axe-core';
import { afterEach, expect, it, vi } from 'vitest';
import { IdentitySession } from '../../core/identity/application/identity-session';
import { IdentityPort } from '../../core/identity/domain/ports/identity-port';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { GlobalAdminPort } from './domain/ports/global-admin-port';
import { GlobalAdminPage } from './global-admin-page';

const globalSession = {
  account_id: 'master',
  roles: ['administrador_global'],
  capabilities: ['accounts.list', 'accounts.read','accounts.update','accounts.roles','accounts.block','publications.list','publications.read','publications.update','publications.moderate','kpi.read','audit.read','audit.export'],
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
    kpis: vi.fn().mockResolvedValue({generated_at:overview.generated_at,period:{from:overview.generated_at,until:overview.generated_at,time_zone:'America/Santiago'},accounts:{activo:2},account_roles:{arrendatario:2},registrations_in_period:1,publication_owner_distribution:[],publications:[],reservations:[],operations:[],finance:[{kind:'cobro',state:'confirmado',count:1,amount_clp:'9007199254740993',source:'fake',definition:'Importe exacto'}],queues:[]}),
    audit: vi.fn().mockResolvedValue({items:[],next_cursor:'',from:overview.generated_at,until:overview.generated_at,generated_at:overview.generated_at,snapshot_cutoff:overview.generated_at,applied_filters:{from:overview.generated_at,until:overview.generated_at,page_size:25}}),
    exportAudit: vi.fn().mockResolvedValue(new Blob(['csv'])),
    list: vi.fn().mockResolvedValue({ items: [account], next_cursor: 'cursor-initial', matching_total: 2, generated_at: overview.generated_at }),
    account: vi.fn().mockResolvedValue(account),
    listAccountResources: vi.fn().mockResolvedValue({items:[],next_cursor:'',matching_total:0,generated_at:overview.generated_at}),
    accountResource: vi.fn().mockResolvedValue({id:'resource-1',state:'pagada'}),
    updateProfile:vi.fn().mockResolvedValue({account_id:'customer',email:account.email,display_name:'Updated customer',phone:'987654321',updated_at:'2026-10-14T01:00:00Z'}),
    updateRole:vi.fn().mockResolvedValue(undefined),
    revokeSessions:vi.fn().mockResolvedValue(2),
    setBlocked:vi.fn().mockResolvedValue(undefined),
    listPublications:vi.fn().mockResolvedValue({items:[{publication_id:'space-1',owner_id:'customer',owner_name:'Private customer',owner_email:account.email,category_code:'oficina',category_name:'Oficina',title:'Test space',description:'d'.repeat(120),area_m2:'20.00',capacity:4,usage_rules:'Reglas',address:'Privada',state:'activa',rate_unit:'hora',base_price_clp:'9007199254740999',updated_at:'2026-10-14T00:00:00Z',publicly_visible:false,public_cover_available:false,attribute_schema_version:1,attributes:{wifi:true},attribute_schema:{category_code:'oficina',schema_version:1,attributes:[{code:'wifi',label:'Wi-Fi',type:'boolean'}]}}],next_cursor:'',matching_total:1,generated_at:overview.generated_at}),
    setPublicationState:vi.fn().mockResolvedValue(undefined),
    updatePublication:vi.fn().mockResolvedValue(undefined),
    publicationGallery:vi.fn().mockResolvedValue([]),
    setPublicationPhotoVisibility:vi.fn().mockResolvedValue(undefined),
    selectPublicationCover:vi.fn().mockResolvedValue(undefined),
    removePublicationPhoto:vi.fn().mockResolvedValue(undefined),
    publicationSchedule:vi.fn().mockResolvedValue({space_id:'space-1',time_zone:'America/Santiago',enabled:false,days:Array.from({length:7},(_,index)=>({weekday:index+1,periods:[]}))}),
    savePublicationSchedule:vi.fn().mockResolvedValue(undefined),
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
    fixture,
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

it('separates global sections and returns to an authorized section when a capability is revoked', async () => {
  const { identity, identityApi, api, component, fixture, setConfirmed } = await setup();
  component.showTab('audit');
  expect(component.activeTab()).toBe('audit');

  setConfirmed({ ...globalSession, capabilities: ['accounts.list', 'publications.list'] });
  await identity.refreshSessionIfCurrent('token-a');
  TestBed.tick();
  fixture.detectChanges();

  expect(component.activeTab()).toBe('accounts');
  const navigation = fixture.nativeElement.querySelector('nav[aria-label="Administración global"]') as HTMLElement;
  expect(navigation.textContent).toContain('Cuentas');
  expect(navigation.textContent).toContain('Publicaciones');
  expect(navigation.textContent).not.toContain('Indicadores');
  expect(navigation.textContent).not.toContain('Auditoría');
  component.showTab('audit');
  expect(component.activeTab()).toBe('accounts');
  expect(api.audit).not.toHaveBeenCalled();
  expect(identityApi.logout).not.toHaveBeenCalled();
});

it('has no WCAG A/AA axe violations across the four global workspaces', async () => {
  const { component, fixture } = await setup();
  const tags = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'];

  for (const section of ['accounts', 'publications', 'kpis', 'audit'] as const) {
    component.showTab(section);
    fixture.detectChanges();
    const result = await axe.run(fixture.nativeElement, { runOnly: { type: 'tag', values: tags } });
    expect(result.violations.map(({ id }) => id), section).toEqual([]);
  }
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
  expect(revoked).toEqual({ status: 'success', value: 2 });
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

it('does not close a reopened publication editor when an earlier save finishes', async () => {
  const { api, component, facade } = await setup();
  component.showTab('publications');
  await vi.waitFor(() => expect(facade.publications()?.items).toHaveLength(1));
  const publication = facade.publications()!.items[0];
  component.editPublication(publication);
  let finish!: () => void;
  api.updatePublication.mockImplementationOnce(() => new Promise<void>(resolve => { finish = resolve; }));
  const pending = component.savePublication();
  component.cancelPublicationEdit();
  component.editPublication(publication);
  finish();
  await pending;
  expect(component.editingPublication()?.publication_id).toBe(publication.publication_id);
});

it('does not reopen gallery A after a pending mutation when gallery B is selected', async () => {
  const { api, component, facade } = await setup();
  component.showTab('publications');
  await vi.waitFor(() => expect(facade.publications()?.items).toHaveLength(1));
  const a = facade.publications()!.items[0];
  const b = { ...a, publication_id: 'space-2', title: 'B' };
  const photo = { id: 'photo-1', fixture_code: 'synthetic-png-v1', mime_type: 'image/png', sha256: 'a'.repeat(64), size_bytes: 100, created_at: '2026-10-14T00:00:00Z', public_in_catalog: true, state: 'activa' };
  api.publicationGallery.mockResolvedValue([photo]);
  component.openPublicationGallery(a);
  await vi.waitFor(() => expect(facade.publicationGallery()).toHaveLength(1));
  let finish!: () => void;
  api.setPublicationPhotoVisibility.mockImplementationOnce(() => new Promise<void>(resolve => { finish = resolve; }));
  const pending = component.changePhotoVisibility(a, photo, false);
  component.openPublicationGallery(b);
  finish();
  await pending;
  expect(component.galleryPublication()?.publication_id).toBe('space-2');
});

it('does not apply a late gallery mutation after closing and reopening the same publication', async () => {
  const { api, component, facade } = await setup();
  component.showTab('publications');
  await vi.waitFor(() => expect(facade.publications()?.items).toHaveLength(1));
  const a = facade.publications()!.items[0];
  const photo = { id: 'photo-1', fixture_code: 'synthetic-png-v1', mime_type: 'image/png', sha256: 'a'.repeat(64), size_bytes: 100, created_at: '2026-10-14T00:00:00Z', public_in_catalog: true, state: 'activa' };
  api.publicationGallery.mockResolvedValue([photo]);
  component.openPublicationGallery(a);
  await vi.waitFor(() => expect(facade.publicationGallery()).toHaveLength(1));
  let finish!: () => void;
  api.selectPublicationCover.mockImplementationOnce(() => new Promise<void>(resolve => { finish = resolve; }));
  const pending = component.selectPhotoCover(a, photo);
  component.closePublicationGallery();
  component.openPublicationGallery(a);
  finish();
  await pending;
  expect(component.galleryPublication()?.publication_id).toBe(a.publication_id);
  expect(api.listPublications).toHaveBeenCalledTimes(1);
});

it('does not let a late schedule save for A replace the schedule panel for B', async () => {
  const { api, component, facade } = await setup();
  component.showTab('publications');
  await vi.waitFor(() => expect(facade.publications()?.items).toHaveLength(1));
  const a = facade.publications()!.items[0];
  const b = { ...a, publication_id: 'space-2', title: 'B' };
  api.publicationSchedule.mockImplementation(async (_token: string, id: string) => ({ space_id: id, time_zone: 'America/Santiago', enabled: false, days: Array.from({ length: 7 }, (_, index) => ({ weekday: index + 1, periods: [] })) }));
  await component.openPublicationSchedule(a);
  let finish!: () => void;
  api.savePublicationSchedule.mockImplementationOnce(() => new Promise<void>(resolve => { finish = resolve; }));
  const pending = component.savePublicationSchedule();
  await component.openPublicationSchedule(b);
  finish();
  await pending;
  expect(component.schedulePublication()?.publication_id).toBe('space-2');
  expect(facade.publicationSchedule()?.space_id).toBe('space-2');
});

it('does not apply an old schedule save after closing and reopening the same publication', async () => {
  const { api, component, facade } = await setup();
  component.showTab('publications');
  await vi.waitFor(() => expect(facade.publications()?.items).toHaveLength(1));
  const item = facade.publications()!.items[0];
  await component.openPublicationSchedule(item);
  let finish!: () => void;
  api.savePublicationSchedule.mockImplementationOnce(() => new Promise<void>(resolve => { finish = resolve; }));
  const pending = component.savePublicationSchedule();
  component.closePublicationSchedule();
  await component.openPublicationSchedule(item);
  finish();
  await pending;
  expect(component.schedulePublication()?.publication_id).toBe(item.publication_id);
  expect(facade.publicationSchedule()?.space_id).toBe(item.publication_id);
  expect(api.listPublications).toHaveBeenCalledTimes(1);
});

it('clears the private publication editor and its fields on real logout', async () => {
  const { identity, component, facade } = await setup();
  component.showTab('publications');
  await vi.waitFor(() => expect(facade.publications()?.items).toHaveLength(1));
  component.editPublication(facade.publications()!.items[0]);
  expect(component.publicationEditForm.controls.description.value).not.toBe('');

  await identity.signOut();
  TestBed.tick();

  expect(component.editingPublication()).toBeNull();
  expect(component.publicationEditForm.getRawValue()).toEqual({
    title: '', description: '', capacity: '', usageRules: '', price: '',
  });
});

it('keeps selected account B when a profile update for A completes later', async () => {
  const { api, component, facade } = await setup();
  await component.open('customer');
  let finish!: (value: Awaited<ReturnType<typeof api.updateProfile>>) => void;
  api.updateProfile.mockImplementationOnce(() => new Promise((resolve) => { finish = resolve; }));
  const update = facade.updateProfile({
    display_name: 'Saved A', phone: null,
    expected_updated_at: account.profile_updated_at,
    reason: 'correccion_perfil',
  });
  api.account.mockResolvedValueOnce({ ...account, account_id: 'customer-b', email: 'b@example.invalid' });
  await component.open('customer-b');

  finish({ account_id: 'customer', email: account.email, display_name: 'Saved A', phone: null, updated_at: '2026-10-14T01:00:00Z' });
  await update;

  expect(facade.selected()?.account_id).toBe('customer-b');
});

it('does not announce an old session revocation in a later login', async () => {
  const { identity, api, component } = await setup();
  await component.open('customer');
  let finish!: (value: number) => void;
  api.revokeSessions.mockImplementationOnce(() => new Promise((resolve) => { finish = resolve; }));
  component.confirmSessionRevocation.set(true);
  const revocation = component.revokeSessions();

  await identity.signOut();
  await identity.signIn('master@example.invalid', 'synthetic-test-only');
  TestBed.tick();
  finish(3);
  await revocation;

  expect(component.notice()).toBe('');
});

it('does not report session revocation success when the API rejects', async () => {
  const { api, component } = await setup();
  await component.open('customer');
  api.revokeSessions.mockRejectedValueOnce(new Error('backend unavailable'));
  component.confirmSessionRevocation.set(true);

  await component.revokeSessions();

  expect(component.notice()).toBe('');
  expect(component.error()).toContain('backend unavailable');
});

it('loads owner-scoped resources and ignores a detail response after changing accounts', async () => {
  const { api, component, facade } = await setup();
  await component.open('customer');
  api.listAccountResources.mockResolvedValueOnce({items:[{id:'reservation-1',kind:'reservation',space_id:'space-1',reservation_id:'reservation-1',state:'pagada',participant_role:'arrendador',occurred_at:overview.generated_at}],next_cursor:'next',matching_total:1,generated_at:overview.generated_at});
  await component.loadAccountResources('reservations');
  expect(facade.accountResources()?.items[0].reservation_id).toBe('reservation-1');

  let finish!: (value: Record<string, unknown>) => void;
  api.accountResource.mockImplementationOnce(() => new Promise((resolve) => { finish = resolve; }));
  const detail = component.openResourceDetail('reservation-1');
  api.account.mockResolvedValueOnce({...account,account_id:'customer-b',email:'b@example.invalid'});
  await component.open('customer-b');
  finish({id:'reservation-1',state:'pagada'});
  await detail;
  expect(facade.accountResourceDetail()).toBeNull();
});

it('loads exact KPI text and exports the applied audit filters at the displayed snapshot', async () => {
  const { api, component, facade, fixture } = await setup();
  component.showTab('kpis');
  await vi.waitFor(() => expect(facade.kpis()?.finance[0].amount_clp).toBe('9007199254740993'));
  fixture.detectChanges();
  expect(fixture.nativeElement.textContent).toContain('9007199254740993');
  expect(fixture.nativeElement.textContent).toContain('Indicadores persistidos');
  expect(fixture.nativeElement.textContent).not.toContain('Auditoría global');
  component.showTab('audit');
  fixture.detectChanges();
  expect(fixture.nativeElement.textContent).toContain('Auditoría global');
  expect(fixture.nativeElement.textContent).not.toContain('Indicadores persistidos');
  component.searchAudit();
  const applied = component.appliedAudit()!;
  await vi.waitFor(() => expect(facade.auditPage()).not.toBeNull());
  component.auditActor.setValue('changed-after-apply');
  const create = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:gadm-audit');
  const revoke = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => undefined);
  const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined);
  await component.downloadAudit('csv');
  expect(api.audit).toHaveBeenCalledWith('token-a', expect.objectContaining({ actor_id: undefined, target_account_id: undefined }));
  expect(api.exportAudit).toHaveBeenCalledWith('token-a', expect.objectContaining({ ...applied, snapshot_cutoff: overview.generated_at }), 'csv');
  expect(create).toHaveBeenCalledOnce();
  expect(click).toHaveBeenCalledOnce();
  expect(revoke).toHaveBeenCalledWith('blob:gadm-audit');
});

it('discards an export response after a newer audit query is applied', async () => {
  const { api, component, facade } = await setup();
  component.showTab('audit');
  component.searchAudit();
  const filters = { ...component.appliedAudit()!, snapshot_cutoff: overview.generated_at };
  let fail!: (error: unknown) => void;
  api.exportAudit.mockImplementationOnce(() => new Promise((_resolve, reject) => { fail = reject; }));
  const staleExport = facade.exportAudit(filters, 'json');
  component.searchAudit();
  await vi.waitFor(() => expect(facade.auditPage()).not.toBeNull());
  fail(new Error('private upstream details'));
  expect(await staleExport).toBeNull();
  expect(facade.exportError()).toBe('');
});

it('shows contextual report errors and clears each after a successful retry', async () => {
  const { api, component, fixture, facade } = await setup();
  component.showTab('kpis');
  api.kpis.mockRejectedValueOnce({ status: 401, message: 'private server detail' });
  component.searchKPIs();
  await vi.waitFor(() => expect(facade.reportError()).toContain('sesión'));
  fixture.detectChanges();
  expect(fixture.nativeElement.querySelector('[aria-labelledby="report-title"] [role="alert"]')?.textContent).toContain('Inicia sesión');
  expect(fixture.nativeElement.textContent).not.toContain('private server detail');

  component.searchKPIs();
  await vi.waitFor(() => expect(facade.kpis()).not.toBeNull());
  expect(facade.reportError()).toBe('');

  api.audit.mockRejectedValueOnce({ status: 403, message: 'private permission detail' });
  component.showTab('audit');
  component.searchAudit();
  await vi.waitFor(() => expect(facade.auditError()).toContain('permisos se revalidaron'));
  expect(facade.error()).toBe('');
  fixture.detectChanges();
  expect(fixture.nativeElement.querySelector('h2 + [role="alert"]')?.textContent).toContain('auditoría');
  expect(fixture.nativeElement.textContent).not.toContain('private permission detail');

  component.auditFrom.setValue('2026-10-08T00:00');
  component.auditUntil.setValue('2026-10-15T00:00');
  component.searchAudit();
  await vi.waitFor(() => expect(facade.auditPage()).not.toBeNull());
  expect(facade.auditError()).toBe('');

  api.exportAudit.mockRejectedValueOnce(new Error('private export details'));
  await component.downloadAudit('csv');
  fixture.detectChanges();
  expect(facade.exportError()).toContain('No se pudo exportar');
  expect(fixture.nativeElement.textContent).toContain('Reintenta con el mismo corte');
  expect(fixture.nativeElement.textContent).not.toContain('private export details');

  await component.downloadAudit('csv');
  expect(facade.exportError()).toBe('');
});

it('does not publish an export denial after its audit session changes', async () => {
  const { identity, api, component, facade } = await setup();
  component.showTab('audit');
  component.searchAudit();
  await vi.waitFor(() => expect(facade.auditPage()).not.toBeNull());
  let fail!: (error: unknown) => void;
  api.exportAudit.mockImplementationOnce(() => new Promise((_resolve, reject) => { fail = reject; }));
  const filters = { ...component.appliedAudit()!, snapshot_cutoff: overview.generated_at };
  const pending = facade.exportAudit(filters, 'csv');
  await identity.signOut();
  fail({ status: 403 });
  expect(await pending).toBeNull();
  expect(facade.exportError()).toBe('');
});

it('shows a current export 403 in audit and permits retry after reloading the audit page', async () => {
  const { api, component, fixture, facade } = await setup();
  component.showTab('audit');
  component.searchAudit();
  await vi.waitFor(() => expect(facade.auditPage()).not.toBeNull());
  api.exportAudit.mockRejectedValueOnce({ status: 403, message: 'private denial detail' });

  await component.downloadAudit('json');
  fixture.detectChanges();
  expect(facade.exportError()).toContain('acceso a la exportación fue rechazado');
  expect(fixture.nativeElement.textContent).not.toContain('private denial detail');
  expect(fixture.nativeElement.textContent).toContain('reintenta con el mismo corte');

  component.auditFrom.setValue('2026-10-08T00:00');
  component.auditUntil.setValue('2026-10-15T00:00');
  component.searchAudit();
  await vi.waitFor(() => expect(facade.auditPage()).not.toBeNull());
  await component.downloadAudit('json');
  expect(facade.exportError()).toBe('');
});
