import { UiButton } from '../../shared/ui/button/button';
import { Component, ElementRef, HostListener, computed, inject, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { ExploreFacade } from './application/explore-facade';

@Component({
  providers: [ExploreFacade],
  selector: 'app-explore-page',
  imports: [UiButton, RouterLink],
  templateUrl: './explore-page.html',
  styleUrl: './explore-page.scss',
})
export class ExplorePage {
  private readonly host = inject(ElementRef<HTMLElement>);
  private readonly facade = inject(ExploreFacade);
  readonly filtersOpen = signal(false);
  private filterTrigger: HTMLElement | null = null;
  readonly activeFilters = computed(() => this.buildActiveFilters());
  get categories() {
    return this.facade.categories;
  }
  get locations() { return this.facade.locations; }
  get attributeFilters() { return this.facade.attributeFilters; }
  get items() {
    return this.facade.items;
  }
  get nextCursor() {
    return this.facade.nextCursor;
  }
  get busy() {
    return this.facade.busy;
  }
  get error() {
    return this.facade.error;
  }
  get filters() {
    return this.facade.filters;
  }
  ngOnInit(): void {
    this.facade.ngOnInit();
  }
  readonly updateFilter = this.facade.updateFilter.bind(this.facade);
  readonly setAttribute = this.facade.setAttribute.bind(this.facade);
  readonly updateDate = this.facade.updateDate.bind(this.facade);
  readonly dateInput = this.facade.dateInput.bind(this.facade);
  readonly booleanValue = (value: boolean | undefined): string => value === undefined ? '' : String(value);
  readonly search = this.facade.search.bind(this.facade);
  readonly clearFilters = this.facade.clearFilters.bind(this.facade);
  readonly loadMore = this.facade.loadMore.bind(this.facade);
  readonly price = this.facade.price.bind(this.facade);

  openFilters(trigger: HTMLElement): void {
    this.filterTrigger = trigger;
    this.filtersOpen.set(true);
    setTimeout(() => ((this.host.nativeElement as HTMLElement).querySelector('.filter-close') as HTMLElement | null)?.focus());
  }

  closeFilters(): void {
    this.filtersOpen.set(false);
    setTimeout(() => this.filterTrigger?.focus());
  }

  removeFilter(key: string): void {
    if (key.startsWith('attribute:')) this.setAttribute(key.slice(10), null);
    else if (key === 'region_code') { this.updateFilter('region_code', ''); this.updateFilter('comuna_code', ''); }
    else this.updateFilter(key as Parameters<typeof this.updateFilter>[0], '');
    void this.search();
  }

  private buildActiveFilters(): { key: string; label: string }[] {
    const f = this.filters();
    const labels: Record<string, string> = {
      q: 'Búsqueda', category_code: 'Categoría', region_code: 'Región', comuna_code: 'Comuna', rate_unit: 'Tarifa',
      min_unit_price_clp: 'Precio desde', max_unit_price_clp: 'Precio hasta', min_area_m2: 'Superficie mínima',
      start_at: 'Desde', end_at: 'Hasta', min_total_clp: 'Total estimado desde', max_total_clp: 'Total estimado hasta',
    };
    const active = Object.entries(labels).filter(([key]) => Boolean(f[key as keyof typeof f])).map(([key, label]) => {
      const value = String(f[key as keyof typeof f]);
      const category = this.categories().find((item) => item.code === value)?.name;
      const location = this.locations().find((item) => item.region_code === value || item.comuna_code === value);
      const attribute = key === 'start_at' || key === 'end_at' ? this.dateInput(value).replace('T', ' ') : '';
      const formatted = category ?? location?.region_name ?? location?.comuna_name ?? (key.includes('price') || key.includes('total') ? this.price(value) : attribute || value);
      return { key, label: `${label}: ${formatted}` };
    });
    return [...active, ...Object.entries(f.attributes).map(([key, value]) => ({ key: `attribute:${key}`, label: `${this.attributeFilters().find((filter) => filter.code === key)?.label ?? key}: ${value ? 'Sí' : 'No'}` }))];
  }

  @HostListener('document:keydown', ['$event'])
  onDialogKeydown(event: KeyboardEvent): void {
    if (!this.filtersOpen()) return;
    if (event.key === 'Escape') { event.preventDefault(); this.closeFilters(); return; }
    if (event.key !== 'Tab') return;
    const focusable = [...((this.host.nativeElement as HTMLElement).querySelector('.filters-open')?.querySelectorAll<HTMLElement>('button:not(:disabled), input:not(:disabled), select:not(:disabled)') ?? [])];
    if (!focusable.length) return;
    const first = focusable[0]; const last = focusable.at(-1);
    if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
    else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
  }
}
