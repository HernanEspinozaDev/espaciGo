import { provideHttpClient } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, Router } from '@angular/router';
import { BehaviorSubject, of } from 'rxjs';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { SpaceDraft } from '../../../generated/api/types.gen';
import { IdentityApi } from '../../core/identity/identity-api';
import { IdentitySession } from '../../core/identity/identity-session';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { parseExactClpJson, stringifyExactClpJson } from '../../core/exact-json';
import { HostingApi, HostingApiError } from './hosting-api';
import { HostingSpacesPage } from './hosting-spaces-page';
import { zonedLocalToRfc3339 } from './calendar-time';

const ownerSession = { account_id: 'owner-a', roles: ['arrendador'], restricted_mode: false };
const login = (account_id: string, access_token: string) => ({
  account_id,
  roles: ['arrendador'],
  access_token,
  token_type: 'Bearer',
  expires_at: '2027-01-01T00:00:00Z',
  restricted_mode: false,
});
const space = (id = 'space-a', state: 'borrador' | 'activa' | 'oculta' = 'borrador') => ({
  id,
  category_code: 'oficina',
  category_name: 'Oficina',
  title: 'Sala de trabajo',
  description: 'Un espacio sintético para trabajar.',
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
    profile: vi.fn(async (_token: string, category: string, version?: number) => ({
      category_code: category,
      schema_version: version ?? 2,
      attributes: [
        {
          code: 'estacionamiento',
          label: 'Estacionamiento',
          type: 'boolean' as const,
          order: 1,
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

function setup(
  mockApi: ReturnType<typeof apiFor>,
  routeId: string | null = null,
  identityApi = defaultIdentityApi(),
) {
  const params = new BehaviorSubject(MapParamMap(routeId));
  TestBed.configureTestingModule({
    imports: [HostingSpacesPage],
    providers: [
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

describe('FE-06 HostingSpacesPage', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    TestBed.resetTestingModule();
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

  it('keeps third-party drafts private when Go rejects the owner-only detail', async () => {
    const api = apiFor();
    api.list.mockResolvedValue({ items: [] });
    api.detail.mockRejectedValue(new HostingApiError('No visible', 404));
    const idApi = {
      ...defaultIdentityApi(),
      login: vi.fn().mockResolvedValue(login('other', 'bearer-other')),
      session: vi
        .fn()
        .mockResolvedValue({ account_id: 'other', roles: ['arrendador'], restricted_mode: false }),
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
    page(fixture).form.description = 'Descripción modificada';
    await page(fixture).saveDraft();
    const payload = api.update.mock.calls[0][2] as {
      base_price_clp: string;
      title: string;
      description: string;
    };
    expect(payload.base_price_clp).toBe(exactPrice);
    expect(payload.title).toBe('Título actualizado desde Go');
    expect(payload.description).toBe('Descripción modificada');
    fixture.destroy();
  });

  it('does not let a pre-write detail response overwrite the confirmed publication update', async () => {
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
    page(fixture).publishedForm.title = 'Título confirmado';
    api.updatePublished.mockResolvedValueOnce({
      ...space('space-a', 'activa'),
      title: 'Título confirmado',
    });
    await page(fixture).savePublishedContent();
    finishOld(space('space-a', 'activa'));
    await oldRead;
    expect(page(fixture).selectedSpace()?.title).toBe('Título confirmado');
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
        .mockResolvedValueOnce(login('owner-b', 'bearer-b')),
      session: vi.fn().mockImplementation(async (token: string) => ({
        account_id: token === 'bearer-a' ? 'owner-a' : 'owner-b',
        roles: ['arrendador'],
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

  it('converts unique and repeated DST wall times but rejects nonexistent local times', () => {
    expect(zonedLocalToRfc3339('2026-11-01T01:30', 'America/New_York', 'earlier')).toBe(
      '2026-11-01T01:30:00-04:00',
    );
    expect(zonedLocalToRfc3339('2026-11-01T01:30', 'America/New_York', 'later')).toBe(
      '2026-11-01T01:30:00-05:00',
    );
    expect(() => zonedLocalToRfc3339('2026-03-08T02:30', 'America/New_York')).toThrow(/no existe/);
  });
});
