import { provideHttpClient } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, Router } from '@angular/router';
import { BehaviorSubject, of } from 'rxjs';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { parseExactClpJson, stringifyExactClpJson } from '../../core/exact-json';
import { IdentitySession } from '../../core/identity/application/identity-session';
import { IdentityApi } from '../../core/identity/infrastructure/identity-api';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { provideFrontendPorts } from '../../frontend.providers';
import { zonedIsoLabel, zonedLocalToRfc3339 } from './calendar-time';
import { HostingSpacesPage } from './hosting-spaces-page';
import { HostingApi, HostingApiError } from './infrastructure/hosting-api';

const ownerSession = { account_id: 'owner-a', roles: ['arrendador'], capabilities: [], restricted_mode: false };
const login = (account_id: string, access_token: string) => ({
  account_id,
  roles: ['arrendador'],
  access_token,
  token_type: 'Bearer',
  expires_at: '2027-01-01T00:00:00Z',
  capabilities: [],
  restricted_mode: false,
});
const validDescription = 'Un espacio sintético, cómodo y bien equipado para trabajar, reunirse y recibir visitas durante toda la jornada. '.repeat(2);
const space = (id = 'space-a', state: 'borrador' | 'activa' | 'oculta' = 'borrador') => ({
  id,
  category_code: 'oficina',
  category_name: 'Oficina',
  title: 'Sala de trabajo',
  description: validDescription,
  area_m2: 30,
  capacity: 6,
  usage_rules: 'No fumar',
  rate_unit: 'hora' as const,
  base_price_clp: '8000',
  address: 'Dirección privada A',
  state,
  time_zone: 'America/Santiago',
  attribute_schema_version: 2,
  attributes: { estacionamiento: true },
});

function apiFor(base = space()) {
  return {
    list: vi.fn(async () => ({ items: [base] })),
    categories: vi.fn(async () => ({ items: [{ code: 'oficina', name: 'Oficina' }] })),
    publicLocations: vi.fn(async () => ({ items: [
      { region_code: '13', region_name: 'Metropolitana de Santiago', comuna_code: '13123', comuna_name: 'Providencia' },
      { region_code: '13', region_name: 'Metropolitana de Santiago', comuna_code: '13101', comuna_name: 'Santiago' },
      { region_code: '06', region_name: "O’Higgins", comuna_code: '06101', comuna_name: 'Rancagua' },
    ] })),
    profile: vi.fn(async (_token: string, category: string, version?: number) => ({
      category_code: category,
      schema_version: version ?? 2,
      attributes: [
        {
          code: 'estacionamiento',
          label: 'Estacionamiento',
          type: 'boolean' as const,
          order: 1,
          required: false,
          filter_candidate: true,
        },
      ],
    })),
    detail: vi.fn(async () => base),
    create: vi.fn(async (_token: string, payload: unknown) => ({
      ...base,
      ...(payload as object),
    })),
    update: vi.fn(async (_token: string, _id: string, payload: unknown) => ({
      ...base,
      ...(payload as object),
    })),
    updatePublished: vi.fn(async (_token: string, _id: string, payload: unknown) => ({
      ...base,
      ...(payload as object),
    })),
    publication: vi.fn(
      async (_token: string, _id: string, payload: { state: 'activa' | 'oculta' }) => ({
        ...base,
        state: payload.state,
      }),
    ),
    remove: vi.fn(async () => ({ files_pending_cleanup: 0 })),
    gallery: vi.fn(async () => ({ items: [], max_photos: 10 as const, synthetic: true as const })),
    setPhotoPublic: vi.fn(async (_token: string, _spaceId: string, photoId: string, public_in_catalog: boolean) => ({
      id: photoId, public_in_catalog,
    } as never)),
    addPhoto: vi.fn(async (_token: string, _spaceId: string, _key: string) => ({
      id: 'photo-1',
      fixture_code: 'synthetic-png-v1' as const,
      mime_type: 'image/png' as const,
      sha256: 'abc',
      size_bytes: 12,
      created_at: '2026-10-12T00:00:00Z',
    })),
    cover: vi.fn(async (_token: string, space_id: string, photo_id: string) => ({
      space_id,
      photo_id,
      selected: true as const,
    })),
    removePhoto: vi.fn(async () => ({
      photo_id: 'photo-1',
      removed: true as const,
      reused: false,
    })),
    uploadPhoto: vi.fn(async (_token: string, _spaceId: string, _file: File, _key: string, progress: (value: number | null) => void) => {
      progress(65);
      return {
        id: 'media-photo-1', space_id: base.id, alt_text: '', visible: false, cover: false,
        position: 1, state: 'ready' as const, created_at: '2026-10-22T00:00:00Z', variants: [],
      };
    }),
    updateMediaPhoto: vi.fn(async (_token: string, _spaceId: string, photoId: string, update: object) => ({
      id: photoId, space_id: base.id, alt_text: '', visible: false, cover: false,
      position: 1, state: 'ready' as const, created_at: '2026-10-22T00:00:00Z', variants: [], ...update,
    })),
    mediaVariant: vi.fn(async () => new Blob(['webp'], { type: 'image/webp' })),
    photoContent: vi.fn(async () => new Blob(['png'], { type: 'image/png' })),
    timeZone: vi.fn(async () => ({ space_id: base.id, time_zone: 'America/Santiago' })),
    configureTimeZone: vi.fn(async (_token: string, space_id: string, time_zone: string) => ({
      space_id,
      time_zone,
    })),
    checkAvailability: vi.fn(async () => ({
      space_id: base.id,
      time_zone: 'America/Santiago',
      start_at: '2026-10-15T12:00:00Z',
      end_at: '2026-10-15T13:00:00Z',
      available: true,
    })),
    tariff: vi.fn(async () => ({
      space_id: base.id,
      version: 1,
      rate_unit: 'hora' as 'hora' | 'dia' | 'mes',
      base_price: '8000',
      currency: 'CLP' as const,
      created_at: '2026-10-12T00:00:00Z',
    })),
    updateTariff: vi.fn(async () => ({
      space_id: base.id,
      version: 2,
      rate_unit: 'hora' as const,
      base_price: '8000',
      currency: 'CLP' as const,
      created_at: '2026-10-12T00:00:00Z',
    })),
    tariffVersions: vi.fn(async () => ({ items: [] })),
    simulate: vi.fn(async () => ({
      id: 'sim-1',
      space_id: base.id,
      rate_version: 1,
      rate_unit: 'hora' as const,
      base_price: '8000',
      billed_units: 2,
      currency: 'CLP' as const,
      base_subtotal: '16000',
      start_at: '2026-10-15T12:00:00Z',
      end_at: '2026-10-15T14:00:00Z',
      time_zone: 'America/Santiago',
      private: true as const,
      created_at: '2026-10-12T00:00:00Z',
    })),
    blocks: vi.fn(async () => ({ space_id: base.id, time_zone: 'America/Santiago', items: [] })),
    addBlock: vi.fn(async () => ({
      id: 'block-1',
      space_id: base.id,
      start_at: '2026-10-15T12:00:00Z',
      end_at: '2026-10-15T13:00:00Z',
      time_zone: 'America/Santiago',
      reason: 'bloqueo_anfitrion',
      created_at: '2026-10-12T00:00:00Z',
    })),
    removeBlock: vi.fn(async () => undefined),
    weekly: vi.fn(async () => ({
      safety_notice: 'ENSAYO LOCAL — SIN COBRO REAL' as const,
      data: {
        space_id: base.id,
        enabled: false,
        time_zone: 'America/Santiago',
        days: Array.from({ length: 7 }, (_, index) => ({
          weekday: index + 1,
          periods: [],
        })) as unknown as [
          { weekday: number; periods: [] },
          { weekday: number; periods: [] },
          { weekday: number; periods: [] },
          { weekday: number; periods: [] },
          { weekday: number; periods: [] },
          { weekday: number; periods: [] },
          { weekday: number; periods: [] },
        ],
      },
    })),
    saveWeekly: vi.fn(
      async (_token: string, _id: string, payload: { enabled: boolean; days: never }) => ({
        safety_notice: 'ENSAYO LOCAL — SIN COBRO REAL' as const,
        data: {
          space_id: base.id,
          enabled: payload.enabled,
          time_zone: 'America/Santiago',
          days: payload.days,
        },
      }),
    ),
  };
}

