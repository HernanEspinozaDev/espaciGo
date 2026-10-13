import { UiButton } from '../../shared/ui/button/button';
import { Component, effect, inject, signal } from '@angular/core';
import { RouterLink } from '@angular/router';
import { SpaceDetailFacade } from './application/space-detail-facade';

@Component({
  providers: [SpaceDetailFacade],
  selector: 'app-space-detail-page',
  imports: [UiButton, RouterLink],
  templateUrl: './space-detail-page.html',
  styleUrl: './space-detail-page.scss',
})
export class SpaceDetailPage {
  private readonly facade = inject(SpaceDetailFacade);
  readonly failedImages = signal<{ spaceId: string; generation: number; ids: Set<string> }>({ spaceId: '', generation: 0, ids: new Set() });
  readonly selectedImage = signal<{ spaceId: string; generation: number; imageId: string } | null>(null);
  readonly spaceGeneration = signal(0);
  private observedSpaceId = '';
  constructor() {
    effect(() => {
      const spaceId = this.space()?.space_id ?? '';
      if (spaceId !== this.observedSpaceId) {
        this.observedSpaceId = spaceId;
        const generation = this.spaceGeneration() + 1;
        this.spaceGeneration.set(generation);
        this.failedImages.set({ spaceId, generation, ids: new Set() });
        this.selectedImage.set(null);
      }
    });
  }
  get space() {
    return this.facade.space;
  }
  get privateSpace() { return this.facade.privateSpace; }
  get canBook() { return this.facade.canBook.bind(this.facade); }
  get profile() {
    return this.facade.profile;
  }
  get date() {
    return this.facade.date;
  }
  get duration() {
    return this.facade.duration;
  }
  get options() {
    return this.facade.options;
  }
  get selected() {
    return this.facade.selected;
  }
  get quote() {
    return this.facade.quote;
  }
  get notice() {
    return this.facade.notice;
  }
  get busy() {
    return this.facade.busy;
  }
  get error() {
    return this.facade.error;
  }
  get requestingReservation() {
    return this.facade.requestingReservation;
  }
  ngOnInit(): void {
    this.facade.ngOnInit();
  }
  readonly chooseDate = this.facade.chooseDate.bind(this.facade);
  readonly chooseDuration = this.facade.chooseDuration.bind(this.facade);
  readonly chooseInterval = this.facade.chooseInterval.bind(this.facade);
  readonly durationOptions = this.facade.durationOptions.bind(this.facade);
  readonly loadAvailability = this.facade.loadAvailability.bind(this.facade);
  readonly createQuote = this.facade.createQuote.bind(this.facade);
  readonly requestReservation = this.facade.requestReservation.bind(this.facade);
  readonly formatInterval = this.facade.formatInterval.bind(this.facade);
  readonly formatClp = this.facade.formatClp.bind(this.facade);
  readonly formatDateTime = this.facade.formatDateTime.bind(this.facade);
  readonly attributeLabel = this.facade.attributeLabel.bind(this.facade);
  readonly attributeValue = this.facade.attributeValue.bind(this.facade);
  readonly publicImageUrl = this.facade.publicImageUrl.bind(this.facade);
  orderedImages(images: { image_id: string; is_cover: boolean }[] = []) {
    return [...images].sort((a, b) => Number(b.is_cover) - Number(a.is_cover));
  }
  imageFailed(spaceId: string, imageId: string, generation: number): void {
    if (this.space()?.space_id !== spaceId || this.spaceGeneration() !== generation) return;
    const state = this.failedImages();
    const ids = state.spaceId === spaceId && state.generation === generation ? new Set(state.ids) : new Set<string>();
    ids.add(imageId);
    this.failedImages.set({ spaceId, generation, ids });
  }
  imageAvailable(spaceId: string, imageId: string): boolean {
    const state = this.failedImages();
    return state.spaceId !== spaceId || state.generation !== this.spaceGeneration() || !state.ids.has(imageId);
  }
  hasAvailableImage(spaceId: string, images: { image_id: string; is_cover: boolean }[] = []): boolean {
    return images.some((image) => this.imageAvailable(spaceId, image.image_id));
  }
  selectImage(spaceId: string, imageId: string): void {
    if (this.space()?.space_id === spaceId && this.imageAvailable(spaceId, imageId))
      this.selectedImage.set({ spaceId, generation: this.spaceGeneration(), imageId });
  }
  imageSelected(spaceId: string, imageId: string): boolean {
    const selected = this.selectedImage();
    return selected?.spaceId === spaceId && selected.generation === this.spaceGeneration() && selected.imageId === imageId;
  }
}
