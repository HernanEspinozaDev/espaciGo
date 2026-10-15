import { provideHttpClient } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { OwnIdentityExport, Profile } from '../../core/contracts/models';
import { IdentitySession } from '../../core/identity/application/identity-session';
import { IdentityApi } from '../../core/identity/infrastructure/identity-api';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { provideFrontendPorts } from '../../frontend.providers';
import { AccountPrivacyPage } from './account-privacy-page';
import { AccountPrivacyApi, AccountPrivacyApiError } from './infrastructure/account-privacy-api';

const sessionA = { account_id: 'account-a', roles: ['arrendatario'], capabilities: [], restricted_mode: false };
const sessionB = { account_id: 'account-b', roles: ['arrendador'], capabilities: [], restricted_mode: false };
const login = (account_id: string, access_token: string) => ({
  account_id,
  roles: [account_id === 'account-a' ? 'arrendatario' : 'arrendador'],
  access_token,
  token_type: 'Bearer',
  expires_at: '2027-01-01T00:00:00Z',
  capabilities: [],
  restricted_mode: false,
});
const profile = (display_name: string): Profile => ({
  display_name,
  phone: '912345678',
  updated_at: '2026-10-01T12:00:00Z',
});
const ownExport = (email: string): OwnIdentityExport => ({
  scope: 'identidad_local_v1',
  account: {
    email,
    state: 'activo',
    use_preference: 'arrendar',
    created_at: '2026-01-01T00:00:00Z',
  },
  profile: { display_name: 'Cuenta propia', phone: null, updated_at: '2026-01-01T00:00:00Z' },
  roles: ['arrendatario'],
  terms_acceptances: [],
  rights_requests: [],
});
const metadata = {
  id: 'photo-a',
  fixture_code: 'synthetic-png-v1' as const,
  mime_type: 'image/png' as const,
  sha256: 'sha256-synthetic',
  size_bytes: 8,
  created_at: '2026-10-01T12:00:00Z',
};
const accountApiDefaults = () => ({
  profile: vi.fn(async (token: string) =>
    profile(token === 'bearer-a' ? 'Titular A' : 'Titular B'),
  ),
  updateProfile: vi.fn(async (_token: string, value: Profile) => value),
  photoMetadata: vi.fn().mockRejectedValue(new AccountPrivacyApiError('not found', 404)),
  photoContent: vi
    .fn()
    .mockResolvedValue(new Blob([new Uint8Array([137, 80, 78, 71])], { type: 'image/png' })),
  generatePhoto: vi.fn().mockResolvedValue(undefined),
  removePhoto: vi.fn().mockResolvedValue(undefined),
  payoutAccount: vi.fn().mockResolvedValue(null),
  createPayoutAccount: vi
    .fn()
    .mockResolvedValue({
      id: 'payout-a',
      adapter: 'fake-local-v1',
      reference: 'demo-test',
      state: 'activa',
      updated_at: '2026-10-01T00:00:00Z',
    }),
  revokePayoutAccount: vi.fn().mockResolvedValue(undefined),
  rightsRequests: vi.fn().mockResolvedValue({ items: [] }),
  requestRight: vi
    .fn()
    .mockResolvedValue({
      id: 'right-a',
      kind: 'supresion',
      state: 'en_revision',
      created_at: '2026-10-01T00:00:00Z',
    }),
  identityExport: vi.fn(async (token: string) =>
    ownExport(token === 'bearer-a' ? 'a@example.invalid' : 'b@example.invalid'),
  ),
  archive: vi
    .fn()
    .mockResolvedValue({
      body: new Blob([new Uint8Array([0x50, 0x4b, 0x03, 0x04]), 'manifest.json']),
    }),
});

function setup(identityApi: unknown, accountApi: unknown) {
  TestBed.configureTestingModule({
    imports: [AccountPrivacyPage],
    providers: [
      ...provideFrontendPorts(),
      provideHttpClient(),
      provideRouter([]),
      PrivateSessionState,
      { provide: IdentityApi, useValue: identityApi },
      { provide: AccountPrivacyApi, useValue: accountApi },
    ],
  });
  return TestBed;
}
const pageAccess = (fixture: ReturnType<typeof TestBed.createComponent>) =>
  fixture.componentInstance as unknown as {
    profile: () => Profile | null;
    photoUrl: () => string | null;
    load(): Promise<boolean>;
    downloadJson(): Promise<void>;
    downloadZip(): Promise<void>;
    removePhoto(): Promise<void>;
    createPhoto(): Promise<void>;
    saveProfile(): Promise<void>;
    requestRight(type: 'acceso' | 'supresion'): Promise<void>;
    displayName: { (): string; set(value: string): void };
    phone: { (): string; set(value: string): void };
    savingProfile: () => boolean;
    busy: () => boolean;
    notice: () => string;
    error(): string;
  };