async function populateDraft(page: HostingSpacesPage, title: string): Promise<void> {
  await page.categoryChanged('oficina');
  Object.assign(page.form, {
    category_code: 'oficina',
    title,
    description: validDescription,
    area_m2: 30,
    capacity: 6,
    usage_rules: 'No fumar y respetar el horario acordado.',
    rate_unit: 'hora',
    base_price_clp: '8000',
    address: 'Dirección privada sintética 123',
    region_code: '',
    comuna_code: '',
    attributes: {},
  });
}

function setup(
  mockApi: ReturnType<typeof apiFor>,
  routeId: string | null = null,
  identityApi = defaultIdentityApi(),
) {
  const params = new BehaviorSubject(MapParamMap(routeId));
  TestBed.configureTestingModule({
    imports: [HostingSpacesPage],
    providers: [
      ...provideFrontendPorts(),
      provideHttpClient(),
      PrivateSessionState,
      { provide: Router, useValue: { navigate: vi.fn(async () => true) } },
      {
        provide: ActivatedRoute,
        useValue: { paramMap: params.asObservable(), data: of({ section: 'datos' }) },
      },
      { provide: IdentityApi, useValue: identityApi },
      { provide: HostingApi, useValue: mockApi },
    ],
  });
  return { params, identity: TestBed.inject(IdentitySession) };
}
function defaultIdentityApi() {
  return {
    login: vi
      .fn()
      .mockResolvedValueOnce(login('owner-a', 'bearer-a'))
      .mockResolvedValueOnce(login('owner-a', 'bearer-a2'))
      .mockResolvedValueOnce(login('owner-b', 'bearer-b')),
    session: vi.fn().mockResolvedValue(ownerSession),
    logout: vi.fn().mockResolvedValue(undefined),
  };
}
function MapParamMap(id: string | null) {
  return { get: (key: string) => (key === 'spaceId' ? id : null) };
}
const page = (fixture: ReturnType<typeof TestBed.createComponent>) =>
  fixture.componentInstance as unknown as HostingSpacesPage;
async function flush(): Promise<void> {
  for (let i = 0; i < 8; i++) await Promise.resolve();
}
function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
}

