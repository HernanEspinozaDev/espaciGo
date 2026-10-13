import { ApplicationConfig, inject, mergeApplicationConfig } from '@angular/core';
import { HTTP_TRANSFER_CACHE_ORIGIN_MAP } from '@angular/common/http';
import { provideServerRendering, withRoutes } from '@angular/ssr';
import { appConfig } from './app.config';
import { serverRoutes } from './app.routes.server';
import { API_PUBLIC_BASE_URL, API_SERVER_BASE_URL, FRONTEND_PUBLIC_ORIGIN, configuredApiBase, configuredPublicOrigin } from './core/public-runtime-config';

function transferOriginMap(): Record<string, string> {
  const internal = inject(API_SERVER_BASE_URL);
  const publicApi = inject(API_PUBLIC_BASE_URL);
  if (!internal || !publicApi) return {};
  const internalOrigin = new URL(internal).origin;
  const publicOrigin = new URL(publicApi).origin;
  return internalOrigin === publicOrigin ? {} : { [internalOrigin]: publicOrigin };
}

const serverConfig: ApplicationConfig = {
  providers: [
    provideServerRendering(withRoutes(serverRoutes)),
    { provide: API_SERVER_BASE_URL, useFactory: () => configuredApiBase(process.env['API_INTERNAL_BASE_URL']) },
    { provide: API_PUBLIC_BASE_URL, useFactory: () => configuredApiBase(process.env['API_PUBLIC_BASE_URL']) },
    { provide: FRONTEND_PUBLIC_ORIGIN, useFactory: () => configuredPublicOrigin(process.env['FRONTEND_PUBLIC_ORIGIN']) },
    { provide: HTTP_TRANSFER_CACHE_ORIGIN_MAP, useFactory: transferOriginMap },
  ],
};

export const config = mergeApplicationConfig(appConfig, serverConfig);
