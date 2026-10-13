import { UiButton } from '../../shared/ui/button/button';
import { Component, inject } from '@angular/core';
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
  private readonly facade = inject(ExploreFacade);
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
}
