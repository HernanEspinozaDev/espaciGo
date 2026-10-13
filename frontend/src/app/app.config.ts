import { isPlatformBrowser } from '@angular/common';
import { provideHttpClient, withFetch } from '@angular/common/http';
import {
  ApplicationConfig,
  inject,
  PLATFORM_ID,
  provideAppInitializer,
  provideBrowserGlobalErrorListeners,
} from '@angular/core';
import { provideClientHydration } from '@angular/platform-browser';
import { provideRouter } from '@angular/router';
import { client } from '../generated/api/client.gen';
import { provideHeyApiClient } from '../generated/api/client/client.gen';
import { routes } from './app.routes';
import { ApiRuntimeConfig } from './core/api-runtime-config';
import { provideFrontendPorts } from './frontend.providers';

export const appConfig: ApplicationConfig = {
  providers: [
    ...provideFrontendPorts(),
    provideBrowserGlobalErrorListeners(),
    provideHttpClient(withFetch()),
    provideHeyApiClient(client),
    provideRouter(routes),
    provideClientHydration(),
    provideAppInitializer(() => {
      const platformId = inject(PLATFORM_ID);
      return isPlatformBrowser(platformId) ? inject(ApiRuntimeConfig).load() : undefined;
    }),
  ],
};
