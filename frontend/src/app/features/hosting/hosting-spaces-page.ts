import { UiButton } from '../../shared/ui/button/button';
import { DatePipe } from '@angular/common';
import { Component, inject } from '@angular/core';
import { FormControl, FormGroup, FormRecord, ReactiveFormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { ModelControls } from '../../shared/forms/model-controls';
import { HostingSpacesFacade } from './application/hosting-spaces-facade';
import type { SpaceGalleryPhoto, SpaceMediaPhoto } from '../../core/contracts/models';

@Component({
  providers: [ModelControls, HostingSpacesFacade],
  selector: 'app-hosting-spaces-page',
  imports: [UiButton, ReactiveFormsModule, RouterLink, DatePipe],
  templateUrl: './hosting-spaces-page.html',
  styleUrl: './hosting-spaces-page.scss',
})
export class HostingSpacesPage {
  private readonly attributeModels = new Map<string, { value: string }>();
  attributeControl(code: string, type: string): FormControl<string> {
    const key = `${code}/${type}`;
    const model = this.attributeModels.get(key) ?? { value: '' };
    model.value = this.facade.attributeValue(code);
    this.attributeModels.set(key, model);
    const control = this.controls.forProperty(model, 'value', (value) =>
      this.facade.setAttribute(code, String(value), type),
    );
    if (this.attributeControls.get(code) !== control) this.attributeControls.setControl(code, control);
    return control;
  }
  private readonly controls = inject(ModelControls);
  readonly controlFor = this.controls.forProperty.bind(this.controls);
  readonly signalControl = this.controls.forSignal.bind(this.controls);
  private readonly facade = inject(HostingSpacesFacade);
  readonly attributeControls = new FormRecord<FormControl<string>>({});
  readonly draftForm = new FormGroup({
    category_code: this.controls.forProperty(
      this.facade.form,
      'category_code',
      this.facade.categoryChanged.bind(this.facade),
    ),
    title: this.controls.forProperty(this.facade.form, 'title'),
    address: this.controls.forProperty(this.facade.form, 'address'),
    region_code: this.controls.forProperty(
      this.facade.form,
      'region_code',
      this.facade.regionChanged.bind(this.facade),
    ),
    comuna_code: this.controls.forProperty(this.facade.form, 'comuna_code'),
    description: this.controls.forProperty(this.facade.form, 'description'),
    area_m2: this.controls.forProperty(this.facade.form, 'area_m2'),
    capacity: this.controls.forProperty(this.facade.form, 'capacity'),
    usage_rules: this.controls.forProperty(this.facade.form, 'usage_rules'),
    rate_unit: this.controls.forProperty(this.facade.form, 'rate_unit'),
    base_price_clp: this.controls.forProperty(this.facade.form, 'base_price_clp'),
    attributes: this.attributeControls,
  });
  get session() {
    return this.facade.session;
  }
  get spaces() {
    return this.facade.spaces;
  }
  get categories() {
    return this.facade.categories;
  }
  get publicLocations() {
    return this.facade.publicLocations;
  }
  get publicRegions() {
    const regions = new Map<string, string>();
    for (const location of this.facade.publicLocations())
      regions.set(location.region_code, location.region_name);
    return [...regions].map(([code, name]) => ({ code, name }));
  }
  get publicCommunes() {
    const regionCode = this.facade.form.region_code;
    return this.facade.publicLocations().filter((location) => location.region_code === regionCode);
  }
  get locationsLoading() {
    return this.facade.locationsLoading;
  }
  get locationsError() {
    return this.facade.locationsError;
  }
  get selectedSpace() {
    return this.facade.selectedSpace;
  }
  get profile() {
    return this.facade.profile;
  }
  get gallery() {
    return this.facade.gallery;
  }
  get maxPhotos() {
    return this.facade.maxPhotos;
  }
  get uploadQueue() { return this.facade.uploadQueue; }
  get uploadRunning() { return this.facade.uploadRunning; }
  get uploadMessage() { return this.facade.uploadMessage; }
  get uploadSlotsAvailable() { return this.facade.uploadSlotsAvailable; }
  get queuedUploadCount() { return this.facade.queuedUploadCount; }
  get photoUrl() {
    return this.facade.photoUrl;
  }
  get previewPhotoId() {
    return this.facade.previewPhotoId;
  }
  get tariff() {
    return this.facade.tariff;
  }
  get tariffHistory() {
    return this.facade.tariffHistory;
  }
  get simulation() {
    return this.facade.simulation;
  }
  get timeZone() {
    return this.facade.timeZone;
  }
  get availability() {
    return this.facade.availability;
  }
  get blocks() {
    return this.facade.blocks;
  }
  get weekly() {
    return this.facade.weekly;
  }
  get weeklyForm() {
    return this.facade.weeklyForm;
  }
  set weeklyForm(value: HostingSpacesFacade['weeklyForm']) {
    this.facade.weeklyForm = value;
  }
  get section() {
    return this.facade.section;
  }
  get loading() {
    return this.facade.loading;
  }
  get detailLoading() {
    return this.facade.detailLoading;
  }
  get saving() {
    return this.facade.saving;
  }
  get loadingError() {
    return this.facade.loadingError;
  }
  get error() {
    return this.facade.error;
  }
  get notice() {
    return this.facade.notice;
  }
  get conflict() {
    return this.facade.conflict;
  }
  get form() {
    return this.facade.form;
  }
  get rateForm() {
    return this.facade.rateForm;
  }
  get publishedForm() {
    return this.facade.publishedForm;
  }
  get simulationForm() {
    return this.facade.simulationForm;
  }
  get blockForm() {
    return this.facade.blockForm;
  }
  get availabilityForm() {
    return this.facade.availabilityForm;
  }
  get days() {
    return this.facade.days;
  }
  get disambiguation() {
    return this.facade.disambiguation;
  }
  get sectionTitle() {
    return this.facade.sectionTitle;
  }
  ngOnInit(): void {
    this.facade.ngOnInit();
  }
  ngOnDestroy(): void {
    this.facade.ngOnDestroy();
  }
  readonly load = this.facade.load.bind(this.facade);
  readonly loadSpace = this.facade.loadSpace.bind(this.facade);
  readonly categoryChanged = this.facade.categoryChanged.bind(this.facade);
  readonly regionChanged = this.facade.regionChanged.bind(this.facade);
  readonly createDraft = this.facade.createDraft.bind(this.facade);
  readonly saveDraft = this.facade.saveDraft.bind(this.facade);
  readonly savePublishedContent = this.facade.savePublishedContent.bind(this.facade);
  readonly changePublication = this.facade.changePublication.bind(this.facade);
  readonly deleteSpace = this.facade.deleteSpace.bind(this.facade);
  readonly saveTariff = this.facade.saveTariff.bind(this.facade);
  readonly simulatePrice = this.facade.simulatePrice.bind(this.facade);
  readonly saveTimeZone = this.facade.saveTimeZone.bind(this.facade);
  readonly checkAvailability = this.facade.checkAvailability.bind(this.facade);
  readonly addManualBlock = this.facade.addManualBlock.bind(this.facade);
  readonly removeManualBlock = this.facade.removeManualBlock.bind(this.facade);
  readonly loadBlocks = this.facade.loadBlocks.bind(this.facade);
  readonly saveWeekly = this.facade.saveWeekly.bind(this.facade);
  readonly addSyntheticPhoto = this.facade.addSyntheticPhoto.bind(this.facade);
  readonly startPhotoUploads = this.facade.startPhotoUploads.bind(this.facade);
  readonly removeQueuedPhoto = this.facade.removeQueuedPhoto.bind(this.facade);
  readonly retryQueuedPhoto = this.facade.retryQueuedPhoto.bind(this.facade);
  readonly selectCover = this.facade.selectCover.bind(this.facade);
  readonly showPhoto = this.facade.showPhoto.bind(this.facade);
  readonly removePhoto = this.facade.removePhoto.bind(this.facade);
  readonly setPhotoVisible = this.facade.setPhotoVisible.bind(this.facade);
  readonly setPhotoPublic = this.facade.setPhotoPublic.bind(this.facade);
  readonly setRateUnit = this.facade.setRateUnit.bind(this.facade);
  readonly setTimeZone = this.facade.setTimeZone.bind(this.facade);
  readonly setDisambiguation = this.facade.setDisambiguation.bind(this.facade);
  readonly setAttribute = this.facade.setAttribute.bind(this.facade);
  readonly addPeriod = this.facade.addPeriod.bind(this.facade);
  readonly removePeriod = this.facade.removePeriod.bind(this.facade);
  readonly attributeValue = this.facade.attributeValue.bind(this.facade);
  readonly formatClp = this.facade.formatClp.bind(this.facade);
  readonly labelState = this.facade.labelState.bind(this.facade);
  readonly zoneLabel = this.facade.zoneLabel.bind(this.facade);
  readonly goTo = this.facade.goTo.bind(this.facade);
  readonly newDraft = this.facade.newDraft.bind(this.facade);
  readonly closePhoto = this.facade.closePhoto.bind(this.facade);
  readonly setNotice = this.facade.setNotice.bind(this.facade);
  readonly zoneDateTime = this.facade.zoneDateTime.bind(this.facade);

  queuePhotoFiles(event: Event): void {
    const input = event.target as HTMLInputElement | null;
    if (input?.files) this.facade.queuePhotoFiles(input.files);
    if (input) input.value = '';
  }
  isMediaPhoto(photo: SpaceGalleryPhoto): photo is SpaceMediaPhoto {
    return this.facade.isMediaPhoto(photo);
  }
  photoIsVisible(photo: SpaceGalleryPhoto): boolean { return this.facade.photoIsVisible(photo); }
  photoIsCover(photo: SpaceGalleryPhoto): boolean { return this.facade.photoIsCover(photo); }
  uploadFileSize(size: number): string { return this.facade.uploadFileSize(size); }
  saveAltText(photo: SpaceMediaPhoto, event: Event): void {
    const value = (event.target as HTMLInputElement).value;
    void this.facade.updateMediaPhoto(photo.id, { alt_text: value });
  }
  savePosition(photo: SpaceMediaPhoto, event: Event): void {
    const value = Number((event.target as HTMLInputElement).value);
    if (Number.isInteger(value) && value >= 1 && value <= this.gallery().length)
      void this.facade.updateMediaPhoto(photo.id, { position: value });
  }
}
