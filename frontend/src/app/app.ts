import { afterNextRender, Component, inject, signal } from '@angular/core';
import { RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { Readiness } from './core/readiness';
import { IdentitySession } from './core/identity/identity-session';
import { Router } from '@angular/router';

@Component({
  imports: [RouterLink, RouterLinkActive, RouterOutlet],
  selector: 'app-root',
  styleUrl: './app.scss',
  templateUrl: './app.html',
})
export class App {
  protected readonly menuOpen = signal(false);
  protected readonly readiness = inject(Readiness);
  protected readonly identity = inject(IdentitySession);
  private readonly router = inject(Router);

  constructor() {
    afterNextRender(() => this.readiness.refresh());
  }

  protected toggleMenu(): void {
    this.menuOpen.update((open) => !open);
  }

  protected closeMenu(): void {
    this.menuOpen.set(false);
  }

  protected async signOut(): Promise<void> {
    try {
      await this.identity.signOut();
      await this.router.navigateByUrl('/acceso');
    } catch { /* IdentitySession already reports the unconfirmed revocation. */ }
    this.closeMenu();
  }
}
