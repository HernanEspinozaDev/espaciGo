import { provideHttpClient } from '@angular/common/http';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type {
  SyntheticEvidence,
  VerificationCase,
  VerificationEligibilityList,
  VerificationHistoryList,
  VerificationList,
} from '../../../generated/api/types.gen';
import { IdentityApi } from '../../core/identity/identity-api';
import { IdentitySession } from '../../core/identity/identity-session';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { VerificationApi, VerificationApiError } from './verification-api';
import { VerificationPage } from './verification-page';

const sessionA = { account_id: 'a', roles: ['arrendatario'], restricted_mode: false };
const sessionB = { account_id: 'b', roles: ['arrendador'], restricted_mode: true };
const login = (account_id: string, access_token: string) => ({
  account_id,
  roles: account_id === 'a' ? ['arrendatario'] : ['arrendador'],
  access_token,
  token_type: 'Bearer',
  expires_at: '2027-01-01T00:00:00Z',
  restricted_mode: account_id === 'b',
});
const item = (
  id: string,
  state: VerificationCase['state'] = 'en_revision',
  type: VerificationCase['type'] = 'kyc',
): VerificationCase => ({
  id,
  type,
  state,
  provider: 'local-fixture-v1',
  evidence_ref: `fixture:${id}`,
  created_at: '2026-10-01T12:00:00Z',
});
const eligibility = (id: string): VerificationEligibilityList => ({
  scope: 'synthetic_local',
  items: [
    {
      type: 'kyc',
      eligible: false,
      verification_id: id,
      granted_at: null,
      revoked_at: null,
      revocation_reason_code: '',
    },
    {
      type: 'kyb',
      eligible: false,
      verification_id: '',
      granted_at: null,
      revoked_at: null,
      revocation_reason_code: '',
    },
  ],
});
const evidence: SyntheticEvidence = {
  id: 'e-a',
  verification_id: 'v-a',
  fixture_code: 'synthetic-png-v1',
  mime_type: 'image/png',
  size_bytes: 8,
  sha256: 'abc',
  created_at: '2026-10-01T12:00:00Z',
  file_available: true,
};
const emptyHistory: VerificationHistoryList = { items: [] };
const defaults = () => ({
  list: vi.fn(async (_token: string) => ({ items: [item('v-a')] }) as VerificationList),
  eligibility: vi.fn(async (_token: string) => eligibility('v-a')),
  detail: vi.fn(async (_token: string, id: string) => item(id)),
  history: vi.fn(async (_token: string, _id: string) => emptyHistory),
  evidence: vi.fn(async (_token: string, _id: string) => [] as SyntheticEvidence[]),
  create: vi.fn(async (_token: string, _type: 'kyc' | 'kyb', _key: string) => item('v-new')),
  retry: vi.fn(
    async (
      _token: string,
      _id: string,
      _code:
        | 'fixture_vigente_actualizado'
        | 'antecedentes_fixture_actualizados'
        | 'inicio_actividades_fixture_actualizadas',
      _key: string,
    ) => item('v-retry'),
  ),
  generateEvidence: vi.fn(async (_token: string, _id: string, _key: string) => evidence),
  content: vi.fn(
    async (_token: string, _id: string, _evidenceId: string) =>
      new Blob([new Uint8Array([137, 80, 78, 71])], { type: 'image/png' }),
  ),
});
function fixture(api: ReturnType<typeof defaults>, identityApi?: unknown) {
  TestBed.configureTestingModule({
    imports: [VerificationPage],
    providers: [
      provideHttpClient(),
      provideRouter([]),
      PrivateSessionState,
      {
        provide: IdentityApi,
        useValue: identityApi ?? {
          login: vi
            .fn()
            .mockResolvedValueOnce(login('a', 'ta'))
            .mockResolvedValueOnce(login('b', 'tb')),
          session: vi.fn().mockResolvedValueOnce(sessionA).mockResolvedValueOnce(sessionB),
          logout: vi.fn().mockResolvedValue(undefined),
        },
      },
      { provide: VerificationApi, useValue: api },
    ],
  });
  const f = TestBed.createComponent(VerificationPage);
  f.detectChanges();
  return { f, identity: TestBed.inject(IdentitySession) };
}
const page = (f: ReturnType<typeof TestBed.createComponent>) =>
  f.componentInstance as unknown as {
    load(): Promise<void>;
    select(v: VerificationCase): Promise<void>;
    createCase(): Promise<void>;
    retryCase(): Promise<void>;
    createEvidence(): Promise<void>;
    stateLabel(state: VerificationCase['state']): string;
    cases(): VerificationCase[];
    selected(): VerificationCase | null;
    evidenceUrl(): string | null;
    error(): string;
    notice(): string;
    eligibility(): VerificationEligibilityList | null;
  };

