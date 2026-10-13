import { UiButton } from '../../shared/ui/button/button';
import { Component, inject, signal } from '@angular/core';
import { Router, RouterLink } from '@angular/router';
import { categories } from './categories.data';

@Component({
  imports: [UiButton, RouterLink],
  selector: 'app-home',
  styleUrl: './home.scss',
  templateUrl: './home.html',
})
export class Home {
  private readonly router = inject(Router);
  protected readonly categories = categories;
  protected readonly searchText = signal('');

  protected search(): void {
    void this.router.navigate(['/explorar'], { queryParams: { q: this.searchText().trim() || null } });
  }
}
