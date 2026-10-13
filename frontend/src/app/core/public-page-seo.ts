import { DOCUMENT, isPlatformServer } from '@angular/common';
import { HttpErrorResponse } from '@angular/common/http';
import { Injectable, PLATFORM_ID, RESPONSE_INIT, inject } from '@angular/core';
import { Meta, Title } from '@angular/platform-browser';
import { ApiRuntimeConfig } from './api-runtime-config';

@Injectable({ providedIn: 'root' })
export class PublicPageSeo {
  private readonly document = inject(DOCUMENT);
  private readonly meta = inject(Meta);
  private readonly title = inject(Title);
  private readonly runtime = inject(ApiRuntimeConfig);
  private readonly response = inject(RESPONSE_INIT, { optional: true });
  private readonly server = isPlatformServer(inject(PLATFORM_ID));

  catalog(filtered: boolean): void {
    const indexable = !filtered;
    this.update({
      title: 'Explorar espacios | EspaciGo',
      description: 'Explora espacios publicados para reuniones, trabajo y eventos. Compara categorías y tarifas en CLP.',
      path: '/explorar',
      indexable,
    });
  }

  detail(input: { id: string; title: string; description: string; imageId?: string; contextual: boolean }): void {
    const id = encodeURIComponent(input.id);
    const image = input.imageId
      ? `${this.runtime.publicSiteOrigin() ?? ''}/public/spaces/${id}/images/${encodeURIComponent(input.imageId)}`
      : undefined;
    this.update({
      title: `${input.title} | EspaciGo`,
      description: summarize(input.description),
      path: `/espacios/${id}`,
      indexable: !input.contextual,
      image,
    });
  }

  unavailable(): void {
    this.setStatus(404);
    this.title.setTitle('Espacio no disponible | EspaciGo');
    this.robot('noindex, nofollow');
    this.meta.updateTag({ name: 'description', content: 'Este espacio ya no está disponible.' });
    this.meta.removeTag("property='og:image'");
    this.meta.removeTag("property='og:title'");
    this.meta.removeTag("property='og:description'");
    this.meta.removeTag("property='og:url'");
    this.setCanonical(null);
  }

  unavailableService(): void {
    this.setStatus(503);
    this.robot('noindex, nofollow');
  }

  responseForApiError(error: unknown): void {
    if (!this.server) return;
    const status = error instanceof HttpErrorResponse ? error.status : (error as { status?: number })?.status;
    if (status === 404) this.setStatus(404);
    else if (status === 422) this.setStatus(422);
    else if (!status || status >= 500) this.unavailableService();
    else this.setStatus(status);
  }

  private update(input: { title: string; description: string; path: string; indexable: boolean; image?: string }): void {
    const origin = this.runtime.publicSiteOrigin();
    if (!origin) {
      this.unavailableService();
      this.robot('noindex, nofollow');
      this.setCanonical(null);
      return;
    }
    const canonical = `${origin}${input.path}`;
    this.title.setTitle(input.title);
    this.meta.updateTag({ name: 'description', content: input.description });
    this.meta.updateTag({ property: 'og:type', content: 'website' });
    this.meta.updateTag({ property: 'og:title', content: input.title });
    this.meta.updateTag({ property: 'og:description', content: input.description });
    this.meta.updateTag({ property: 'og:url', content: canonical });
    this.meta.updateTag({ name: 'twitter:card', content: input.image ? 'summary_large_image' : 'summary' });
    if (input.image) this.meta.updateTag({ property: 'og:image', content: input.image });
    else this.meta.removeTag("property='og:image'");
    this.robot(input.indexable ? 'index, follow' : 'noindex, follow');
    this.setCanonical(canonical);
  }

  private robot(content: string): void {
    this.meta.updateTag({ name: 'robots', content });
  }

  private setCanonical(url: string | null): void {
    const existing = this.document.head.querySelector('link[rel="canonical"]');
    if (!url) {
      existing?.remove();
      return;
    }
    const link = existing ?? this.document.createElement('link');
    link.setAttribute('rel', 'canonical');
    link.setAttribute('href', url);
    if (!existing) this.document.head.appendChild(link);
  }

  private setStatus(status: number): void {
    if (this.server && this.response) this.response.status = status;
  }
}

function summarize(value: string): string {
  const normalized = value.replace(/\s+/g, ' ').trim();
  return [...normalized].slice(0, 160).join('');
}
