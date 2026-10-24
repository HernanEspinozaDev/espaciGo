import { DatePipe } from '@angular/common';
import { Component, DestroyRef, effect, inject, signal } from '@angular/core';
import { FormControl, FormGroup, FormRecord, ReactiveFormsModule } from '@angular/forms';
import { IdentitySession } from '../../core/identity/application/identity-session';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { GlobalAdminFacade, type DirectoryCriteria } from './application/global-admin-facade';
import type { Publication } from './domain/global-admin-models';
import type { AccountResourceKind } from './domain/global-admin-models';
import type { AuditFilter } from './domain/global-admin-models';

type DirectoryFilters = FormGroup<{
  accountId: FormControl<string>;
  emailPrefix: FormControl<string>;
  namePrefix: FormControl<string>;
  role: FormControl<string>;
  state: FormControl<string>;
  blocked: FormControl<string>;
  createdFrom: FormControl<string>;
  createdUntil: FormControl<string>;
}>;
type PublicationFilters = FormGroup<{search:FormControl<string>;ownerId:FormControl<string>;category:FormControl<string>;state:FormControl<string>;visible:FormControl<string>}>;
type GlobalAdminSection = 'accounts' | 'publications' | 'kpis' | 'audit';

@Component({
  selector: 'app-global-admin-page',
  imports: [DatePipe, ReactiveFormsModule],
  templateUrl: './global-admin-page.html',
  styleUrl: './global-admin-page.scss',
  providers: [GlobalAdminFacade],
})
export class GlobalAdminPage {
  private readonly identity = inject(IdentitySession);
  private readonly privateSession = inject(PrivateSessionState);
  private readonly destroyRef = inject(DestroyRef);
  private previousIdentityContext: string | undefined;
  private previousResetRevision: number | undefined;
  private privateControlsGeneration = 0;
  private publicationEditorGeneration = 0;
  private galleryPanelGeneration = 0;
  private schedulePanelGeneration = 0;
  readonly vm = inject(GlobalAdminFacade);
  readonly hasCapability=(capability:import('../../core/contracts/models').AdminCapability)=>this.identity.session()?.capabilities?.includes(capability)??false;
  readonly objectKeys=(value:Record<string,unknown>)=>Object.keys(value);
  readonly overview = this.vm.overview;
  readonly page = this.vm.page;
  readonly selected = this.vm.selected;
  readonly accountResources = this.vm.accountResources;
  readonly accountResourceDetail = this.vm.accountResourceDetail;
  readonly loadingResources = this.vm.loadingResources;
  readonly accountResourceKind = signal<AccountResourceKind>('reservations');
  readonly loadingDirectory = this.vm.loadingDirectory;
  readonly loadingDetail = this.vm.loadingDetail;
  readonly publications = this.vm.publications;
  readonly loadingPublications = this.vm.loadingPublications;
  readonly activeTab = signal<GlobalAdminSection>('accounts');
  readonly reportFrom=new FormControl('',{nonNullable:true});readonly reportUntil=new FormControl('',{nonNullable:true});readonly reportZone=new FormControl('America/Santiago',{nonNullable:true});
  readonly auditFrom=new FormControl('',{nonNullable:true});readonly auditUntil=new FormControl('',{nonNullable:true});readonly auditActor=new FormControl('',{nonNullable:true});readonly auditTarget=new FormControl('',{nonNullable:true});readonly auditResource=new FormControl('',{nonNullable:true});readonly auditResourceId=new FormControl('',{nonNullable:true});readonly auditAction=new FormControl('',{nonNullable:true});readonly auditReason=new FormControl('',{nonNullable:true});readonly auditCorrelation=new FormControl('',{nonNullable:true});readonly auditResult=new FormControl('',{nonNullable:true});
  readonly appliedAudit=signal<AuditFilter|null>(null);
  readonly appliedPublicationCriteria = signal<DirectoryCriteria>({});
  readonly publicationFilters:PublicationFilters=new FormGroup({search:new FormControl('',{nonNullable:true}),ownerId:new FormControl('',{nonNullable:true}),category:new FormControl('',{nonNullable:true}),state:new FormControl('',{nonNullable:true}),visible:new FormControl('',{nonNullable:true})});
  readonly error = this.vm.error;
  readonly appliedCriteria = signal<DirectoryCriteria>({});
  readonly filters: DirectoryFilters = new FormGroup({
    accountId: new FormControl('', { nonNullable: true }),
    emailPrefix: new FormControl('', { nonNullable: true }),
    namePrefix: new FormControl('', { nonNullable: true }),
    role: new FormControl('', { nonNullable: true }),
    state: new FormControl('', { nonNullable: true }),
    blocked: new FormControl('', { nonNullable: true }),
    createdFrom: new FormControl('', { nonNullable: true }),
    createdUntil: new FormControl('', { nonNullable: true }),
  });
  readonly profileForm = new FormGroup({
    displayName: new FormControl('', { nonNullable: true }),
    phone: new FormControl('', { nonNullable: true }),
    reason: new FormControl('correccion_perfil', { nonNullable: true }),
  });
  readonly canEditAccount = () => this.identity.session()?.capabilities?.includes('accounts.update') ?? false;
  readonly canManageRoles = () => this.identity.session()?.capabilities?.includes('accounts.roles') ?? false;
  readonly canRevokeSessions = () => this.identity.session()?.capabilities?.includes('accounts.block') ?? false;
  readonly canBlockAccounts = this.canRevokeSessions;
  readonly canModeratePublications = () => this.identity.session()?.capabilities?.includes('publications.moderate') ?? false;
  readonly canRetirePublications = () => this.identity.session()?.capabilities?.includes('publications.retire') ?? false;
  readonly canEditPublications = () => this.identity.session()?.capabilities?.includes('publications.update') ?? false;
  readonly canReadPublications = () => this.identity.session()?.capabilities?.includes('publications.read') ?? false;
  readonly galleryPublication = signal<Publication|null>(null);
  readonly publicationGallery = this.vm.publicationGallery;
  readonly loadingPublicationGallery = this.vm.loadingPublicationGallery;
  readonly schedulePublication = signal<Publication|null>(null);
  readonly publicationSchedule = this.vm.publicationSchedule;
  readonly loadingPublicationSchedule = this.vm.loadingPublicationSchedule;
  readonly scheduleEnabled = new FormControl(false,{nonNullable:true});
  readonly schedulePeriods = new FormRecord<FormControl<string>>({});
  readonly editingPublication = signal<Publication|null>(null);
  readonly retiringPublication = signal<Publication|null>(null);
  readonly retirementReason = new FormControl<'revision_administrativa'|'moderacion_contenido'|'riesgo_seguridad'|'solicitud_titular'>('revision_administrativa', {nonNullable:true});
  readonly publicationEditForm=new FormGroup({title:new FormControl('',{nonNullable:true}),description:new FormControl('',{nonNullable:true}),capacity:new FormControl('',{nonNullable:true}),usageRules:new FormControl('',{nonNullable:true}),price:new FormControl('',{nonNullable:true})});
  readonly attributeForm = new FormRecord<FormControl<string|boolean>>({});
  readonly confirmSessionRevocation = signal(false);
  readonly notice = signal('');

