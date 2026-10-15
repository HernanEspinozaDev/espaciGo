import { DatePipe } from '@angular/common';
import { Component, DestroyRef, effect, inject, signal } from '@angular/core';
import { FormControl, FormGroup, ReactiveFormsModule } from '@angular/forms';
import { IdentitySession } from '../../core/identity/application/identity-session';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { GlobalAdminFacade, type DirectoryCriteria } from './application/global-admin-facade';
import type { Publication } from './domain/global-admin-models';

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
  readonly vm = inject(GlobalAdminFacade);
  readonly overview = this.vm.overview;
  readonly page = this.vm.page;
  readonly selected = this.vm.selected;
  readonly loadingDirectory = this.vm.loadingDirectory;
  readonly loadingDetail = this.vm.loadingDetail;
  readonly publications = this.vm.publications;
  readonly loadingPublications = this.vm.loadingPublications;
  readonly activeTab = signal<'accounts'|'publications'>('accounts');
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
  readonly canEditPublications = () => this.identity.session()?.capabilities?.includes('publications.update') ?? false;
  readonly editingPublication = signal<Publication|null>(null);
  readonly publicationEditForm=new FormGroup({title:new FormControl('',{nonNullable:true}),description:new FormControl('',{nonNullable:true}),capacity:new FormControl('',{nonNullable:true}),usageRules:new FormControl('',{nonNullable:true}),price:new FormControl('',{nonNullable:true})});
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

  saveProfile(): void {
    const account=this.selected();if(!account||!account.profile_updated_at||this.profileForm.invalid)return;
    const value=this.profileForm.getRawValue();
    void this.vm.updateProfile({display_name:value.displayName,phone:value.phone.trim()||null,expected_updated_at:account.profile_updated_at,reason:value.reason});
  }

  setRole(role:string,granted:boolean):void { const reason=this.profileForm.controls.reason.value;void this.vm.updateRole(role,{granted,reason}); }
  async revokeSessions():Promise<void>{if(!this.confirmSessionRevocation())return;const count=await this.vm.revokeSessions({reason:this.profileForm.controls.reason.value});this.notice.set(`Sesiones revocadas: ${count}.`);this.confirmSessionRevocation.set(false);}
  async setBlocked(blocked:boolean):Promise<void>{const selected=this.selected();if(!selected)return;await this.vm.setBlocked({blocked,reason:blocked?'revision_administrativa':'riesgo_resuelto'});}

  clearFilters(): void {
    this.filters.reset();
    this.appliedCriteria.set({});
    void this.vm.load({});
  }

  showTab(tab:'accounts'|'publications'):void{this.activeTab.set(tab);if(tab==='publications'&&!this.publications())void this.searchPublications();}
  searchPublications():Promise<void>{const v=this.publicationFilters.getRawValue();const criteria:DirectoryCriteria={search:v.search.trim()||undefined,owner_id:v.ownerId.trim()||undefined,category:v.category||undefined,state:v.state||undefined,publicly_visible:v.visible||undefined};this.appliedPublicationCriteria.set(criteria);return this.vm.loadPublications(criteria);}
  nextPublications():void{const cursor=this.publications()?.next_cursor;if(cursor)void this.vm.loadPublications(this.appliedPublicationCriteria(),cursor);}
  async changePublicationState(item:import('./domain/global-admin-models').Publication,state:'activa'|'oculta'):Promise<void>{await this.vm.setPublicationState(item.publication_id,{state,expected_updated_at:item.updated_at,reason:'revision_administrativa'});if(!this.error())await this.vm.loadPublications(this.appliedPublicationCriteria());}
  editPublication(item:Publication):void{this.editingPublication.set(item);this.publicationEditForm.setValue({title:item.title,description:item.description,capacity:String(item.capacity),usageRules:item.usage_rules,price:item.base_price_clp});}
  async savePublication():Promise<void>{const item=this.editingPublication();const v=this.publicationEditForm.getRawValue();const capacity=Number(v.capacity);if(!item||!Number.isSafeInteger(capacity)||capacity<1)return;await this.vm.updatePublication(item.publication_id,{title:v.title,description:v.description,capacity,usage_rules:v.usageRules,base_price_clp:v.price,expected_updated_at:item.updated_at,reason:'revision_administrativa'});if(!this.error()){this.editingPublication.set(null);await this.vm.loadPublications(this.appliedPublicationCriteria());}}
  openOwner(id:string):void{this.activeTab.set('accounts');void this.open(id);}

  next(): void {
    const cursor = this.page()?.next_cursor;
    if (cursor) void this.vm.load(this.appliedCriteria(), cursor);
  }

  private clearPrivateControls(): void {
    this.filters.reset();
    this.appliedCriteria.set({});
    this.profileForm.reset({displayName:'',phone:'',reason:'correccion_perfil'});
    this.publicationFilters.reset();this.appliedPublicationCriteria.set({});
    this.confirmSessionRevocation.set(false);
    this.notice.set('');
  }
}
