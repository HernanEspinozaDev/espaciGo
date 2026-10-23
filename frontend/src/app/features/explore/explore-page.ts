import { UiButton } from '../../shared/ui/button/button';
import { DOCUMENT } from '@angular/common';
import { Component, ElementRef, computed, inject, signal } from '@angular/core';
import { NgOptimizedImage } from '@angular/common';
import { RouterLink } from '@angular/router';
import { ExploreFacade } from './application/explore-facade';

@Component({
  providers: [ExploreFacade],
  selector: 'app-explore-page',
  imports: [UiButton, RouterLink, NgOptimizedImage],
  host: {
    '(document:keydown)': 'onDialogKeydown($event)',
    '(window:resize)': 'onViewportResize()',
  },
  templateUrl: './explore-page.html',
  styleUrl: './explore-page.scss',
})
export class ExplorePage {
  private readonly host = inject(ElementRef<HTMLElement>);
  private readonly document = inject(DOCUMENT);
  private readonly facade = inject(ExploreFacade);
  readonly filtersOpen = signal(false);
  private filterTrigger: HTMLElement | null = null;
  readonly activeFilters = computed(() => this.buildActiveFilters());
  readonly sortRequirement = computed(() => {
    const sort = this.facade.pendingSort() || this.facade.appliedFilters().sort;
    return this.facade.sortRequirement(sort, this.facade.filters());
  });
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
  get appliedFilters() { return this.facade.appliedFilters; }
  get hasUnappliedChanges() { return this.facade.hasUnappliedChanges; }
  get pendingSort() { return this.facade.pendingSort; }
  ngOnInit(): void {
    this.facade.ngOnInit();
  }
  readonly updateFilter = this.facade.updateFilter.bind(this.facade);
  readonly setAttribute = this.facade.setAttribute.bind(this.facade);
  readonly updateDate = this.facade.updateDate.bind(this.facade);
  readonly dateInput = this.facade.dateInput.bind(this.facade);
  readonly booleanValue = (value: boolean | undefined): string => value === undefined ? '' : String(value);
  readonly attributeValue = (code:string,kind:'minimum'|'maximum'|'enum'|'boolean'):string => {const p=this.filters().attributes[code]; if(kind==='boolean')return p?.kind==='boolean'?String(p.value):'';if(kind==='enum')return p?.kind==='enum'?p.value:'';return p?.kind==='range'?p[kind]??'':'';};
  readonly setAttributeRange = (code:string,side:'minimum'|'maximum',raw:string):void => {const old=this.filters().attributes[code];const next=old?.kind==='range'?{...old}:{kind:'range' as const};if(raw)next[side]=raw;else delete next[side];this.setAttribute(code,next.minimum||next.maximum?next:null);};
  readonly setAttributeOptions = (code:string,event:Event):void => {const values=[...((event.target as HTMLSelectElement).selectedOptions)].map(o=>o.value);this.setAttribute(code,values.length?{kind:'enum_list',values}:null);};
  readonly optionSelected = (code:string,value:string):boolean => {const p=this.filters().attributes[code];return p?.kind==='enum_list'&&p.values.includes(value);};
  readonly search = this.facade.search.bind(this.facade);
  readonly clearFilters = this.facade.clearFilters.bind(this.facade);
  readonly requestSort = this.facade.requestSort.bind(this.facade);
  readonly confirmSort = this.facade.confirmSort.bind(this.facade);
  readonly cancelSort = this.facade.cancelSort.bind(this.facade);
  readonly loadMore = this.facade.loadMore.bind(this.facade);
  readonly price = this.facade.price.bind(this.facade);
  readonly detailLink = this.facade.detailLink.bind(this.facade);
  readonly changePageSize = this.facade.changePageSize.bind(this.facade);
  readonly imageFailures = signal(new Set<string>());
  readonly imageUrl = this.facade.publicImageUrl.bind(this.facade);
  readonly imageFailed = (spaceId: string): boolean => this.imageFailures().has(spaceId);
  readonly markImageFailed = (spaceId: string): void => this.imageFailures.update((items) => new Set(items).add(spaceId));

  async submitSearch(): Promise<void> {
    await this.facade.search();
    if (!this.facade.error()) this.closeFilters();
  }

