import { UiButton } from '../../shared/ui/button/button';
import { DatePipe } from '@angular/common';
import { Component, DestroyRef, ElementRef, ViewChild, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import {
  AbstractControl,
  FormControl,
  FormGroup,
  FormRecord,
  ReactiveFormsModule,
  ValidatorFn,
  Validators,
} from '@angular/forms';
import { RouterLink } from '@angular/router';
import { ModelControls } from '../../shared/forms/model-controls';
import { HostingSpacesFacade } from './application/hosting-spaces-facade';
import type { SpaceAttributeProfile, SpaceGalleryPhoto, SpaceMediaPhoto } from '../../core/contracts/models';

@Component({
  providers: [ModelControls, HostingSpacesFacade],
  selector: 'app-hosting-spaces-page',
  imports: [UiButton, ReactiveFormsModule, RouterLink, DatePipe],
  templateUrl: './hosting-spaces-page.html',
  styleUrl: './hosting-spaces-page.scss',
})
export class HostingSpacesPage {
  private readonly destroyRef = inject(DestroyRef);
  private readonly host = inject(ElementRef<HTMLElement>);
  @ViewChild('draftErrorSummary') private errorSummary?: ElementRef<HTMLElement>;
  readonly draftAttempted = signal(false);
  private readonly attributeModels = new Map<string, { value: string }>();
  attributeControl(code: string, type: string): FormControl<string> {
    const key = `${this.profile()?.schema_version ?? 0}/${code}/${type}`;
    const model = this.attributeModels.get(key) ?? { value: '' };
    model.value = this.facade.attributeValue(code);
    this.attributeModels.set(key, model);
    const control = this.controls.forProperty(model, 'value', (value) =>
      this.facade.setAttribute(code, String(value), type),
    );
    if (this.attributeControls.get(code) !== control) {
      this.attributeControls.setControl(code, control);
      const definition = this.profile()?.attributes.find((attribute) => attribute.code === code);
      control.setValidators(attributeValidators(definition));
      control.updateValueAndValidity({ emitEvent: false });
      control.valueChanges.pipe(takeUntilDestroyed(this.destroyRef)).subscribe(() => {
        this.facade.clearDraftFieldError(`attributes.${code}`);
      });
    }
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
  readonly publishedEditorForm = new FormGroup({
    title: this.controls.forProperty(this.facade.publishedForm, 'title'),
    description: this.controls.forProperty(this.facade.publishedForm, 'description'),
    capacity: this.controls.forProperty(this.facade.publishedForm, 'capacity'),
    usage_rules: this.controls.forProperty(this.facade.publishedForm, 'usage_rules'),
    region_code: this.controls.forProperty(this.facade.form, 'region_code', this.facade.regionChanged.bind(this.facade)),
    comuna_code: this.controls.forProperty(this.facade.form, 'comuna_code'),
  });
  constructor() {
    this.configureDraftValidation();
    const fieldNames = Object.keys(this.draftForm.controls).filter((field) => field !== 'attributes');
    for (const field of fieldNames) {
      const control = this.draftForm.controls[field as Exclude<keyof typeof this.draftForm.controls, 'attributes'>] as FormControl<unknown>;
      control.valueChanges
        .pipe(takeUntilDestroyed(this.destroyRef))
        .subscribe(() => this.facade.clearDraftFieldError(field));
    }
    for (const [field, control] of Object.entries(this.publishedEditorForm.controls) as Array<[string, AbstractControl]>) {
      control.valueChanges
        .pipe(takeUntilDestroyed(this.destroyRef))
        .subscribe(() => this.facade.clearDraftFieldError(field));
    }
  }

  private configureDraftValidation(): void {
    const controls = this.draftForm.controls;
    controls.category_code.setValidators(Validators.required);
    controls.title.setValidators([Validators.required, maxRunes(70)]);
    controls.description.setValidators([Validators.required, minTrimmedRunes(100)]);
    controls.area_m2.setValidators([Validators.required, areaValidator]);
    controls.capacity.setValidators([Validators.required, integerRange(1, 2_147_483_647)]);
    controls.usage_rules.setValidators([Validators.required, maxRunes(250)]);
    controls.rate_unit.setValidators([Validators.required, oneOf(['hora', 'dia', 'mes'])]);
    controls.base_price_clp.setValidators([Validators.required, clpValidator]);
    controls.address.setValidators([Validators.required, maxRunes(500)]);
    this.draftForm.setValidators(locationPairValidator);
    this.draftForm.updateValueAndValidity({ emitEvent: false });
  }
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
  get fieldErrors() {
    return this.facade.fieldErrors;
  }
  draftIssues(): Array<{ field: string; label: string; message: string }> {
    const issues = new Map<string, { field: string; label: string; message: string }>();
    const server = this.facade.fieldErrors();
    const fields: Array<[string, string, AbstractControl]> = [
      ['category_code', 'Categoría', this.draftForm.controls.category_code],
      ['title', 'Título', this.draftForm.controls.title],
      ['description', 'Descripción', this.draftForm.controls.description],
      ['area_m2', 'Superficie', this.draftForm.controls.area_m2],
      ['capacity', 'Capacidad', this.draftForm.controls.capacity],
      ['usage_rules', 'Reglas de uso', this.draftForm.controls.usage_rules],
      ['rate_unit', 'Modalidad base', this.draftForm.controls.rate_unit],
      ['base_price_clp', 'Tarifa base (CLP)', this.draftForm.controls.base_price_clp],
      ['address', 'Dirección privada', this.draftForm.controls.address],
      ['region_code', 'Región pública', this.draftForm.controls.region_code],
      ['comuna_code', 'Comuna pública', this.draftForm.controls.comuna_code],
    ];
    for (const [field, label, control] of fields) {
      const message = server[field] ?? (this.draftAttempted() || control.touched ? controlErrorMessage(control.errors) : '');
      if (message) issues.set(field, { field, label, message });
    }
    if ((this.draftAttempted() || this.draftForm.touched) && this.draftForm.errors?.['locationPair']) {
      for (const field of ['region_code', 'comuna_code']) {
        const label = field === 'region_code' ? 'Región pública' : 'Comuna pública';
        issues.set(field, { field, label, message: 'Selecciona región y comuna, o deja ambas sin indicar.' });
      }
    }
    for (const definition of this.profile()?.attributes ?? []) {
      const field = `attributes.${definition.code}`;
      const control = this.attributeControls.get(definition.code);
      const message = server[field] ?? (control && (this.draftAttempted() || control.touched) ? controlErrorMessage(control.errors) : '');
      if (message) issues.set(field, { field, label: `Característica: ${definition.label}`, message });
    }
    for (const [field, message] of Object.entries(server)) {
      if (issues.has(field)) continue;
      const definition = field.startsWith('attributes.')
        ? this.profile()?.attributes.find((attribute) => field === `attributes.${attribute.code}`)
        : undefined;
      issues.set(field, {
        field,
        label: definition ? `Característica: ${definition.label}` : field === 'attributes' ? 'Características' : 'Perfil de características',
        message,
      });
    }
    return [...issues.values()];
  }
  fieldError(field: string): string {
    return this.draftIssues().find((issue) => issue.field === field)?.message ?? '';
  }

  async submitDraft(event: Event): Promise<void> {
    event.preventDefault();
    this.draftAttempted.set(true);
    this.draftForm.markAllAsTouched();
    if (this.draftForm.invalid) {
      this.focusErrorSummary();
      return;
    }
    if (this.selectedSpace()) await this.saveDraft();
    else await this.createDraft();
    if (this.draftIssues().length) this.focusErrorSummary();
  }

  focusDraftField(field: string): void {
    const published = !!this.selectedSpace() && this.selectedSpace()?.state !== 'borrador';
    const name = field.startsWith('attributes.')
      ? `attr-${field.slice('attributes.'.length)}`
      : ({
          area_m2: 'area',
          capacity: published ? 'publishedCapacity' : 'capacity',
          description: published ? 'publishedDescription' : 'description',
          title: published ? 'publishedTitle' : 'title',
          usage_rules: published ? 'publishedRules' : 'rules',
          rate_unit: 'rateUnit',
          base_price_clp: 'basePrice',
          region_code: published ? 'publishedRegion' : 'publicRegion',
          comuna_code: published ? 'publishedCommune' : 'publicCommune',
          attribute_schema_version: 'category',
          attributes: '',
        } as Record<string, string>)[field] ?? field;
    const elements = this.host.nativeElement.querySelectorAll('[name]') as NodeListOf<HTMLElement>;
    const control = Array.from(elements)
      .find((element) => name
        ? element.getAttribute('name') === name
        : element.getAttribute('name')?.startsWith('attr-'));
    control?.focus();
  }

  private focusErrorSummary(): void {
    queueMicrotask(() => this.errorSummary?.nativeElement.focus());
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

function maxRunes(maximum: number): ValidatorFn {
  return (control) => Array.from(String(control.value ?? '')).length <= maximum
    ? null
    : { maxRunes: { maximum } };
}

function minTrimmedRunes(minimum: number): ValidatorFn {
  return (control) => Array.from(String(control.value ?? '').trim()).length >= minimum
    ? null
    : { minTrimmedRunes: { minimum } };
}

const areaValidator: ValidatorFn = (control) => {
  if (control.value === '' || control.value === null || control.value === undefined) return null;
  const value = Number(control.value);
  if (!Number.isFinite(value)) return { invalidValue: true };
  if (value < 0.01) return { areaMinimum: true };
  if (value > 99_999_999.99) return { areaMaximum: true };
  return Math.abs(value * 100 - Math.round(value * 100)) <= 1e-7 ? null : { areaStep: true };
};

function integerRange(minimum: number, maximum: number): ValidatorFn {
  return (control) => {
    if (control.value === '' || control.value === null || control.value === undefined) return null;
    const value = Number(control.value);
    if (!Number.isInteger(value)) return { wholeNumber: true };
    if (value < minimum) return { minimum: true };
    if (value > maximum) return { maximum: true };
    return null;
  };
}

const clpValidator: ValidatorFn = (control) => {
  const value = String(control.value ?? '');
  return value === '' || validClp(value) ? null : { clp: true };
};

function oneOf(values: readonly string[]): ValidatorFn {
  return (control) => values.includes(String(control.value)) ? null : { invalidOption: true };
}

const locationPairValidator: ValidatorFn = (control) => {
  const form = control as FormGroup;
  const region = String(form.get('region_code')?.value ?? '');
  const commune = String(form.get('comuna_code')?.value ?? '');
  if (!!region !== !!commune) return { locationPair: true };
  if (region && (!/^\d{2}$/.test(region) || !/^\d{5}$/.test(commune) || !commune.startsWith(region)))
    return { locationPair: true };
  return null;
};

function attributeValidators(attribute: SpaceAttributeProfile['attributes'][number] | undefined): ValidatorFn[] {
  if (!attribute) return [];
  const validators: ValidatorFn[] = [];
  if (attribute.required) validators.push(Validators.required);
  validators.push((control) => {
    const raw = String(control.value ?? '').trim();
    if (!raw) return null;
    if (attribute.type === 'boolean') return raw === 'true' || raw === 'false' ? null : { invalidAttribute: true };
    if (attribute.type === 'enum') return (attribute.options ?? []).includes(raw) ? null : { invalidAttribute: true };
    if (attribute.type === 'enum_list') {
      const values = raw.split(',').map((value) => value.trim()).filter(Boolean);
      return values.length > 0 && values.every((value) => (attribute.options ?? []).includes(value)) &&
        new Set(values).size === values.length ? null : { invalidAttribute: true };
    }
    const value = Number(raw);
    if (!Number.isFinite(value) || (attribute.type === 'integer' && !Number.isInteger(value)))
      return { invalidAttribute: true };
    if (attribute.minimum !== undefined && value < attribute.minimum) return { minimum: true };
    if (attribute.maximum !== undefined && value > attribute.maximum) return { maximum: true };
    if (attribute.step !== undefined && Math.abs(value / attribute.step - Math.round(value / attribute.step)) > 1e-7)
      return { step: true };
    return null;
  });
  return validators;
}

function controlErrorMessage(errors: Record<string, unknown> | null): string {
  if (!errors) return '';
  if (errors['required']) return 'Completa este campo.';
  if (errors['minTrimmedRunes']) return 'Escribe al menos 100 caracteres.';
  if (errors['maxRunes']) {
    const maximum = (errors['maxRunes'] as { maximum?: number }).maximum;
    return maximum ? `Usa ${maximum} caracteres o menos.` : 'El contenido supera el máximo permitido.';
  }
  if (errors['areaMinimum']) return 'La superficie mínima es 0,01 m².';
  if (errors['areaMaximum']) return 'La superficie máxima es 99.999.999,99 m².';
  if (errors['areaStep']) return 'Usa incrementos de 0,01 m².';
  if (errors['minimum']) return 'El valor está bajo el mínimo permitido.';
  if (errors['maximum']) return 'El valor supera el máximo permitido.';
  if (errors['wholeNumber']) return 'Indica un número entero.';
  if (errors['clp']) return 'La tarifa inicial debe ser un entero CLP mayor que 5.000.';
  if (errors['locationPair']) return 'Selecciona región y comuna, o deja ambas sin indicar.';
  if (errors['invalidOption'] || errors['invalidValue'] || errors['invalidAttribute'])
    return 'Selecciona o escribe un valor válido.';
  return 'Revisa este campo.';
}

function validClp(value: string): boolean {
  return /^\d+$/.test(value) && BigInt(value) >= 5001n && BigInt(value) <= 9223372036854775807n;
}