describe('FE-06 HostingSpacesPage', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    TestBed.resetTestingModule();
  });

  it('replaces a dynamic attribute control when the Go profile changes its type', async () => {
    const { identity } = setup(apiFor());
    await identity.signIn('owner@example.invalid', 'secret');
    const fixture = TestBed.createComponent(HostingSpacesPage);
    fixture.detectChanges();
    await flush();
    const numeric = page(fixture).attributeControl('dynamic', 'number');
    numeric.setValue('3');
    expect(page(fixture).form.attributes['dynamic']).toBe(3);
    const boolean = page(fixture).attributeControl('dynamic', 'boolean');
    expect(boolean).not.toBe(numeric);
    expect(page(fixture).attributeControls.get('dynamic')).toBe(boolean);
    boolean.setValue('true');
    expect(page(fixture).form.attributes['dynamic']).toBe(true);
    fixture.destroy();
  });

  it('loads owner spaces with the server-selected category profile version and hides no server state', async () => {
    const api = apiFor(space('space-a', 'oculta'));
    const { identity } = setup(api);
    await identity.signIn('owner@example.invalid', 'secret');
    const fixture = TestBed.createComponent(HostingSpacesPage);
    fixture.detectChanges();
    await flush();
    expect(api.profile).toHaveBeenCalledWith('bearer-a', 'oficina', 2);
    expect(page(fixture).selectedSpace()?.state).toBe('oculta');
    fixture.destroy();
  });

  it('presents the selected space with its current server state and preserves section navigation', async () => {
    const api = apiFor(space('synthetic-space-a', 'borrador'));
    const { identity } = setup(api);
    await identity.signIn('owner@example.invalid', 'synthetic-secret');
    const router = TestBed.inject(Router);
    const fixture = TestBed.createComponent(HostingSpacesPage);
    fixture.detectChanges();
    await flush();
    fixture.detectChanges();

    const root = fixture.nativeElement as HTMLElement;
    expect(root.querySelector('h1')?.textContent).toContain('Mis espacios');
    expect(root.querySelector('.page-subtitle')?.textContent).toContain('con el próximo paso a la vista');
    expect(root.querySelector('.space-choice[aria-current="true"]')?.textContent).toContain('Sala de trabajo');
    expect(root.querySelector('.space-choice[aria-current="true"]')?.textContent).toContain('Borrador');
    expect(root.querySelector('.resource-context')?.textContent).not.toContain('synthetic-space-a');
    expect(root.querySelector('.next-step h3')?.textContent).toContain('Continúa con los datos del borrador');
    const sections = [...root.querySelectorAll<HTMLButtonElement>('.section-tabs button')];
    expect(sections.map((button) => button.textContent?.trim())).toEqual(['Datos', 'Tarifas', 'Calendario', 'Galería']);
    expect(root.querySelector('.section-tabs button[aria-current="page"]')?.textContent).toContain('Datos');
    sections.forEach((button) => button.click());
    expect(router.navigate).toHaveBeenNthCalledWith(1, ['/panel/espacios', 'synthetic-space-a', 'editar']);
    expect(router.navigate).toHaveBeenNthCalledWith(2, ['/panel/espacios', 'synthetic-space-a', 'tarifas']);
    expect(router.navigate).toHaveBeenNthCalledWith(3, ['/panel/espacios', 'synthetic-space-a', 'calendario']);
    expect(router.navigate).toHaveBeenNthCalledWith(4, ['/panel/espacios', 'synthetic-space-a', 'galeria']);
    fixture.destroy();
  });

  it('offers a retry when the owner-space list fails instead of presenting a false empty state', async () => {
    const api = apiFor();
    api.list.mockRejectedValueOnce(new Error('synthetic unavailable'));
    const { identity } = setup(api);
    await identity.signIn('owner@example.invalid', 'synthetic-secret');
    const fixture = TestBed.createComponent(HostingSpacesPage);
    fixture.detectChanges();
    await flush();
    fixture.detectChanges();

    const root = fixture.nativeElement as HTMLElement;
    expect(root.querySelector('[role="alert"]')?.textContent).toContain('No fue posible cargar tus espacios');
    expect(root.querySelector('.space-list-empty')).toBeNull();
    (root.querySelector('.load-error button') as HTMLButtonElement).click();
    await flush();
    fixture.detectChanges();

    expect(api.list).toHaveBeenCalledTimes(2);
    expect(root.querySelector('[role="alert"]')).toBeNull();
    expect(root.querySelector('.space-choice')).toBeTruthy();
    fixture.destroy();
  });

  it('discards a stale public-location response after the authenticated account changes', async () => {
    const api = apiFor();
    const first = deferred<{ items: { region_code: string; region_name: string; comuna_code: string; comuna_name: string }[] }>();
    const second = deferred<{ items: { region_code: string; region_name: string; comuna_code: string; comuna_name: string }[] }>();
    api.publicLocations.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise);
    const { identity } = setup(api);
    await identity.signIn('owner-a@example.invalid', 'secret');
    const fixture = TestBed.createComponent(HostingSpacesPage);
    fixture.detectChanges();
    await flush();
    await identity.signIn('owner-b@example.invalid', 'secret');
    fixture.detectChanges();
    await flush();
    second.resolve({ items: [{ region_code: '06', region_name: 'O’Higgins', comuna_code: '06101', comuna_name: 'Rancagua' }] });
    await flush();
    first.resolve({ items: [{ region_code: '13', region_name: 'Metropolitana', comuna_code: '13123', comuna_name: 'Providencia' }] });
    await flush();
    expect(page(fixture).publicLocations().map(location => location.region_code)).toEqual(['06']);
    fixture.destroy();
  });

  it('releases the create operation after its own confirmed detail reload', async () => {
    const api = apiFor();
    api.list.mockResolvedValue({ items: [] });
    const { identity } = setup(api);
    await identity.signIn('owner@example.invalid', 'secret');
    const fixture = TestBed.createComponent(HostingSpacesPage);
    fixture.detectChanges();
    await flush();
    await populateDraft(page(fixture), 'Borrador nuevo');
    Object.assign(page(fixture).form, {
      region_code: '13',
      comuna_code: '13123',
    });
    await page(fixture).createDraft();
    expect(api.create).toHaveBeenCalledOnce();
    expect(api.create.mock.calls[0][1]).toMatchObject({ region_code: '13', comuna_code: '13123' });
    expect(page(fixture).selectedSpace()?.id).toBe('space-a');
    expect(page(fixture).detailLoading()).toBe(false);
    expect(page(fixture).saving()).toBe(false);
    fixture.destroy();
  });

  it('shows accessible Go field errors and preserves completed draft values after 422', async () => {
    const api = apiFor();
    api.list.mockResolvedValue({ items: [] });
    api.create.mockRejectedValueOnce({
      status: 422,
      error: JSON.stringify({
        error: {
          code: 'validation_error',
          message: 'Revisa los campos obligatorios y sus límites.',
          details: [{ field: 'description', code: 'min_length' }],
        },
      }),
    });
    const { identity } = setup(api);
    await identity.signIn('owner@example.invalid', 'secret');
    const fixture = TestBed.createComponent(HostingSpacesPage);
    fixture.detectChanges();
    await flush();
    const component = page(fixture);
    const controls = component.draftForm.controls;
    controls.category_code.setValue('oficina');
    await flush();
    controls.title.setValue('Borrador completo');
    controls.description.setValue(validDescription);
    controls.area_m2.setValue(30);
    controls.capacity.setValue(6);
    controls.usage_rules.setValue('No fumar y respetar el horario acordado.');
    controls.rate_unit.setValue('hora');
    controls.base_price_clp.setValue('8000');
    controls.address.setValue('Dirección privada sintética 123');
    await component.submitDraft(new Event('submit', { cancelable: true }));
    await flush();
    await fixture.whenStable();
    expect(api.create).toHaveBeenCalledOnce();
    expect(component.form.title).toBe('Borrador completo');
    expect(component.form.description).toBe(validDescription);
    expect(component.fieldErrors()['description']).toContain('100 caracteres');
    const summary = fixture.nativeElement.querySelector('#draft-error-summary') as HTMLElement | null;
    expect(summary?.getAttribute('role')).toBe('alert');
    expect(fixture.nativeElement.querySelector('[name="description"]')?.getAttribute('aria-invalid')).toBe('true');
    expect(fixture.nativeElement.querySelector('[name="description"]')?.getAttribute('aria-describedby')).toBe('draft-error-summary');
    expect(summary?.textContent).toContain('Descripción');
    fixture.destroy();
  });

  it('matches Go draft limits and distinguishes optional profile values from false', async () => {
    const api = apiFor();
    api.profile.mockResolvedValue({
      category_code: 'oficina',
      schema_version: 2,
      attributes: [{
        code: 'estacionamiento', label: 'Estacionamiento', type: 'boolean', order: 1,
        filter_candidate: true, required: true,
      }],
    });
    const { identity } = setup(api);
    await identity.signIn('owner@example.invalid', 'secret');
    const fixture = TestBed.createComponent(HostingSpacesPage);
    fixture.detectChanges();
    await flush();
    const component = page(fixture);
    await component.categoryChanged('oficina');
    const attribute = component.attributeControl('estacionamiento', 'boolean');
    expect(attribute.hasError('required')).toBe(true);
    component.setAttribute('estacionamiento', 'false', 'boolean');
    expect(component.form.attributes['estacionamiento']).toBe(false);
    component.setAttribute('estacionamiento', '', 'boolean');
    expect(Object.hasOwn(component.form.attributes, 'estacionamiento')).toBe(false);

    const controls = component.draftForm.controls;
    controls.title.setValue('x'.repeat(71));
    controls.description.setValue('corto');
    controls.area_m2.setValue(12.345);
    controls.capacity.setValue(0);
    controls.usage_rules.setValue('x'.repeat(251));
    controls.base_price_clp.setValue('5000');
    controls.address.setValue('x'.repeat(501));
    component.draftAttempted.set(true);
    component.draftForm.markAllAsTouched();
    const invalid = new Set(component.draftIssues().map((issue) => issue.field));
    for (const field of ['title', 'description', 'area_m2', 'capacity', 'usage_rules', 'base_price_clp', 'address', 'attributes.estacionamiento']) {
      expect(invalid.has(field), field).toBe(true);
    }
    await component.submitDraft(new Event('submit', { cancelable: true }));
    expect(api.create).not.toHaveBeenCalled();
    fixture.destroy();
  });

  it.each([{ initialRoles: [] }, { initialRoles: ['arrendatario'] }])('refreshes business roles %j after the first draft before entering the editor', async ({ initialRoles }) => {
    const api = apiFor();
    api.list.mockResolvedValue({ items: [] });
    const identityApi = defaultIdentityApi();
    identityApi.session
      .mockReset()
      .mockResolvedValueOnce({ ...ownerSession, roles: initialRoles })
      .mockResolvedValueOnce({ ...ownerSession, roles: [...initialRoles, 'arrendador'] });
    const { identity } = setup(api, null, identityApi);
    await identity.signIn('owner@example.invalid', 'secret');
    expect(identity.session()?.roles).toEqual(initialRoles);

    const fixture = TestBed.createComponent(HostingSpacesPage);
    fixture.detectChanges();
    await flush();
    await populateDraft(page(fixture), 'Primer borrador');

    await page(fixture).createDraft();

    expect(identityApi.session).toHaveBeenCalledTimes(2);
    expect(identity.session()?.roles).toEqual([...initialRoles, 'arrendador']);
    expect(TestBed.inject(Router).navigate).toHaveBeenCalledWith([
      '/panel/espacios',
      'space-a',
      'editar',
    ]);
    fixture.destroy();
  });

  it('does not let a stale create completion unlock a newer write after switching spaces', async () => {
    const api = apiFor();
    api.list.mockResolvedValue({ items: [] });
    const create = deferred<ReturnType<typeof space>>();
    const update = deferred<ReturnType<typeof space>>();
    api.create.mockReturnValue(create.promise);
    api.detail.mockResolvedValue(space('space-b', 'activa'));
    api.updatePublished.mockReturnValue(update.promise);
    const { identity } = setup(api);
    await identity.signIn('owner@example.invalid', 'secret');
    const fixture = TestBed.createComponent(HostingSpacesPage);
    fixture.detectChanges();
    await flush();
    await populateDraft(page(fixture), 'Borrador A');
    const pendingCreate = page(fixture).createDraft();
    await flush();
    const loadingB = page(fixture).loadSpace('space-b');
    await loadingB;
    page(fixture).publishedForm.title = 'Título de B';
    const pendingWrite = page(fixture).savePublishedContent();
    create.resolve(space('space-a'));
    await pendingCreate;
    expect(page(fixture).saving()).toBe(true);
    update.resolve({ ...space('space-b', 'activa'), title: 'Título de B' });
    await pendingWrite;
    expect(page(fixture).saving()).toBe(false);
    fixture.destroy();
  });

  it('keeps third-party drafts private when Go rejects the owner-only detail', async () => {
    const api = apiFor();
    api.list.mockResolvedValue({ items: [] });
    api.detail.mockRejectedValue(new HostingApiError('No visible', 404));
    const idApi = {
      ...defaultIdentityApi(),
      login: vi.fn().mockResolvedValue(login('other', 'bearer-other')),
      session: vi
        .fn()
        .mockResolvedValue({ account_id: 'other', roles: ['arrendador'], capabilities: [], restricted_mode: false }),
    };
    setup(api, 'secret-space-id', idApi);
    await TestBed.inject(IdentitySession).signIn('other@example.invalid', 'secret');
    const fixture = TestBed.createComponent(HostingSpacesPage);
    fixture.detectChanges();
    await flush();
    expect(api.detail).toHaveBeenCalledWith('bearer-other', 'secret-space-id');
    expect(page(fixture).selectedSpace()).toBeNull();
    expect(page(fixture).error()).toContain('no encontró');
    fixture.destroy();
  });

  it('sends only modified published fields and leaves the tariff untouched', async () => {
    const api = apiFor(space('space-a', 'activa'));
    const { identity } = setup(api);
    await identity.signIn('owner@example.invalid', 'secret');
    const fixture = TestBed.createComponent(HostingSpacesPage);
    fixture.detectChanges();
    await flush();
    page(fixture).publishedForm.title = 'Título nuevo';
    await page(fixture).savePublishedContent();
    expect(api.updatePublished).toHaveBeenCalledWith('bearer-a', 'space-a', {
      title: 'Título nuevo',
    });
    expect(page(fixture).selectedSpace()?.base_price_clp).toBe('8000');
    fixture.destroy();
  });

  it('clears a published field error when that field is edited', async () => {
    const api = apiFor(space('space-a', 'activa'));
    api.updatePublished.mockRejectedValueOnce({
      status: 422,
      error: { error: { code: 'validation_error', details: [{ field: 'title', code: 'max_length' }] } },
    });
    const { identity } = setup(api);
    await identity.signIn('owner@example.invalid', 'secret');
    const fixture = TestBed.createComponent(HostingSpacesPage);
    fixture.detectChanges();
    await flush();
    const component = page(fixture);
    component.publishedForm.title = 'Título demasiado largo';
    await component.savePublishedContent();
    expect(component.fieldErrors()['title']).toContain('70 caracteres');
    component.publishedEditorForm.controls.title.setValue('Título corregido');
    expect(component.fieldErrors()['title']).toBeUndefined();
    fixture.destroy();
  });

  it('saves the public location for a published space and clears the commune when the region changes', async () => {
    const api = apiFor(space('space-a', 'activa'));
    const { identity } = setup(api);
    await identity.signIn('owner@example.invalid', 'secret');
    const fixture = TestBed.createComponent(HostingSpacesPage);
    fixture.detectChanges();
    await flush();
    page(fixture).form.region_code = '13';
    page(fixture).form.comuna_code = '13123';
    await page(fixture).savePublishedContent();
    expect(api.updatePublished).toHaveBeenCalledWith('bearer-a', 'space-a', {
      region_code: '13',
      comuna_code: '13123',
    });
    page(fixture).form.region_code = '06';
    page(fixture).form.comuna_code = '06101';
    page(fixture).regionChanged('13');
    expect(page(fixture).form.comuna_code).toBe('');
    const regionControl = page(fixture).controlFor(page(fixture).form, 'region_code', page(fixture).regionChanged);
    const communeControl = page(fixture).controlFor(page(fixture).form, 'comuna_code');
    communeControl.setValue('13123');
    regionControl.setValue('06');
    expect(page(fixture).form.comuna_code).toBe('');
    fixture.detectChanges();
    expect(communeControl.value).toBe('');
    fixture.destroy();
  });

  it('does not submit an incomplete public region/commune pair', async () => {
    const api = apiFor(space('space-a', 'activa'));
    const { identity } = setup(api);
    await identity.signIn('owner@example.invalid', 'secret');
    const fixture = TestBed.createComponent(HostingSpacesPage);
    fixture.detectChanges();
    await flush();
    page(fixture).form.region_code = '13';
    page(fixture).form.comuna_code = '';
    await page(fixture).savePublishedContent();
    expect(api.updatePublished).not.toHaveBeenCalled();
    expect(page(fixture).error()).toContain('Selecciona una comuna');
    fixture.destroy();
  });

  it('preserves the latest exact tariff and every unmodified draft field in the required full PUT', async () => {
    const api = apiFor();
    const exactPrice = '9007199254740993';
    api.tariff.mockResolvedValue({
      space_id: 'space-a',
      version: 9,
      rate_unit: 'mes',
      base_price: exactPrice,
      currency: 'CLP',
      created_at: '2026-10-12T00:00:00Z',
    });
    const { identity } = setup(api);
    await identity.signIn('owner@example.invalid', 'secret');
    const fixture = TestBed.createComponent(HostingSpacesPage);
    fixture.detectChanges();
    await flush();
    api.detail.mockResolvedValue({
      ...space(),
      title: 'Título actualizado desde Go',
      description: 'Descripción confirmada remota',
    });
    const component = page(fixture);
    component.conflict.set('Go detectó un conflicto con el estado, la elegibilidad o el calendario vigente.');
    fixture.detectChanges();
    expect((fixture.nativeElement as HTMLElement).textContent).toContain('Go detectó un conflicto');
    page(fixture).form.description = validDescription.replace('Un espacio sintético', 'Descripción modificada');
    await page(fixture).saveDraft();
    fixture.detectChanges();
    expect(component.notice()).toBe('Cambios del borrador confirmados por Go.');
    expect(component.conflict()).toBe('');
    expect((fixture.nativeElement as HTMLElement).textContent).not.toContain('Go detectó un conflicto');
    const payload = api.update.mock.calls[0][2] as {
      base_price_clp: string;
      title: string;
      description: string;
    };
    expect(payload.base_price_clp).toBe(exactPrice);
    expect(payload.title).toBe('Título actualizado desde Go');
    expect(payload.description).toBe(validDescription.replace('Un espacio sintético', 'Descripción modificada'));
    fixture.destroy();
  });

  it('merges public location as a pair while retaining remote fields', async () => {
    const scenarios = [
      {
        name: 'only commune changed locally',
        original: ['13', '13123'],
        edited: ['13', '13101'],
        latest: ['06', '06101'],
        expected: ['13', '13101'],
      },
      {
        name: 'selection from empty location',
        original: ['', ''],
        edited: ['13', '13123'],
        latest: ['06', '06101'],
        expected: ['13', '13123'],
      },
      {
        name: 'location retired locally',
        original: ['13', '13123'],
        edited: ['', ''],
        latest: ['06', '06101'],
        expected: ['', ''],
      },
      {
        name: 'remote location retained without local edit',
        original: ['13', '13123'],
        edited: ['13', '13123'],
        latest: ['06', '06101'],
        expected: ['06', '06101'],
      },
    ] as const;

    for (const scenario of scenarios) {
      const [originalRegion, originalCommune] = scenario.original;
      const [editedRegion, editedCommune] = scenario.edited;
      const [latestRegion, latestCommune] = scenario.latest;
      const originalSpace = {
        ...space(),
        region_code: originalRegion || undefined,
        comuna_code: originalCommune || undefined,
      };
      const api = apiFor(originalSpace);
      const { identity } = setup(api);
      await identity.signIn('owner@example.invalid', 'secret');
      const fixture = TestBed.createComponent(HostingSpacesPage);
      fixture.detectChanges();
      await flush();
      Object.assign(page(fixture).form, {
        region_code: editedRegion,
        comuna_code: editedCommune,
      });
      api.detail.mockResolvedValue({
        ...originalSpace,
        title: 'Título remoto conservado',
        region_code: latestRegion || undefined,
        comuna_code: latestCommune || undefined,
      } as ReturnType<typeof space>);

      await page(fixture).saveDraft();

      const payload = api.update.mock.calls[0][2] as {
        title: string;
        region_code?: string;
        comuna_code?: string;
      };
      expect([payload.region_code ?? '', payload.comuna_code ?? ''], scenario.name).toEqual(
        [...scenario.expected],
      );
      expect(payload.title, scenario.name).toBe('Título remoto conservado');
      fixture.destroy();
      TestBed.resetTestingModule();
    }
  });

  it('does not let an older space read overwrite the newer selected space', async () => {
    const api = apiFor(space('space-a', 'activa'));
    const { identity } = setup(api);
    await identity.signIn('owner@example.invalid', 'secret');
    const fixture = TestBed.createComponent(HostingSpacesPage);
    fixture.detectChanges();
    await flush();
    let finishOld!: (value: ReturnType<typeof space>) => void;
    api.detail.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          finishOld = resolve;
        }),
    );
    const oldRead = page(fixture).loadSpace('space-a');
    await flush();
    api.detail.mockResolvedValueOnce(space('space-b', 'borrador'));
    await page(fixture).loadSpace('space-b');
    finishOld(space('space-a', 'activa'));
    await oldRead;
    expect(page(fixture).selectedSpace()?.id).toBe('space-b');
    fixture.destroy();
  });

  it('clears space A state before loading B and keeps partial 404 resources empty', async () => {
    const api = apiFor(space('space-a', 'activa'));
    const { identity } = setup(api);
    await identity.signIn('owner@example.invalid', 'secret');
    const fixture = TestBed.createComponent(HostingSpacesPage);
    fixture.detectChanges();
    await flush();
    page(fixture).simulation.set({
      base_subtotal: '12000',
      billed_units: 2,
      rate_unit: 'hora',
      time_zone: 'America/Santiago',
    });
    page(fixture).availability.set({
      available: false,
      start_at: '2026-10-15T10:00:00-03:00',
      end_at: '2026-10-15T11:00:00-03:00',
    });
    page(fixture).blocks.set([
      {
        id: 'block-a',
        start_at: '2026-10-15T10:00:00Z',
        end_at: '2026-10-15T11:00:00Z',
        reason: 'test',
        time_zone: 'America/Santiago',
      },
    ]);
    const next = space('space-b', 'borrador');
    api.detail.mockResolvedValue(next);
    api.gallery.mockRejectedValue(new HostingApiError('empty', 404));
    api.tariff.mockRejectedValue(new HostingApiError('empty', 404));
    api.tariffVersions.mockRejectedValue(new HostingApiError('empty', 404));
    api.weekly.mockRejectedValue(new HostingApiError('empty', 404));
    const loading = page(fixture).loadSpace('space-b');
    expect(page(fixture).detailLoading()).toBe(true);
    expect(page(fixture).selectedSpace()).toBeNull();
    expect(page(fixture).simulation()).toBeNull();
    expect(page(fixture).availability()).toBeNull();
    await loading;
    expect(page(fixture).selectedSpace()?.id).toBe('space-b');
    expect(page(fixture).gallery()).toEqual([]);
    expect(page(fixture).tariff()).toBeNull();
    expect(page(fixture).tariffHistory()).toEqual([]);
    expect(page(fixture).weekly()).toBeNull();
    expect(page(fixture).blocks()).toEqual([]);
    fixture.destroy();
  });

  it('expires the current bearer when A configuration returns 401 after selecting B', async () => {
    const api = apiFor(space('space-a', 'activa'));
    const { identity } = setup(api);
    await identity.signIn('owner@example.invalid', 'secret');
    const fixture = TestBed.createComponent(HostingSpacesPage);
    fixture.detectChanges();
    await flush();
    const component = page(fixture);
    const pendingProfile = deferred<Awaited<ReturnType<typeof api.profile>>>();
    api.profile.mockReturnValueOnce(pendingProfile.promise);
    const loadingA = component.loadSpace('space-a');
    await flush();

    api.detail.mockResolvedValueOnce(space('space-b', 'borrador'));
    await component.loadSpace('space-b');
    expect(component.selectedSpace()?.id).toBe('space-b');
    expect(identity.accessToken()).toBe('bearer-a');

    pendingProfile.reject(new HostingApiError('unauthorized', 401));
    await loadingA;
    expect(identity.accessToken()).toBeNull();
    expect(identity.session()).toBeNull();
    expect(component.spaces()).toEqual([]);
    expect(component.selectedSpace()).toBeNull();
    expect(component.gallery()).toEqual([]);
    fixture.destroy();
  });

  it('ignores a parallel configuration 401 from the old bearer after logout and login', async () => {
    const api = apiFor(space('space-a', 'activa'));
    const identityApi = {
      login: vi
        .fn()
        .mockResolvedValueOnce(login('owner-a', 'bearer-a'))
        .mockResolvedValueOnce(login('owner-b', 'bearer-b')),
      session: vi.fn(async (token: string) => ({
        ...ownerSession,
        account_id: token === 'bearer-a' ? 'owner-a' : 'owner-b',
      })),
      logout: vi.fn().mockResolvedValue(undefined),
    };
    const { identity } = setup(api, null, identityApi);
    await identity.signIn('owner-a@example.invalid', 'secret');
    const fixture = TestBed.createComponent(HostingSpacesPage);
    fixture.detectChanges();
    await flush();
    const component = page(fixture);
    const pendingProfile = deferred<Awaited<ReturnType<typeof api.profile>>>();
    api.profile.mockReturnValueOnce(pendingProfile.promise);
    const loadingA = component.loadSpace('space-a');
    await flush();

    await identity.signOut();
    await identity.signIn('owner-b@example.invalid', 'secret');
    await component.load();
    expect(identity.accessToken()).toBe('bearer-b');
    expect(component.spaces()).toHaveLength(1);

    pendingProfile.reject(new HostingApiError('unauthorized', 401));
    await loadingA;
    expect(identity.accessToken()).toBe('bearer-b');
    expect(identity.session()?.account_id).toBe('owner-b');
    expect(component.spaces()).toHaveLength(1);
    expect(component.selectedSpace()?.id).toBe('space-a');
    fixture.destroy();
  });

  it('retains an uncertain gallery key after logout/login to the same account', async () => {
    const api = apiFor();
    api.addPhoto.mockRejectedValueOnce(new Error('network timeout')).mockResolvedValueOnce({
      id: 'photo-1',
      fixture_code: 'synthetic-png-v1',
      mime_type: 'image/png',
      sha256: 'abc',
      size_bytes: 12,
      created_at: '2026-10-12T00:00:00Z',
    });
    const { identity } = setup(api);
    await identity.signIn('owner@example.invalid', 'secret');
    const fixture = TestBed.createComponent(HostingSpacesPage);
    fixture.detectChanges();
    await flush();
    await page(fixture).addSyntheticPhoto();
    const firstKey = api.addPhoto.mock.calls[0][2];
    await identity.signOut();
    await identity.signIn('owner@example.invalid', 'secret');
    await flush();
    await page(fixture).load();
    await page(fixture).addSyntheticPhoto();
    expect(api.addPhoto.mock.calls[1][2]).toBe(firstKey);
    fixture.destroy();
  });

  it('queues a real image, uploads it through Go and releases the local preview task', async () => {
    const api = apiFor();
    const { identity } = setup(api);
    await identity.signIn('owner@example.invalid', 'secret');
    const fixture = TestBed.createComponent(HostingSpacesPage);
    fixture.detectChanges();
    await flush();
    const component = page(fixture);
    const file = new File(['synthetic test bytes'], 'room.png', { type: 'image/png' });
    component.queuePhotoFiles({ target: { files: [file], value: 'selected' } } as unknown as Event);
    expect(component.queuedUploadCount()).toBe(1);
    expect(component.uploadSlotsAvailable()).toBe(9);
    await component.startPhotoUploads();
    expect(api.uploadPhoto).toHaveBeenCalledWith(expect.any(String), 'space-a', file, expect.any(String), expect.any(Function));
    expect(component.uploadQueue()).toEqual([]);
    expect(component.gallery().map((photo) => photo.id)).toContain('media-photo-1');
    fixture.destroy();
  });

  it('revokes private gallery Blob URLs on logout and shows Go calendar conflicts', async () => {
    const api = apiFor();
    const create = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:private-photo');
    const revoke = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => undefined);
    api.addBlock.mockRejectedValueOnce(new HostingApiError('conflict', 409));
    const { identity } = setup(api);
    await identity.signIn('owner@example.invalid', 'secret');
    const fixture = TestBed.createComponent(HostingSpacesPage);
    fixture.detectChanges();
    await flush();
    await page(fixture).showPhoto('photo-1');
    expect(page(fixture).photoUrl()).toBe('blob:private-photo');
    page(fixture).blockForm.start_at = '2026-10-15T10:00';
    page(fixture).blockForm.end_at = '2026-10-15T11:00';
    await page(fixture).addManualBlock();
    expect(page(fixture).conflict()).toContain('conflicto');
    await identity.signOut();
    expect(revoke).toHaveBeenCalledWith('blob:private-photo');
    expect(page(fixture).photoUrl()).toBeNull();
    expect(create).toHaveBeenCalledTimes(1);
    fixture.destroy();
  });

  it('discards a private photo response from account A after account B signs in', async () => {
    const api = apiFor();
    let finishPhoto!: (blob: Blob) => void;
    api.photoContent.mockImplementationOnce(
      () => new Promise((resolve) => (finishPhoto = resolve)),
    );
    const create = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:account-a-photo');
    const idApi = {
      login: vi
        .fn()
        .mockResolvedValueOnce(login('owner-a', 'bearer-a'))
        .mockResolvedValueOnce(login('owner-b', 'bearer-b'))
        .mockResolvedValueOnce(login('owner-b', 'bearer-b2')),
      session: vi.fn().mockImplementation(async (token: string) => ({
        account_id: token === 'bearer-a' ? 'owner-a' : 'owner-b',
        roles: ['arrendador'],
        capabilities: [],
        restricted_mode: false,
      })),
      logout: vi.fn().mockResolvedValue(undefined),
    };
    const { identity } = setup(api, null, idApi);
    await identity.signIn('owner@example.invalid', 'secret');
    const fixture = TestBed.createComponent(HostingSpacesPage);
    fixture.detectChanges();
    await flush();
    const pending = page(fixture).showPhoto('photo-1');
    await identity.signOut();
    await identity.signIn('other@example.invalid', 'secret');
    finishPhoto(new Blob(['private A photo'], { type: 'image/png' }));
    await pending;
    expect(create).not.toHaveBeenCalled();
    expect(page(fixture).photoUrl()).toBeNull();
    expect(identity.session()?.account_id).toBe('owner-b');
    fixture.destroy();
  });

  it('clears simulation when modality changes and preserves exact CLP JSON', () => {
    const api = apiFor();
    setup(api);
    const huge = '9223372036854775807';
    const parsed = parseExactClpJson<{ base_price: string; billed_units: number }>(
      `{"base_price":${huge},"billed_units":2}`,
    );
    expect(parsed.base_price).toBe(huge);
    expect(stringifyExactClpJson({ base_price: huge })).toBe(`{"base_price":${huge}}`);
    const fixture = TestBed.createComponent(HostingSpacesPage);
    page(fixture).simulation.set({
      base_subtotal: huge,
      billed_units: 2,
      rate_unit: 'hora',
      time_zone: 'America/Santiago',
    });
    page(fixture).simulationForm.start_at = '2026-10-15T10:00';
    page(fixture).setRateUnit('mes');
    expect(page(fixture).simulation()).toBeNull();
    expect(page(fixture).simulationForm.start_at).toBe('');
    fixture.destroy();
  });

  it('discards simulation and availability after criteria edits and accepts only the newest query', async () => {
    const api = apiFor();
    const oldSimulation = deferred<Awaited<ReturnType<typeof api.simulate>>>();
    api.simulate.mockReturnValueOnce(oldSimulation.promise);
    const oldAvailability = deferred<Awaited<ReturnType<typeof api.checkAvailability>>>();
    api.checkAvailability.mockReturnValueOnce(oldAvailability.promise);
    const { identity } = setup(api);
    await identity.signIn('owner@example.invalid', 'secret');
    const fixture = TestBed.createComponent(HostingSpacesPage);
    fixture.detectChanges();
    await flush();
    const component = page(fixture);
    component.simulationForm.start_at = '2026-10-15T10:00';
    component.simulationForm.end_at = '2026-10-15T11:00';
    const simulationPending = component.simulatePrice();
    component.zoneDateTime('2026-10-15T12:00', 'simulation', 'start_at');
    oldSimulation.resolve({
      id: 'old-sim',
      space_id: 'space-a',
      rate_version: 1,
      rate_unit: 'hora',
      base_price: '8000',
      billed_units: 1,
      currency: 'CLP',
      base_subtotal: '8000',
      start_at: '2026-10-15T13:00:00Z',
      end_at: '2026-10-15T14:00:00Z',
      time_zone: 'America/Santiago',
      private: true,
      created_at: '2026-10-12T00:00:00Z',
    });
    await simulationPending;
    expect(component.simulation()).toBeNull();

    component.availabilityForm.start_at = '2026-10-15T10:00';
    component.availabilityForm.end_at = '2026-10-15T11:00';
    const availabilityPending = component.checkAvailability();
    component.setTimeZone('America/New_York');
    oldAvailability.resolve({
      space_id: 'space-a',
      time_zone: 'America/Santiago',
      start_at: '2026-10-15T13:00:00Z',
      end_at: '2026-10-15T14:00:00Z',
      available: false,
    });
    await availabilityPending;
    expect(component.availability()).toBeNull();
    expect(component.conflict()).toBe('');

    const oldAvailabilityQuery = deferred<Awaited<ReturnType<typeof api.checkAvailability>>>();
    const newAvailabilityQuery = deferred<Awaited<ReturnType<typeof api.checkAvailability>>>();
    api.checkAvailability
      .mockReturnValueOnce(oldAvailabilityQuery.promise)
      .mockReturnValueOnce(newAvailabilityQuery.promise);
    component.setTimeZone('America/Santiago');
    component.availabilityForm.start_at = '2026-10-15T15:00';
    component.availabilityForm.end_at = '2026-10-15T16:00';
    const oldAvailabilityResult = component.checkAvailability();
    const newAvailabilityResult = component.checkAvailability();
    newAvailabilityQuery.resolve({
      space_id: 'space-a',
      time_zone: 'America/Santiago',
      start_at: '2026-10-15T18:00:00Z',
      end_at: '2026-10-15T19:00:00Z',
      available: false,
    });
    await newAvailabilityResult;
    oldAvailabilityQuery.resolve({
      space_id: 'space-a',
      time_zone: 'America/Santiago',
      start_at: '2026-10-15T18:00:00Z',
      end_at: '2026-10-15T19:00:00Z',
      available: true,
    });
    await oldAvailabilityResult;
    expect(component.availability()?.available).toBe(false);

    const first = deferred<Awaited<ReturnType<typeof api.simulate>>>();
    const second = deferred<Awaited<ReturnType<typeof api.simulate>>>();
    api.simulate.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise);
    component.simulationForm.start_at = '2026-10-15T15:00';
    component.simulationForm.end_at = '2026-10-15T16:00';
    const firstQuery = component.simulatePrice();
    const secondQuery = component.simulatePrice();
    const result = (price: string) => ({
      id: `sim-${price}`,
      space_id: 'space-a',
      rate_version: 1,
      rate_unit: 'hora' as const,
      base_price: price,
      billed_units: 1,
      currency: 'CLP' as const,
      base_subtotal: price,
      start_at: '2026-10-15T18:00:00Z',
      end_at: '2026-10-15T19:00:00Z',
      time_zone: 'America/New_York',
      private: true as const,
      created_at: '2026-10-12T00:00:00Z',
    });
    second.resolve(result('9000'));
    await secondQuery;
    first.resolve(result('8000'));
    await firstQuery;
    expect(component.simulation()?.base_subtotal).toBe('9000');
    fixture.destroy();
  });

  it('keeps late 409 and load errors from a prior space and account out of the current view', async () => {
    const api = apiFor(space('space-a', 'activa'));
    const idApi = {
      login: vi
        .fn()
        .mockResolvedValueOnce(login('owner-a', 'bearer-a'))
        .mockResolvedValueOnce(login('owner-b', 'bearer-b'))
        .mockResolvedValueOnce(login('owner-b', 'bearer-b2')),
      session: vi.fn().mockImplementation(async (token: string) => ({
        account_id: token === 'bearer-a' ? 'owner-a' : 'owner-b',
        roles: ['arrendador'],
        capabilities: [],
        restricted_mode: false,
      })),
      logout: vi.fn().mockResolvedValue(undefined),
    };
    const { identity } = setup(api, null, idApi);
    await identity.signIn('owner-a@example.invalid', 'secret');
    const fixture = TestBed.createComponent(HostingSpacesPage);
    fixture.detectChanges();
    await flush();
    const component = page(fixture);
    const oldBlock = deferred<Awaited<ReturnType<typeof api.addBlock>>>();
    api.addBlock.mockReturnValueOnce(oldBlock.promise);
    component.blockForm.start_at = '2026-10-15T10:00';
    component.blockForm.end_at = '2026-10-15T11:00';
    const oldWrite = component.addManualBlock();
    api.detail.mockResolvedValue(space('space-b', 'borrador'));
    await component.loadSpace('space-b');
    oldBlock.reject(new HostingApiError('conflict from A', 409));
    await oldWrite;
    expect(component.conflict()).toBe('');
    expect(component.error()).toBe('');

    const oldList = deferred<Awaited<ReturnType<typeof api.list>>>();
    api.list.mockReturnValueOnce(oldList.promise);
    const priorLoad = component.load();
    await identity.signOut();
    await identity.signIn('owner-b@example.invalid', 'secret');
    await flush();
    oldList.reject(new Error('late load from A'));
    await priorLoad;
    expect(identity.session()?.account_id).toBe('owner-b');
    expect(component.loadingError()).toBe('');
    expect(component.error()).toBe('');
    expect(component.conflict()).toBe('');
    fixture.destroy();
  });

  it('expires the current bearer on a 401 after criteria changed but ignores an old bearer 401', async () => {
    const api = apiFor();
    const idApi = {
      login: vi
        .fn()
        .mockResolvedValueOnce(login('owner-a', 'bearer-a'))
        .mockResolvedValueOnce(login('owner-b', 'bearer-b'))
        .mockResolvedValueOnce(login('owner-b', 'bearer-b2')),
      session: vi.fn().mockImplementation(async (token: string) => ({
        account_id: token === 'bearer-a' ? 'owner-a' : 'owner-b',
        roles: ['arrendador'],
        capabilities: [],
        restricted_mode: false,
      })),
      logout: vi.fn().mockResolvedValue(undefined),
    };
    const { identity } = setup(api, null, idApi);
    await identity.signIn('owner-a@example.invalid', 'secret');
    const fixture = TestBed.createComponent(HostingSpacesPage);
    fixture.detectChanges();
    await flush();
    const component = page(fixture);
    const pendingCurrent = deferred<Awaited<ReturnType<typeof api.simulate>>>();
    api.simulate.mockReturnValueOnce(pendingCurrent.promise);
    component.simulationForm.start_at = '2026-10-15T10:00';
    component.simulationForm.end_at = '2026-10-15T11:00';
    const currentRequest = component.simulatePrice();
    component.zoneDateTime('2026-10-15T12:00', 'simulation', 'start_at');
    pendingCurrent.reject(new HostingApiError('unauthorized', 401));
    await currentRequest;
    expect(identity.accessToken()).toBeNull();

    await identity.signIn('owner-b@example.invalid', 'secret');
    await flush();
    await component.load();
    const pendingOld = deferred<Awaited<ReturnType<typeof api.simulate>>>();
    api.simulate.mockReturnValueOnce(pendingOld.promise);
    component.simulationForm.start_at = '2026-10-15T15:00';
    component.simulationForm.end_at = '2026-10-15T16:00';
    const oldRequest = component.simulatePrice();
    await identity.signOut();
    await identity.signIn('owner-b@example.invalid', 'secret');
    await flush();
    await component.load();
    expect(component.selectedSpace()).not.toBeNull();
    pendingOld.reject(new HostingApiError('old token unauthorized', 401));
    await oldRequest;
    expect(identity.session()?.account_id).toBe('owner-b');
    fixture.destroy();
  });

  it('converts unique and repeated DST wall times but rejects nonexistent local times', () => {
    expect(zonedLocalToRfc3339('2026-11-01T01:30', 'America/New_York', 'earlier')).toBe(
      '2026-11-01T01:30:00-04:00',
    );
    expect(zonedLocalToRfc3339('2026-11-01T01:30', 'America/New_York', 'later')).toBe(
      '2026-11-01T01:30:00-05:00',
    );
    expect(() => zonedLocalToRfc3339('2026-03-08T02:30', 'America/New_York')).toThrow(/no existe/);
  });

  it('formats wall-clock labels in the requested IANA zone, including repeated DST times', () => {
    const santiago = zonedIsoLabel('2026-10-15T13:00:00Z', 'America/Santiago');
    expect(santiago).toContain('15');
    expect(santiago).toContain('oct');
    expect(santiago).toMatch(/CLST|CLT/);
    const before = zonedIsoLabel('2026-11-01T05:30:00Z', 'America/New_York');
    const after = zonedIsoLabel('2026-11-01T06:30:00Z', 'America/New_York');
    expect(before).toContain('01:30');
    expect(after).toContain('01:30');
    expect(before).not.toBe(after);
  });

  it('ignores stale photo publication failures without clearing a newer save or session', async () => {
    const api=apiFor();
    const idApi={login:vi.fn().mockResolvedValueOnce(login('owner-a','bearer-a')).mockResolvedValueOnce(login('owner-b','bearer-b')),session:vi.fn().mockImplementation(async(token:string)=>({account_id:token==='bearer-a'?'owner-a':'owner-b',roles:['arrendador'],capabilities: [], restricted_mode:false})),logout:vi.fn().mockResolvedValue(undefined)};
    const {identity}=setup(api,null,idApi);await identity.signIn('owner-a@example.invalid','secret');
    const fixture=TestBed.createComponent(HostingSpacesPage);fixture.detectChanges();await flush();
    const component=page(fixture);const first=deferred<never>();const second=deferred<never>();
    api.setPhotoPublic.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise);
    const oldWrite=component.setPhotoPublic('photo-1',true);await flush();
    await identity.signOut();await identity.signIn('owner-b@example.invalid','secret');await flush();await vi.waitFor(()=>expect(component.selectedSpace()?.id).toBe('space-a'));
    const newWrite=component.setPhotoPublic('photo-1',true);await flush();
    first.reject(new HostingApiError('stale unauthorized',401));await oldWrite;
    expect(identity.accessToken()).toBe('bearer-b');expect(component.saving()).toBe(true);expect(component.error()).toBe('');
    second.reject(new HostingApiError('current failure',503));await newWrite;
    expect(component.saving()).toBe(false);expect(component.error()).not.toBe('');fixture.destroy();
  });

  it('expires the current bearer on a photo publication 401 even when its resource revision changed', async () => {
    const api=apiFor();const {identity}=setup(api);await identity.signIn('owner@example.invalid','secret');
    const fixture=TestBed.createComponent(HostingSpacesPage);fixture.detectChanges();await flush();const component=page(fixture);
    const pending=deferred<never>();api.setPhotoPublic.mockReturnValueOnce(pending.promise);
    const write=component.setPhotoPublic('photo-1',true);await flush();
    // A newer resource write supersedes presentation of the old response while retaining its Bearer.
    const superseding=component.setPhotoPublic('photo-1',false);await flush();
    pending.reject(new HostingApiError('unauthorized',401));await write;
    expect(identity.accessToken()).toBeNull();await superseding;fixture.destroy();
  });

  it('does not let a late successful photo update finish a newer pending write', async () => {
    const api=apiFor();const {identity}=setup(api);await identity.signIn('owner@example.invalid','secret');
    const fixture=TestBed.createComponent(HostingSpacesPage);fixture.detectChanges();await flush();const component=page(fixture);await vi.waitFor(()=>expect(component.selectedSpace()?.id).toBe('space-a'));
    const old=deferred<never>();const current=deferred<never>();api.setPhotoPublic.mockReturnValueOnce(old.promise).mockReturnValueOnce(current.promise);
    const oldWrite=component.setPhotoPublic('photo-1',true);await flush();const currentWrite=component.setPhotoPublic('photo-1',false);await flush();
    old.resolve({id:'photo-1',public_in_catalog:true} as never);await oldWrite;
    expect(component.saving()).toBe(true);const photo=component.gallery().find(item=>item.id==='photo-1');expect(photo && component.photoIsVisible(photo)).not.toBe(true);
    current.reject(new HostingApiError('current failure',503));await currentWrite;fixture.destroy();
  });
});
