import { Component, inject } from '@angular/core';
import { RouterLink } from '@angular/router';
import { UiButton } from '../../shared/ui/button/button';
import { ConsentDocumentFacade } from './consent-document-facade';

@Component({
  selector: 'app-consent-document-page',
  imports: [RouterLink, UiButton],
  providers: [ConsentDocumentFacade],
  styleUrl: './consent-document-page.scss',
  template: `
    <main class="page-width legal-page" [attr.aria-busy]="facade.state() === 'loading'">
      <a class="back-link" routerLink="/registro">← Volver a crear cuenta</a>
      @if (facade.state() === 'loading') {
        <p class="document-loading" role="status">Cargando documento…</p>
      }
      @if (facade.state() === 'error') {
        <section class="document-error" role="alert" aria-labelledby="document-error-title">
          <p class="eyebrow">DOCUMENTO DE CUENTA</p>
          <h1 id="document-error-title">Documento no disponible</h1>
          <p>No podemos mostrar esta versión. Inténtalo de nuevo antes de aceptar.</p>
          <button appButton="secondary" type="button" (click)="facade.load()">Volver a intentar</button>
        </section>
      }
      @if (facade.item(); as item) {
        @if (facade.state() === 'ready') {
          <header class="legal-heading">
            <p class="eyebrow">DOCUMENTO DE CUENTA · {{ item.document.scope }}</p>
            <h1 id="legal-title">{{ item.document.title }}</h1>
            <p>Revisa la versión vigente antes de continuar con la creación de tu cuenta.</p>
          </header>
          <div class="legal-meta">
            <span class="document-status">{{ item.document.status }}</span>
            <p>Versión publicada <time [attr.datetime]="item.published_at">{{ item.published_at.slice(0, 10) }}</time></p>
          </div>
          <div class="legal-layout">
            @if (item.document.sections.length) {
              <nav class="legal-index" aria-label="Secciones del documento">
                <h2>En este documento</h2>
                <ol>
                  @for (section of item.document.sections; track section.heading; let index = $index) {
                    <li><a [href]="'#legal-section-' + index">{{ section.heading }}</a></li>
                  }
                </ol>
              </nav>
            }
            <article aria-labelledby="legal-title">
              @for (section of item.document.sections; track section.heading; let index = $index) {
                <section [id]="'legal-section-' + index">
                  <h2>{{ section.heading }}</h2>
                  <p>{{ section.body }}</p>
                </section>
              }
            </article>
          </div>
        }
      }
    </main>
  `,
})
export class ConsentDocumentPage {
  readonly facade = inject(ConsentDocumentFacade);
}