  openFilters(trigger: HTMLElement): void {
    this.filterTrigger = trigger;
    this.filtersOpen.set(true);
    setTimeout(() => ((this.host.nativeElement as HTMLElement).querySelector('.filter-close') as HTMLElement | null)?.focus());
  }

  closeFilters(): void {
    this.filtersOpen.set(false);
    setTimeout(() => {
      if (this.filterTrigger?.getClientRects().length) this.filterTrigger.focus();
      else (this.host.nativeElement as HTMLElement).querySelector<HTMLElement>('.filters input')?.focus();
    });
  }

  focusSortRequirement(): void {
    const target = this.sortRequirement();
    if (!target) return;
    const selector = target.target === 'query' ? '#catalog-search' : target.target === 'rate_unit' ? '#rate-unit' : '#start-at';
    setTimeout(() => (this.host.nativeElement.querySelector(selector) as HTMLElement | null)?.focus());
  }

  removeFilter(key: string): void { this.facade.removeAppliedFilter(key); }

  onViewportResize(): void {
    if (this.filtersOpen() && (this.document.defaultView?.innerWidth ?? 0) > 760) this.closeFilters();
  }

  private buildActiveFilters(): { key: string; label: string }[] {
    const f = this.facade.appliedFilters();
    const labels: Record<string, string> = {
      q: 'Búsqueda', category_code: 'Categoría', region_code: 'Región', comuna_code: 'Comuna', rate_unit: 'Tarifa',
      min_unit_price_clp: 'Precio desde', max_unit_price_clp: 'Precio hasta', min_area_m2: 'Superficie mínima',
      start_at: 'Desde', end_at: 'Hasta', min_total_clp: 'Total estimado desde', max_total_clp: 'Total estimado hasta',
      sort: 'Orden',
    };
    const active = Object.entries(labels).filter(([key]) => Boolean(f[key as keyof typeof f]) && !(key === 'sort' && f.sort === 'default')).map(([key, label]) => {
      const value = String(f[key as keyof typeof f]);
      const formatted = key === 'category_code'
        ? this.categories().find((item) => item.code === value)?.name ?? value
        : key === 'region_code'
          ? this.locations().find((item) => item.region_code === value)?.region_name ?? value
          : key === 'comuna_code'
            ? this.locations().find((item) => item.comuna_code === value)?.comuna_name ?? value
            : key === 'rate_unit'
              ? ({ hora: 'Hora', dia: 'Día', mes: 'Mes' } as Record<string, string>)[value] ?? value
                  : key === 'sort'
                    ? ({ title_asc: 'Nombre A–Z', title_desc: 'Nombre Z–A', unit_price_asc: 'Precio menor (misma modalidad)', unit_price_desc: 'Precio mayor (misma modalidad)', relevance: 'Relevancia', estimated_total_asc: 'Total estimado menor', estimated_total_desc: 'Total estimado mayor' } as Record<string, string>)[value] ?? value
              : key === 'start_at' || key === 'end_at'
                ? this.dateInput(value).replace('T', ' ')
                : key.includes('price') || key.includes('total')
                  ? this.price(value)
                  : key === 'min_area_m2' ? `${value} m²` : value;
      return { key, label: `${label}: ${formatted}` };
    });
    return [...active, ...Object.entries(f.attributes).map(([key, value]) => ({ key: `attribute:${key}`, label: `${this.attributeFilters().find((filter) => filter.code === key)?.label ?? key}: ${value.kind==='boolean'?(value.value?'Sí':'No'):value.kind==='range'?`${value.minimum??'…'}–${value.maximum??'…'}`:value.kind==='enum'?value.value:value.values.join(', ')}` }))];
  }

  onDialogKeydown(event: KeyboardEvent): void {
    if (!this.filtersOpen()) return;
    if (event.key === 'Escape') { event.preventDefault(); this.closeFilters(); return; }
    if (event.key !== 'Tab') return;
    const focusable = [...((this.host.nativeElement as HTMLElement).querySelector('.filters-open')?.querySelectorAll<HTMLElement>('button:not(:disabled), input:not(:disabled), select:not(:disabled)') ?? [])];
    if (!focusable.length) return;
    const first = focusable[0]; const last = focusable.at(-1);
    if (event.shiftKey && this.document.activeElement === first) { event.preventDefault(); last?.focus(); }
    else if (!event.shiftKey && this.document.activeElement === last) { event.preventDefault(); first.focus(); }
  }

}
