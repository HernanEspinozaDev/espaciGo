import { DestroyRef, inject, Injectable, signal } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import type { TermsResponse } from '../../core/contracts/models';
import { IdentityPort } from '../../core/identity/domain/ports/identity-port';

@Injectable()
export class ConsentDocumentFacade {
  private readonly api = inject(IdentityPort);
  private readonly route = inject(ActivatedRoute);
  private readonly destroyRef = inject(DestroyRef);
  private destroyed = false;
  readonly state = signal<'loading' | 'ready' | 'error'>('loading');
  readonly item = signal<TermsResponse['items'][number] | null>(null);
  constructor() {
    this.destroyRef.onDestroy(() => {
      this.destroyed = true;
    });
    void this.load();
  }
  async load(): Promise<void> {
    this.state.set('loading');
    try {
      const result = await this.api.terms();
      if (this.destroyed) return;
      const type = this.route.snapshot.data['documentType'];
      const requested = this.route.snapshot.queryParamMap.getAll('version');
      const item = result.items.find(
        (candidate) =>
          candidate.type === type &&
          (requested.length === 0 || (requested.length === 1 && requested[0] === candidate.id)),
      );
      if (!item?.document || item.document.id !== item.id) throw new Error('Document unavailable');
      this.item.set(item);
      this.state.set('ready');
    } catch {
      if (!this.destroyed) this.state.set('error');
    }
  }
}
