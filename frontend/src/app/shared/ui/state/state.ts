import { Component, computed, input } from '@angular/core';

@Component({
  selector: 'app-state',
  host: {
    '[attr.role]': 'role()',
    '[attr.aria-live]': "kind() === 'error' ? 'assertive' : 'polite'",
    '[attr.data-ui-state]': 'kind()',
  },
  template: '<ng-content />',
  styles: [
    `
      :host {
        display: block;
        padding: var(--space-4);
        border-radius: var(--radius-md);
        color: var(--color-text);
        background: var(--color-surface-subtle);
      }
      :host([data-ui-state='error']) {
        color: var(--color-error);
        background: var(--color-error-soft);
      }
      :host([data-ui-state='success']) {
        color: var(--color-success);
        background: var(--color-success-soft);
      }
    `,
  ],
})
export class UiState {
  readonly kind = input.required<'loading' | 'empty' | 'error' | 'success'>();
  readonly role = computed(() => (this.kind() === 'error' ? 'alert' : 'status'));
}
