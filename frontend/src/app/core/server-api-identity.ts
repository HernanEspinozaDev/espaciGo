import { isPlatformServer } from '@angular/common';
import { HttpInterceptorFn } from '@angular/common/http';
import { inject, InjectionToken, PLATFORM_ID } from '@angular/core';
import { from, switchMap } from 'rxjs';
import { API_SERVER_BASE_URL } from './public-runtime-config';

export const SERVER_API_IDENTITY_HEADERS = new InjectionToken<() => Promise<Record<string, string>>>(
  'SERVER_API_IDENTITY_HEADERS',
);
const uuid = '[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}';
const publicReadPaths = [
  /^\/public\/spaces$/,
  new RegExp(`^/public/spaces/${uuid}$`, 'i'),
  /^\/public\/space-categories$/,
  /^\/public\/space-categories\/[a-z0-9_-]+\/filters$/,
  /^\/public\/locations$/,
];

export const serverApiIdentityInterceptor: HttpInterceptorFn = (request, next) => {
  if (!isPlatformServer(inject(PLATFORM_ID))) return next(request);
  const base = inject(API_SERVER_BASE_URL, { optional: true });
  const identityHeaders = inject(SERVER_API_IDENTITY_HEADERS, { optional: true });
  if (!base || !identityHeaders) return next(request);
  if (request.method !== 'GET' || request.headers.has('authorization') || request.headers.has('cookie')) {
    return next(request);
  }
  try {
    const api = new URL(base);
    const target = new URL(request.url, 'http://angular.invalid');
    const apiPath = api.pathname.replace(/\/$/, '');
    if (
      target.origin !== api.origin ||
      target.username ||
      target.password ||
      (target.pathname !== apiPath && !target.pathname.startsWith(`${apiPath}/`))
    ) return next(request);
    const publicPath = target.pathname.slice(apiPath.length);
    const allowed = publicReadPaths.some((path) => path.test(publicPath));
    if (!allowed) return next(request);
  } catch {
    return next(request);
  }
  return from(identityHeaders()).pipe(
    switchMap((headers) => next(request.clone({
      setHeaders: headers,
      transferCache: false,
    }))),
  );
};
