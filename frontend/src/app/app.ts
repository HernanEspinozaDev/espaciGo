import { FocusMonitor } from '@angular/cdk/a11y';
import { UiButton } from './shared/ui/button/button';
import { DOCUMENT, isPlatformBrowser } from '@angular/common';
import {
  afterNextRender,
  Component,
  DestroyRef,
  inject,
  Injector,
  PLATFORM_ID,
  signal,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { NavigationEnd, Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { filter } from 'rxjs/operators';
import { IdentitySession } from './core/identity/application/identity-session';
import { Readiness } from './core/readiness';

@Component({
  imports: [UiButton, RouterLink, RouterLinkActive, RouterOutlet],
  selector: 'app-root',
  host: {
    '(document:keydown.escape)': 'closeMenuOnEscape()',
    '(window:offline)': 'setOffline()',
    '(window:online)': 'setOnline()',
  },
  styleUrl: './app.scss',
  templateUrl: './app.html',
})
export class App {
  protected readonly menuOpen = signal(false);
  protected readonly offline = signal(false);
  protected readonly readiness = inject(Readiness);
  protected readonly identity = inject(IdentitySession);
  private readonly focusMonitor = inject(FocusMonitor);
  private readonly document = inject(DOCUMENT);
  private readonly platformId = inject(PLATFORM_ID);
  private readonly router = inject(Router);
  private readonly injector = inject(Injector);
  private readonly destroyRef = inject(DestroyRef);
  private navigationId = 0;

  constructor() {
    this.router.events
      .pipe(
        filter((event): event is NavigationEnd => event instanceof NavigationEnd),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe((event) => this.focusAfterNavigation(event));
    afterNextRender(() => {
      this.offline.set(!navigator.onLine);
      void this.readiness.refresh();
    });
  }

  protected toggleMenu(): void {
    this.menuOpen.update((open) => !open);
  }

  protected closeMenu(): void {
    this.menuOpen.set(false);
  }

  protected closeMenuOnEscape(): void {
    if (!this.menuOpen()) return;
    this.closeMenu();
    if (isPlatformBrowser(this.platformId)) {
      const trigger = this.document.getElementById('main-menu-toggle');
      if (trigger) this.focusMonitor.focusVia(trigger, 'keyboard');
    }
  }

  protected setOffline(): void {
    this.offline.set(true);
  }

  protected setOnline(): void {
    this.offline.set(false);
    void this.readiness.refresh();
  }

  private focusAfterNavigation(event: NavigationEnd): void {
    if (!isPlatformBrowser(this.platformId)) return;
    const navigationId = event.id;
    this.navigationId = navigationId;
    afterNextRender(
      () => {
        if (this.navigationId !== navigationId || this.router.url !== event.urlAfterRedirects)
          return;
        const heading = this.document.querySelector<HTMLElement>('#contenido h1');
        if (!heading) return;
        heading.tabIndex = -1;
        this.focusMonitor.focusVia(heading, 'program');
      },
      { injector: this.injector },
    );
  }

  protected async signOut(): Promise<void> {
    try {
      const isCurrentLogout = await this.identity.signOut();
      if (isCurrentLogout) await this.router.navigateByUrl('/acceso');
    } catch {
      /* IdentitySession owns local cleanup and reports revocation status. */
    }
    this.closeMenu();
  }
}
