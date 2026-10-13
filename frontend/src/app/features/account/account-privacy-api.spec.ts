import { HttpClient, provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { describe, expect, it, vi } from 'vitest';
import { client } from '../../../generated/api/client.gen';
import { provideHeyApiClient } from '../../../generated/api/client/client.gen';
import { M02Local } from '../../../generated/api/sdk.gen';
import { ApiRuntimeConfig } from '../../core/api-runtime-config';
import { provideFrontendPorts } from '../../frontend.providers';
import { AccountPrivacyApi } from './infrastructure/account-privacy-api';

describe('AccountPrivacyApi', () => {
  it('sends profile reads through the generated client with the Bearer header', async () => {
    TestBed.configureTestingModule({
      providers: [
        ...provideFrontendPorts(),
        provideHttpClient(),
        provideHttpClientTesting(),
        provideHeyApiClient(client),
      ],
    });
    client.setConfig({ baseUrl: 'http://api.test/api/v1', httpClient: TestBed.inject(HttpClient) });
    const api = TestBed.inject(AccountPrivacyApi);
    const http = TestBed.inject(HttpTestingController);
    const profilePromise = api.profile('bearer-owner');
    await new Promise((resolve) => setTimeout(resolve, 0));
    const request = http.expectOne('http://api.test/api/v1/profile');
    expect(request.request.headers.get('Authorization')).toBe('Bearer bearer-owner');
    request.flush({
      display_name: 'Titular autorizado',
      phone: null,
      updated_at: '2026-10-01T00:00:00Z',
    });
    expect(await profilePromise).toMatchObject({ display_name: 'Titular autorizado' });
    http.verify();
    client.setConfig({ baseUrl: '/api/v1', httpClient: undefined });
    TestBed.resetTestingModule();
  });

  it('attaches the in-memory Bearer to generated profile and rights operations', async () => {
    const profile = { display_name: 'Titular A', phone: null, updated_at: '2026-10-01T00:00:00Z' };
    const request = {
      id: 'request-a',
      kind: 'supresion',
      state: 'en_revision',
      created_at: '2026-10-01T00:00:00Z',
    };
    const m02 = {
      getOwnProfile: vi.fn().mockResolvedValue({ data: profile, error: undefined }),
      listOwnRightsRequests: vi
        .fn()
        .mockResolvedValue({ data: { items: [request] }, error: undefined }),
      requestDataRight: vi.fn().mockResolvedValue({ data: request, error: undefined }),
      generateOwnSyntheticProfilePhoto: vi
        .fn()
        .mockResolvedValue({ data: { synthetic: true }, error: undefined }),
      createOrReplaceOwnSyntheticPayoutAccount: vi
        .fn()
        .mockResolvedValue({
          data: {
            account: {
              id: 'payout',
              adapter: 'fake-local-v1',
              reference: 'demo-test',
              state: 'activa',
              updated_at: '2026-10-01T00:00:00Z',
            },
          },
          error: undefined,
        }),
    };
    TestBed.configureTestingModule({
      providers: [
        ...provideFrontendPorts(),
        provideHttpClient(),
        { provide: M02Local, useValue: m02 },
      ],
    });
    const api = TestBed.inject(AccountPrivacyApi);
    expect(await api.profile('bearer-a')).toEqual(profile);
    expect(await api.rightsRequests('bearer-a')).toEqual({ items: [request] });
    expect(await api.requestRight('bearer-a', 'supresion')).toEqual(request);
    await api.generatePhoto('bearer-a', 'photo-idem-key-01');
    expect(await api.createPayoutAccount('bearer-a', 'payout-idem-key-01')).toMatchObject({
      reference: 'demo-test',
    });
    for (const operation of [m02.getOwnProfile, m02.listOwnRightsRequests, m02.requestDataRight]) {
      expect(operation.mock.calls[0][0].headers.Authorization).toBe('Bearer bearer-a');
    }
    expect(m02.requestDataRight.mock.calls[0][0].body).toEqual({ type: 'supresion' });
    expect(m02.generateOwnSyntheticProfilePhoto.mock.calls[0][0].headers).toEqual({
      Authorization: 'Bearer bearer-a',
      'Idempotency-Key': 'photo-idem-key-01',
    });
    expect(m02.createOrReplaceOwnSyntheticPayoutAccount.mock.calls[0][0].headers).toEqual({
      Authorization: 'Bearer bearer-a',
      'Idempotency-Key': 'payout-idem-key-01',
    });
    expect(m02.createOrReplaceOwnSyntheticPayoutAccount.mock.calls[0][0].body).toBeUndefined();
    TestBed.resetTestingModule();
  });

  it('downloads profile PNG and ZIP through authenticated Blob requests', async () => {
    TestBed.configureTestingModule({
      providers: [...provideFrontendPorts(), provideHttpClient(), provideHttpClientTesting()],
    });
    const runtime = TestBed.inject(ApiRuntimeConfig);
    runtime.apiBaseUrl.set('http://api.test/api/v1');
    const api = TestBed.inject(AccountPrivacyApi);
    const http = TestBed.inject(HttpTestingController);
    const photoPromise = api.photoContent('bearer-a');
    const photoRequest = http.expectOne('http://api.test/api/v1/profile/photo/content');
    expect(photoRequest.request.headers.get('Authorization')).toBe('Bearer bearer-a');
    photoRequest.flush(new Blob([new Uint8Array([137, 80, 78, 71])], { type: 'image/png' }));
    expect(new Uint8Array(await (await photoPromise).arrayBuffer())).toEqual(
      new Uint8Array([137, 80, 78, 71]),
    );

    const archivePromise = api.archive('bearer-a');
    const archiveRequest = http.expectOne('http://api.test/api/v1/privacy/export/archive');
    expect(archiveRequest.request.headers.get('Authorization')).toBe('Bearer bearer-a');
    const zip = new Blob([new Uint8Array([0x50, 0x4b, 0x03, 0x04]), 'manifest.json']);
    archiveRequest.flush(zip, {
      headers: { 'Content-Disposition': 'attachment; filename=own-data.zip' },
    });
    const response = await archivePromise;
    expect(response.body).toBeInstanceOf(Blob);
    expect(new Uint8Array(await response.body!.arrayBuffer()).slice(0, 4)).toEqual(
      new Uint8Array([0x50, 0x4b, 0x03, 0x04]),
    );
    http.verify();
    TestBed.resetTestingModule();
  });
});
