import { backendOperation, type BackendOperation } from '../../../core/contracts/operations';
import { DestroyRef, effect, inject, Injectable, signal } from '@angular/core';
import { Meta } from '@angular/platform-browser';
import { Router } from '@angular/router';
import { IdentitySession } from '../../../core/identity/application/identity-session';
import { FinanceApiError, FinancePort } from '../../finance/domain/ports/finance-port';
import { AdminPort } from '../domain/ports/admin-port';

type AdminSection =
  | 'kyc'
  | 'privacy'
  | 'reviews'
  | 'notices'
  | 'credentials'
  | 'accounts'
  | 'spaces'
  | 'reservations'
  | 'audit'
  | 'reports';
type AdminItem = Record<string, unknown>;
type AdminAction = {
  label: string;
  path: (item: AdminItem) => BackendOperation;
  body: (reason: string) => unknown;
  idempotency: (item: AdminItem) => string | undefined;
};

const sections: Array<{ id: AdminSection; label: string }> = [
  { id: 'kyc', label: 'Cola KYC' },
  { id: 'privacy', label: 'Privacidad' },
  { id: 'reviews', label: 'Reportes de reseñas' },
  { id: 'notices', label: 'Avisos locales' },
  { id: 'credentials', label: 'Avisos de credenciales' },
  { id: 'accounts', label: 'Cuentas y roles' },
  { id: 'spaces', label: 'Publicaciones' },
  { id: 'reservations', label: 'Reservas' },
  { id: 'audit', label: 'Auditoría' },
  { id: 'reports', label: 'Reportes agregados' },
];

@Injectable()
export class AdminFacade {
  private readonly api = inject(AdminPort);
  private readonly financeApi = inject(FinancePort);
  private readonly identity = inject(IdentitySession);
  private readonly router = inject(Router);
  private readonly meta = inject(Meta);
  private readonly destroyRef = inject(DestroyRef);
  private requestRevision = 0;
  private evidenceRevision = 0;
  private previousIdentity = '';
  private previousPermissionRevision = 0;
  private previousCapabilitiesRevision = 0;
  private auditRange: { from: string; until: string } | null = null;
  private verificationRejected = false;

  readonly sections = sections;
  readonly active = signal<AdminSection>('kyc');
  readonly items = signal<AdminItem[]>([]);
  readonly result = signal<unknown>(null);
  readonly cursor = signal('');
  readonly nextCursor = signal('');
  readonly filter = signal('');
  readonly accountId = signal('');
  readonly resourceId = signal('');
  readonly fromDate = signal('');
  readonly untilDate = signal('');
  readonly timezone = signal('America/Santiago');
  readonly loading = signal(false);
  readonly busy = signal(false);
  readonly error = signal('');
  readonly success = signal('');
  readonly reason = signal('');
  readonly selected = signal<AdminItem | null>(null);
  readonly selectedAction = signal<AdminAction | null>(null);
  readonly evidenceItems = signal<AdminItem[]>([]);
  readonly evidenceCaseId = signal('');
  readonly evidenceUrl = signal<string | null>(null);
  readonly evidenceError = signal('');

  constructor() {
    this.meta.addTag({ name: 'robots', content: 'noindex, nofollow' });
    this.destroyRef.onDestroy(() => {
      ++this.requestRevision;
      ++this.evidenceRevision;
      this.clearEvidencePreview();
      this.evidenceItems.set([]);
      this.evidenceCaseId.set('');
      this.meta.removeTag("name='robots'");
    });
    effect(() => {
      const deniedRevision = this.api.permissionDeniedRevision();
      const capabilitiesRevision = this.api.capabilitiesRevision();
      if (deniedRevision !== this.previousPermissionRevision) {
        this.previousPermissionRevision = deniedRevision;
        this.clearAdminControls();
        this.error.set(
          'Go rechazó el acceso. Se limpiaron los controles y se están revalidando tus permisos.',
        );
      }
      if (capabilitiesRevision !== this.previousCapabilitiesRevision) {
        this.previousCapabilitiesRevision = capabilitiesRevision;
        if (!this.identity.session()?.capabilities?.includes('legacy.administration') && !this.identity.session()?.roles.includes('administrador'))
          void this.router.navigateByUrl('/mi-cuenta');
      }
      const token = this.identity.accessToken();
      const account = this.identity.session()?.account_id ?? '';
      const key = `${account}:${token ?? ''}`;
      if (key === this.previousIdentity) return;
      this.previousIdentity = key;
      this.clearAdminControls();
      this.error.set('');
      this.success.set('');
      if (token && (this.identity.session()?.capabilities?.includes('legacy.administration') || this.identity.session()?.roles.includes('administrador'))) void this.load();
    });
  }

