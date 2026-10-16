import { DOCUMENT, isPlatformBrowser } from '@angular/common';
import { inject, InjectionToken, PLATFORM_ID } from '@angular/core';
import { CanActivateFn, RouterStateSnapshot } from '@angular/router';
import { ApiRuntimeConfig } from '../api-runtime-config';
import { parseIdentityReturn } from './contextual-return';

export const PRIVATE_ENTRY_DOCUMENT_NAVIGATOR = new InjectionToken<(url: string) => void>(
  'PRIVATE_ENTRY_DOCUMENT_NAVIGATOR',
);

export function privateEntryDocumentNavigator(): (url: string) => void {
  const document = inject(DOCUMENT);
  return (url) => document.defaultView?.location.assign(url);
}

export const privateEntryGuard: CanActivateFn = (_route, state) => {
  const platformId = inject(PLATFORM_ID);
  const runtime = inject(ApiRuntimeConfig);
  const navigator = inject(PRIVATE_ENTRY_DOCUMENT_NAVIGATOR);
  const privateOrigin = runtime.privateDemoOrigin();
  if (!isPlatformBrowser(platformId) || !privateOrigin) return true;
  const sourceOrigin = inject(DOCUMENT).defaultView?.location.origin;
  if (!sourceOrigin) {
    navigator(`${privateOrigin}/acceso`);
    return false;
  }
  if (sourceOrigin === privateOrigin) return true;
  const target = safePrivateTarget(state, sourceOrigin, privateOrigin);
  navigator(target);
  return false;
};

function safePrivateTarget(state: RouterStateSnapshot, sourceOrigin: string, privateOrigin: string): string {
  const raw = state.url;
  if (!raw.startsWith('/') || raw.startsWith('//') || raw.length > 4096) return `${privateOrigin}/acceso`;
  try {
    const requested = new URL(raw, sourceOrigin);
    if (requested.origin !== sourceOrigin || !isPrivatePath(requested.pathname)) return `${privateOrigin}/acceso`;
    const path = requested.pathname;
    const target = new URL(path, privateOrigin);
    const returns = requested.searchParams.getAll('returnUrl');
    if (returns.length === 1) {
      const parsed = parseIdentityReturn(returns);
      const value = typeof parsed === 'string' ? parsed : parsed?.identityReturnUrl;
      if (value) target.searchParams.set('returnUrl', value);
    }
    if (path === '/verificar-correo' || path === '/recuperar-acceso') {
      const tokenIds = requested.searchParams.getAll('tokenId');
      const tokens = requested.searchParams.getAll('token');
      if (tokenIds.length === 1 && /^[0-9a-f-]{36}$/i.test(tokenIds[0]) && tokens.length === 1 && /^[A-Za-z0-9_-]{16,2048}$/.test(tokens[0])) {
        target.searchParams.set('tokenId', tokenIds[0]);
        target.searchParams.set('token', tokens[0]);
      }
    }
    return target.toString();
  } catch {
    return `${privateOrigin}/acceso`;
  }
}

function isPrivatePath(path: string): boolean {
  return /^\/(acceso|registro|verificar-correo|recuperar-acceso|mi-cuenta|reservas|panel|panel-anfitrion|administracion)(\/|$)/.test(path);
}
