import { Component, computed, input } from '@angular/core';

@Component({
  selector: 'app-state',
  host: {
    '[attr.role]': 'role()',
    '[attr.aria-live]': "kind() === 'error' ? 'assertive' : 'polite'",
    '[attr.data-ui-state]': 'kind()',
  },
  template: '<ng-content />',
  styleUrl: './state.scss',
})
export class UiState {
  readonly kind = input.required<'loading' | 'empty' | 'error' | 'success'>();
  readonly role = computed(() => (this.kind() === 'error' ? 'alert' : 'status'));
}