  visibleItems(): AdminItem[] {
    const term = this.filter().trim().toLocaleLowerCase();
    if (!term) return this.items();
    return this.items().filter((item) => JSON.stringify(item).toLocaleLowerCase().includes(term));
  }

  async selectSection(section: AdminSection): Promise<void> {
    this.auditRange = section === 'audit' ? createAuditRange() : null;
    ++this.requestRevision;
    this.active.set(section);
    this.cursor.set('');
    this.nextCursor.set('');
    this.selected.set(null);
    this.selectedAction.set(null);
    this.success.set('');
    this.busy.set(false);
    ++this.evidenceRevision;
    this.clearEvidencePreview();
    this.evidenceItems.set([]);
    this.evidenceCaseId.set('');
    await this.load();
  }

  setVerificationQueue(rejected: boolean): void {
    this.verificationRejected = rejected;
    ++this.evidenceRevision;
    this.clearEvidencePreview();
    this.evidenceItems.set([]);
    this.evidenceCaseId.set('');
    this.selected.set(null);
    this.selectedAction.set(null);
    this.cursor.set('');
    this.nextCursor.set('');
    void this.load();
  }

  async load(cursor = ''): Promise<void> {
    const token = this.identity.accessToken();
    const account = this.identity.session()?.account_id;
    if (!token || !account) return;
    if (this.active() === 'accounts' || this.active() === 'reports') {
      this.loading.set(false);
      return;
    }
    const revision = ++this.requestRevision;
    this.cursor.set(cursor);
    this.loading.set(true);
    this.error.set('');
    try {
      const query = this.queryFor(this.active(), cursor);
      const path = this.pathFor(this.active());
      const data = await this.api.read<unknown>(token, path, query);
      if (!this.isCurrent(revision, token, account)) return;
      this.result.set(data);
      const page = unwrapPage(data);
      this.items.set(page.items);
      this.nextCursor.set(page.nextCursor);
    } catch (error) {
      if (!this.isCurrent(revision, token, account)) return;
      this.error.set(error instanceof Error ? error.message : 'No se pudo cargar esta sección.');
      this.items.set([]);
      this.result.set(null);
      this.nextCursor.set('');
    } finally {
      if (this.isCurrent(revision, token, account)) this.loading.set(false);
    }
  }

  async applyFilter(): Promise<void> {
    ++this.requestRevision;
    this.cursor.set('');
    this.nextCursor.set('');
    this.selected.set(null);
    this.busy.set(false);
    ++this.evidenceRevision;
    this.clearEvidencePreview();
    this.evidenceItems.set([]);
    this.evidenceCaseId.set('');
    await this.load('');
  }

  select(item: AdminItem, action: AdminAction): void {
    if (this.active() !== 'kyc' || this.itemId(item, 'kyc') !== this.evidenceCaseId()) {
      ++this.evidenceRevision;
      this.clearEvidencePreview();
      this.evidenceItems.set([]);
      this.evidenceCaseId.set('');
      this.evidenceError.set('');
    }
    this.selected.set(item);
    this.selectedAction.set(action);
    this.reason.set('');
    this.error.set('');
    this.success.set('');
  }

