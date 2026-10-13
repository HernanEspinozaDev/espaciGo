import { configuredApiBase, configuredPublicOrigin, publicTransferRequest } from './public-runtime-config';

describe('public SSR runtime configuration', () => {
  it('accepts only an HTTP API base at the versioned API path', () => {
    expect(configuredApiBase('http://backend:8080/api/v1/')).toBe('http://backend:8080/api/v1');
    expect(configuredApiBase('https://api.example.test/api/v1')).toBe('https://api.example.test/api/v1');
    expect(configuredApiBase('https://user:secret@api.example.test/api/v1')).toBeNull();
    expect(configuredApiBase('http://api.example.test/private')).toBeNull();
    expect(configuredApiBase('javascript:alert(1)')).toBeNull();
  });

  it('accepts only a fixed public origin without path or credentials', () => {
    expect(configuredPublicOrigin('https://spaces.example.test')).toBe('https://spaces.example.test');
    expect(configuredPublicOrigin('http://127.0.0.1:4200')).toBe('http://127.0.0.1:4200');
    expect(configuredPublicOrigin('https://user:secret@spaces.example.test')).toBeNull();
    expect(configuredPublicOrigin('https://spaces.example.test/path')).toBeNull();
    expect(configuredPublicOrigin('https://spaces.example.test?next=evil')).toBeNull();
  });

  it('transfers only anonymous JSON public reads and excludes photos and private API calls', () => {
    const anonymous = { method: 'GET', url: 'http://backend:8080/api/v1/public/spaces?sort=default' };
    expect(publicTransferRequest(anonymous)).toBe(true);
    expect(publicTransferRequest({ ...anonymous, method: 'POST' })).toBe(false);
    expect(publicTransferRequest({ ...anonymous, url: 'http://backend:8080/api/v1/public/spaces/123/images/456' })).toBe(false);
    expect(publicTransferRequest({ ...anonymous, url: 'http://backend:8080/api/v1/reservations' })).toBe(false);
    expect(publicTransferRequest({
      ...anonymous,
      headers: { has: (name: string) => name === 'authorization' },
    })).toBe(false);
    expect(publicTransferRequest({
      ...anonymous,
      headers: { has: (name: string) => name === 'cookie' },
    })).toBe(false);
  });
});
