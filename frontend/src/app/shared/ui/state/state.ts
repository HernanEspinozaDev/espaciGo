import { Component, computed, input } from '@angular/core';

@Component({
  selector: 'app-state',
  host: {
    '[attr.role]': 'role()',
    '[attr.aria-live]': "kind() === 'error' ? 'assertive' : 'polite'",
    '[attr.aria-atomic]': 'true',
    '[attr.data-ui-state]': 'kind()',
  },
  template: '<span class="state-icon" aria-hidden="true">{{ symbol() }}</span><span class="state-content"><ng-content /></span>',
  styleUrl: './state.scss',
})
export class UiState {
  readonly kind = input.required<'loading' | 'pending' | 'empty' | 'error' | 'success' | 'restricted'>();
  readonly role = computed(() => (this.kind() === 'error' ? 'alert' : 'status'));
  readonly symbol = computed(() => ({
    loading: '…',
    pending: '◷',
    empty: '○',
    error: '!',
    success: '✓',
    restricted: '⊘',
  })[this.kind()]);
}
