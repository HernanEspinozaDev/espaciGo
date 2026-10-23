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
} from '../../core/contracts/models';
import { IdentitySession } from '../../core/identity/application/identity-session';
import { IdentityApi } from '../../core/identity/infrastructure/identity-api';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { provideFrontendPorts } from '../../frontend.providers';
import { VerificationFacade } from './application/verification-facade';
import { VerificationApi, VerificationApiError } from './infrastructure/verification-api';
import { VerificationPage } from './verification-page';

const sessionA = { account_id: 'a', roles: ['arrendatario'], capabilities: [], restricted_mode: false };
const sessionB = { account_id: 'b', roles: ['arrendador'], capabilities: [], restricted_mode: true };
const login = (account_id: string, access_token: string) => ({
  account_id,
  roles: account_id === 'a' ? ['arrendatario'] : ['arrendador'],
  access_token,
  token_type: 'Bearer',
  expires_at: '2027-01-01T00:00:00Z',
  capabilities: [],
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
  deleteEvidence: vi.fn(async (_token: string, _id: string, _evidenceId: string) => undefined),
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
      ...provideFrontendPorts(),
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
    deleteEvidence(entry: SyntheticEvidence): Promise<void>;
    stateLabel(state: VerificationCase['state']): string;
    cases(): VerificationCase[];
    selected(): VerificationCase | null;
    evidence(): SyntheticEvidence[];
    evidenceUrl(): string | null;
    error(): string;
    notice(): string;
    eligibility(): VerificationEligibilityList | null;
  };
async function ensureLoaded(f: ReturnType<typeof TestBed.createComponent>): Promise<void> {
  const view = page(f);
  for (let attempt = 0; attempt < 5 && !view.cases().length; attempt++) {
    await view.load();
    for (let turn = 0; turn < 5; turn++) await Promise.resolve();
  }
  expect(view.cases().length).toBeGreaterThan(0);
}

describe('VerificationPage', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    TestBed.resetTestingModule();
  });
  it('uses Go-confirmed status and effective eligibility, creates synthetic cases only', async () => {
    const api = defaults();
    const { f, identity } = fixture(api);
    await identity.signIn('a@example.invalid', 'secret');
    f.detectChanges();
    await ensureLoaded(f);
    expect([...f.nativeElement.querySelectorAll('.verification-index a')].map((link: HTMLAnchorElement) => link.getAttribute('href'))).toEqual([
      '#eligibility-heading',
      '#create-heading',
      '#cases-heading',
    ]);
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
    await ensureLoaded(f);
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
    await ensureLoaded(f);
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
    await ensureLoaded(f);
    api.eligibility.mockRejectedValueOnce(new VerificationApiError(401));
    await page(f).load();
    expect(identity.accessToken()).toBeNull();
    expect(page(f).cases()).toEqual([]);
    expect(page(f).eligibility()).toBeNull();
    f.destroy();
  });
  it('expires the current bearer when case A returns 401 after the user selects case B', async () => {
    const api = defaults();
    api.list.mockResolvedValue({ items: [item('v-a'), item('v-b')] });
    const { f, identity } = fixture(api);
    await identity.signIn('a', 'pw');
    await ensureLoaded(f);
    let rejectHistoryA!: (error: unknown) => void;
    api.history.mockImplementation((_token, id) =>
      id === 'v-a'
        ? new Promise((_resolve, reject) => (rejectHistoryA = reject))
        : Promise.resolve(emptyHistory),
    );
    const oldSelection = page(f).select(item('v-a'));
    const untilHistoryStarted = async () => {
      for (let attempt = 0; attempt < 10 && !rejectHistoryA; attempt++) await Promise.resolve();
    };
    await untilHistoryStarted();
    await page(f).select(item('v-b'));
    rejectHistoryA(new VerificationApiError(401));
    await oldSelection;
    expect(identity.accessToken()).toBeNull();
    expect(page(f).cases()).toEqual([]);
    f.destroy();
  });
  it('expires the current bearer when private PNG content returns a late 401 after selection changes', async () => {
    const api = defaults();
    api.list.mockResolvedValue({ items: [item('v-a'), item('v-b')] });
    api.evidence.mockImplementation(async (_token, id) => (id === 'v-a' ? [evidence] : []));
    const { f, identity } = fixture(api);
    await identity.signIn('a', 'pw');
    await ensureLoaded(f);
    let rejectContentA!: (error: unknown) => void;
    api.content.mockImplementationOnce(
      () => new Promise((_resolve, reject) => (rejectContentA = reject)),
    );
    const oldSelection = page(f).select(item('v-a'));
    for (let attempt = 0; attempt < 10 && !rejectContentA; attempt++) await Promise.resolve();
    await page(f).select(item('v-b'));
    rejectContentA(new VerificationApiError(401));
    await oldSelection;
    expect(identity.accessToken()).toBeNull();
    expect(page(f).selected()).toBeNull();
    f.destroy();
  });
  it('uses only authorized PNG fixtures and revokes the temporary URL on selection change and destroy', async () => {
    const api = defaults();
    api.evidence.mockResolvedValue([evidence]);
    const create = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:private');
    const revoke = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => undefined);
    const { f, identity } = fixture(api);
    await identity.signIn('a', 'pw');
    await ensureLoaded(f);
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
  it('asks for accessible confirmation, leaves cancel unchanged, and refreshes only after owner DELETE succeeds', async () => {
    const api = defaults();
    let removed = false;
    api.evidence.mockImplementation(async () => (removed ? [] : [evidence]));
    api.deleteEvidence.mockImplementation(async () => {
      removed = true;
    });
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false);
    const create = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:private');
    const revoke = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => undefined);
    const { f, identity } = fixture(api);
    await identity.signIn('a', 'pw');
    await ensureLoaded(f);
    await page(f).select(item('v-a'));
    expect(page(f).evidence()).toEqual([evidence]);
    await page(f).deleteEvidence(evidence);
    expect(confirm).toHaveBeenCalledWith(
      '¿Retirar este fixture PNG y sus metadatos del caso de verificación? Esta acción no elimina el volumen local.',
    );
    expect(api.deleteEvidence).not.toHaveBeenCalled();
    expect(page(f).selected()?.id).toBe('v-a');

    confirm.mockReturnValue(true);
    const historyReads = api.history.mock.calls.length;
    const deletion = page(f).deleteEvidence(evidence);
    await vi.waitFor(() => expect(api.deleteEvidence).toHaveBeenCalledWith('ta', 'v-a', 'e-a'));
    await vi.waitFor(() => expect(api.evidence.mock.calls.length).toBeGreaterThan(2));
    await deletion;
    expect(api.history.mock.calls.length).toBeGreaterThan(historyReads);
    expect(page(f).selected()?.id).toBe('v-a');
    expect(page(f).evidence()).toEqual([]);
    expect(revoke).toHaveBeenCalledWith('blob:private');
    expect(revoke).toHaveBeenCalledTimes(create.mock.calls.length);
    f.destroy();
  });
  it('discards a late owner DELETE after logout/login, even when the same account signs in again', async () => {
    const api = defaults();
    api.evidence.mockResolvedValue([evidence]);
    vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:old-owner');
    vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => undefined);
    let finishDelete!: () => void;
    api.deleteEvidence.mockImplementationOnce(
      () => new Promise<undefined>((resolve) => (finishDelete = () => resolve(undefined))),
    );
    const identityApi = {
      login: vi.fn().mockResolvedValueOnce(login('a', 'ta')).mockResolvedValueOnce(login('a', 'ta-new')),
      session: vi.fn().mockResolvedValue(sessionA),
      logout: vi.fn().mockResolvedValue(undefined),
    };
    const { f, identity } = fixture(api, identityApi);
    await identity.signIn('a', 'pw');
    await ensureLoaded(f);
    await page(f).select(item('v-a'));
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    page(f).deleteEvidence(evidence);
    for (let attempt = 0; attempt < 10 && !finishDelete; attempt++) await Promise.resolve();
    await identity.signOut();
    await identity.signIn('a', 'pw');
    await ensureLoaded(f);
    const newSession = identity.session();
    const historyAfterNewLogin = api.history.mock.calls.length;
    finishDelete();
    await vi.waitFor(() => expect(api.deleteEvidence).toHaveBeenCalledOnce());
    await Promise.resolve();
    expect(identity.accessToken()).toBe('ta-new');
    expect(identity.session()).toBe(newSession);
    expect(page(f).selected()?.id).toBe('v-a');
    expect(api.history.mock.calls.length).toBe(historyAfterNewLogin);
    expect(page(f).evidence()).toEqual([evidence]);
    expect(page(f).notice()).toBe('');
    f.destroy();
  });
  it('keeps the new account selection and evidence when an older account DELETE resolves late', async () => {
    const api = defaults();
    api.list.mockResolvedValue({ items: [item('v-a'), item('v-b')] });
    const evidenceB = { ...evidence, id: 'e-b', verification_id: 'v-b' };
    api.evidence.mockImplementation(async (_token, id) => (id === 'v-b' ? [evidenceB] : [evidence]));
    vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:stale-owner');
    vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => undefined);
    let finishDelete!: () => void;
    api.deleteEvidence.mockImplementationOnce(
      () => new Promise<undefined>((resolve) => (finishDelete = () => resolve(undefined))),
    );
    const { f, identity } = fixture(api);
    await identity.signIn('a', 'pw');
    await ensureLoaded(f);
    await page(f).select(item('v-a'));
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    page(f).deleteEvidence(evidence);
    for (let attempt = 0; attempt < 10 && !finishDelete; attempt++) await Promise.resolve();
    await identity.signOut();
    await identity.signIn('b', 'pw');
    await ensureLoaded(f);
    await page(f).select(item('v-b'));
    const newSession = identity.session();
    const historyAfterNewLogin = api.history.mock.calls.length;
    finishDelete();
    await Promise.resolve();
    expect(identity.accessToken()).toBe('tb');
    expect(identity.session()).toBe(newSession);
    expect(page(f).selected()?.id).toBe('v-b');
    expect(api.history.mock.calls.length).toBe(historyAfterNewLogin);
    expect(page(f).evidence()).toEqual([evidenceB]);
    expect(page(f).notice()).toBe('');
    f.destroy();
  });
  it.each([403, 404])('keeps owner data on DELETE %i and shows Go’s safe error', async (status) => {
    const api = defaults();
    api.evidence.mockResolvedValue([evidence]);
    vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:failed-delete');
    vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => undefined);
    api.deleteEvidence.mockRejectedValue(new VerificationApiError(status));
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    const { f, identity } = fixture(api);
    await identity.signIn('a', 'pw');
    await ensureLoaded(f);
    await page(f).select(item('v-a'));
    page(f).deleteEvidence(evidence);
    await vi.waitFor(() => expect(api.deleteEvidence).toHaveBeenCalledOnce());
    await vi.waitFor(() => expect(page(f).error()).not.toBe(''));
    expect(identity.accessToken()).toBe('ta');
    expect(page(f).selected()?.id).toBe('v-a');
    expect(page(f).evidence()).toEqual([evidence]);
    f.destroy();
  });
  it('expires only the current session after owner DELETE returns 401', async () => {
    const api = defaults();
    api.evidence.mockResolvedValue([evidence]);
    vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:expired-delete');
    vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => undefined);
    api.deleteEvidence.mockRejectedValue(new VerificationApiError(401));
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    const { f, identity } = fixture(api);
    await identity.signIn('a', 'pw');
    await ensureLoaded(f);
    await page(f).select(item('v-a'));
    page(f).deleteEvidence(evidence);
    await vi.waitFor(() => expect(api.deleteEvidence).toHaveBeenCalledOnce());
    await vi.waitFor(() => expect(identity.accessToken()).toBeNull());
    expect(page(f).selected()).toBeNull();
    expect(page(f).evidence()).toEqual([]);
    f.destroy();
  });
  it('does not submit a case’s evidence id from another verification', async () => {
    const api = defaults();
    const { f, identity } = fixture(api);
    await identity.signIn('a', 'pw');
    await ensureLoaded(f);
    await page(f).select(item('v-a'));
    await f.debugElement.injector
      .get(VerificationFacade)
      .deleteEvidence({ ...evidence, verification_id: 'v-other' });
    expect(api.deleteEvidence).not.toHaveBeenCalled();
    f.destroy();
  });
  it('discards a late evidence response from A after switching to B', async () => {
    const api = defaults();
    const { f, identity } = fixture(api);
    await identity.signIn('a', 'pw');
    await ensureLoaded(f);
    api.list.mockImplementation(async (token) => ({
      items: [item(token === 'tb' ? 'v-b' : 'v-a')],
    }));
    api.evidence.mockImplementation(async (_token, id) => (id === 'v-a' ? [evidence] : []));
    let finish!: (blob: Blob) => void;
    api.content.mockImplementationOnce(() => new Promise((resolve) => (finish = resolve)));
    const create = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:stale');
    const pending = page(f).select(item('v-a'));
    for (let attempt = 0; attempt < 20 && !api.content.mock.calls.length; attempt++)
      await new Promise((resolve) => setTimeout(resolve, 0));
    await identity.signOut();
    await identity.signIn('b', 'pw');
    await ensureLoaded(f);
    await page(f).select(item('v-b'));
    const urlsBeforeStaleResponse = create.mock.calls.length;
    finish(new Blob(['stale'], { type: 'image/png' }));
    await pending;
    expect(page(f).selected()?.id).toBe('v-b');
    expect(page(f).evidenceUrl()).toBeNull();
    expect(create).toHaveBeenCalledTimes(urlsBeforeStaleResponse);
    f.destroy();
  });
  it('does not select or query an A case with B after create reload is invalidated', async () => {
    const api = defaults();
    api.create.mockResolvedValue(item('created-a'));
    const { f, identity } = fixture(api);
    await identity.signIn('a', 'pw');
    await ensureLoaded(f);
    let finishAList!: (result: VerificationList) => void;
    api.list.mockImplementation((token) =>
      token === 'ta'
        ? new Promise((resolve) => (finishAList = resolve))
        : Promise.resolve({ items: [item('case-b')] }),
    );

    const creating = page(f).createCase();
    for (let attempt = 0; attempt < 10 && !finishAList; attempt++) await Promise.resolve();
    await identity.signOut();
    await identity.signIn('b', 'pw');
    finishAList({ items: [item('created-a')] });
    await creating;

    expect(api.detail).not.toHaveBeenCalledWith('tb', 'created-a');
    expect(api.history).not.toHaveBeenCalledWith('tb', 'created-a');
    expect(page(f).selected()?.id).not.toBe('created-a');
    f.destroy();
  });
  it('does not select or query an A retry result with B after retry reload is invalidated', async () => {
    const api = defaults();
    api.list.mockResolvedValue({ items: [item('rejected-a', 'rechazada')] });
    api.detail.mockImplementation(async (_token, id) =>
      item(id, id === 'rejected-a' ? 'rechazada' : 'en_revision'),
    );
    api.retry.mockResolvedValue(item('retry-a'));
    const { f, identity } = fixture(api);
    await identity.signIn('a', 'pw');
    await ensureLoaded(f);
    let finishAList!: (result: VerificationList) => void;
    api.list.mockImplementation((token) =>
      token === 'ta'
        ? new Promise((resolve) => (finishAList = resolve))
        : Promise.resolve({ items: [item('case-b')] }),
    );

    const retrying = page(f).retryCase();
    for (let attempt = 0; attempt < 10 && !finishAList; attempt++) await Promise.resolve();
    await identity.signOut();
    await identity.signIn('b', 'pw');
    finishAList({ items: [item('retry-a')] });
    await retrying;

    expect(api.detail).not.toHaveBeenCalledWith('tb', 'retry-a');
    expect(api.history).not.toHaveBeenCalledWith('tb', 'retry-a');
    expect(page(f).selected()?.id).not.toBe('retry-a');
    f.destroy();
  });
  it('keeps evidence writes and notices attached to the originating selection', async () => {
    const api = defaults();
    api.list.mockResolvedValue({ items: [item('v-a'), item('v-b')] });
    let confirmEvidence!: () => void;
    api.generateEvidence.mockImplementationOnce(
      () => new Promise((resolve) => (confirmEvidence = () => resolve(evidence))),
    );
    const { f, identity } = fixture(api);
    await identity.signIn('a', 'pw');
    await ensureLoaded(f);
    await page(f).select(item('v-a'));
    const detailCallsBeforeWrite = api.detail.mock.calls.length;
    const writing = page(f).createEvidence();
    await page(f).select(item('v-b'));
    confirmEvidence();
    await writing;
    expect(page(f).selected()?.id).toBe('v-b');
    expect(page(f).notice()).toBe('');
    expect(api.detail.mock.calls.slice(detailCallsBeforeWrite)).not.toContainEqual(['ta', 'v-a']);
    f.destroy();
  });
  it('keeps a pending correction attached to its originating selection', async () => {
    const api = defaults();
    api.list.mockResolvedValue({ items: [item('reject-a', 'rechazada'), item('v-b')] });
    api.detail.mockImplementation(async (_token, id) =>
      item(id, id === 'reject-a' ? 'rechazada' : 'en_revision'),
    );
    let confirmRetry!: (value: VerificationCase) => void;
    api.retry.mockImplementationOnce(() => new Promise((resolve) => (confirmRetry = resolve)));
    const { f, identity } = fixture(api);
    await identity.signIn('a', 'pw');
    await ensureLoaded(f);
    await page(f).select(item('reject-a', 'rechazada'));
    const detailCallsBeforeWrite = api.detail.mock.calls.length;
    const correcting = page(f).retryCase();
    await page(f).select(item('v-b'));
    confirmRetry(item('retry-a'));
    await correcting;
    expect(page(f).selected()?.id).toBe('v-b');
    expect(page(f).notice()).toBe('');
    expect(api.detail.mock.calls.slice(detailCallsBeforeWrite)).not.toContainEqual([
      'ta',
      'reject-a',
    ]);
    f.destroy();
  });
  it('retries an uncertain create with the same key and payload but changes key when type changes', async () => {
    const api = defaults();
    api.create.mockRejectedValueOnce(new Error('network timeout'));
    api.create.mockRejectedValueOnce(new Error('network timeout'));
    api.create.mockResolvedValue(item('created-a'));
    api.list.mockImplementation(async (_token) => ({ items: [item('v-a'), item('created-a')] }));
    const { f, identity } = fixture(api);
    await identity.signIn('a', 'pw');
    await ensureLoaded(f);
    await page(f).createCase();
    const kycKey = api.create.mock.calls[0][2];
    (
      f.componentInstance as unknown as { createType: { set(value: 'kyc' | 'kyb'): void } }
    ).createType.set('kyb');
    await page(f).createCase();
    const kybKey = api.create.mock.calls[1][2];
    (
      f.componentInstance as unknown as { createType: { set(value: 'kyc' | 'kyb'): void } }
    ).createType.set('kyc');
    await page(f).createCase();
    expect(api.create.mock.calls.map((call) => call[1])).toEqual(['kyc', 'kyb', 'kyc']);
    expect(api.create.mock.calls[2][2]).toBe(kycKey);
    expect(kycKey).not.toBe(kybKey);
    f.destroy();
  });
  it('retries uncertain correction and evidence writes with their original keys and payloads', async () => {
    const api = defaults();
    api.list.mockResolvedValue({ items: [item('reject-a', 'rechazada'), item('pending-a')] });
    api.detail.mockImplementation(async (_token, id) =>
      item(id, id === 'reject-a' ? 'rechazada' : 'en_revision'),
    );
    api.retry
      .mockRejectedValueOnce(new Error('network timeout'))
      .mockResolvedValue(item('retry-a'));
    api.generateEvidence
      .mockRejectedValueOnce(new Error('network timeout'))
      .mockResolvedValue(evidence);
    const { f, identity } = fixture(api);
    await identity.signIn('a', 'pw');
    await ensureLoaded(f);
    await page(f).select(item('reject-a', 'rechazada'));
    await page(f).retryCase();
    const retryKey = api.retry.mock.calls[0][3];
    await page(f).retryCase();
    expect(api.retry.mock.calls[1].slice(1, 3)).toEqual([
      'reject-a',
      'fixture_vigente_actualizado',
    ]);
    expect(api.retry.mock.calls[1][3]).toBe(retryKey);

    await page(f).select(item('pending-a'));
    await page(f).createEvidence();
    const evidenceKey = api.generateEvidence.mock.calls[0][2];
    await page(f).createEvidence();
    expect(api.generateEvidence.mock.calls[1].slice(1, 2)).toEqual(['pending-a']);
    expect(api.generateEvidence.mock.calls[1][2]).toBe(evidenceKey);
    f.destroy();
  });
  it('keeps the new uncertain create attempt when the previous same-account request succeeds late', async () => {
    const api = defaults();
    let finishOld!: (value: VerificationCase) => void;
    api.create
      .mockImplementationOnce(() => new Promise((resolve) => (finishOld = resolve)))
      .mockRejectedValueOnce(new Error('network timeout'))
      .mockResolvedValue(item('created-new'));
    const identityApi = {
      login: vi
        .fn()
        .mockResolvedValueOnce(login('a', 'ta'))
        .mockResolvedValueOnce(login('a', 'ta2')),
      session: vi.fn().mockResolvedValue(sessionA),
      logout: vi.fn().mockResolvedValue(undefined),
    };
    const { f, identity } = fixture(api, identityApi);
    await identity.signIn('a', 'pw');
    await ensureLoaded(f);
    const oldOperation = page(f).createCase();
    for (let attempt = 0; attempt < 10 && api.create.mock.calls.length < 1; attempt++)
      await Promise.resolve();

    await identity.signOut();
    await identity.signIn('a', 'pw');
    await ensureLoaded(f);
    await page(f).createCase();
    const newKey = api.create.mock.calls[1][2];
    const attempts = (
      f.debugElement.injector.get(VerificationFacade) as unknown as {
        writeAttempts: Map<string, unknown>;
      }
    ).writeAttempts;
    expect(JSON.stringify([...attempts.values()])).not.toContain('ta2');
    finishOld(item('created-old'));
    await oldOperation;
    await page(f).createCase();

    expect(api.create.mock.calls.map((call) => call[1])).toEqual(['kyc', 'kyc', 'kyc']);
    expect(api.create.mock.calls[2].slice(1)).toEqual(api.create.mock.calls[1].slice(1));
    expect(api.create.mock.calls[2][2]).toBe(newKey);
    f.destroy();
  });
  it('keeps the new uncertain create attempt when the previous same-account request fails definitively late', async () => {
    const api = defaults();
    let rejectOld!: (error: unknown) => void;
    api.create
      .mockImplementationOnce(() => new Promise((_resolve, reject) => (rejectOld = reject)))
      .mockRejectedValueOnce(new Error('network timeout'))
      .mockResolvedValue(item('created-new'));
    const identityApi = {
      login: vi
        .fn()
        .mockResolvedValueOnce(login('a', 'ta'))
        .mockResolvedValueOnce(login('a', 'ta2')),
      session: vi.fn().mockResolvedValue(sessionA),
      logout: vi.fn().mockResolvedValue(undefined),
    };
    const { f, identity } = fixture(api, identityApi);
    await identity.signIn('a', 'pw');
    await ensureLoaded(f);
    const oldOperation = page(f).createCase();
    for (let attempt = 0; attempt < 10 && api.create.mock.calls.length < 1; attempt++)
      await Promise.resolve();

    await identity.signOut();
    await identity.signIn('a', 'pw');
    await ensureLoaded(f);
    await page(f).createCase();
    const newKey = api.create.mock.calls[1][2];
    rejectOld(new VerificationApiError(422));
    await oldOperation;
    await page(f).createCase();

    expect(api.create.mock.calls.map((call) => call[1])).toEqual(['kyc', 'kyc', 'kyc']);
    expect(api.create.mock.calls[2].slice(1)).toEqual(api.create.mock.calls[1].slice(1));
    expect(api.create.mock.calls[2][2]).toBe(newKey);
    f.destroy();
  });
  it('keeps unavailable evidence empty and does not request content', async () => {
    const api = defaults();
    api.evidence.mockResolvedValue([{ ...evidence, file_available: false }]);
    const { f, identity } = fixture(api);
    await identity.signIn('a', 'pw');
    await ensureLoaded(f);
    await page(f).select(item('v-a'));
    expect(api.content).not.toHaveBeenCalled();
    expect(page(f).evidenceUrl()).toBeNull();
    f.destroy();
  });
});
