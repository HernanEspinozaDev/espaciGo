import { UiButton } from '../../shared/ui/button/button';
import { KeyValuePipe } from '@angular/common';
import { Component, inject } from '@angular/core';
import { RouterLink } from '@angular/router';
import { ExploreFacade } from './application/explore-facade';

@Component({
  providers: [ExploreFacade],
  selector: 'app-explore-page',
  imports: [UiButton, RouterLink, KeyValuePipe],
  templateUrl: './explore-page.html',
  styleUrl: './explore-page.scss',
})
export class ExplorePage {
  private readonly facade = inject(ExploreFacade);
  get categories() {
    return this.facade.categories;
  }
  get profile() {
    return this.facade.profile;
  }
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
  get attributeFilters() {
    return this.facade.attributeFilters;
  }
  get mapView() {
    return this.facade.mapView;
  }
  get safetyNotice() {
    return this.facade.safetyNotice;
  }
  ngOnInit(): void {
    this.facade.ngOnInit();
  }
  readonly updateFilter = this.facade.updateFilter.bind(this.facade);
  readonly setAttribute = this.facade.setAttribute.bind(this.facade);
  readonly search = this.facade.search.bind(this.facade);
  readonly clearFilters = this.facade.clearFilters.bind(this.facade);
  readonly toggleMap = this.facade.toggleMap.bind(this.facade);
  readonly loadMore = this.facade.loadMore.bind(this.facade);
  readonly attributeValue = this.facade.attributeValue.bind(this.facade);
  readonly attributeLabel = this.facade.attributeLabel.bind(this.facade);
  readonly displayValue = this.facade.displayValue.bind(this.facade);
  readonly price = this.facade.price.bind(this.facade);
  readonly position = this.facade.position.bind(this.facade);
}
