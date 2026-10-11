import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting, HttpTestingController } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { Readiness } from './readiness';
import { ApiRuntimeConfig } from './api-runtime-config';

describe('Readiness', () => {
  let service: Readiness;
  let http: HttpTestingController;
  let apiConfig: ApiRuntimeConfig;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideHttpClient(), provideHttpClientTesting()],
    });
    service = TestBed.inject(Readiness);
    http = TestBed.inject(HttpTestingController);
    apiConfig = TestBed.inject(ApiRuntimeConfig);
  });

  afterEach(() => http.verify());

  it('checks the configured Go readiness endpoint', async () => {
    const result = service.refresh();
    http.expectOne('/config.json').flush({ apiBaseUrl: 'http://localhost:8080/api/v1' });
    await vi.waitFor(() =>
      http.expectOne('http://localhost:8080/health/ready').flush({ status: 'ready' }),
    );
    await result;

    expect(service.status()).toBe('ready');
  });

  it('shows an unavailable state when the API cannot be reached', async () => {
    const result = service.refresh();
    http.expectOne('/config.json').flush({ apiBaseUrl: 'http://localhost:8080/api/v1' });
    await vi.waitFor(() =>
      http.expectOne('http://localhost:8080/health/ready').error(new ProgressEvent('error')),
    );
    await result;

    expect(service.status()).toBe('unavailable');
  });

  it('sets the generated API client base URL from the public runtime config', async () => {
    const result = apiConfig.load();
    http.expectOne('/config.json').flush({ apiBaseUrl: 'http://localhost:8080/api/v1' });
    await result;

    expect(apiConfig.apiBaseUrl()).toBe('http://localhost:8080/api/v1');
    expect(apiConfig.readinessUrl()).toBe('http://localhost:8080/health/ready');
  });
});
