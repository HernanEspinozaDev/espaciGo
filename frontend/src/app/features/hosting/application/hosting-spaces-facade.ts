import { isPlatformBrowser } from '@angular/common';
import {
  DestroyRef,
  Injectable,
  OnDestroy,
  OnInit,
  PLATFORM_ID,
  computed,
  effect,
  inject,
  signal,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { Meta, Title } from '@angular/platform-browser';
import { ActivatedRoute, Router } from '@angular/router';
import type {
  LocalWeeklyHoursEnvelope,
  PublicLocationList,
  SpaceAttributeProfile,
  SpaceCategoryList,
  SpaceDraft,
  SpaceDraftInput,
  SpaceGalleryPhoto,
  SpaceMediaPhoto,
  SpaceMediaPhotoUpdate,
  SyntheticSpaceGalleryPhoto,
} from '../../../core/contracts/models';
import { IdentitySession } from '../../../core/identity/application/identity-session';
import { PrivateSessionState } from '../../../core/identity/private-session-state';
import { zonedIsoLabel, zonedLocalToRfc3339 } from '../calendar-time';
import { HostingApiError, HostingPort } from '../domain/ports/hosting-port';

type Section = 'datos' | 'tarifas' | 'calendario' | 'galeria';
type ExactSpace = Omit<SpaceDraft, 'base_price_clp'> & { base_price_clp: string };
type ExactDraftList = { items: ExactSpace[] };
type GalleryAttempt = { key: string; payload: { fixture_code: 'synthetic-png-v1' } };
type PhotoUploadTask = {
  id: string;
  key: string;
  file: File;
  previewUrl: string | null;
  status: 'queued' | 'uploading' | 'error';
  progress: number | null;
  error: string;
};
type RequestContext = {
  token: string;
  session: NonNullable<ReturnType<IdentitySession['session']>>;
};
type DraftForm = {
  category_code: string;
  title: string;
  description: string;
  area_m2: number;
  capacity: number;
  usage_rules: string;
  rate_unit: 'hora' | 'dia' | 'mes';
  base_price_clp: string;
  address: string;
  region_code: string;
  comuna_code: string;
  attribute_schema_version?: number;
  attributes: Record<string, unknown>;
};

const emptyForm = (): DraftForm => ({
  category_code: '',
  title: '',
  description: '',
  area_m2: 1,
  capacity: 1,
  usage_rules: '',
  rate_unit: 'hora',
  base_price_clp: '5001',
  address: '',
  region_code: '',
  comuna_code: '',
  attributes: {},
});
const dayNames = ['Lunes', 'Martes', 'Miércoles', 'Jueves', 'Viernes', 'Sábado', 'Domingo'];
const maxPhotoInputBytes = 10 << 20;

function isMediaPhoto(photo: SpaceGalleryPhoto): photo is SpaceMediaPhoto {
  return !('fixture_code' in photo);
}

@Injectable()
export class HostingSpacesFacade implements OnInit, OnDestroy {
  private readonly api = inject(HostingPort);
  private readonly identity = inject(IdentitySession);
  private readonly privateState = inject(PrivateSessionState);
  private readonly platform = inject(PLATFORM_ID);
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly meta = inject(Meta);
  private readonly title = inject(Title);
  private readonly destroyRef = inject(DestroyRef);
  private loadGeneration = 0;
  private resourceGeneration = 0;
  private savingGeneration = 0;
  private queryRevision = 0;
  private simulationQueryGeneration = 0;
  private availabilityQueryGeneration = 0;
  private galleryGeneration = 0;
  private uploadGeneration = 0;
  private profileGeneration = 0;
  private selectionRevision = 0;
  private ownerToken = this.identity.accessToken();
  private ownerSession = this.identity.session();
  private destroyed = false;
  private unregister!: () => void;
  private blobUrl: string | null = null;
  private readonly galleryAttempts = new Map<string, GalleryAttempt>();
  private readonly originalForm = new Map<string, DraftForm>();
  private routeSpaceId: string | null = null;
  private activeSpaceId: string | null = null;

  readonly session = this.identity.session;
  readonly spaces = signal<ExactSpace[]>([]);
  readonly categories = signal<SpaceCategoryList['items']>([]);
  readonly publicLocations = signal<PublicLocationList['items']>([]);
  readonly locationsLoading = signal(false);
  readonly locationsError = signal('');
  readonly selectedSpace = signal<ExactSpace | null>(null);
  readonly profile = signal<SpaceAttributeProfile | null>(null);
  readonly gallery = signal<SpaceGalleryPhoto[]>([]);
  readonly maxPhotos = signal(10);
  readonly uploadQueue = signal<PhotoUploadTask[]>([]);
  readonly uploadRunning = signal(false);
  readonly uploadMessage = signal('');
  readonly uploadSlotsAvailable = computed(() => Math.max(0,
    this.maxPhotos() - this.gallery().length - this.uploadQueue().length,
  ));
  readonly queuedUploadCount = computed(() => this.uploadQueue().filter((task) => task.status === 'queued').length);
  readonly photoUrl = signal<string | null>(null);
  readonly previewPhotoId = signal<string | null>(null);
  readonly tariff = signal<{
    rate_unit: 'hora' | 'dia' | 'mes';
    base_price: string;
    version: number;
  } | null>(null);
  readonly tariffHistory = signal<
    Array<{
      rate_unit: 'hora' | 'dia' | 'mes';
      base_price: string;
      version: number;
      created_at: string;
    }>
  >([]);
  readonly simulation = signal<{
    base_subtotal: string;
    billed_units: number;
    rate_unit: string;
    time_zone: string;
  } | null>(null);
  readonly timeZone = signal('America/Santiago');
  readonly availability = signal<{ available: boolean; start_at: string; end_at: string } | null>(
    null,
  );
  readonly blocks = signal<
    Array<{ id: string; start_at: string; end_at: string; reason: string; time_zone: string }>
  >([]);
  readonly weekly = signal<LocalWeeklyHoursEnvelope['data'] | null>(null);
  weeklyForm: {
    enabled: boolean;
    days: Array<{ weekday: number; periods: Array<{ open: string; close: string }> }>;
  } = {
    enabled: false,
    days: dayNames.map((_, index) => ({ weekday: index + 1, periods: [] })),
  };
  readonly section = signal<Section>('datos');
  readonly loading = signal(false);
  readonly detailLoading = signal(false);
  readonly saving = signal(false);
  readonly loadingError = signal('');
  readonly error = signal('');
  readonly notice = signal('');
  readonly conflict = signal('');
  readonly form = emptyForm();
  readonly rateForm = { rate_unit: 'hora' as 'hora' | 'dia' | 'mes', base_price: '5001' };
  readonly publishedForm = { title: '', description: '', capacity: 1, usage_rules: '' };
  readonly simulationForm = { start_at: '', end_at: '' };
  readonly blockForm = { start_at: '', end_at: '', reason: 'bloqueo_anfitrion' };
  readonly availabilityForm = { start_at: '', end_at: '' };
  readonly days = dayNames;
  readonly disambiguation = signal<'earlier' | 'later'>('earlier');
  readonly sectionTitle = signal('Datos del espacio');

  constructor() {
    this.title.setTitle('Mis espacios | EspaciGo');
    this.meta.addTag({ name: 'robots', content: 'noindex, nofollow' });
    this.destroyRef.onDestroy(() => this.meta.removeTag("name='robots'"));
    this.unregister = this.privateState.register(() => this.clearPrivate());
    effect(() => {
      const token = this.identity.accessToken();
      const session = this.identity.session();
      if (token !== this.ownerToken || session !== this.ownerSession) {
        this.ownerToken = token;
        this.ownerSession = session;
        this.clearPrivate();
        if (token && session) void this.load();
      }
    });
  }

  ngOnInit(): void {
    this.route.paramMap.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((params) => {
      const nextId = params.get('spaceId');
      if (nextId !== this.routeSpaceId) {
        this.routeSpaceId = nextId;
        const current = this.selectedSpace();
        if (nextId && current?.id !== nextId && this.spaces().length) void this.loadSpace(nextId);
      }
    });
    this.route.data.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((data) => {
      const value = data['section'] as Section | undefined;
      if (value) this.section.set(value);
      this.sectionTitle.set(
        value === 'calendario'
          ? 'Disponibilidad y calendario'
          : value === 'galeria'
            ? 'Galería'
            : value === 'tarifas'
              ? 'Tarifas y simulaciones'
              : 'Datos del espacio',
      );
    });
    if (isPlatformBrowser(this.platform)) void this.load();
  }

  ngOnDestroy(): void {
    this.destroyed = true;
    this.loadGeneration++;
    this.resourceGeneration++;
    this.profileGeneration++;
    this.galleryGeneration++;
    this.galleryAttempts.clear();
    this.unregister?.();
    this.clearUploadQueue();
    this.revokePhoto();
  }

  async load(): Promise<void> {
    const context = this.context();
    if (!context || !isPlatformBrowser(this.platform)) return;
    const generation = ++this.loadGeneration;
    this.loading.set(true);
    this.loadingError.set('');
    this.locationsLoading.set(true);
    this.locationsError.set('');
    try {
      const [list, categories] = await Promise.all([
        this.api.list(context.token) as unknown as Promise<ExactDraftList>,
        this.api.categories(context.token),
      ]);
      if (!this.loadCurrent(context, generation)) return;
      this.spaces.set(list.items);
      this.categories.set(categories.items);
      try {
        const locations = await this.api.publicLocations();
        if (!this.loadCurrent(context, generation)) return;
        this.publicLocations.set(locations.items);
      } catch (error) {
        if (!this.loadCurrent(context, generation)) return;
        this.locationsError.set('No fue posible cargar el catálogo territorial público.');
        this.fail(error, context, true);
      } finally {
        if (this.loadCurrent(context, generation)) this.locationsLoading.set(false);
      }
      const nextId = this.routeSpaceId ?? this.selectedSpace()?.id ?? list.items[0]?.id ?? null;
      if (nextId) await this.loadSpace(nextId);
      else {
        this.activeSpaceId = null;
        this.resetResourceState();
        this.form.category_code = categories.items[0]?.code ?? '';
        if (this.form.category_code) await this.loadProfile(context, this.form.category_code);
      }
      if (!this.loadCurrent(context, generation)) return;
    } catch (error) {
      const current = this.loadCurrent(context, generation);
      this.fail(error, context, current);
      if (current)
        this.loadingError.set(
          'No fue posible cargar tus espacios. Go mantiene la decisión de permisos.',
        );
    } finally {
      if (this.loadCurrent(context, generation)) {
        this.loading.set(false);
        this.locationsLoading.set(false);
      }
    }
  }

  async loadSpace(spaceId: string): Promise<void> {
    const context = this.context();
    if (!context || !spaceId || this.destroyed) return;
    const generation = ++this.resourceGeneration;
    this.profileGeneration++;
    const selection = ++this.selectionRevision;
    this.activeSpaceId = spaceId;
    this.resetResourceState();
    this.detailLoading.set(true);
    this.error.set('');
    this.notice.set('');
    this.revokePhoto();
    try {
      const draft = (await this.api.detail(context.token, spaceId)) as unknown as ExactSpace;
      if (!this.resourceCurrent(context, generation, selection, spaceId)) return;
      this.form.rate_unit = draft.rate_unit;
      this.rateForm.rate_unit = draft.rate_unit;
      this.rateForm.base_price = String(draft.base_price_clp);
      this.selectedSpace.set(draft);
      this.setForm(draft);
      const [profile, photos, tariff, tz, weekly, versions] = await Promise.allSettled([
        this.api.profile(context.token, draft.category_code, draft.attribute_schema_version),
        this.api.gallery(context.token, spaceId),
        this.api.tariff(context.token, spaceId),
        this.api.timeZone(context.token, spaceId),
        this.api.weekly(context.token, spaceId),
        this.api.tariffVersions(context.token, spaceId),
      ]);
      // A resource switch must not hide an unauthorized response for the same
      // still-current bearer. Process all parallel 401s before discarding data
      // whose resource selection is stale; expireIfCurrent protects a newer
      // session that may have replaced this token meanwhile.
      for (const result of [profile, photos, tariff, tz, weekly, versions]) {
        if (result.status === 'rejected' && statusOf(result.reason) === 401) {
          this.fail(result.reason, context, false);
          if (!this.contextCurrent(context)) return;
        }
      }
      if (!this.resourceCurrent(context, generation, selection, spaceId)) return;
      for (const result of [profile, photos, tariff, tz, weekly, versions]) {
        if (
          result.status === 'rejected' &&
          statusOf(result.reason) !== 401 &&
          statusOf(result.reason) !== 404 &&
          statusOf(result.reason) !== undefined
        ) {
          this.error.set('Go no pudo cargar una parte de la configuración del espacio.');
        }
      }
      if (!this.contextCurrent(context)) return;
      if (!this.resourceCurrent(context, generation, selection, spaceId)) return;
      if (profile.status === 'fulfilled') this.profile.set(profile.value);
      if (photos.status === 'fulfilled') {
        this.gallery.set(photos.value.items);
        this.maxPhotos.set(photos.value.max_photos);
      }
      if (tariff.status === 'fulfilled') {
        this.tariff.set(tariff.value as never);
        this.rateForm.rate_unit = tariff.value.rate_unit;
        this.rateForm.base_price = String(tariff.value.base_price);
        this.form.base_price_clp = String(tariff.value.base_price);
      } else this.tariff.set(null);
      if (tz.status === 'fulfilled') this.timeZone.set(tz.value.time_zone);
      else this.timeZone.set(draft.time_zone ?? '');
      if (weekly.status === 'fulfilled') {
        this.weekly.set(weekly.value.data);
        this.weeklyForm = {
          enabled: weekly.value.data.enabled,
          days: weekly.value.data.days.map((day) => ({
            weekday: day.weekday,
            periods: day.periods.map((period) => ({ ...period })),
          })),
        };
      } else this.weekly.set(null);
      if (versions.status === 'fulfilled') this.tariffHistory.set(versions.value.items as never);
      await this.loadBlocks(context, generation, selection, spaceId);
      if (!this.resourceCurrent(context, generation, selection, spaceId)) return;
      this.publishedForm.title = draft.title;
      this.publishedForm.description = draft.description;
      this.publishedForm.capacity = draft.capacity;
      this.publishedForm.usage_rules = draft.usage_rules;
    } catch (error) {
      this.fail(error, context, this.resourceCurrent(context, generation, selection, spaceId));
    } finally {
      if (this.resourceCurrent(context, generation, selection, spaceId))
        this.detailLoading.set(false);
    }
  }

  async categoryChanged(code: string): Promise<void> {
    this.profileGeneration++;
    this.resourceGeneration++;
    this.form.category_code = code;
    this.form.attributes = {};
    this.form.attribute_schema_version = undefined;
    this.profile.set(null);
    const context = this.context();
    if (context && code) await this.loadProfile(context, code);
  }

  regionChanged(code: string): void {
    this.form.region_code = code;
    this.form.comuna_code = '';
  }

  async createDraft(): Promise<void> {
    const context = this.context();
    if (!context) return;
    const payload = this.makePayload(this.form);
    if (!payload) return;
    const revision = ++this.resourceGeneration;
    const saveOperation = this.beginSaving();
    this.error.set('');
    this.notice.set('');
    try {
      const created = (await this.api.create(context.token, payload)) as unknown as ExactSpace;
      if (!this.writeCurrent(context, revision, null)) return;
      this.spaces.update((items) => [created, ...items.filter((item) => item.id !== created.id)]);
      this.selectedSpace.set(created);
      this.setForm(created);
      this.notice.set('Borrador creado por Go.');
      await this.router.navigate(['/panel/espacios', created.id, 'editar']);
      if (!this.contextCurrent(context)) return;
      await this.loadSpace(created.id);
    } catch (error) {
      this.fail(error, context, this.writeCurrent(context, revision, null));
    } finally {
      this.finishSaving(saveOperation);
    }
  }

  async saveDraft(): Promise<void> {
    const context = this.context();
    const space = this.selectedSpace();
    if (!context || !space) return this.createDraft();
    const edited = structuredClone(this.form);
    const original = structuredClone(this.originalForm.get(space.id) ?? this.form);
    const revision = ++this.resourceGeneration;
    const selection = this.selectionRevision;
    const saveOperation = this.beginSaving();
    this.error.set('');
    this.notice.set('');
    try {
      // The draft PUT is a full replacement. Start from Go's latest record and
      // overlay only fields the user changed since the last confirmed read.
      const latest = (await this.api.detail(context.token, space.id)) as unknown as ExactSpace;
      if (!this.draftWriteCurrent(context, revision, selection, space.id)) return;
      let latestTariff: string | undefined;
      try {
        latestTariff = String((await this.api.tariff(context.token, space.id)).base_price);
      } catch (error) {
        if (!(error instanceof HostingApiError) || error.status !== 404) throw error;
      }
      if (!this.draftWriteCurrent(context, revision, selection, space.id)) return;
      const merged = this.mergeChangedFields(original, edited, latest);
      if (latestTariff !== undefined) merged.base_price_clp = latestTariff;
      const payload = this.makePayload(merged);
      if (!payload) return;
      const saved = (await this.api.update(
        context.token,
        space.id,
        payload,
      )) as unknown as ExactSpace;
      if (!this.draftWriteCurrent(context, revision, selection, space.id)) return;
      this.applyDraft(saved);
      this.notice.set('Cambios del borrador confirmados por Go.');
    } catch (error) {
      this.fail(error, context, this.draftWriteCurrent(context, revision, selection, space.id));
    } finally {
      this.finishSaving(saveOperation);
    }
  }

  async savePublishedContent(): Promise<void> {
    const context = this.context();
    const space = this.selectedSpace();
    if (!context || !space || space.state === 'borrador') return;
    const patch: Record<string, unknown> = {};
    for (const key of ['title', 'description', 'capacity', 'usage_rules'] as const) {
      if (this.publishedForm[key] !== space[key]) patch[key] = this.publishedForm[key];
    }
    const regionCode = this.form.region_code;
    const communeCode = this.form.comuna_code;
    if (!!regionCode !== !!communeCode)
      return this.error.set('Selecciona una comuna de la región para guardar la ubicación pública.');
    if (regionCode !== (space.region_code ?? '') || communeCode !== (space.comuna_code ?? '')) {
      patch['region_code'] = regionCode;
      patch['comuna_code'] = communeCode;
    }
    if (!Object.keys(patch).length)
      return this.setNotice('No hay campos modificados para guardar.');
    const revision = ++this.resourceGeneration;
    const saveOperation = this.beginSaving();
    try {
      const updated = (await this.api.updatePublished(
        context.token,
        space.id,
        patch as never,
      )) as unknown as ExactSpace;
      if (
        !this.contextCurrent(context) ||
        revision !== this.resourceGeneration ||
        this.selectedSpace()?.id !== space.id
      )
        return;
      this.applyDraft(updated);
      this.notice.set('Los campos modificados de la publicación fueron confirmados por Go.');
    } catch (error) {
      this.fail(error, context, this.writeCurrent(context, revision, space.id));
    } finally {
      this.finishSaving(saveOperation);
    }
  }

  async changePublication(state: 'activa' | 'oculta'): Promise<void> {
    const context = this.context();
    const space = this.selectedSpace();
    if (!context || !space) return;
    const revision = ++this.resourceGeneration;
    const saveOperation = this.beginSaving();
    this.error.set('');
    this.conflict.set('');
    try {
      const updated = (await this.api.publication(context.token, space.id, {
        state,
      })) as unknown as ExactSpace;
      if (
        !this.contextCurrent(context) ||
        revision !== this.resourceGeneration ||
        this.selectedSpace()?.id !== space.id
      )
        return;
      this.applyDraft(updated);
      this.notice.set(
        state === 'activa'
          ? 'Publicación activa confirmada por Go.'
          : 'Publicación oculta confirmada por Go.',
      );
    } catch (error) {
      this.fail(error, context, this.writeCurrent(context, revision, space.id));
    } finally {
      this.finishSaving(saveOperation);
    }
  }

  async deleteSpace(): Promise<void> {
    const context = this.context();
    const space = this.selectedSpace();
    if (
      !context ||
      !space ||
      !globalThis.confirm(
        `Eliminar ${space.title || 'este espacio'} y solicitar retiro de su publicación?`,
      )
    )
      return;
    const revision = ++this.resourceGeneration;
    const saveOperation = this.beginSaving();
    try {
      const receipt = await this.api.remove(context.token, space.id);
      if (
        !this.contextCurrent(context) ||
        revision !== this.resourceGeneration ||
        this.selectedSpace()?.id !== space.id
      )
        return;
      this.spaces.update((items) => items.filter((item) => item.id !== space.id));
      this.selectedSpace.set(null);
      this.clearEditor();
      this.notice.set(
        `Go confirmó la eliminación. Archivos pendientes de limpieza: ${receipt.files_pending_cleanup}.`,
      );
      await this.router.navigate(['/panel/espacios']);
    } catch (error) {
      this.fail(error, context, this.writeCurrent(context, revision, space.id));
    } finally {
      this.finishSaving(saveOperation);
    }
  }

  async saveTariff(): Promise<void> {
    const context = this.context();
    const space = this.selectedSpace();
    if (!context || !space) return;
    if (!validClp(this.rateForm.base_price))
      return this.error.set('La tarifa debe ser un entero CLP exacto.');
    const revision = ++this.resourceGeneration;
    const saveOperation = this.beginSaving();
    this.simulation.set(null);
    try {
      const value = await this.api.updateTariff(context.token, space.id, {
        rate_unit: this.rateForm.rate_unit,
        base_price: this.rateForm.base_price as unknown as number,
      });
      if (
        !this.contextCurrent(context) ||
        revision !== this.resourceGeneration ||
        this.selectedSpace()?.id !== space.id
      )
        return;
      this.tariff.set(value as never);
      const history = await this.api.tariffVersions(context.token, space.id);
      if (
        !this.contextCurrent(context) ||
        revision !== this.resourceGeneration ||
        this.selectedSpace()?.id !== space.id
      )
        return;
      this.tariffHistory.set(history.items as never);
      this.notice.set(`Tarifa versión ${value.version} guardada por Go.`);
    } catch (error) {
      this.fail(error, context, this.writeCurrent(context, revision, space.id));
    } finally {
      this.finishSaving(saveOperation);
    }
  }

  async simulatePrice(): Promise<void> {
    const context = this.context();
    const space = this.selectedSpace();
    if (!context || !space) return;
    const revision = this.resourceGeneration;
    const selection = this.selectionRevision;
    const request = ++this.simulationQueryGeneration;
    const criteriaRevision = this.queryRevision;
    const criteria = {
      start: this.simulationForm.start_at,
      end: this.simulationForm.end_at,
      timeZone: this.timeZone(),
      rateUnit: this.tariff()?.rate_unit ?? this.rateForm.rate_unit,
      disambiguation: this.disambiguation(),
    };
    this.simulation.set(null);
    try {
      const result = await this.api.simulate(context.token, space.id, {
        start_at: zonedLocalToRfc3339(criteria.start, criteria.timeZone, criteria.disambiguation),
        end_at: zonedLocalToRfc3339(criteria.end, criteria.timeZone, criteria.disambiguation),
      });
      if (
        !this.queryCurrent(
          context,
          revision,
          selection,
          space.id,
          criteriaRevision,
          request,
          criteria,
          'simulation',
        )
      )
        return;
      this.simulation.set({
        base_subtotal: String(result.base_subtotal),
        billed_units: result.billed_units,
        rate_unit: result.rate_unit,
        time_zone: result.time_zone,
      });
      this.notice.set('Simulación privada calculada por Go; no crea una reserva.');
    } catch (error) {
      this.fail(
        error,
        context,
        this.queryCurrent(
          context,
          revision,
          selection,
          space.id,
          criteriaRevision,
          request,
          criteria,
          'simulation',
        ),
      );
    }
  }

  async saveTimeZone(): Promise<void> {
    const context = this.context();
    const space = this.selectedSpace();
    if (!context || !space) return;
    const revision = ++this.resourceGeneration;
    const saveOperation = this.beginSaving();
    this.availability.set(null);
    this.blocks.set([]);
    try {
      const result = await this.api.configureTimeZone(context.token, space.id, this.timeZone());
      if (
        !this.contextCurrent(context) ||
        revision !== this.resourceGeneration ||
        this.selectedSpace()?.id !== space.id
      )
        return;
      this.timeZone.set(result.time_zone);
      this.notice.set(`Go confirmó la zona ${result.time_zone}.`);
      await this.loadBlocks(context, revision, this.selectionRevision, space.id);
      if (
        !this.contextCurrent(context) ||
        revision !== this.resourceGeneration ||
        this.selectedSpace()?.id !== space.id
      )
        return;
    } catch (error) {
      this.fail(error, context, this.writeCurrent(context, revision, space.id));
    } finally {
      this.finishSaving(saveOperation);
    }
  }

  async checkAvailability(): Promise<void> {
    const context = this.context();
    const space = this.selectedSpace();
    if (!context || !space) return;
    const generation = this.resourceGeneration;
    const selection = this.selectionRevision;
    const request = ++this.availabilityQueryGeneration;
    const criteriaRevision = this.queryRevision;
    const criteria = {
      start: this.availabilityForm.start_at,
      end: this.availabilityForm.end_at,
      timeZone: this.timeZone(),
      rateUnit: this.tariff()?.rate_unit ?? this.rateForm.rate_unit,
      disambiguation: this.disambiguation(),
    };
    this.availability.set(null);
    this.conflict.set('');
    try {
      const from = zonedLocalToRfc3339(criteria.start, criteria.timeZone, criteria.disambiguation);
      const to = zonedLocalToRfc3339(criteria.end, criteria.timeZone, criteria.disambiguation);
      const result = await this.api.checkAvailability(context.token, space.id, from, to);
      if (
        !this.queryCurrent(
          context,
          generation,
          selection,
          space.id,
          criteriaRevision,
          request,
          criteria,
          'availability',
        )
      )
        return;
      this.availability.set(result);
      this.conflict.set(
        result.available ? '' : 'Go confirmó un conflicto de disponibilidad en este intervalo.',
      );
    } catch (error) {
      this.fail(
        error,
        context,
        this.queryCurrent(
          context,
          generation,
          selection,
          space.id,
          criteriaRevision,
          request,
          criteria,
          'availability',
        ),
      );
    }
  }

  async addManualBlock(): Promise<void> {
    const context = this.context();
    const space = this.selectedSpace();
    if (!context || !space) return;
    const revision = ++this.resourceGeneration;
    const saveOperation = this.beginSaving();
    this.conflict.set('');
    try {
      const block = await this.api.addBlock(context.token, space.id, {
        start_at: this.toInstant(this.blockForm.start_at),
        end_at: this.toInstant(this.blockForm.end_at),
        reason: this.blockForm.reason,
      });
      if (
        !this.contextCurrent(context) ||
        revision !== this.resourceGeneration ||
        this.selectedSpace()?.id !== space.id
      )
        return;
      this.blocks.update((items) => [block, ...items]);
      this.notice.set('Bloqueo manual confirmado por Go.');
    } catch (error) {
      this.fail(error, context, this.writeCurrent(context, revision, space.id));
    } finally {
      this.finishSaving(saveOperation);
    }
  }

  async removeManualBlock(blockId: string): Promise<void> {
    const context = this.context();
    const space = this.selectedSpace();
    if (!context || !space) return;
    const revision = ++this.resourceGeneration;
    try {
      await this.api.removeBlock(context.token, space.id, blockId);
      if (
        !this.contextCurrent(context) ||
        revision !== this.resourceGeneration ||
        this.selectedSpace()?.id !== space.id
      )
        return;
      this.blocks.update((items) => items.filter((item) => item.id !== blockId));
      this.notice.set('Go desactivó el bloqueo.');
    } catch (error) {
      this.fail(error, context, this.writeCurrent(context, revision, space.id));
    }
  }

  async loadBlocks(
    context = this.context(),
    generation = this.resourceGeneration,
    selection = this.selectionRevision,
    spaceId = this.selectedSpace()?.id ?? '',
  ): Promise<void> {
    if (!context || !spaceId) return;
    const start = new Date();
    const end = new Date(start.getTime() + 45 * 24 * 60 * 60 * 1000);
    try {
      const result = await this.api.blocks(
        context.token,
        spaceId,
        start.toISOString(),
        end.toISOString(),
      );
      if (!this.resourceCurrent(context, generation, selection, spaceId)) return;
      this.blocks.set(result.items);
    } catch (error) {
      this.fail(error, context, this.resourceCurrent(context, generation, selection, spaceId));
    }
  }

  async saveWeekly(): Promise<void> {
    const context = this.context();
    const space = this.selectedSpace();
    const weekly = this.weekly();
    if (!context || !space || !weekly) return;
    const revision = ++this.resourceGeneration;
    const saveOperation = this.beginSaving();
    try {
      const result = await this.api.saveWeekly(context.token, space.id, {
        enabled: this.weeklyForm.enabled,
        days: this.weeklyForm.days as LocalWeeklyHoursEnvelope['data']['days'],
      });
      if (
        !this.contextCurrent(context) ||
        revision !== this.resourceGeneration ||
        this.selectedSpace()?.id !== space.id
      )
        return;
      this.weekly.set(result.data);
      this.weeklyForm = {
        enabled: result.data.enabled,
        days: result.data.days.map((day) => ({
          weekday: day.weekday,
          periods: day.periods.map((period) => ({ ...period })),
        })),
      };
      this.notice.set('Horario semanal confirmado por Go.');
    } catch (error) {
      this.fail(error, context, this.writeCurrent(context, revision, space.id));
    } finally {
      this.finishSaving(saveOperation);
    }
  }

  queuePhotoFiles(files: FileList | readonly File[]): void {
    if (!isPlatformBrowser(this.platform) || !this.selectedSpace()) return;
    const remaining = this.uploadSlotsAvailable();
    const selected = Array.from(files);
    const accepted = selected.slice(0, remaining);
    const tasks = accepted.map((file): PhotoUploadTask => {
      const oversized = file.size > maxPhotoInputBytes;
      let previewUrl: string | null = null;
      if (!oversized && file.type.startsWith('image/')) {
        try {
          previewUrl = URL.createObjectURL(file);
        } catch {
          previewUrl = null;
        }
      }
      return {
        id: this.newIdempotencyKey(),
        key: this.newIdempotencyKey(),
        file,
        previewUrl,
        status: oversized ? 'error' : 'queued',
        progress: 0,
        error: oversized ? 'El archivo supera el máximo de 10 MiB.' : '',
      };
    });
    this.uploadQueue.update((queue) => [...queue, ...tasks]);
    this.uploadMessage.set(
      selected.length > accepted.length
        ? `Solo quedan ${remaining} cupos en esta galería. Se añadieron ${accepted.length} archivos.`
        : '',
    );
  }

  removeQueuedPhoto(id: string): void {
    const task = this.uploadQueue().find((item) => item.id === id);
    if (!task || task.status === 'uploading') return;
    this.revokeUploadUrl(task.previewUrl);
    this.uploadQueue.update((items) => items.filter((item) => item.id !== id));
  }

  async startPhotoUploads(): Promise<void> {
    const context = this.context();
    const space = this.selectedSpace();
    if (!context || !space || !isPlatformBrowser(this.platform) || this.uploadRunning()) return;
    if (!this.uploadQueue().some((task) => task.status === 'queued')) return;
    const generation = this.uploadGeneration;
    this.uploadRunning.set(true);
    this.uploadMessage.set('');
    await Promise.all([this.uploadWorker(context, space.id, generation), this.uploadWorker(context, space.id, generation)]);
    if (this.uploadCurrent(context, space.id, generation)) {
      this.uploadRunning.set(false);
      if (!this.uploadQueue().some((task) => task.status === 'error')) {
        this.notice.set('Las imágenes seleccionadas se guardaron y Go confirmó su estado.');
      }
    }
  }

  retryQueuedPhoto(id: string): void {
    if (this.uploadRunning()) return;
    this.uploadQueue.update((items) => items.map((task) =>
      task.id === id && task.status === 'error'
        ? { ...task, status: 'queued', progress: 0, error: '' }
        : task,
    ));
    void this.startPhotoUploads();
  }

  async updateMediaPhoto(photoId: string, update: SpaceMediaPhotoUpdate): Promise<void> {
    const context = this.context();
    const space = this.selectedSpace();
    if (!context || !space) return;
    const revision = ++this.resourceGeneration;
    const saveOperation = this.beginSaving();
    try {
      const updated = await this.api.updateMediaPhoto(context.token, space.id, photoId, update);
      if (!this.writeCurrent(context, revision, space.id)) return;
      this.gallery.update((items) => [...items.filter((item) => item.id !== photoId), updated]
        .sort((a, b) => (a.position ?? 0) - (b.position ?? 0)));
      this.notice.set('Cambios de la imagen confirmados por Go.');
    } catch (error) {
      this.fail(error, context, this.writeCurrent(context, revision, space.id));
    } finally {
      this.finishSaving(saveOperation);
    }
  }

  async setPhotoVisible(photoId: string, visible: boolean): Promise<void> {
    const photo = this.gallery().find((item) => item.id === photoId);
    if (!photo) return;
    if (isMediaPhoto(photo)) {
      await this.updateMediaPhoto(photoId, { visible });
      return;
    }
    await this.setSyntheticPhotoPublic(photoId, visible);
  }

  async setPhotoPublic(photoId: string, publicInCatalog: boolean): Promise<void> {
    const context = this.context();
    const space = this.selectedSpace();
    if (!context || !space) return;
    const revision = ++this.resourceGeneration;
    const operation = this.beginSaving();
    this.error.set('');
    try {
      const updated = await this.api.setPhotoPublic(context.token, space.id, photoId, publicInCatalog);
      if (!this.writeCurrent(context, revision, space.id)) return;
      this.gallery.update((items) => items.map((item) => item.id === photoId ? updated : item));
    } catch (error) {
      this.fail(error, context, this.writeCurrent(context, revision, space.id));
    } finally {
      this.finishSaving(operation);
    }
  }

  isMediaPhoto(photo: SpaceGalleryPhoto): photo is SpaceMediaPhoto {
    return isMediaPhoto(photo);
  }

  photoIsVisible(photo: SpaceGalleryPhoto): boolean {
    return isMediaPhoto(photo) ? photo.visible : photo.public_in_catalog;
  }

  photoIsCover(photo: SpaceGalleryPhoto): boolean {
    return isMediaPhoto(photo) ? photo.cover : this.selectedSpace()?.cover_photo_id === photo.id;
  }

  uploadFileSize(size: number): string {
    return `${(size / (1024 * 1024)).toFixed(size < 1024 * 1024 ? 2 : 1)} MiB`;
  }

  async addSyntheticPhoto(): Promise<void> {
    const context = this.context();
    const space = this.selectedSpace();
    if (!context || !space) return;
    const payload = { fixture_code: 'synthetic-png-v1' as const };
    const keyId = JSON.stringify([context.session.account_id, space.id, payload]);
    let attempt = this.galleryAttempts.get(keyId);
    if (!attempt) {
      attempt = {
        key: globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random()}`,
        payload,
      };
      this.galleryAttempts.set(keyId, attempt);
    }
    const revision = ++this.resourceGeneration;
    const saveOperation = this.beginSaving();
    try {
      const photo = await this.api.addPhoto(context.token, space.id, attempt.key);
      if (
        !this.contextCurrent(context) ||
        revision !== this.resourceGeneration ||
        this.selectedSpace()?.id !== space.id
      )
        return;
      if (this.galleryAttempts.get(keyId) === attempt) this.galleryAttempts.delete(keyId);
      this.gallery.update((items) => [photo, ...items.filter((item) => item.id !== photo.id)]);
      this.notice.set('Go generó una imagen sintética autorizada.');
    } catch (error) {
      if (
        this.writeCurrent(context, revision, space.id) &&
        statusOf(error) !== undefined &&
        statusOf(error)! >= 400 &&
        statusOf(error)! < 500 &&
        statusOf(error) !== 408 &&
        statusOf(error) !== 429
      ) {
        if (this.galleryAttempts.get(keyId) === attempt) this.galleryAttempts.delete(keyId);
      }
      this.fail(error, context, this.writeCurrent(context, revision, space.id));
    } finally {
      this.finishSaving(saveOperation);
    }
  }

  async selectCover(photoId: string): Promise<void> {
    const context = this.context();
    const space = this.selectedSpace();
    if (!context || !space) return;
    const revision = ++this.resourceGeneration;
    try {
      const cover = await this.api.cover(context.token, space.id, photoId);
      if (
        !this.contextCurrent(context) ||
        revision !== this.resourceGeneration ||
        this.selectedSpace()?.id !== space.id
      )
        return;
      this.selectedSpace.update((item) =>
        item ? { ...item, cover_photo_id: cover.photo_id } : item,
      );
      this.gallery.update((items) => items.map((item) =>
        isMediaPhoto(item) ? { ...item, cover: item.id === photoId } : item,
      ));
      this.notice.set('Portada confirmada por Go.');
    } catch (error) {
      this.fail(error, context, this.writeCurrent(context, revision, space.id));
    }
  }

  async showPhoto(photoId: string): Promise<void> {
    const context = this.context();
    const space = this.selectedSpace();
    if (!context || !space) return;
    const generation = ++this.galleryGeneration;
    const selection = this.selectionRevision;
    this.revokePhoto();
    try {
      const photo = this.gallery().find((item) => item.id === photoId);
      const blob = photo && isMediaPhoto(photo)
        ? await this.api.mediaVariant(context.token, space.id, photoId, 'display')
        : await this.api.photoContent(context.token, space.id, photoId);
      if (
        !this.contextCurrent(context) ||
        generation !== this.galleryGeneration ||
        selection !== this.selectionRevision ||
        this.selectedSpace()?.id !== space.id
      )
        return;
      this.blobUrl = URL.createObjectURL(blob);
      this.previewPhotoId.set(photoId);
      this.photoUrl.set(this.blobUrl);
    } catch (error) {
      this.fail(
        error,
        context,
        this.contextCurrent(context) &&
          generation === this.galleryGeneration &&
          this.activeSpaceId === space.id &&
          this.selectedSpace()?.id === space.id,
      );
    }
  }

  async removePhoto(photoId: string): Promise<void> {
    const context = this.context();
    const space = this.selectedSpace();
    if (!context || !space || !globalThis.confirm('¿Retirar esta imagen de la galería?'))
      return;
    const revision = ++this.resourceGeneration;
    try {
      await this.api.removePhoto(context.token, space.id, photoId);
      if (
        !this.contextCurrent(context) ||
        revision !== this.resourceGeneration ||
        this.selectedSpace()?.id !== space.id
      )
        return;
      this.gallery.update((items) => items.filter((item) => item.id !== photoId));
      if (space.cover_photo_id === photoId)
        this.selectedSpace.update((item) => (item ? { ...item, cover_photo_id: undefined } : item));
      this.revokePhoto();
      this.notice.set('Go retiró la imagen de la galería.');
    } catch (error) {
      this.fail(error, context, this.writeCurrent(context, revision, space.id));
    }
  }

  private async setSyntheticPhotoPublic(photoId:string,enabled:boolean):Promise<void>{
    const context=this.context();const space=this.selectedSpace();if(!context||!space)return;
    const revision=++this.resourceGeneration;const operation=this.beginSaving();this.error.set('');
    try{const updated=await this.api.setPhotoPublic(context.token,space.id,photoId,enabled);if(!this.writeCurrent(context,revision,space.id))return;this.gallery.update(items=>items.map(item=>item.id===photoId?updated:item));}
    catch(error){this.fail(error,context,this.writeCurrent(context,revision,space.id));}
    finally{this.finishSaving(operation);}
  }

  setRateUnit(unit: 'hora' | 'dia' | 'mes'): void {
    if (this.rateForm.rate_unit === unit) return;
    this.rateForm.rate_unit = unit;
    this.simulationForm.start_at = '';
    this.simulationForm.end_at = '';
    this.invalidateQueries();
    this.notice.set('Modalidad cambiada. Revisa el intervalo antes de simular de nuevo.');
  }

  setTimeZone(value: string): void {
    if (value === this.timeZone()) return;
    this.timeZone.set(value);
    this.invalidateQueries();
  }

  setDisambiguation(value: 'earlier' | 'later'): void {
    if (this.disambiguation() === value) return;
    this.disambiguation.set(value);
    this.invalidateQueries();
  }

  setAttribute(code: string, raw: string, type: string): void {
    const next = { ...this.form.attributes };
    if (type === 'boolean') next[code] = raw === 'true';
    else if (type === 'enum_list')
      next[code] = raw
        .split(',')
        .map((value) => value.trim())
        .filter(Boolean);
    else if (type === 'integer' || type === 'number') next[code] = raw === '' ? null : Number(raw);
    else next[code] = raw;
    this.form.attributes = next;
  }

  addPeriod(dayIndex: number): void {
    const day = this.weeklyForm.days[dayIndex];
    if (!day) return;
    if (day.periods.length >= 2) {
      this.error.set('Go admite hasta dos tramos horarios por día.');
      return;
    }
    day.periods.push({ open: '09:00', close: '17:00' });
  }
  removePeriod(dayIndex: number, periodIndex: number): void {
    this.weeklyForm.days[dayIndex]?.periods.splice(periodIndex, 1);
  }

  attributeValue(code: string): string {
    const value = this.form.attributes[code];
    if (Array.isArray(value)) return value.join(', ');
    if (value === null || value === undefined) return '';
    return String(value);
  }

  formatClp(value: string | number | undefined): string {
    if (value === undefined) return 'Sin tarifa';
    try {
      return `CLP ${BigInt(value).toLocaleString('es-CL')}`;
    } catch {
      return 'Importe no disponible';
    }
  }
  labelState(state: ExactSpace['state']): string {
    return state === 'activa' ? 'Publicada' : state === 'oculta' ? 'Oculta' : 'Borrador';
  }
  zoneLabel(value: string): string {
    try {
      return zonedIsoLabel(value, this.timeZone());
    } catch {
      return value;
    }
  }
  async goTo(id: string, section: Section = 'datos'): Promise<void> {
    const part = section === 'datos' ? 'editar' : section;
    await this.router.navigate(['/panel/espacios', id, part]);
  }
  newDraft(): void {
    this.resourceGeneration++;
    this.selectionRevision++;
    this.activeSpaceId = null;
    this.detailLoading.set(false);
    this.resetResourceState();
    const context = this.context();
    if (context && this.form.category_code) void this.loadProfile(context, this.form.category_code);
    void this.router.navigate(['/panel/espacios']);
  }
  closePhoto(): void {
    this.revokePhoto();
  }
  setNotice(value: string): void {
    this.notice.set(value);
  }
  zoneDateTime(
    value: string,
    kind: 'block' | 'availability' | 'simulation',
    endpoint: 'start_at' | 'end_at',
  ): void {
    if (kind === 'block') this.blockForm[endpoint] = value;
    if (kind === 'availability') this.availabilityForm[endpoint] = value;
    if (kind === 'simulation') this.simulationForm[endpoint] = value;
    this.invalidateQueries();
  }

  private async loadProfile(context: RequestContext, category: string): Promise<void> {
    const generation = ++this.profileGeneration;
    try {
      const result = await this.api.profile(context.token, category);
      if (
        !this.contextCurrent(context) ||
        generation !== this.profileGeneration ||
        this.form.category_code !== category
      )
        return;
      this.profile.set(result);
      this.form.attribute_schema_version = result.schema_version;
    } catch (error) {
      this.fail(
        error,
        context,
        this.contextCurrent(context) &&
          generation === this.profileGeneration &&
          this.form.category_code === category,
      );
    }
  }
  private makePayload(form: DraftForm): SpaceDraftInput | null {
    if (
      !form.category_code ||
      !form.title.trim() ||
      !form.address.trim() ||
      (!!form.region_code !== !!form.comuna_code) ||
      !validClp(form.base_price_clp)
    ) {
      this.error.set('Completa categoría, título, dirección y una tarifa CLP entera válida.');
      return null;
    }
    return {
      category_code: form.category_code,
      title: form.title.trim(),
      description: form.description,
      area_m2: Number(form.area_m2),
      capacity: Number(form.capacity),
      usage_rules: form.usage_rules,
      rate_unit: form.rate_unit,
      base_price_clp: form.base_price_clp as unknown as number,
      address: form.address,
      ...(form.region_code && form.comuna_code
        ? { region_code: form.region_code, comuna_code: form.comuna_code }
        : {}),
      attribute_schema_version: form.attribute_schema_version,
      attributes: form.attributes,
    };
  }
  private mergeChangedFields(
    original: DraftForm,
    edited: DraftForm,
    latest: ExactSpace,
  ): DraftForm {
    const merged: DraftForm = {
      category_code: latest.category_code,
      title: latest.title,
      description: latest.description,
      area_m2: latest.area_m2,
      capacity: latest.capacity,
      usage_rules: latest.usage_rules,
      rate_unit: latest.rate_unit,
      base_price_clp: String(latest.base_price_clp),
      address: latest.address,
      region_code: latest.region_code ?? '',
      comuna_code: latest.comuna_code ?? '',
      attribute_schema_version: latest.attribute_schema_version,
      attributes: { ...latest.attributes },
    };
    const locationChanged =
      edited.region_code !== original.region_code || edited.comuna_code !== original.comuna_code;
    for (const key of Object.keys(merged) as Array<keyof DraftForm>) {
      if (key === 'attributes' || key === 'region_code' || key === 'comuna_code') continue;
      if (JSON.stringify(edited[key]) !== JSON.stringify(original[key])) {
        (merged as unknown as Record<string, unknown>)[key] = edited[key];
      }
    }
    if (locationChanged) {
      merged.region_code = edited.region_code;
      merged.comuna_code = edited.comuna_code;
    }
    const attributeKeys = new Set([
      ...Object.keys(original.attributes),
      ...Object.keys(edited.attributes),
    ]);
    for (const key of attributeKeys) {
      if (JSON.stringify(edited.attributes[key]) !== JSON.stringify(original.attributes[key])) {
        if (edited.attributes[key] === undefined) delete merged.attributes[key];
        else merged.attributes[key] = edited.attributes[key];
      }
    }
    // Version follows the changed category/profile, while keeping a newer
    // server version when the category itself was not edited.
    if (edited.category_code !== original.category_code) {
      merged.attribute_schema_version = edited.attribute_schema_version;
    }
    return merged;
  }
  private setForm(space: ExactSpace): void {
    const next: DraftForm = {
      category_code: space.category_code,
      title: space.title,
      description: space.description,
      area_m2: space.area_m2,
      capacity: space.capacity,
      usage_rules: space.usage_rules,
      rate_unit: space.rate_unit,
      base_price_clp: String(space.base_price_clp),
      address: space.address,
      region_code: space.region_code ?? '',
      comuna_code: space.comuna_code ?? '',
      attribute_schema_version: space.attribute_schema_version,
      attributes: { ...space.attributes },
    };
    Object.assign(this.form, next);
    this.originalForm.set(space.id, structuredClone(next));
  }
  private applyDraft(space: ExactSpace): void {
    this.selectedSpace.set(space);
    this.spaces.update((items) => items.map((item) => (item.id === space.id ? space : item)));
    this.setForm(space);
    this.publishedForm.title = space.title;
    this.publishedForm.description = space.description;
    this.publishedForm.capacity = space.capacity;
    this.publishedForm.usage_rules = space.usage_rules;
  }
  private toInstant(value: string): string {
    return zonedLocalToRfc3339(value, this.timeZone(), this.disambiguation());
  }
  private context(): RequestContext | null {
    const token = this.identity.accessToken();
    const session = this.identity.session();
    return token && session ? { token, session } : null;
  }
  private contextCurrent(context: RequestContext): boolean {
    return (
      !this.destroyed &&
      this.identity.accessToken() === context.token &&
      this.identity.session() === context.session
    );
  }

  private uploadCurrent(context: RequestContext, spaceId: string, generation: number): boolean {
    return this.contextCurrent(context) && generation === this.uploadGeneration &&
      this.selectedSpace()?.id === spaceId;
  }

  private async uploadWorker(context: RequestContext, spaceId: string, generation: number): Promise<void> {
    while (this.uploadCurrent(context, spaceId, generation)) {
      const task = this.takeQueuedUpload();
      if (!task) return;
      try {
        const photo = await this.api.uploadPhoto(context.token, spaceId, task.file, task.key, (progress) => {
          if (!this.uploadCurrent(context, spaceId, generation)) return;
          this.uploadQueue.update((items) => items.map((item) =>
            item.id === task.id ? { ...item, progress } : item,
          ));
        });
        if (!this.uploadCurrent(context, spaceId, generation)) return;
        this.gallery.update((items) => [...items.filter((item) => item.id !== photo.id), photo]
          .sort((a, b) => (a.position ?? 0) - (b.position ?? 0)));
        this.releaseUpload(task.id);
        this.notice.set('Go guardó la imagen y confirmó su estado.');
      } catch (error) {
        if (!this.uploadCurrent(context, spaceId, generation)) return;
        const message = uploadFailureMessage(error);
        this.uploadQueue.update((items) => items.map((item) =>
          item.id === task.id ? { ...item, status: 'error', progress: null, error: message } : item,
        ));
        this.fail(error, context, true);
      }
    }
  }

  private takeQueuedUpload(): PhotoUploadTask | null {
    let next: PhotoUploadTask | null = null;
    this.uploadQueue.update((items) => items.map((item) => {
      if (!next && item.status === 'queued') {
        next = { ...item, status: 'uploading', progress: 0, error: '' };
        return next;
      }
      return item;
    }));
    return next;
  }

  private releaseUpload(id: string): void {
    const task = this.uploadQueue().find((item) => item.id === id);
    if (task?.previewUrl) this.revokeUploadUrl(task.previewUrl);
    this.uploadQueue.update((items) => items.filter((item) => item.id !== id));
  }

  private revokeUploadUrl(url: string | null): void {
    if (url && typeof URL.revokeObjectURL === 'function') URL.revokeObjectURL(url);
  }

  private clearUploadQueue(): void {
    this.uploadGeneration++;
    for (const task of this.uploadQueue()) this.revokeUploadUrl(task.previewUrl);
    this.uploadQueue.set([]);
    this.uploadRunning.set(false);
    this.uploadMessage.set('');
  }

  private newIdempotencyKey(): string {
    return globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random()}`;
  }

  private draftWriteCurrent(
    context: RequestContext,
    revision: number,
    selection: number,
    spaceId: string,
  ): boolean {
    return (
      this.contextCurrent(context) &&
      revision === this.resourceGeneration &&
      selection === this.selectionRevision &&
      this.selectedSpace()?.id === spaceId
    );
  }
  private writeCurrent(context: RequestContext, revision: number, spaceId: string | null): boolean {
    return (
      this.contextCurrent(context) &&
      revision === this.resourceGeneration &&
      this.activeSpaceId === spaceId &&
      (spaceId === null ? this.selectedSpace() === null : this.selectedSpace()?.id === spaceId)
    );
  }
  private queryCurrent(
    context: RequestContext,
    revision: number,
    selection: number,
    spaceId: string,
    criteriaRevision: number,
    request: number,
    criteria: {
      start: string;
      end: string;
      timeZone: string;
      rateUnit: string;
      disambiguation: 'earlier' | 'later';
    },
    kind: 'simulation' | 'availability',
  ): boolean {
    const requestCurrent =
      kind === 'simulation'
        ? request === this.simulationQueryGeneration
        : request === this.availabilityQueryGeneration;
    const form = kind === 'simulation' ? this.simulationForm : this.availabilityForm;
    return (
      this.resourceCurrent(context, revision, selection, spaceId) &&
      criteriaRevision === this.queryRevision &&
      requestCurrent &&
      form.start_at === criteria.start &&
      form.end_at === criteria.end &&
      this.timeZone() === criteria.timeZone &&
      this.disambiguation() === criteria.disambiguation &&
      (this.tariff()?.rate_unit ?? this.rateForm.rate_unit) === criteria.rateUnit
    );
  }
  private loadCurrent(context: RequestContext, generation: number): boolean {
    return this.contextCurrent(context) && generation === this.loadGeneration;
  }
  private resourceCurrent(
    context: RequestContext,
    generation: number,
    selection: number,
    spaceId: string,
  ): boolean {
    return (
      this.contextCurrent(context) &&
      generation === this.resourceGeneration &&
      selection === this.selectionRevision &&
      this.activeSpaceId === spaceId
    );
  }
  private fail(error: unknown, context: RequestContext, canPresent: boolean): void {
    if (statusOf(error) === 401) {
      if (this.identity.expireIfCurrent(context.token)) this.clearPrivate();
      return;
    }
    const message = goErrorMessage(error);
    if (!canPresent) return;
    if (statusOf(error) === 409) this.conflict.set(message);
    this.error.set(message);
  }
  private revokePhoto(): void {
    if (this.blobUrl) URL.revokeObjectURL(this.blobUrl);
    this.blobUrl = null;
    this.photoUrl.set(null);
    this.previewPhotoId.set(null);
  }
  private clearEditor(): void {
    this.clearUploadQueue();
    this.profile.set(null);
    this.gallery.set([]);
    this.tariff.set(null);
    this.tariffHistory.set([]);
    this.weekly.set(null);
    this.blocks.set([]);
    this.availability.set(null);
    this.simulation.set(null);
    this.form.category_code = this.categories()[0]?.code ?? '';
    Object.assign(this.form, emptyForm(), { category_code: this.categories()[0]?.code ?? '' });
    this.revokePhoto();
  }
  private resetResourceState(): void {
    this.clearUploadQueue();
    this.profileGeneration++;
    this.selectedSpace.set(null);
    this.profile.set(null);
    this.gallery.set([]);
    this.maxPhotos.set(10);
    this.tariff.set(null);
    this.tariffHistory.set([]);
    this.weekly.set(null);
    this.blocks.set([]);
    this.availability.set(null);
    this.simulation.set(null);
    this.conflict.set('');
    this.error.set('');
    this.notice.set('');
    this.timeZone.set('America/Santiago');
    Object.assign(this.form, emptyForm(), { category_code: this.categories()[0]?.code ?? '' });
    this.publishedForm.title = '';
    this.publishedForm.description = '';
    this.publishedForm.capacity = 1;
    this.publishedForm.usage_rules = '';
    this.rateForm.rate_unit = 'hora';
    this.rateForm.base_price = '5001';
    this.weeklyForm = {
      enabled: false,
      days: dayNames.map((_, index) => ({ weekday: index + 1, periods: [] })),
    };
    this.simulationForm.start_at = '';
    this.simulationForm.end_at = '';
    this.blockForm.start_at = '';
    this.blockForm.end_at = '';
    this.blockForm.reason = 'bloqueo_anfitrion';
    this.availabilityForm.start_at = '';
    this.availabilityForm.end_at = '';
    this.invalidateQueries();
    this.revokePhoto();
  }
  private beginSaving(): number {
    this.savingGeneration++;
    this.saving.set(true);
    return this.savingGeneration;
  }
  private finishSaving(operation: number): void {
    if (operation === this.savingGeneration) this.saving.set(false);
  }
  private invalidateQueries(): void {
    this.queryRevision++;
    this.simulationQueryGeneration++;
    this.availabilityQueryGeneration++;
    this.simulation.set(null);
    this.availability.set(null);
    this.conflict.set('');
  }
  private clearPrivate(): void {
    this.loadGeneration++;
    this.resourceGeneration++;
    this.profileGeneration++;
    this.savingGeneration++;
    this.galleryGeneration++;
    this.clearUploadQueue();
    this.selectionRevision++;
    this.activeSpaceId = null;
    this.originalForm.clear();
    this.spaces.set([]);
    this.categories.set([]);
    this.publicLocations.set([]);
    this.locationsLoading.set(false);
    this.locationsError.set('');
    this.selectedSpace.set(null);
    this.gallery.set([]);
    this.tariff.set(null);
    this.tariffHistory.set([]);
    this.weekly.set(null);
    this.blocks.set([]);
    this.profile.set(null);
    this.loading.set(false);
    this.detailLoading.set(false);
    this.saving.set(false);
    this.error.set('');
    this.loadingError.set('');
    this.notice.set('');
    this.conflict.set('');
    this.simulation.set(null);
    this.availability.set(null);
    this.queryRevision++;
    this.simulationQueryGeneration++;
    this.availabilityQueryGeneration++;
    Object.assign(this.form, emptyForm());
    this.timeZone.set('America/Santiago');
    this.publishedForm.title = '';
    this.publishedForm.description = '';
    this.publishedForm.capacity = 1;
    this.publishedForm.usage_rules = '';
    this.rateForm.rate_unit = 'hora';
    this.rateForm.base_price = '5001';
    this.simulationForm.start_at = '';
    this.simulationForm.end_at = '';
    this.blockForm.start_at = '';
    this.blockForm.end_at = '';
    this.availabilityForm.start_at = '';
    this.availabilityForm.end_at = '';
    this.weeklyForm = {
      enabled: false,
      days: dayNames.map((_, index) => ({ weekday: index + 1, periods: [] })),
    };
    this.revokePhoto();
  }
}

function validClp(value: string): boolean {
  return /^\d+$/.test(value) && BigInt(value) >= 5001n && BigInt(value) <= 9223372036854775807n;
}
function statusOf(error: unknown): number | undefined {
  return (error as { status?: number })?.status;
}

function goErrorMessage(error: unknown): string {
  const status = statusOf(error);
  let body = (error as { error?: unknown })?.error;
  if (typeof body === 'string') {
    try {
      body = JSON.parse(body) as unknown;
    } catch {
      body = undefined;
    }
  }
  const envelope = body as
    { error?: { code?: string; message?: string }; code?: string } | undefined;
  const code = envelope?.error?.code ?? envelope?.code;
  const byCode: Record<string, string> = {
    eligibility_required:
      'Go no habilita esta publicación: verifica la elegibilidad KYC efectiva de la cuenta.',
    time_zone_required: 'Go requiere una zona horaria IANA confirmada antes de publicar.',
    time_zone_locked: 'Go no permite cambiar la zona horaria de un espacio con calendario en uso.',
    fixture_enabled: 'Go no permite ocultar el espacio mientras siga habilitado como fixture.',
    availability_conflict: 'Go detectó un conflicto de calendario para este intervalo.',
    calendar_conflict: 'Go detectó un conflicto con el calendario vigente.',
  };
  if (code && byCode[code]) return byCode[code];
  if (status === 401) return 'Go cerró la sesión. Inicia sesión nuevamente.';
  if (status === 403) return 'Go rechazó la operación por permisos de esta cuenta.';
  if (status === 404) return 'Go no encontró un espacio propio disponible para esta cuenta.';
  if (status === 409)
    return 'Go detectó un conflicto con el estado, la elegibilidad o el calendario vigente.';
  if (status === 422)
    return 'Go rechazó los campos o intervalos enviados. Revisa las reglas del formulario.';
  if (error instanceof Error && !(error as { error?: unknown }).error) return error.message;
  return 'Go no pudo completar la solicitud.';
}

function uploadFailureMessage(error: unknown): string {
  const status = statusOf(error);
  if (status === 0 || status === undefined)
    return 'No se confirmó la carga. Puedes reintentar; se conservará la misma clave segura de operación.';
  if (status === 409)
    return 'La galería cambió durante la carga. Actualiza el espacio e inténtalo otra vez.';
  if (status === 413)
    return 'El archivo supera el máximo de carga permitido de 10 MiB.';
  if (status === 415 || status === 422)
    return 'Go no pudo validar la imagen. Usa un JPEG, PNG o WebP estático de hasta 25 megapíxeles.';
  if (status === 503)
    return 'El almacenamiento no está disponible ahora. Reintenta esta imagen más tarde.';
  return goErrorMessage(error);
}
