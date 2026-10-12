import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ModelControls } from './model-controls';

describe('typed projection controls', () => {
  afterEach(() => TestBed.resetTestingModule());
  it('keeps model updates and programmatic resets synchronized without another write', () => {
    TestBed.configureTestingModule({ providers: [ModelControls] });
    const controls = TestBed.inject(ModelControls);
    const model = { title: 'original', capacity: 1 };
    const control = controls.forProperty(model, 'title');
    control.setValue('confirmed');
    expect(model.title).toBe('confirmed');
    model.title = '';
    expect(controls.forProperty(model, 'title')).toBe(control);
    expect(control.value).toBe('');
    expect(model.title).toBe('');
    TestBed.resetTestingModule();
    control.setValue('late');
    expect(model.title).toBe('');
  });
  it('delegates transitions to the feature handler before changing criteria', () => {
    TestBed.configureTestingModule({ providers: [ModelControls] });
    const model = { rate: 'hora' };
    const onChange = vi.fn((value: string) => {
      expect(model.rate).toBe('hora');
      model.rate = value;
    });
    const control = TestBed.inject(ModelControls).forProperty(model, 'rate', onChange);
    control.setValue('dia');
    expect(onChange).toHaveBeenCalledWith('dia');
    expect(model.rate).toBe('dia');
  });
  it('synchronizes signal-backed controls through clearing private state', () => {
    TestBed.configureTestingModule({ providers: [ModelControls] });
    const value = signal('private reason');
    const control = TestBed.inject(ModelControls).forSignal(value);
    control.setValue('edited reason');
    expect(value()).toBe('edited reason');
    value.set('');
    TestBed.tick();
    expect(control.value).toBe('');
  });
});
