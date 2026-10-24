"use client";

import "client-only";
import { serializeExactJson } from "./exact-json";
import { createMutationIntentCore } from "./mutation-intent-core.mjs";

export class MutationIntent<T extends object> {
  readonly idempotencyKey: string;
  readonly body: Readonly<T>;
  readonly serializedBody: string;

  constructor(body: T) {
    const intent = createMutationIntentCore(body, serializeExactJson);
    this.idempotencyKey = intent.idempotencyKey;
    this.body = intent.body;
    this.serializedBody = intent.serializedBody;
  }
}

export function createMutationIntent<T extends object>(
  body: T,
): MutationIntent<T> {
  return new MutationIntent(body);
}
