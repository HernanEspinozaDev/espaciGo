import { Directive, input } from '@angular/core';

/** Native button/link semantics, shared sizing and tokens. No business or HTTP dependency. */
@Directive({
  selector: 'button[appButton], a[appButton]',
  host: { '[attr.data-ui-variant]': 'variant()' },
})
export class UiButton {
  readonly variant = input<'primary' | 'secondary' | 'text'>('primary', { alias: 'appButton' });
}