  async loadEvidence(item: AdminItem): Promise<void> {
    const token = this.identity.accessToken();
    const account = this.identity.session()?.account_id;
    const caseId = this.itemId(item, 'kyc');
    if (!token || !account || !caseId) return;
    const revision = ++this.evidenceRevision;
    this.evidenceCaseId.set(caseId);
    this.evidenceItems.set([]);
    this.evidenceError.set('');
    this.clearEvidencePreview();
    try {
      const result = await this.api.read<unknown>(
        token,
        backendOperation('verificationEvidenceList', { verificationId: caseId }),
      );
      if (
        revision !== this.evidenceRevision ||
        !this.isIdentityCurrent(token, account) ||
        caseId !== this.evidenceCaseId()
      )
        return;
      this.evidenceItems.set(unwrapPage(result).items);
    } catch (error) {
      if (revision === this.evidenceRevision && this.isIdentityCurrent(token, account))
        this.evidenceError.set(
          error instanceof Error ? error.message : 'No se pudo consultar la evidencia.',
        );
    }
  }

  async openEvidence(evidence: AdminItem): Promise<void> {
    const token = this.identity.accessToken();
    const account = this.identity.session()?.account_id;
    const caseId = this.evidenceCaseId();
    const evidenceId = typeof evidence['id'] === 'string' ? evidence['id'] : '';
    if (!token || !account || !caseId || !evidenceId || evidence['file_available'] !== true) return;
    const revision = this.evidenceRevision;
    this.evidenceError.set('');
    try {
      const blob = await this.api.downloadEvidence(
        token,
        backendOperation('verificationEvidence', {
          verificationId: caseId,
          evidenceId: evidenceId,
        }),
      );
      if (
        revision !== this.evidenceRevision ||
        !this.isIdentityCurrent(token, account) ||
        caseId !== this.evidenceCaseId()
      )
        return;
      if (blob.type !== 'image/png') throw new Error('Go no devolvió el PNG sintético esperado.');
      this.clearEvidencePreview();
      this.evidenceUrl.set(URL.createObjectURL(blob));
    } catch (error) {
      if (revision === this.evidenceRevision && this.isIdentityCurrent(token, account))
        this.evidenceError.set(
          error instanceof Error ? error.message : 'No se pudo abrir el archivo sintético.',
        );
    }
  }

  async deleteEvidence(evidence: AdminItem): Promise<void> {
    const token = this.identity.accessToken();
    const account = this.identity.session()?.account_id;
    const caseId = this.evidenceCaseId();
    const evidenceId = typeof evidence['id'] === 'string' ? evidence['id'] : '';
    if (!token || !account || !caseId || !evidenceId) return;
    if (!window.confirm('Se eliminará este archivo PNG sintético de ensayo. ¿Quieres continuar?'))
      return;
    const revision = this.evidenceRevision;
    try {
      await this.api.remove(
        token,
        backendOperation('verificationEvidence', {
          verificationId: caseId,
          evidenceId: evidenceId,
        }),
      );
      if (revision !== this.evidenceRevision || !this.isIdentityCurrent(token, account)) return;
      this.clearEvidencePreview();
      await this.loadEvidence({ id: caseId });
      if (this.isIdentityCurrent(token, account))
        this.success.set('Go confirmó el retiro del archivo sintético.');
    } catch (error) {
      if (this.isIdentityCurrent(token, account))
        this.evidenceError.set(
          error instanceof Error ? error.message : 'No se pudo retirar el archivo.',
        );
    }
  }

  async submitAction(): Promise<void> {
    const token = this.identity.accessToken();
    const account = this.identity.session()?.account_id;
    const item = this.selected();
    const action = this.selectedAction();
    if (!token || !account || !item || !action) return;
    const id = this.itemId(item, this.active());
    if (!id) {
      this.error.set('El registro no incluye un identificador utilizable.');
      return;
    }
    const revision = this.requestRevision;
    this.busy.set(true);
    this.error.set('');
    this.success.set('');
    try {
      await this.api.execute(
        token,
        account,
        action.path(item),
        action.body(this.reason()),
        action.idempotency(item),
      );
      if (!this.isCurrent(revision, token, account)) return;
      this.success.set('Go confirmó la operación administrativa.');
      this.selected.set(null);
      this.selectedAction.set(null);
      this.reason.set('');
      this.busy.set(false);
      await this.load(this.cursor());
    } catch (error) {
      if (!this.isCurrent(revision, token, account)) return;
      this.error.set(error instanceof Error ? error.message : 'No se pudo confirmar la operación.');
    } finally {
      if (this.isCurrent(revision, token, account)) this.busy.set(false);
    }
  }

