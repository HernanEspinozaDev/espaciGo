import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { PLATFORM_ID } from '@angular/core';
import { provideFrontendPorts } from '../frontend.providers';
import { ApiRuntimeConfig } from './api-runtime-config';

describe('ApiRuntimeConfig', () => {
  let service: ApiRuntimeConfig;
  let http: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [
        ...provideFrontendPorts(), provideHttpClient(), provideHttpClientTesting(),
        { provide: PLATFORM_ID, useValue: 'browser' },
      ],
    });
    service = TestBed.inject(ApiRuntimeConfig);
    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => http.verify());

  it('rejects a public API URL containing credentials', async () => {
    const result = service.load();
    http.expectOne('/config.json').flush({
      apiBaseUrl: 'https://user:secret@example.test/api/v1',
      siteOrigin: 'https://example.test',
    });

    await expect(result).rejects.toThrow('Invalid public runtime configuration');
    expect(service.apiBaseUrl()).toBeNull();
  });

  it('loads a public API base and fixed site origin without accepting request hosts', async () => {
    const result = service.load();
    http.expectOne('/config.json').flush({
      apiBaseUrl: 'https://api.example.test/api/v1/',
      siteOrigin: 'https://spaces.example.test',
    });

    await result;
    expect(service.apiBaseUrl()).toBe('https://api.example.test/api/v1');
    expect(service.publicApiBaseUrl()).toBe('https://api.example.test/api/v1');
    expect(service.publicSiteOrigin()).toBe('https://spaces.example.test');
  });

  it('loads the same-origin gateway and resolves readiness on the frontend host', async () => {
    const result = service.load();
    http.expectOne('/config.json').flush({
      apiBaseUrl: '/api/v1',
      siteOrigin: 'https://spaces.example.test',
    });

    await result;
    expect(service.apiBaseUrl()).toBe('/api/v1');
    expect(service.publicApiBaseUrl()).toBe('/api/v1');
    expect(service.readinessUrl()).toBe('/health/ready');
  });
});