describe('AccountPrivacyPage', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    TestBed.resetTestingModule();
  });

  it('discards A data after logout/login as B and uses only B data in the JSON export', async () => {
    let finishA!: (value: Profile) => void;
    const pendingA = new Promise<Profile>((resolve) => (finishA = resolve));
    const api = accountApiDefaults();
    api.profile.mockImplementation((token: string) =>
      token === 'bearer-a' ? pendingA : Promise.resolve(profile('Titular B')),
    );
    const identityApi = {
      login: vi
        .fn()
        .mockResolvedValueOnce(login('account-a', 'bearer-a'))
        .mockResolvedValueOnce(login('account-b', 'bearer-b')),
      session: vi.fn().mockResolvedValueOnce(sessionA).mockResolvedValueOnce(sessionB),
      logout: vi.fn().mockResolvedValue(undefined),
    };
    setup(identityApi, api);
    const identity = TestBed.inject(IdentitySession);
    await identity.signIn('a@example.invalid', 'Synthetic#123');
    const fixture = TestBed.createComponent(AccountPrivacyPage);
    fixture.detectChanges();
    await vi.waitFor(() => expect(api.profile).toHaveBeenCalledWith('bearer-a'));

    await identity.signOut();
    await identity.signIn('b@example.invalid', 'Synthetic#456');
    await vi.waitFor(() => expect(pageAccess(fixture).profile()?.display_name).toBe('Titular B'));
    finishA(profile('Respuesta privada tardía de A'));
    await Promise.resolve();
    expect(pageAccess(fixture).profile()?.display_name).toBe('Titular B');
    await pageAccess(fixture).downloadJson();
    expect(api.identityExport).toHaveBeenLastCalledWith('bearer-b');
    fixture.destroy();
  });

  it('expires only the current Bearer on 401 and ignores a late 401 from A after B signs in', async () => {
    let rejectA!: (error: unknown) => void;
    const pendingA = new Promise<Profile>((_resolve, reject) => (rejectA = reject));
    const api = accountApiDefaults();
    api.profile.mockImplementation((token: string) =>
      token === 'bearer-a' ? pendingA : Promise.resolve(profile('Titular B')),
    );
    const identityApi = {
      login: vi
        .fn()
        .mockResolvedValueOnce(login('account-a', 'bearer-a'))
        .mockResolvedValueOnce(login('account-b', 'bearer-b')),
      session: vi.fn().mockResolvedValueOnce(sessionA).mockResolvedValueOnce(sessionB),
      logout: vi.fn().mockResolvedValue(undefined),
    };
    setup(identityApi, api);
    const identity = TestBed.inject(IdentitySession);
    await identity.signIn('a@example.invalid', 'Synthetic#123');
    const fixture = TestBed.createComponent(AccountPrivacyPage);
    fixture.detectChanges();
    await vi.waitFor(() => expect(api.profile).toHaveBeenCalledWith('bearer-a'));
    await identity.signOut();
    await identity.signIn('b@example.invalid', 'Synthetic#456');
    await vi.waitFor(() => expect(pageAccess(fixture).profile()?.display_name).toBe('Titular B'));
    rejectA(new AccountPrivacyApiError('unauthorized', 401));
    await Promise.resolve();
    expect(identity.accessToken()).toBe('bearer-b');
    expect(identity.session()?.account_id).toBe('account-b');
    expect(pageAccess(fixture).profile()?.display_name).toBe('Titular B');
    fixture.destroy();
  });

  it('clears current identity and account data when the active account receives 401', async () => {
    const api = accountApiDefaults();
    api.profile.mockRejectedValue(new AccountPrivacyApiError('unauthorized', 401));
    setup(
      {
        login: vi.fn().mockResolvedValue(login('account-a', 'bearer-a')),
        session: vi.fn().mockResolvedValue(sessionA),
        logout: vi.fn(),
      },
      api,
    );
    const identity = TestBed.inject(IdentitySession);
    await identity.signIn('a@example.invalid', 'Synthetic#123');
    const fixture = TestBed.createComponent(AccountPrivacyPage);
    fixture.detectChanges();
    await vi.waitFor(() => expect(identity.accessToken()).toBeNull());
    expect(identity.session()).toBeNull();
    expect(pageAccess(fixture).profile()).toBeNull();
    fixture.destroy();
  });

  it('treats a missing profile as empty, allows creating it, and keeps rights and exports available', async () => {
    const api = accountApiDefaults();
    api.profile.mockRejectedValue(new AccountPrivacyApiError('profile not created', 404));
    const identityApi = {
      login: vi.fn().mockResolvedValue(login('account-a', 'bearer-a')),
      session: vi.fn().mockResolvedValue(sessionA),
      logout: vi.fn(),
    };
    setup(identityApi, api);
    const identity = TestBed.inject(IdentitySession);
    await identity.signIn('a@example.invalid', 'Synthetic#123');
    const fixture = TestBed.createComponent(AccountPrivacyPage);
    fixture.detectChanges();
    await vi.waitFor(() =>
      expect(fixture.nativeElement.querySelector('form.profile-form')).not.toBeNull(),
    );
    await vi.waitFor(() => expect(pageAccess(fixture).busy()).toBe(false));
    expect(fixture.nativeElement.textContent).toContain('Aún no has creado un perfil');
    expect(fixture.nativeElement.textContent).toContain('Solicitar acceso');
    expect(fixture.nativeElement.textContent).toContain('Descargar JSON');
    const accessButton = [...fixture.nativeElement.querySelectorAll('button')].find(
      (button) => button.textContent?.trim() === 'Solicitar acceso',
    ) as HTMLButtonElement;
    accessButton.click();
    await vi.waitFor(() => expect(api.requestRight).toHaveBeenCalledWith('bearer-a', 'acceso'));
    await vi.waitFor(() => expect(api.rightsRequests).toHaveBeenCalledTimes(2));

    const page = pageAccess(fixture);
    page.displayName.set('Primer perfil');
    const created = { ...profile('Primer perfil'), updated_at: '2026-10-12T12:00:00Z' };
    api.updateProfile.mockResolvedValueOnce(created);
    fixture.nativeElement
      .querySelector('form.profile-form')
      .dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
    await vi.waitFor(() => expect(api.updateProfile).toHaveBeenCalled());
    await vi.waitFor(() => expect(page.savingProfile()).toBe(false));
    fixture.detectChanges();
    expect(api.updateProfile).toHaveBeenCalledWith('bearer-a', {
      display_name: 'Primer perfil',
      phone: '',
    });
    expect(page.profile()?.display_name).toBe('Primer perfil');
    expect(page.notice()).toBe('Perfil guardado por Go.');
    expect(fixture.nativeElement.querySelector('#identity-heading').textContent).toContain(
      'Primer perfil',
    );
    fixture.destroy();
  });

  it('finishes a profile save after another account action and reflects the confirmed response', async () => {
    let finishUpdate!: (value: Profile) => void;
    const pendingUpdate = new Promise<Profile>((resolve) => (finishUpdate = resolve));
    const api = accountApiDefaults();
    api.updateProfile.mockReturnValueOnce(pendingUpdate);
    setup(
      {
        login: vi.fn().mockResolvedValue(login('account-a', 'bearer-a')),
        session: vi.fn().mockResolvedValue(sessionA),
        logout: vi.fn(),
      },
      api,
    );
    const identity = TestBed.inject(IdentitySession);
    await identity.signIn('a@example.invalid', 'Synthetic#123');
    const fixture = TestBed.createComponent(AccountPrivacyPage);
    fixture.detectChanges();
    const page = pageAccess(fixture);
    await vi.waitFor(() => expect(page.profile()?.display_name).toBe('Titular A'));

    page.displayName.set('Nombre confirmado');
    const save = page.saveProfile();
    await vi.waitFor(() => expect(page.savingProfile()).toBe(true));
    const accessButton = [...fixture.nativeElement.querySelectorAll('button')].find(
      (button) => button.textContent?.trim() === 'Solicitar acceso',
    ) as HTMLButtonElement;
    expect(accessButton.disabled).toBe(false);
    accessButton.click();
    await vi.waitFor(() => expect(api.requestRight).toHaveBeenCalledWith('bearer-a', 'acceso'));
    await vi.waitFor(() => expect(api.rightsRequests).toHaveBeenCalledTimes(2));
    expect(page.savingProfile()).toBe(true);

    const confirmed = profile('Nombre confirmado');
    finishUpdate(confirmed);
    await save;
    fixture.detectChanges();
    expect(page.savingProfile()).toBe(false);
    expect(page.profile()?.display_name).toBe('Nombre confirmado');
    expect(page.notice()).toBe('Perfil guardado por Go.');
    expect(fixture.nativeElement.querySelector('#identity-heading').textContent).toContain(
      'Nombre confirmado',
    );
    fixture.destroy();
  });

  it('keeps a confirmed PUT over an older profile GET during photo refresh without blocking other controls', async () => {
    let finishPut!: (value: Profile) => void;
    let finishOldRead!: (value: Profile) => void;
    const pendingPut = new Promise<Profile>((resolve) => (finishPut = resolve));
    const pendingOldRead = new Promise<Profile>((resolve) => (finishOldRead = resolve));
    const api = accountApiDefaults();
    let reads = 0;
    const old = { ...profile('Old A'), phone: '111222333' };
    api.profile.mockImplementation(() => (++reads === 1 ? Promise.resolve(old) : pendingOldRead));
    api.updateProfile.mockReturnValueOnce(pendingPut);
    api.photoMetadata
      .mockRejectedValueOnce(new AccountPrivacyApiError('not found', 404))
      .mockResolvedValue(metadata);
    setup(
      {
        login: vi.fn().mockResolvedValue(login('account-a', 'bearer-a')),
        session: vi.fn().mockResolvedValue(sessionA),
        logout: vi.fn(),
      },
      api,
    );
    const identity = TestBed.inject(IdentitySession);
    await identity.signIn('a@example.invalid', 'Synthetic#123');
    const fixture = TestBed.createComponent(AccountPrivacyPage);
    fixture.detectChanges();
    const page = pageAccess(fixture);
    await vi.waitFor(() => expect(page.profile()?.display_name).toBe('Old A'));

    page.displayName.set('Updated A');
    page.phone.set('987654321');
    const save = page.saveProfile();
    await vi.waitFor(() => expect(page.savingProfile()).toBe(true));
    const photoButton = [...fixture.nativeElement.querySelectorAll('button')].find(
      (button) => button.textContent?.trim() === 'Generar foto sintética',
    ) as HTMLButtonElement;
    expect(photoButton.disabled).toBe(false);
    photoButton.click();
    await vi.waitFor(() => expect(api.profile).toHaveBeenCalledTimes(2));
    expect(page.busy()).toBe(true);

    const confirmed = { ...profile('Updated A'), phone: '987654321' };
    finishPut(confirmed);
    await save;
    expect(page.profile()).toEqual(confirmed);
    expect(page.displayName()).toBe('Updated A');
    expect(page.phone()).toBe('987654321');

    finishOldRead(old);
    await vi.waitFor(() => expect(page.busy()).toBe(false));
    fixture.detectChanges();
    expect(page.profile()).toEqual(confirmed);
    expect(page.displayName()).toBe('Updated A');
    expect(page.phone()).toBe('987654321');
    expect(page.savingProfile()).toBe(false);
    expect(
      [...fixture.nativeElement.querySelectorAll('button')].find(
        (button) => button.textContent?.trim() === 'Solicitar acceso',
      )?.disabled,
    ).toBe(false);
    expect(
      [...fixture.nativeElement.querySelectorAll('button')].find(
        (button) => button.textContent?.trim() === 'Generar/reemplazar fixture',
      )?.disabled,
    ).toBe(false);
    expect(fixture.nativeElement.querySelector('#identity-heading').textContent).toContain(
      'Updated A',
    );
    fixture.destroy();
  });

  it('discards a photo success notice when its own refresh finishes after another account signs in', async () => {
    let finishARefresh!: (value: Profile) => void;
    let profileCallsA = 0;
    const pendingARefresh = new Promise<Profile>((resolve) => (finishARefresh = resolve));
    const api = accountApiDefaults();
    api.profile.mockImplementation((token: string) => {
      if (token === 'bearer-a' && ++profileCallsA === 2) return pendingARefresh;
      return Promise.resolve(profile(token === 'bearer-a' ? 'Titular A' : 'Titular B'));
    });
    api.photoMetadata
      .mockRejectedValueOnce(new AccountPrivacyApiError('not found', 404))
      .mockResolvedValue(metadata);
    const identityApi = {
      login: vi
        .fn()
        .mockResolvedValueOnce(login('account-a', 'bearer-a'))
        .mockResolvedValueOnce(login('account-b', 'bearer-b')),
      session: vi.fn().mockResolvedValueOnce(sessionA).mockResolvedValueOnce(sessionB),
      logout: vi.fn().mockResolvedValue(undefined),
    };
    setup(identityApi, api);
    const identity = TestBed.inject(IdentitySession);
    await identity.signIn('a@example.invalid', 'Synthetic#123');
    const fixture = TestBed.createComponent(AccountPrivacyPage);
    fixture.detectChanges();
    const page = pageAccess(fixture);
    await vi.waitFor(() => expect(page.profile()?.display_name).toBe('Titular A'));

    const photoOperation = page.createPhoto();
    await vi.waitFor(() => expect(api.profile).toHaveBeenCalledTimes(2));
    await identity.signOut();
    await identity.signIn('b@example.invalid', 'Synthetic#456');
    await vi.waitFor(() => expect(page.profile()?.display_name).toBe('Titular B'));
    finishARefresh(profile('Respuesta A tardía'));
    await photoOperation;
    fixture.detectChanges();
    expect(identity.accessToken()).toBe('bearer-b');
    expect(page.profile()?.display_name).toBe('Titular B');
    expect(page.notice()).toBe('');
    fixture.destroy();
  });

  it('keeps the successful photo notice when its own refresh is current', async () => {
    const api = accountApiDefaults();
    api.photoMetadata
      .mockRejectedValueOnce(new AccountPrivacyApiError('not found', 404))
      .mockResolvedValue(metadata);
    setup(
      {
        login: vi.fn().mockResolvedValue(login('account-a', 'bearer-a')),
        session: vi.fn().mockResolvedValue(sessionA),
        logout: vi.fn(),
      },
      api,
    );
    const identity = TestBed.inject(IdentitySession);
    await identity.signIn('a@example.invalid', 'Synthetic#123');
    const fixture = TestBed.createComponent(AccountPrivacyPage);
    fixture.detectChanges();
    const page = pageAccess(fixture);
    await vi.waitFor(() => expect(page.profile()?.display_name).toBe('Titular A'));
    await page.createPhoto();
    fixture.detectChanges();
    expect(page.notice()).toBe('Foto sintética privada generada por Go.');
    fixture.destroy();
  });

  it('discards a photo success notice if the view is destroyed during its refresh', async () => {
    let finishRefresh!: (value: Profile) => void;
    let profileCalls = 0;
    const pendingRefresh = new Promise<Profile>((resolve) => (finishRefresh = resolve));
    const secondApi = accountApiDefaults();
    secondApi.profile.mockImplementation(() =>
      ++profileCalls === 1 ? Promise.resolve(profile('Titular A')) : pendingRefresh,
    );
    secondApi.photoMetadata
      .mockRejectedValueOnce(new AccountPrivacyApiError('not found', 404))
      .mockResolvedValue(metadata);
    setup(
      {
        login: vi.fn().mockResolvedValue(login('account-a', 'bearer-a')),
        session: vi.fn().mockResolvedValue(sessionA),
        logout: vi.fn(),
      },
      secondApi,
    );
    const secondIdentity = TestBed.inject(IdentitySession);
    await secondIdentity.signIn('a@example.invalid', 'Synthetic#123');
    const fixture = TestBed.createComponent(AccountPrivacyPage);
    fixture.detectChanges();
    const page = pageAccess(fixture);
    await vi.waitFor(() => expect(page.profile()?.display_name).toBe('Titular A'));
    const photoOperation = page.createPhoto();
    await vi.waitFor(() => expect(secondApi.profile).toHaveBeenCalledTimes(2));
    fixture.destroy();
    finishRefresh(profile('Vista destruida'));
    await photoOperation;
    expect(page.notice()).toBe('');
  });

  it('revokes private photo Blob URLs on logout and view destruction', async () => {
    let sequence = 0;
    const createUrl = vi.fn(() => `blob:private-photo-${++sequence}`);
    const revokeUrl = vi.fn();
    Object.defineProperty(URL, 'createObjectURL', { configurable: true, value: createUrl });
    Object.defineProperty(URL, 'revokeObjectURL', { configurable: true, value: revokeUrl });
    const api = accountApiDefaults();
    api.photoMetadata
      .mockResolvedValueOnce(metadata)
      .mockResolvedValueOnce({ ...metadata, id: 'photo-b' });
    const identityApi = {
      login: vi.fn().mockResolvedValue(login('account-a', 'bearer-a')),
      session: vi.fn().mockResolvedValue(sessionA),
      logout: vi.fn().mockResolvedValue(undefined),
    };
    setup(identityApi, api);
    const identity = TestBed.inject(IdentitySession);
    await identity.signIn('a@example.invalid', 'Synthetic#123');
    const fixture = TestBed.createComponent(AccountPrivacyPage);
    fixture.detectChanges();
    await vi.waitFor(() => expect(pageAccess(fixture).photoUrl()).toBe('blob:private-photo-1'));
    await pageAccess(fixture).removePhoto();
    await vi.waitFor(() => expect(pageAccess(fixture).photoUrl()).toBe('blob:private-photo-2'));
    expect(pageAccess(fixture).notice()).toBe(
      'Foto retirada. El estado se muestra sin reconstruir el archivo.',
    );
    expect(revokeUrl).toHaveBeenCalledWith('blob:private-photo-1');
    await identity.signOut();
    expect(pageAccess(fixture).photoUrl()).toBeNull();
    expect(revokeUrl).toHaveBeenCalledWith('blob:private-photo-2');
    fixture.destroy();
  });

  it('downloads inspectable JSON and ZIP response blobs and revokes URLs on destroy', async () => {
    const createUrl = vi.fn((blob: Blob) =>
      blob.type === 'application/json;charset=utf-8' ? 'blob:export-json' : 'blob:export-zip',
    );
    const revokeUrl = vi.fn();
    const originalCreate = Object.getOwnPropertyDescriptor(URL, 'createObjectURL');
    const originalRevoke = Object.getOwnPropertyDescriptor(URL, 'revokeObjectURL');
    Object.defineProperty(URL, 'createObjectURL', { configurable: true, value: createUrl });
    Object.defineProperty(URL, 'revokeObjectURL', { configurable: true, value: revokeUrl });
    const api = accountApiDefaults();
    const anchorClick = vi
      .spyOn(HTMLAnchorElement.prototype, 'click')
      .mockImplementation(() => undefined);
    setup(
      {
        login: vi.fn().mockResolvedValue(login('account-b', 'bearer-b')),
        session: vi.fn().mockResolvedValue(sessionB),
        logout: vi.fn(),
      },
      api,
    );
    const identity = TestBed.inject(IdentitySession);
    await identity.signIn('b@example.invalid', 'Synthetic#456');
    const fixture = TestBed.createComponent(AccountPrivacyPage);
    fixture.detectChanges();
    const page = pageAccess(fixture);
    await vi.waitFor(() => expect(page.profile()?.display_name).toBe('Titular B'));
    await page.downloadJson();
    const jsonBlob = createUrl.mock.calls.find(([blob]) =>
      blob.type.startsWith('application/json'),
    )?.[0];
    expect(jsonBlob).toBeInstanceOf(Blob);
    expect(JSON.parse(await jsonBlob!.text()).account.email).toBe('b@example.invalid');
    await page.downloadZip();
    const zipBlob = createUrl.mock.calls.find(([blob]) => blob.type === '')?.[0];
    expect(new Uint8Array(await zipBlob!.arrayBuffer()).slice(0, 4)).toEqual(
      new Uint8Array([0x50, 0x4b, 0x03, 0x04]),
    );
    expect(anchorClick).toHaveBeenCalledTimes(2);
    fixture.destroy();
    expect(revokeUrl).toHaveBeenCalledWith('blob:export-json');
    expect(revokeUrl).toHaveBeenCalledWith('blob:export-zip');
    if (originalCreate) Object.defineProperty(URL, 'createObjectURL', originalCreate);
    else Reflect.deleteProperty(URL, 'createObjectURL');
    if (originalRevoke) Object.defineProperty(URL, 'revokeObjectURL', originalRevoke);
    else Reflect.deleteProperty(URL, 'revokeObjectURL');
  });
});
