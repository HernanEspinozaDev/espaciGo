import {
  ApplicationConfig,
  inject,
  PLATFORM_ID,
  provideAppInitializer,
  provideBrowserGlobalErrorListeners,
} from '@angular/core';
import { provideHttpClient, withFetch } from '@angular/common/http';
import { isPlatformBrowser } from '@angular/common';
import { provideRouter } from '@angular/router';
import { routes } from './app.routes';
import { provideClientHydration } from '@angular/platform-browser';
import { client } from '../generated/api/client.gen';
import { provideHeyApiClient } from '../generated/api/client/client.gen';
import { ApiRuntimeConfig } from './core/api-runtime-config';

export const appConfig: ApplicationConfig = {
  providers: [
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
