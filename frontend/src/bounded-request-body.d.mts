import type { IncomingMessage } from 'node:http';

export function readBoundedRequestBody(request: IncomingMessage, maxBytes: number): Promise<Buffer | null>;
