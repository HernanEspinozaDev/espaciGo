import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { provideFrontendPorts } from '../frontend.providers';
import { ApiRuntimeConfig } from './api-runtime-config';

describe('ApiRuntimeConfig', () => {
  let service: ApiRuntimeConfig;
  let http: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [...provideFrontendPorts(), provideHttpClient(), provideHttpClientTesting()],
    });
    service = TestBed.inject(ApiRuntimeConfig);
    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => http.verify());

  it('rejects a public API URL containing credentials', async () => {
    const result = service.load();
    http.expectOne('/config.json').flush({ apiBaseUrl: 'https://user:secret@example.test/api/v1' });

    await expect(result).rejects.toThrow('Invalid public API URL');
    expect(service.apiBaseUrl()).toBeNull();
  });
});
