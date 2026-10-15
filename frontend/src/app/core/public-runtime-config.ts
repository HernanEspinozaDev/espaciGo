import { InjectionToken } from '@angular/core';

export const API_SERVER_BASE_URL = new InjectionToken<string | null>('API_SERVER_BASE_URL');
export const API_PUBLIC_BASE_URL = new InjectionToken<string | null>('API_PUBLIC_BASE_URL');
export const FRONTEND_PUBLIC_ORIGIN = new InjectionToken<string | null>('FRONTEND_PUBLIC_ORIGIN');

export function configuredApiBase(value: string | undefined): string | null {
  if (!value) return null;
  try {
    const url = new URL(value);
    if (
      !['http:', 'https:'].includes(url.protocol) ||
      url.username ||
      url.password ||
      url.search ||
      url.hash ||
      !/^\/api\/v1\/?$/.test(url.pathname)
    ) return null;
    return `${url.origin}/api/v1`;
  } catch {
    return null;
  }
}

export function configuredPublicApiBase(value: string | undefined): string | null {
  if (value?.trim() === '/api/v1' || value?.trim() === '/api/v1/') return '/api/v1';
  return configuredApiBase(value);
}

export function configuredPublicOrigin(value: string | undefined): string | null {
  if (!value) return null;
  try {
    const url = new URL(value);
    if (
      !['http:', 'https:'].includes(url.protocol) ||
      url.username ||
      url.password ||
      url.pathname !== '/' ||
      url.search ||
      url.hash
    ) return null;
    return url.origin;
  } catch {
    return null;
  }
}

export function publicTransferRequest(request: {
  method: string;
  url: string;
  headers?: { has(name: string): boolean };
}): boolean {
  if (request.method !== 'GET') return false;
  if (request.headers?.has('authorization') || request.headers?.has('cookie')) return false;
  try {
    const path = new URL(request.url).pathname;
    return (
      path === '/api/v1/public/spaces' ||
      /^\/api\/v1\/public\/spaces\/[0-9a-f-]{36}$/.test(path) ||
      path === '/api/v1/public/space-categories' ||
      /^\/api\/v1\/public\/space-categories\/[a-z0-9_-]+\/filters$/.test(path) ||
      path === '/api/v1/public/locations'
    );
  } catch {
    return false;
  }
}
