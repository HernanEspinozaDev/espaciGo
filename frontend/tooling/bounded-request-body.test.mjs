import assert from 'node:assert/strict';
import test from 'node:test';
import { Readable } from 'node:stream';
import { readBoundedRequestBody } from '../src/bounded-request-body.mjs';

test('bounded request reader accepts exact-limit bodies and rejects larger streams', async () => {
  const accepted = await readBoundedRequestBody(Readable.from([Buffer.from('1234'), Buffer.from('5678')]), 8);
  assert.equal(accepted?.toString('utf8'), '12345678');

  const rejected = await readBoundedRequestBody(Readable.from([Buffer.from('1234'), Buffer.from('56789')]), 8);
  assert.equal(rejected, null);
});