describe('VerificationPage', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    TestBed.resetTestingModule();
  });
  it('uses Go-confirmed status and effective eligibility, creates synthetic cases only', async () => {
    const api = defaults();
    const { f, identity } = fixture(api);
    await identity.signIn('a@example.invalid', 'secret');
    await page(f).load();
    expect(page(f).cases()[0].state).toBe('en_revision');
    expect(page(f).eligibility()?.items[0].eligible).toBe(false);
    expect(api.create).not.toHaveBeenCalled();
    await page(f).createCase();
    expect(api.create).toHaveBeenCalledWith('ta', 'kyc', expect.any(String));
    expect(api.generateEvidence).not.toHaveBeenCalled();
    f.destroy();
  });
  it('renders all server states and permits correction only after a Go rejection', async () => {
    const api = defaults();
    api.list.mockResolvedValue({
      items: [
        item('pending', 'en_revision'),
        item('approved', 'aprobada'),
        item('rejected', 'rechazada'),
        item('revoked', 'revocada'),
      ],
    });
    api.detail.mockImplementation(async (_token, id) =>
      item(
        id,
        id === 'rejected'
          ? 'rechazada'
          : id === 'approved'
            ? 'aprobada'
            : id === 'revoked'
              ? 'revocada'
              : 'en_revision',
      ),
    );
    const { f, identity } = fixture(api);
    await identity.signIn('a', 'pw');
    expect(
      ['en_revision', 'aprobada', 'rechazada', 'revocada'].map((state) =>
        page(f).stateLabel(state as VerificationCase['state']),
      ),
    ).toEqual(['Pendiente de revisión', 'Aprobada por Go', 'Rechazada por Go', 'Revocada por Go']);
    await page(f).select(item('approved', 'aprobada'));
    await page(f).retryCase();
    expect(api.retry).not.toHaveBeenCalled();
    await page(f).select(item('rejected', 'rechazada'));
    await page(f).retryCase();
    expect(api.retry).toHaveBeenCalledWith(
      'ta',
      'rejected',
      'fixture_vigente_actualizado',
      expect.any(String),
    );
    expect(api.generateEvidence.mock.calls[0]).toBeUndefined();
    expect((f.nativeElement as HTMLElement).querySelector('input[type="file"]')).toBeNull();
    f.destroy();
  });
  it('discards A list and a late A 401 after logout/login B; the B session stays active', async () => {
    const api = defaults();
    let rejectA!: (e: unknown) => void;
    api.list.mockImplementation((token: string) =>
      token === 'ta'
        ? new Promise((_resolve, reject) => (rejectA = reject))
        : Promise.resolve({ items: [item('v-b')] }),
    );
    const { f, identity } = fixture(api);
    await identity.signIn('a', 'pw');
    const pending = page(f).load();
    await identity.signOut();
    await identity.signIn('b', 'pw');
    await page(f).load();
    rejectA(new VerificationApiError(401));
    await pending;
    expect(identity.accessToken()).toBe('tb');
    expect(
      page(f)
        .cases()
        .map((v) => v.id),
    ).toEqual(['v-b']);
    f.destroy();
  });
  it('expires the current bearer after Go returns 401 and clears private state', async () => {
    const api = defaults();
    const { f, identity } = fixture(api);
    await identity.signIn('a', 'pw');
    await page(f).load();
    api.eligibility.mockRejectedValueOnce(new VerificationApiError(401));
    await page(f).load();
    expect(identity.accessToken()).toBeNull();
    expect(page(f).cases()).toEqual([]);
    expect(page(f).eligibility()).toBeNull();
    f.destroy();
  });
  it('uses only authorized PNG fixtures and revokes the temporary URL on selection change and destroy', async () => {
    const api = defaults();
    api.evidence.mockResolvedValue([evidence]);
    const create = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:private');
    const revoke = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => undefined);
    const { f, identity } = fixture(api);
    await identity.signIn('a', 'pw');
    await page(f).select(item('v-a'));
    expect(api.content).toHaveBeenCalledWith('ta', 'v-a', 'e-a');
    expect(page(f).evidenceUrl()).toBe('blob:private');
    await page(f).select(item('v-other'));
    expect(revoke).toHaveBeenCalledWith('blob:private');
    expect(create).toHaveBeenCalled();
    await identity.signOut();
    expect(revoke).toHaveBeenCalledTimes(create.mock.calls.length);
    f.destroy();
  });
  it('discards a late evidence response from A after switching to B', async () => {
    const api = defaults();
    api.evidence.mockImplementation(async (_token, id) => (id === 'v-a' ? [evidence] : []));
    let finish!: (blob: Blob) => void;
    api.content.mockImplementationOnce(() => new Promise((resolve) => (finish = resolve)));
    const create = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:stale');
    const { f, identity } = fixture(api);
    await identity.signIn('a', 'pw');
    const pending = page(f).select(item('v-a'));
    for (let attempt = 0; attempt < 10 && !api.content.mock.calls.length; attempt++)
      await Promise.resolve();
    await identity.signOut();
    await identity.signIn('b', 'pw');
    await page(f).select(item('v-b'));
    finish(new Blob(['stale'], { type: 'image/png' }));
    await pending;
    expect(page(f).selected()?.id).toBe('v-b');
    expect(page(f).evidenceUrl()).toBeNull();
    expect(create).not.toHaveBeenCalled();
    f.destroy();
  });
  it('keeps unavailable evidence empty and does not request content', async () => {
    const api = defaults();
    api.evidence.mockResolvedValue([{ ...evidence, file_available: false }]);
    const { f, identity } = fixture(api);
    await identity.signIn('a', 'pw');
    await page(f).select(item('v-a'));
    expect(api.content).not.toHaveBeenCalled();
    expect(page(f).evidenceUrl()).toBeNull();
    f.destroy();
  });
});