  constructor() {
    effect(() => {
      const account=this.selected();
      if(account)this.profileForm.patchValue({displayName:account.display_name??'',phone:account.phone??''},{emitEvent:false});
      else this.profileForm.reset({displayName:'',phone:'',reason:'correccion_perfil'},{emitEvent:false});
    });
    const unregister = this.privateSession.register(() => this.clearPrivateControls());
    this.destroyRef.onDestroy(unregister);
    effect(() => {
      const controlsResetRevision = this.vm.controlsResetRevision();
      const session = this.identity.session();
      const token = this.identity.accessToken();
      const capabilities = [...(session?.capabilities ?? [])].sort().join(',');
      const context = [token ?? '', session?.account_id ?? '', capabilities].join('|');
      const canList = session?.capabilities?.includes('accounts.list') ?? false;
      if (this.previousIdentityContext !== undefined && context !== this.previousIdentityContext) {
        this.clearPrivateControls();
      }
      if (
        this.previousResetRevision !== undefined &&
        controlsResetRevision !== this.previousResetRevision
      ) {
        this.clearPrivateControls();
      }
      this.previousIdentityContext = context;
      this.previousResetRevision = controlsResetRevision;
      if (!token || !canList) this.clearPrivateControls();
      if (!this.canOpenSection(this.activeTab())) this.activeTab.set('accounts');
    });
  }

