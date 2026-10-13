import { TestBed } from '@angular/core/testing';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { client } from '../../../generated/api/client.gen';
import { IdentitySession } from '../../core/identity/application/identity-session';
import { IdentityApi } from '../../core/identity/infrastructure/identity-api';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { provideFrontendPorts } from '../../frontend.providers';
import { AdminApi, AdminApiError } from './infrastructure/admin-api';

const admin = { account_id: 'admin-a', roles: ['administrador'], restricted_mode: false };
const login = (token: string) => ({
  account_id: 'admin-a',
  roles: ['administrador' as const],
  access_token: token,
  token_type: 'Bearer' as const,
  expires_at: '2027-01-01T00:00:00Z',
  restricted_mode: false,
});

describe('AdminApi', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    TestBed.resetTestingModule();
  });

  function setup() {
    TestBed.configureTestingModule({
      providers: [
        ...provideFrontendPorts(),
        PrivateSessionState,
        {
          provide: IdentityApi,
          useValue: {
            login: vi.fn().mockResolvedValue(login('admin-token')),
            session: vi.fn().mockResolvedValue(admin),
            logout: vi.fn().mockResolvedValue(undefined),
          },
        },
      ],
    });
    return { api: TestBed.inject(AdminApi), identity: TestBed.inject(IdentitySession) };
  }

  it('expires only the Bearer that receives 401 and reports 403 without clearing the session', async () => {
    const { api, identity } = setup();
    await identity.signIn('admin@example.invalid', 'Synthetic#123');
    vi.spyOn(client, 'request').mockResolvedValue({
      data: undefined,
      error: {},
      response: { status: 401 },
    } as never);
    await expect(api.get('admin-token', '/admin/local/notices')).rejects.toMatchObject({
      status: 401,
    });
    expect(identity.accessToken()).toBeNull();

    await identity.signIn('admin@example.invalid', 'Synthetic#123');
    vi.mocked(client.request).mockResolvedValue({
      data: undefined,
      error: {},
      response: { status: 403 },
    } as never);
    await expect(api.get('admin-token', '/admin/local/notices')).rejects.toBeInstanceOf(
      AdminApiError,
    );
    await vi.waitFor(() => expect(api.capabilitiesRevision()).toBe(1));
    expect(identity.accessToken()).toBe('admin-token');
  });

  it('ignores a stale 403 after another account has signed in', async () => {
    const { api, identity } = setup();
    await identity.signIn('admin@example.invalid', 'Synthetic#123');
    const identityApi = TestBed.inject(IdentityApi);
    vi.mocked(identityApi.login).mockResolvedValueOnce({
      ...login('token-b'),
      account_id: 'admin-b',
    });
    vi.mocked(identityApi.session).mockResolvedValueOnce({
      account_id: 'admin-b',
      roles: ['arrendatario'],
      restricted_mode: true,
    });
    await identity.signOut();
    await identity.signIn('member@example.invalid', 'Synthetic#456');
    vi.spyOn(client, 'request').mockResolvedValue({
      data: undefined,
      error: {},
      response: { status: 403 },
    } as never);
    await expect(api.get('admin-token', '/admin/local/notices')).rejects.toMatchObject({
      status: 403,
    });
    await Promise.resolve();
    expect(identity.accessToken()).toBe('token-b');
    expect(identity.session()?.account_id).toBe('admin-b');
    expect(api.permissionDeniedRevision()).toBe(0);
    expect(api.capabilitiesRevision()).toBe(0);
  });

  it('keeps an uncertain idempotent attempt bound to its account and original body', async () => {
    const { api } = setup();
    const request = vi
      .spyOn(client, 'request')
      .mockResolvedValueOnce({ data: undefined, error: {}, response: { status: 503 } } as never)
      .mockResolvedValueOnce({
        data: { data: { state: 'pendiente' } },
        error: undefined,
        response: { status: 200 },
      } as never);
    await expect(
      api.post(
        'token',
        'admin-a',
        '/admin/local/notices/id/reopen',
        { reason_code: 'reintento_operativo' },
        'notice:id',
      ),
    ).rejects.toMatchObject({ status: 503 });
    await expect(
      api.post(
        'token',
        'admin-a',
        '/admin/local/notices/id/reopen',
        { reason_code: 'smtp_restaurado' },
        'notice:id',
      ),
    ).rejects.toMatchObject({ status: 409 });
    await expect(
      api.post(
        'token',
        'admin-a',
        '/admin/local/notices/id/reopen',
        { reason_code: 'reintento_operativo' },
        'notice:id',
      ),
    ).resolves.toBeTruthy();
    expect(request).toHaveBeenCalledTimes(2);
    const firstHeaders = request.mock.calls[0]?.[0] as { headers?: Record<string, string> };
    const secondHeaders = request.mock.calls[1]?.[0] as { headers?: Record<string, string> };
    expect(firstHeaders.headers?.['Idempotency-Key']).toBe(
      secondHeaders.headers?.['Idempotency-Key'],
    );
  });

  it('replays the full original uncertain payload, including its correlation_id', async () => {
    const { api } = setup();
    const request = vi
      .spyOn(client, 'request')
      .mockResolvedValueOnce({ data: undefined, error: {}, response: { status: 503 } } as never)
      .mockResolvedValueOnce({
        data: { accepted: true },
        error: undefined,
        response: { status: 200 },
      } as never);
    const original = { reason_code: 'smtp_restaurado', correlation_id: 'correlation-original' };
    await expect(
      api.post('admin-token', 'admin-a', '/admin/local/notices/id/reopen', original, 'notice:id'),
    ).rejects.toMatchObject({ status: 503 });
    await expect(
      api.post(
        'admin-token',
        'admin-a',
        '/admin/local/notices/id/reopen',
        { reason_code: 'otro_motivo', correlation_id: 'new-correlation' },
        'notice:id',
      ),
    ).rejects.toMatchObject({ status: 409 });
    await expect(
      api.post(
        'admin-token',
        'admin-a',
        '/admin/local/notices/id/reopen',
        { ...original, correlation_id: 'correlation-regenerated' },
        'notice:id',
      ),
    ).resolves.toBeTruthy();
    const first = request.mock.calls[0]?.[0] as {
      body?: unknown;
      headers?: Record<string, string>;
    };
    const retry = request.mock.calls[1]?.[0] as {
      body?: unknown;
      headers?: Record<string, string>;
    };
    expect(first.body).toEqual(original);
    expect(retry.body).toEqual(original);
    expect(retry.headers?.['Idempotency-Key']).toBe(first.headers?.['Idempotency-Key']);
    expect(request).toHaveBeenCalledTimes(2);
  });

  it('revalidates Go capabilities after a current 403 without expiring the valid bearer', async () => {
    const { api, identity } = setup();
    await identity.signIn('admin@example.invalid', 'Synthetic#123');
    const identityApi = TestBed.inject(IdentityApi);
    vi.mocked(identityApi.session).mockResolvedValueOnce({
      account_id: 'admin-a',
      roles: ['arrendatario'],
      restricted_mode: false,
    });
    vi.spyOn(client, 'request').mockResolvedValue({
      data: undefined,
      error: {},
      response: { status: 403 },
    } as never);
    await expect(api.get('admin-token', '/admin/local/notices')).rejects.toMatchObject({
      status: 403,
    });
    await vi.waitFor(() => expect(identity.session()?.roles).toEqual(['arrendatario']));
    expect(identity.accessToken()).toBe('admin-token');
    expect(identity.status()).toBe('authenticated');
    expect(api.permissionDeniedRevision()).toBe(1);
    expect(api.capabilitiesRevision()).toBe(1);
  });
});