  async lookupAccount(): Promise<void> {
    const id = this.accountId().trim();
    if (!id) return;
    await this.loadSingle(backendOperation('lookupAccount', { accountId: id }));
  }

  async openDetail(item: AdminItem): Promise<void> {
    const id = this.itemId(item, this.active());
    if (!id) return;
    const path =
      this.active() === 'reservations'
        ? backendOperation('reservationSummary', { reservationId: id })
        : backendOperation('publishedSpaceSummary', { spaceId: id });
    await this.loadSingle(path);
  }

  async runPrivacyOperation(operation: 'purge' | 'review' | 'execute'): Promise<void> {
    const token = this.identity.accessToken();
    const account = this.identity.session()?.account_id;
    const requestId = this.resourceId().trim();
    if (!token || !account) return;
    if (operation !== 'purge' && !requestId) {
      this.error.set('Ingresa el ID de la solicitud.');
      return;
    }
    if (
      operation === 'execute' &&
      !window.confirm(
        'Go volverá a validar las obligaciones y podrá minimizar los datos sintéticos de esta cuenta. ¿Quieres continuar?',
      )
    )
      return;
    if (
      operation === 'purge' &&
      !window.confirm(
        'Go procesará por lotes la retención vencida de datos sintéticos locales. ¿Quieres continuar?',
      )
    )
      return;
    const paths =
      operation === 'purge'
        ? backendOperation('purgeRetention')
        : backendOperation('suppressionAction', { requestId: requestId, action: operation });
    const scope =
      operation === 'purge' ? 'privacy-retention-purge' : `privacy-${operation}:${requestId}`;
    await this.perform(token, account, paths, undefined, scope);
  }

  async runReport(kind: 'reservations' | 'finance' | 'disputes'): Promise<void> {
    const from = this.fromDate();
    const until = this.untilDate();
    if (!from || !until || !this.timezone()) {
      this.error.set('Completa ambas fechas y una zona IANA.');
      return;
    }
    const query = {
      from: toZonedIso(from, this.timezone()),
      until: toZonedIso(until, this.timezone()),
      timezone: this.timezone(),
    };
    await this.loadSingle(backendOperation('administrativeReport', { report: kind }), query);
  }

  async exportAudit(): Promise<void> {
    const token = this.identity.accessToken();
    const account = this.identity.session()?.account_id;
    if (!token || !account) return;
    const query = this.auditQuery(false);
    await this.performDownload(token, account, backendOperation('auditExport'), query);
  }

  async nextPage(): Promise<void> {
    const next = this.nextCursor();
    if (next) await this.load(next);
  }
  choosePrivacyRequest(item: AdminItem): void {
    const id = this.itemId(item, 'privacy');
    if (id) this.resourceId.set(id);
  }

