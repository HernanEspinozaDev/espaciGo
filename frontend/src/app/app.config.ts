import { isPlatformBrowser } from '@angular/common';
import { provideHttpClient, withFetch, withInterceptors } from '@angular/common/http';
import {
  ApplicationConfig,
  inject,
  PLATFORM_ID,
  provideAppInitializer,
  provideBrowserGlobalErrorListeners,
} from '@angular/core';
import { provideClientHydration, withHttpTransferCacheOptions } from '@angular/platform-browser';
import { provideRouter } from '@angular/router';
import { client } from '../generated/api/client.gen';
import { provideHeyApiClient } from '../generated/api/client/client.gen';
import { routes } from './app.routes';
import { ApiRuntimeConfig } from './core/api-runtime-config';
import { publicTransferRequest } from './core/public-runtime-config';
import { serverApiIdentityInterceptor } from './core/server-api-identity';
import { PRIVATE_ENTRY_DOCUMENT_NAVIGATOR, privateEntryDocumentNavigator } from './core/identity/private-entry.guard';
import { provideFrontendPorts } from './frontend.providers';

export const appConfig: ApplicationConfig = {
  providers: [
    ...provideFrontendPorts(),
    provideBrowserGlobalErrorListeners(),
    provideHttpClient(withFetch(), withInterceptors([serverApiIdentityInterceptor])),
    { provide: PRIVATE_ENTRY_DOCUMENT_NAVIGATOR, useFactory: privateEntryDocumentNavigator },
    provideHeyApiClient(client),
    provideRouter(routes),
    provideClientHydration(withHttpTransferCacheOptions({
      filter: publicTransferRequest,
      includeNonCacheableRequests: true,
    })),
    provideAppInitializer(() => {
      const platformId = inject(PLATFORM_ID);
      return isPlatformBrowser(platformId) ? inject(ApiRuntimeConfig).load() : undefined;
    }),
  ],
};