  search(): Promise<void> {
    const form = this.filters.getRawValue();
    const criteria: DirectoryCriteria = {
      account_id: form.accountId.trim() || undefined,
      email_prefix: form.emailPrefix.trim() || undefined,
      name_prefix: form.namePrefix.trim() || undefined,
      role: form.role || undefined,
      state: form.state || undefined,
      administrative_blocked: form.blocked || undefined,
      created_from: form.createdFrom ? new Date(form.createdFrom).toISOString() : undefined,
      created_until: form.createdUntil ? new Date(form.createdUntil).toISOString() : undefined,
    };
    this.appliedCriteria.set(criteria);
    return this.vm.load(criteria);
  }

  open(accountId: string): Promise<void> {
    return this.vm.open(accountId);
  }

  closeAccountDetail(): void { this.vm.closeAccount(); }
  loadAccountResources(kind: AccountResourceKind): Promise<void> { this.accountResourceKind.set(kind); return this.vm.loadAccountResources(kind); }
  nextAccountResources(): void { const cursor=this.accountResources()?.next_cursor;if(cursor)void this.vm.loadAccountResources(this.accountResourceKind(),cursor); }
  openResourceDetail(id: string): Promise<void> { return this.vm.openAccountResource(this.accountResourceKind(),id); }
  resourceDetailText(): string { const detail=this.accountResourceDetail();return detail?JSON.stringify(detail,null,2):''; }

  saveProfile(): void {
    const account=this.selected();if(!account||!account.profile_updated_at||this.profileForm.invalid)return;
    const value=this.profileForm.getRawValue();
    void this.vm.updateProfile({display_name:value.displayName,phone:value.phone.trim()||null,expected_updated_at:account.profile_updated_at,reason:value.reason});
  }

  setRole(role:string,granted:boolean):void { const reason=this.profileForm.controls.reason.value;void this.vm.updateRole(role,{granted,reason}); }
  async revokeSessions(): Promise<void> {
    if (!this.confirmSessionRevocation()) return;
    const generation = this.privateControlsGeneration;
    const result = await this.vm.revokeSessions({ reason: this.profileForm.controls.reason.value });
    if (generation !== this.privateControlsGeneration || result.status !== 'success') return;
    this.notice.set(`Sesiones revocadas: ${result.value}.`);
    this.confirmSessionRevocation.set(false);
  }
  async setBlocked(blocked:boolean):Promise<void>{const selected=this.selected();if(!selected)return;await this.vm.setBlocked({blocked,reason:blocked?'revision_administrativa':'riesgo_resuelto'});}

  clearFilters(): void {
    this.filters.reset();
    this.appliedCriteria.set({});
    void this.vm.load({});
  }

