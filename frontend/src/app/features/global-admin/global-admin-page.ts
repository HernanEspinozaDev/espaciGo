import { DatePipe } from '@angular/common';
import { Component, DestroyRef, effect, inject, signal } from '@angular/core';
import { FormControl, FormGroup, ReactiveFormsModule } from '@angular/forms';
import { IdentitySession } from '../../core/identity/application/identity-session';
import { PrivateSessionState } from '../../core/identity/private-session-state';
import { GlobalAdminFacade, type DirectoryCriteria } from './application/global-admin-facade';

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
  readonly loading = this.vm.loading;
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

  constructor() {
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

  search(): void {
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
    void this.vm.load(criteria);
  }

  open(accountId: string): void {
    void this.vm.open(accountId);
  }

  clearFilters(): void {
    this.filters.reset();
    this.appliedCriteria.set({});
    void this.vm.load({});
  }

  next(): void {
    const cursor = this.page()?.next_cursor;
    if (cursor) void this.vm.load(this.appliedCriteria(), cursor);
  }

  private clearPrivateControls(): void {
    this.filters.reset();
    this.appliedCriteria.set({});
  }
}