  pretty(value: unknown): string {
    return JSON.stringify(value, null, 2) ?? '';
  }
  title(section: AdminSection): string {
    return sections.find((item) => item.id === section)?.label ?? 'Administración';
  }
  itemId(item: AdminItem, section: AdminSection): string {
    const names =
      section === 'kyc'
        ? ['id']
        : section === 'privacy'
          ? ['request_id']
          : section === 'reviews'
            ? ['id']
            : section === 'credentials'
              ? ['event_id']
              : section === 'notices'
                ? ['event_id', 'notice_id']
                : section === 'accounts'
                  ? ['account_id']
                  : section === 'spaces'
                    ? ['space_id']
                    : section === 'reservations'
                      ? ['id', 'reservation_id']
                      : ['id'];
    for (const name of names) if (typeof item[name] === 'string') return item[name] as string;
    return '';
  }
  actionsForTemplate(): AdminAction[] {
    return this.actionsFor(this.active());
  }
  accountRecord(): AdminItem {
    const value = this.result();
    if (!value || typeof value !== 'object') return {};
    const root = value as Record<string, unknown>;
    const data = root['data'];
    return data && typeof data === 'object' ? (data as AdminItem) : root;
  }
  requiresReason(): boolean {
    return !(this.active() === 'kyc' && this.selectedAction()?.label === 'Aprobar');
  }
  reasonCodes(): string[] {
    const label = this.selectedAction()?.label;
    if (this.active() === 'kyc' && label === 'Rechazar')
      return ['documento_vencido', 'antecedentes_incompletos', 'inicio_actividades_no_confirmado'];
    if (this.active() === 'kyc')
      return ['aprobacion_fixture_incorrecta', 'revision_fixture_actualizada'];
    if (this.active() === 'reviews')
      return ['sin_infraccion', 'contenido_inadecuado', 'duplicado', 'error_registro'];
    if (this.active() === 'notices' || this.active() === 'credentials')
      return ['smtp_restaurado', 'reintento_operativo'];
    if (this.active() === 'accounts' && label === 'Bloquear')
      return ['riesgo_seguridad', 'uso_indebido', 'revision_administrativa'];
    if (this.active() === 'accounts')
      return ['revision_concluida', 'error_bloqueo', 'riesgo_resuelto'];
    return [];
  }

  private async loadSingle(
    path: BackendOperation,
    query?: Record<string, string | number | undefined>,
  ): Promise<void> {
    const token = this.identity.accessToken();
    const account = this.identity.session()?.account_id;
    if (!token || !account) return;
    const revision = ++this.requestRevision;
    this.loading.set(true);
    this.error.set('');
    this.success.set('');
    try {
      const result =
        path.kind === 'administrativeReport' && path.params.report === 'finance'
          ? await this.financeApi.read<unknown>(token, path, query)
          : await this.api.read<unknown>(token, path, query);
      if (!this.isCurrent(revision, token, account)) return;
      this.result.set(result);
      this.items.set(unwrapPage(result).items);
      this.nextCursor.set('');
    } catch (error) {
      if (error instanceof FinanceApiError && error.status === 401)
        this.identity.expireIfCurrent(token);
      if (error instanceof FinanceApiError && error.status === 403) this.api.handleForbidden(token);
      if (this.isCurrent(revision, token, account)) {
        this.result.set(null);
        this.items.set([]);
        this.error.set(
          error instanceof Error ? error.message : 'Go no pudo completar la consulta.',
        );
      }
    } finally {
      if (this.isCurrent(revision, token, account)) this.loading.set(false);
    }
  }

  private async perform(
    token: string,
    account: string,
    path: BackendOperation,
    body?: unknown,
    scope?: string,
  ): Promise<void> {
    const revision = this.requestRevision;
    this.busy.set(true);
    this.error.set('');
    this.success.set('');
    try {
      const data = await this.api.execute<unknown>(token, account, path, body, scope);
      if (!this.isCurrent(revision, token, account)) return;
      this.result.set(data);
      this.success.set('Go confirmó el resultado.');
    } catch (error) {
      if (this.isCurrent(revision, token, account))
        this.error.set(error instanceof Error ? error.message : 'No se pudo completar.');
    } finally {
      if (this.isCurrent(revision, token, account)) this.busy.set(false);
    }
  }

  private async performDownload(
    token: string,
    account: string,
    path: BackendOperation,
    query: Record<string, string | number | undefined>,
  ): Promise<void> {
    const revision = this.requestRevision;
    this.busy.set(true);
    this.error.set('');
    try {
      const data = await this.api.read<unknown>(token, path, query);
      if (!this.isCurrent(revision, token, account)) return;
      const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = url;
      anchor.download = 'auditoria-local-v1.json';
      anchor.click();
      URL.revokeObjectURL(url);
      this.success.set('Exportación JSON descargada desde Go.');
    } catch (error) {
      if (this.isCurrent(revision, token, account))
        this.error.set(error instanceof Error ? error.message : 'No se pudo exportar.');
    } finally {
      if (this.isCurrent(revision, token, account)) this.busy.set(false);
    }
  }