  showTab(tab: GlobalAdminSection): void {
    if (!this.canOpenSection(tab)) return;
    this.activeTab.set(tab);
    if (tab === 'publications' && !this.publications()) void this.searchPublications();
    if (tab !== 'kpis' && tab !== 'audit') return;

    const now = new Date();
    const week = new Date(now.getTime() - 7 * 86400000);
    const local = (date: Date) => new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16);
    const from = local(week);
    const until = local(now);
    if (tab === 'kpis') {
      if (!this.reportFrom.value) this.reportFrom.setValue(from);
      if (!this.reportUntil.value) this.reportUntil.setValue(until);
      void this.searchKPIs();
      return;
    }
    if (!this.auditFrom.value) this.auditFrom.setValue(from);
    if (!this.auditUntil.value) this.auditUntil.setValue(until);
  }

  private canOpenSection(section: GlobalAdminSection): boolean {
    switch (section) {
      case 'accounts': return this.hasCapability('accounts.list');
      case 'publications': return this.hasCapability('publications.list');
      case 'kpis': return this.hasCapability('kpi.read');
      case 'audit': return this.hasCapability('audit.read');
    }
  }
  searchKPIs():void{if(!this.reportFrom.value||!this.reportUntil.value)return;void this.vm.loadKPIs({from:new Date(this.reportFrom.value).toISOString(),until:new Date(this.reportUntil.value).toISOString(),time_zone:this.reportZone.value});}
  searchAudit():void{if(!this.auditFrom.value||!this.auditUntil.value)return;const applied:AuditFilter={from:new Date(this.auditFrom.value).toISOString(),until:new Date(this.auditUntil.value).toISOString(),actor_id:this.auditActor.value.trim()||undefined,target_account_id:this.auditTarget.value.trim()||undefined,resource_type:this.auditResource.value.trim()||undefined,resource_id:this.auditResourceId.value.trim()||undefined,action:this.auditAction.value.trim()||undefined,reason_code:this.auditReason.value.trim()||undefined,correlation_id:this.auditCorrelation.value.trim()||undefined,result:this.auditResult.value||undefined,page_size:25};this.appliedAudit.set(applied);void this.vm.loadAudit(applied);}
  nextAudit():void{const f=this.appliedAudit(),page=this.vm.auditPage();if(f&&page?.next_cursor)void this.vm.loadAudit({...f,cursor:page.next_cursor,snapshot_cutoff:page.snapshot_cutoff});}
  async downloadAudit(format:'json'|'csv'):Promise<void>{const f=this.appliedAudit(),page=this.vm.auditPage();if(!f||!page)return;const result=await this.vm.exportAudit({...f,snapshot_cutoff:page.snapshot_cutoff},format);if(!result)return;const url=URL.createObjectURL(result.blob);try{const a=document.createElement('a');a.href=url;a.download=`gadm-audit.${format}`;a.click();}finally{URL.revokeObjectURL(url);}}
  searchPublications():Promise<void>{const v=this.publicationFilters.getRawValue();const criteria:DirectoryCriteria={search:v.search.trim()||undefined,owner_id:v.ownerId.trim()||undefined,category:v.category||undefined,state:v.state||undefined,publicly_visible:v.visible||undefined};this.appliedPublicationCriteria.set(criteria);return this.vm.loadPublications(criteria);}
  nextPublications():void{const cursor=this.publications()?.next_cursor;if(cursor)void this.vm.loadPublications(this.appliedPublicationCriteria(),cursor);}
  async changePublicationState(
    item: Publication,
    state: 'activa' | 'oculta',
  ): Promise<void> {
    const generation = this.privateControlsGeneration;
    const result = await this.vm.setPublicationState(item.publication_id, {
      state,
      expected_updated_at: item.updated_at,
      reason: 'revision_administrativa',
    });
    if (generation !== this.privateControlsGeneration || result.status !== 'success') return;
    await this.vm.loadPublications(this.appliedPublicationCriteria());
  }

  editPublication(item: Publication): void {
    this.publicationEditorGeneration++;
    this.editingPublication.set(item);
    this.publicationEditForm.setValue({
      title: item.title,
      description: item.description,
      capacity: String(item.capacity),
      usageRules: item.usage_rules,
      price: item.base_price_clp,
    });
    for(const key of Object.keys(this.attributeForm.controls))this.attributeForm.removeControl(key);
    for(const definition of item.attribute_schema.attributes){const value=item.attributes[definition.code];const initial=definition.type==='boolean'?Boolean(value):Array.isArray(value)?value.join(','):value===undefined?'':String(value);this.attributeForm.addControl(definition.code,new FormControl<string|boolean>(initial,{nonNullable:true}));}
  }

  async savePublication(): Promise<void> {
    const item = this.editingPublication();
    const value = this.publicationEditForm.getRawValue();
    const capacity = Number(value.capacity);
    if (!item || !Number.isSafeInteger(capacity) || capacity < 1) return;
    const generation = this.privateControlsGeneration;
    const editorGeneration = this.publicationEditorGeneration;
    const itemAttributes:Record<string,unknown>={};
    for(const definition of item.attribute_schema.attributes){const raw=this.attributeForm.controls[definition.code]?.value;if(definition.type==='boolean'){itemAttributes[definition.code]=raw===true||raw==='true';continue;}const text=String(raw??'').trim();if(!text)continue;if(definition.type==='integer'){const parsed=Number(text);if(!Number.isSafeInteger(parsed))return;itemAttributes[definition.code]=parsed;}else if(definition.type==='number'){const parsed=Number(text);if(!Number.isFinite(parsed))return;itemAttributes[definition.code]=parsed;}else if(definition.type==='enum_list'){itemAttributes[definition.code]=text.split(',').map(part=>part.trim()).filter(Boolean);}else itemAttributes[definition.code]=text;}
    const result = await this.vm.updatePublication(item.publication_id, {
      title: value.title,
      description: value.description,
      capacity,
      usage_rules: value.usageRules,
      base_price_clp: value.price,
      attribute_schema_version:item.attribute_schema_version,
      attributes:itemAttributes,
      expected_updated_at: item.updated_at,
      reason: 'revision_administrativa',
    });
    if (generation !== this.privateControlsGeneration || editorGeneration !== this.publicationEditorGeneration || result.status !== 'success') return;
    if (this.editingPublication()?.publication_id !== item.publication_id) return;
    this.clearPublicationEditor();
    await this.vm.loadPublications(this.appliedPublicationCriteria());
  }

  cancelPublicationEdit(): void {
    this.clearPublicationEditor();
  }

  requestRetirement(item: Publication): void { this.retiringPublication.set(item); }

  openPublicationGallery(item: Publication): void {
    this.galleryPanelGeneration++;
    this.galleryPublication.set(item);
    void this.vm.loadPublicationGallery(item);
  }

  closePublicationGallery(): void {
    this.galleryPanelGeneration++;
    this.galleryPublication.set(null);
    this.vm.clearPublicationGallery();
  }

  async openPublicationSchedule(item: Publication): Promise<void> {
    const panelGeneration = ++this.schedulePanelGeneration;
    this.schedulePublication.set(item);
    this.scheduleEnabled.setValue(false);
    for (const key of Object.keys(this.schedulePeriods.controls)) this.schedulePeriods.removeControl(key);
    await this.vm.loadPublicationSchedule(item);
    if (panelGeneration !== this.schedulePanelGeneration || this.schedulePublication()?.publication_id !== item.publication_id) return;
    const value = this.publicationSchedule();
    if (!value) return;
    this.scheduleEnabled.setValue(value.enabled);
    for (const day of value.days) this.schedulePeriods.addControl(String(day.weekday), new FormControl(day.periods.map(period => `${period.open}-${period.close}`).join(', '), { nonNullable: true }));
  }

  closePublicationSchedule(): void {
    this.schedulePanelGeneration++;
    this.schedulePublication.set(null);
    this.vm.clearPublicationSchedule();
    for (const key of Object.keys(this.schedulePeriods.controls)) this.schedulePeriods.removeControl(key);
  }

  async savePublicationSchedule(): Promise<void> {
    const item = this.schedulePublication(), schedule = this.publicationSchedule();
    if (!item || !schedule) return;
    const panelGeneration = this.schedulePanelGeneration;
    const sessionGeneration = this.privateControlsGeneration;
    const days = schedule.days.map(day => ({ weekday: day.weekday, periods: (this.schedulePeriods.controls[String(day.weekday)]?.value ?? '').split(',').map(part => part.trim()).filter(Boolean).map(part => { const [open, close] = part.split('-').map(value => value.trim()); return { open, close }; }) }));
    const result = await this.vm.savePublicationSchedule(item, { owner_id: item.owner_id, enabled: this.scheduleEnabled.value, days, expected_updated_at: item.updated_at, reason: 'revision_administrativa' });
    if (sessionGeneration !== this.privateControlsGeneration || panelGeneration !== this.schedulePanelGeneration || this.schedulePublication()?.publication_id !== item.publication_id || result.status !== 'success') return;
    await this.reloadPublicationSchedule(item, panelGeneration, sessionGeneration);
  }

  private async reloadPublicationSchedule(item: Publication, panelGeneration: number, sessionGeneration: number): Promise<void> {
    await this.vm.loadPublications(this.appliedPublicationCriteria());
    if (panelGeneration !== this.schedulePanelGeneration || sessionGeneration !== this.privateControlsGeneration || this.schedulePublication()?.publication_id !== item.publication_id) return;
    const updated = this.publications()?.items.find(value => value.publication_id === item.publication_id);
    if (updated) await this.openPublicationSchedule(updated);
  }

  weekdayName(day: number): string {
    return ['', 'Lunes', 'Martes', 'Miércoles', 'Jueves', 'Viernes', 'Sábado', 'Domingo'][day] ?? `Día ${day}`;
  }

  private async reloadPublicationGallery(item: Publication, panelGeneration: number, sessionGeneration: number): Promise<void> {
    await this.vm.loadPublications(this.appliedPublicationCriteria());
    if (panelGeneration !== this.galleryPanelGeneration || sessionGeneration !== this.privateControlsGeneration || this.galleryPublication()?.publication_id !== item.publication_id) return;
    const updated = this.publications()?.items.find(value => value.publication_id === item.publication_id);
    if (updated) this.openPublicationGallery(updated);
  }

  async changePhotoVisibility(item: Publication, photo: import('./domain/global-admin-models').PublicationPhoto, enabled: boolean): Promise<void> {
    const sessionGeneration = this.privateControlsGeneration, panelGeneration = this.galleryPanelGeneration;
    if (this.galleryPublication()?.publication_id !== item.publication_id) return;
    const result = await this.vm.setPublicationPhotoVisibility(item, photo, enabled);
    if (sessionGeneration !== this.privateControlsGeneration || panelGeneration !== this.galleryPanelGeneration || this.galleryPublication()?.publication_id !== item.publication_id || result.status !== 'success') return;
    await this.reloadPublicationGallery(item, panelGeneration, sessionGeneration);
  }

  async selectPhotoCover(item: Publication, photo: import('./domain/global-admin-models').PublicationPhoto): Promise<void> {
    const sessionGeneration = this.privateControlsGeneration, panelGeneration = this.galleryPanelGeneration;
    if (this.galleryPublication()?.publication_id !== item.publication_id) return;
    const result = await this.vm.selectPublicationCover(item, photo);
    if (sessionGeneration !== this.privateControlsGeneration || panelGeneration !== this.galleryPanelGeneration || this.galleryPublication()?.publication_id !== item.publication_id || result.status !== 'success') return;
    await this.reloadPublicationGallery(item, panelGeneration, sessionGeneration);
  }

  async removePhoto(item: Publication, photo: import('./domain/global-admin-models').PublicationPhoto): Promise<void> {
    const sessionGeneration = this.privateControlsGeneration, panelGeneration = this.galleryPanelGeneration;
    if (this.galleryPublication()?.publication_id !== item.publication_id) return;
    const result = await this.vm.removePublicationPhoto(item, photo);
    if (sessionGeneration !== this.privateControlsGeneration || panelGeneration !== this.galleryPanelGeneration || this.galleryPublication()?.publication_id !== item.publication_id || result.status !== 'success') return;
    this.notice.set('Foto retirada. La limpieza de archivo quedó encolada.');
    await this.reloadPublicationGallery(item, panelGeneration, sessionGeneration);
  }

  async confirmRetirement(): Promise<void> {
    const item = this.retiringPublication();
    if (!item) return;
    const generation = this.privateControlsGeneration;
    const result = await this.vm.retirePublication(item.publication_id, {
      owner_id: item.owner_id,
      expected_updated_at: item.updated_at,
      reason: this.retirementReason.value,
    });
    if (generation !== this.privateControlsGeneration || result.status !== 'success') return;
    if (this.retiringPublication()?.publication_id !== item.publication_id) return;
    this.retiringPublication.set(null);
    const pending = result.value['files_pending_cleanup'];
    this.notice.set(`Publicación retirada. Archivos pendientes de limpieza: ${String(pending ?? 0)}.`);
    await this.vm.loadPublications(this.appliedPublicationCriteria());
  }
  openOwner(id:string):void{this.showTab('accounts');void this.open(id);}

  next(): void {
    const cursor = this.page()?.next_cursor;
    if (cursor) void this.vm.load(this.appliedCriteria(), cursor);
  }

  private clearPrivateControls(): void {
    this.privateControlsGeneration++;
    this.filters.reset();
    this.appliedCriteria.set({});
    this.profileForm.reset({displayName:'',phone:'',reason:'correccion_perfil'});
    this.publicationFilters.reset();this.appliedPublicationCriteria.set({});
    this.appliedAudit.set(null);this.auditFrom.reset();this.auditUntil.reset();this.auditActor.reset();this.auditTarget.reset();this.auditResource.reset();this.auditResourceId.reset();this.auditAction.reset();this.auditReason.reset();this.auditCorrelation.reset();this.auditResult.reset();this.reportFrom.reset();this.reportUntil.reset();this.reportZone.setValue('America/Santiago');
    this.clearPublicationEditor();
    this.closePublicationGallery();
    this.closePublicationSchedule();
    this.retiringPublication.set(null);
    this.retirementReason.setValue('revision_administrativa');
    this.confirmSessionRevocation.set(false);
    this.notice.set('');
  }

  private clearPublicationEditor(): void {
    this.publicationEditorGeneration++;
    this.editingPublication.set(null);
    this.publicationEditForm.reset({ title: '', description: '', capacity: '', usageRules: '', price: '' });
    for(const key of Object.keys(this.attributeForm.controls))this.attributeForm.removeControl(key);
  }
}
