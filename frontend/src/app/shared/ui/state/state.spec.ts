import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import axe from 'axe-core';
import { afterEach, describe, expect, it } from 'vitest';
import { UiState } from './state';

type StateKind = 'loading' | 'pending' | 'empty' | 'error' | 'success' | 'restricted';

@Component({
  imports: [UiState],
  template: '<app-state [kind]="kind()"><strong>Estado disponible</strong></app-state>',
})
class StateTestHost {
  readonly kind = signal<StateKind>('loading');
}

describe('UiState', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('announces every state with the appropriate live semantics and a non-verbal icon', async () => {
    TestBed.configureTestingModule({ imports: [StateTestHost] });
    const fixture = TestBed.createComponent(StateTestHost);
    const expectedSymbols: Record<StateKind, string> = {
      loading: '…', pending: '◷', empty: '○', error: '!', success: '✓', restricted: '⊘',
    };

    for (const kind of Object.keys(expectedSymbols) as StateKind[]) {
      fixture.componentInstance.kind.set(kind);
      fixture.detectChanges();
      await fixture.whenStable();
      const state = fixture.nativeElement.querySelector('app-state') as HTMLElement;
      const icon = state.querySelector('.state-icon') as HTMLElement;
      expect(state.getAttribute('data-ui-state')).toBe(kind);
      expect(state.getAttribute('role')).toBe(kind === 'error' ? 'alert' : 'status');
      expect(state.getAttribute('aria-live')).toBe(kind === 'error' ? 'assertive' : 'polite');
      expect(state.getAttribute('aria-atomic')).toBe('true');
      expect(icon.getAttribute('aria-hidden')).toBe('true');
      expect(icon.textContent?.trim()).toBe(expectedSymbols[kind]);
      expect(state.textContent).toContain('Estado disponible');

      const result = await axe.run(state, {
        runOnly: { type: 'tag', values: ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'] },
      });
      expect(result.violations.map(({ id }) => id)).toEqual([]);
    }
    fixture.destroy();
  });
});