  private pathFor(section: AdminSection): BackendOperation {
    return {
      kyc: this.verificationRejected
        ? backendOperation('rejectedVerifications')
        : backendOperation('pendingVerifications'),
      privacy: backendOperation('suppressionRequests'),
      reviews: backendOperation('reviewReports'),
      notices: backendOperation('localNotices'),
      credentials: backendOperation('credentialNotices'),
      accounts: backendOperation('accounts'),
      spaces: backendOperation('publishedSpaces'),
      reservations: backendOperation('reservations'),
      audit: backendOperation('auditEvents'),
      reports: backendOperation('reservationsReport'),
    }[section];
  }

  private queryFor(
    section: AdminSection,
    cursor: string,
  ): Record<string, string | number | undefined> | undefined {
    if (section === 'kyc' && this.filter() === 'rechazadas') return undefined;
    if (section === 'reservations')
      return {
        reservation_id: this.filter().trim() || undefined,
        page_size: 25,
        cursor: cursor || undefined,
      };
    if (section === 'audit')
      return { ...this.auditQuery(true), page_size: 25, cursor: cursor || undefined };
    return undefined;
  }

  private auditQuery(includeCursor: boolean): Record<string, string | number | undefined> {
    this.auditRange ??= createAuditRange();
    return {
      from: this.auditRange.from,
      until: this.auditRange.until,
      actor_id: undefined,
      resource_type: undefined,
      resource_id: this.filter().trim() || undefined,
      action: undefined,
      result: undefined,
      ...(includeCursor ? {} : {}),
    };
  }

  private clearAdminControls(): void {
    ++this.requestRevision;
    ++this.evidenceRevision;
    this.clearEvidencePreview();
    this.evidenceItems.set([]);
    this.evidenceCaseId.set('');
    this.evidenceError.set('');
    this.items.set([]);
    this.result.set(null);
    this.cursor.set('');
    this.nextCursor.set('');
    this.selected.set(null);
    this.selectedAction.set(null);
    this.reason.set('');
    this.accountId.set('');
    this.resourceId.set('');
    this.filter.set('');
    this.fromDate.set('');
    this.untilDate.set('');
    this.success.set('');
    this.loading.set(false);
    this.busy.set(false);
    this.auditRange = null;
    this.verificationRejected = false;
  }

  private actionsFor(section: AdminSection): AdminAction[] {
    const key = (item: AdminItem) => this.itemId(item, section);
    const corr = () => crypto.randomUUID();
    if (section === 'kyc')
      return [
        {
          label: 'Aprobar',
          path: (item) => backendOperation('reviewVerification', { verificationId: key(item) }),
          body: () => ({ decision: 'aprobada' }),
          idempotency: () => undefined,
        },
        {
          label: 'Rechazar',
          path: (item) => backendOperation('reviewVerification', { verificationId: key(item) }),
          body: (reason) => ({ decision: 'rechazada', reason_code: reason }),
          idempotency: () => undefined,
        },
        {
          label: 'Revocar elegibilidad',
          path: (item) => backendOperation('revokeVerification', { verificationId: key(item) }),
          body: (reason) => ({ reason_code: reason }),
          idempotency: (item) => `kyc-revoke:${key(item)}`,
        },
      ];
    if (section === 'privacy')
      return [
        {
          label: 'Revisar obligaciones',
          path: (item) => backendOperation('reviewSuppression', { requestId: key(item) }),
          body: () => undefined,
          idempotency: (item) => `privacy-review:${key(item)}`,
        },
        {
          label: 'Ejecutar supresión local',
          path: (item) => backendOperation('executeSuppression', { requestId: key(item) }),
          body: () => undefined,
          idempotency: (item) => `privacy-execute:${key(item)}`,
        },
      ];
    if (section === 'reviews')
      return ['dismiss', 'hide'].map((action) => ({
        label: action === 'hide' ? 'Ocultar' : 'Desestimar',
        path: (item: AdminItem) =>
          backendOperation('moderateReviewReport', { reviewReportId: key(item), action: action }),
        body: (reason: string) => ({ reason_code: reason, correlation_id: corr() }),
        idempotency: (item: AdminItem) => `review-report:${key(item)}:${action}`,
      }));
    if (section === 'notices')
      return [
        {
          label: 'Reabrir ciclo',
          path: (item) => backendOperation('reopenLocalNotice', { noticeId: key(item) }),
          body: (reason) => ({ reason_code: reason, correlation_id: corr() }),
          idempotency: (item) => `local-notice:${key(item)}`,
        },
      ];
    if (section === 'credentials')
      return [
        {
          label: 'Reabrir ciclo',
          path: (item) => backendOperation('reopenCredentialNotice', { noticeId: key(item) }),
          body: (reason) => ({ reason_code: reason }),
          idempotency: (item) => `credential-notice:${key(item)}`,
        },
      ];
    if (section === 'accounts')
      return [
        {
          label: 'Bloquear',
          path: (item) => backendOperation('blockAccount', { accountId: key(item) }),
          body: (reason) => ({ reason_code: reason }),
          idempotency: () => undefined,
        },
        {
          label: 'Desbloquear',
          path: (item) => backendOperation('unblockAccount', { accountId: key(item) }),
          body: (reason) => ({ reason_code: reason }),
          idempotency: () => undefined,
        },
      ];
    return [];
  }

