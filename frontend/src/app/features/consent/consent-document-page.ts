import { Component, inject } from '@angular/core';
import { RouterLink } from '@angular/router';
import { ConsentDocumentFacade } from './consent-document-facade';

@Component({
  selector: 'app-consent-document-page',
  imports: [RouterLink],
  providers: [ConsentDocumentFacade],
  styleUrl: './consent-document-page.scss',
  template: `
    <main class="page-width legal-page">
      <a routerLink="/registro">← Volver a crear cuenta</a>
      @if (facade.state() === 'loading') {
        <p role="status">Cargando documento…</p>
      }
      @if (facade.state() === 'error') {
        <h1>Documento no disponible</h1>
        <p role="alert">No podemos mostrar esta versión. Inténtalo de nuevo antes de aceptar.</p>
        <button type="button" (click)="facade.load()">Volver a intentar</button>
      }
      @if (facade.item(); as item) {
        @if (facade.state() === 'ready') {
          <article>
            <p class="scope">{{ item.document.scope }}</p>
            <h1>{{ item.document.title }}</h1>
            <p class="status">{{ item.document.status }}</p>
            @for (section of item.document.sections; track section.heading) {
              <section>
                <h2>{{ section.heading }}</h2>
                <p>{{ section.body }}</p>
              </section>
            }
            <p class="version">Versión publicada: {{ item.published_at.slice(0, 10) }}</p>
          </article>
        }
      }
    </main>
  `,
})
export class ConsentDocumentPage {
  readonly facade = inject(ConsentDocumentFacade);
}
