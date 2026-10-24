"use client";

import "client-only";

export interface PrivateBlobUrl {
  readonly url: string;
  revoke(): void;
}

export class PrivateBlobUrlRegistry {
  #urls = new Map<string, string>();

  create(resourceKey: string, blob: Blob): PrivateBlobUrl {
    this.revoke(resourceKey);
    const url = URL.createObjectURL(blob);
    this.#urls.set(resourceKey, url);
    return {
      url,
      revoke: () => this.revoke(resourceKey, url),
    };
  }

  revoke(resourceKey: string, expectedUrl?: string): void {
    const current = this.#urls.get(resourceKey);
    if (!current || (expectedUrl && current !== expectedUrl)) return;
    URL.revokeObjectURL(current);
    this.#urls.delete(resourceKey);
  }

  clear(): void {
    for (const url of this.#urls.values()) URL.revokeObjectURL(url);
    this.#urls.clear();
  }
}

export const privateBlobUrls = new PrivateBlobUrlRegistry();