  private isCurrent(revision: number, token: string, account: string): boolean {
    return (
      revision === this.requestRevision &&
      token === this.identity.accessToken() &&
      account === this.identity.session()?.account_id
    );
  }
  private isIdentityCurrent(token: string, account: string): boolean {
    return token === this.identity.accessToken() && account === this.identity.session()?.account_id;
  }
  private clearEvidencePreview(): void {
    const url = this.evidenceUrl();
    if (url) URL.revokeObjectURL(url);
    this.evidenceUrl.set(null);
  }
}

function unwrapPage(value: unknown): { items: AdminItem[]; nextCursor: string } {
  if (!value || typeof value !== 'object') return { items: [], nextCursor: '' };
  const root = value as Record<string, unknown>;
  const data =
    root['data'] && typeof root['data'] === 'object'
      ? (root['data'] as Record<string, unknown>)
      : root;
  const items = Array.isArray(data['items'])
    ? data['items'].filter(isItem)
    : isItem(data)
      ? [data]
      : [];
  const next = data['next_cursor'];
  return { items, nextCursor: typeof next === 'string' ? next : '' };
}

function isItem(value: unknown): value is AdminItem {
  return !!value && typeof value === 'object' && !Array.isArray(value);
}

function toZonedIso(date: string, timezone: string): string {
  const [year, month, day] = date.split('-').map(Number);
  const localMidnightAsUtc = Date.UTC(year, month - 1, day);
  const firstOffset = offsetAt(localMidnightAsUtc, timezone);
  const resolvedOffset = offsetAt(localMidnightAsUtc - firstOffset, timezone);
  const sign = resolvedOffset < 0 ? '-' : '+';
  const absoluteMinutes = Math.abs(resolvedOffset) / 60000;
  const offset = `${sign}${String(Math.floor(absoluteMinutes / 60)).padStart(2, '0')}:${String(absoluteMinutes % 60).padStart(2, '0')}`;
  return `${date}T00:00:00${offset}`;
}

function offsetAt(timestamp: number, timezone: string): number {
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: timezone,
    timeZoneName: 'shortOffset',
  }).formatToParts(new Date(timestamp));
  const name = parts.find((part) => part.type === 'timeZoneName')?.value ?? 'GMT';
  const match = name.match(/GMT([+-])(\d{1,2})(?::(\d{2}))?/);
  if (!match) return 0;
  const minutes = Number(match[2]) * 60 + Number(match[3] ?? 0);
  return (match[1] === '-' ? -minutes : minutes) * 60000;
}

function createAuditRange(now = new Date()): { from: string; until: string } {
  const until = now.toISOString();
  return { from: new Date(now.getTime() - 7 * 86400000).toISOString(), until };
}
