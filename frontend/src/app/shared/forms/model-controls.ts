import { DestroyRef, Injectable, Injector, WritableSignal, effect, inject } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { FormControl } from '@angular/forms';

/** Typed reactive controls for editable projection objects and existing view signals.
 * Owns form synchronization only; authorization and operations stay in feature facades. */
@Injectable()
export class ModelControls {
  private readonly destroyRef = inject(DestroyRef);
  private readonly injector = inject(Injector);
  private readonly objects = new WeakMap<object, Map<PropertyKey, FormControl<unknown>>>();
  private readonly signals = new WeakMap<object, FormControl<unknown>>();

  forProperty<T extends object, K extends keyof T>(
    model: T,
    key: K,
    onChange?: (value: T[K]) => void,
  ): FormControl<T[K]> {
    let controls = this.objects.get(model);
    if (!controls) {
      controls = new Map();
      this.objects.set(model, controls);
    }
    let control = controls.get(key) as FormControl<T[K]> | undefined;
    if (!control) {
      control = new FormControl(model[key], { nonNullable: true });
      control.valueChanges.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((value) => {
        if (onChange) onChange(value);
        else model[key] = value;
      });
      controls.set(key, control as FormControl<unknown>);
    }
    if (control.value !== model[key]) control.setValue(model[key], { emitEvent: false });
    return control;
  }

  forSignal<T>(source: WritableSignal<T>, onChange?: (value: T) => void): FormControl<T> {
    let control = this.signals.get(source) as FormControl<T> | undefined;
    if (!control) {
      control = new FormControl(source(), { nonNullable: true });
      const current = control;
      effect(
        () => {
          if (current.value !== source()) current.setValue(source(), { emitEvent: false });
        },
        { injector: this.injector },
      );
      control.valueChanges.pipe(takeUntilDestroyed(this.destroyRef)).subscribe((value) => {
        if (onChange) onChange(value);
        else source.set(value);
      });
      this.signals.set(source, control as FormControl<unknown>);
    }
    return control;
  }
}
