import { Injectable, signal } from '@angular/core';

@Injectable({ providedIn: 'root' })
export class PrivateSessionState {
  readonly generation = signal(0);
  private readonly resetters = new Set<() => void>();

  register(reset: () => void): () => void {
    this.resetters.add(reset);
    return () => this.resetters.delete(reset);
  }

  clear(): void {
    this.generation.update((value) => value + 1);
    for (const reset of this.resetters) reset();
  }
}
